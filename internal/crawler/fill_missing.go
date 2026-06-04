package crawler

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/henry/novel-reader/internal/db"
	"github.com/henry/novel-reader/pkg/logger"
)

// MissingChapter 表示一个缺章信息
type MissingChapter struct {
	ChapterNum int
	Title      string // 本地记录的标题（通常是"不存在"）
	PrevTitle  string // 上一章的真实标题，用于在其他源中定位
}

// FillMissingResult 补充缺章的结果
type FillMissingResult struct {
	FilledCount     int
	FailedCount     int
	SkippedCount    int
	SourceUsed      string
	MissingChapters []MissingChapter
}

// FillMissingProgress 补充缺章的进度
type FillMissingProgress struct {
	CurrentChapter int
	TotalChapters  int
	ChapterTitle   string
	Percentage     float64
	Done           bool
	Error          error
	Result         *FillMissingResult
}

// FindMissingChapters 查找指定书籍的缺章
// 缺章定义：title = "不存在" 的章节
func (e *Engine) FindMissingChapters(bookID int64, database *db.DB) ([]MissingChapter, error) {
	logger.Infof("[Crawler/FillMissing] 查找缺章: bookID=%d", bookID)

	// 获取所有章节标题列表
	chapters, err := database.ListChapterTitles(bookID)
	if err != nil {
		logger.Errorf("[Crawler/FillMissing] 获取章节列表失败: %v", err)
		return nil, fmt.Errorf("list chapters failed: %w", err)
	}

	if len(chapters) == 0 {
		logger.Infof("[Crawler/FillMissing] 书籍没有章节: bookID=%d", bookID)
		return nil, nil
	}

	var missing []MissingChapter
	for i, ch := range chapters {
		if ch.Title == "不存在" {
			prevTitle := ""
			if i > 0 {
				prevTitle = chapters[i-1].Title
			}
			missing = append(missing, MissingChapter{
				ChapterNum: ch.Num,
				Title:      ch.Title,
				PrevTitle:  prevTitle,
			})
		}
	}

	logger.Infof("[Crawler/FillMissing] 找到 %d 个缺章", len(missing))
	return missing, nil
}

// FindBestSourceForFill 为补充缺章找到最佳来源
// 排除原始来源，根据 score 选择最佳替代来源
func (e *Engine) FindBestSourceForFill(ctx context.Context, bookTitle, excludeSource string) (Source, string, int, error) {
	logger.Infof("[Crawler/FillMissing] 寻找最佳补充来源: book=%s, exclude=%s", bookTitle, excludeSource)

	var bestSource Source
	var bestURL string
	bestScore := -1

	for _, src := range e.sources {
		if src.Name() == excludeSource {
			logger.Debugf("[Crawler/FillMissing] 跳过原始来源: %s", src.Name())
			continue
		}

		// 使用 SourceMatchScore 评估来源匹配度
		score := e.SourceMatchScore(ctx, src.Name(), bookTitle)
		logger.Infof("[Crawler/FillMissing] 来源 %s 的匹配分数: %d", src.Name(), score)

		if score > bestScore {
			// 搜索获取具体的URL
			results, err := src.Search(ctx, bookTitle)
			if err != nil {
				logger.Warnf("[Crawler/FillMissing] 来源 %s 搜索失败: %v", src.Name(), err)
				continue
			}

			// 找最佳匹配的结果
			var bestResult *db.SearchResult
			bestItemScore := -1
			for i := range results {
				if results[i].Error != nil || results[i].BookTitle == "" {
					continue
				}
				itemScore := CalcItemMatchScore(bookTitle, results[i].BookTitle)
				if itemScore > bestItemScore {
					bestItemScore = itemScore
					bestResult = &results[i]
				}
			}

			if bestResult != nil && bestItemScore >= 70 {
				bestScore = score
				bestSource = src
				bestURL = bestResult.SourceURL
				logger.Infof("[Crawler/FillMissing] 找到候选来源 %s, score=%d, url=%s", src.Name(), score, bestURL)
			}
		}
	}

	if bestSource == nil {
		logger.Warnf("[Crawler/FillMissing] 未找到合适的补充来源")
		return nil, "", 0, fmt.Errorf("no suitable source found for filling missing chapters")
	}

	logger.Infof("[Crawler/FillMissing] 最佳来源: %s, score=%d", bestSource.Name(), bestScore)
	return bestSource, bestURL, bestScore, nil
}

// FillMissingChapters 从其他源补充缺章
// 策略：
// 1. 在其他源搜索同名书籍
// 2. 获取其他源的章节列表
// 3. 对于每个缺章，用上一章标题在其他源中定位对应位置
// 4. 如果上一章标题能匹配上，则缺章的下一章就是目标章节
// 5. 爬取并替换缺章内容
func (e *Engine) FillMissingChapters(ctx context.Context, bookID int64, database *db.DB, progressCh chan<- FillMissingProgress) (*FillMissingResult, error) {
	logger.Infof("[Crawler/FillMissing] 开始补充缺章: bookID=%d", bookID)

	// 获取书籍信息
	book, err := database.GetBook(bookID)
	if err != nil {
		logger.Errorf("[Crawler/FillMissing] 获取书籍信息失败: %v", err)
		return nil, fmt.Errorf("get book failed: %w", err)
	}
	if book == nil {
		return nil, fmt.Errorf("book not found: %d", bookID)
	}

	// 查找缺章
	missingChapters, err := e.FindMissingChapters(bookID, database)
	if err != nil {
		return nil, err
	}
	if len(missingChapters) == 0 {
		logger.Infof("[Crawler/FillMissing] 没有缺章需要补充")
		if progressCh != nil {
			progressCh <- FillMissingProgress{
				Done:   true,
				Result: &FillMissingResult{FilledCount: 0, FailedCount: 0, SourceUsed: ""},
			}
		}
		return &FillMissingResult{FilledCount: 0, FailedCount: 0, SourceUsed: ""}, nil
	}

	// 发送初始进度
	if progressCh != nil {
		progressCh <- FillMissingProgress{
			CurrentChapter: 0,
			TotalChapters:  len(missingChapters),
			ChapterTitle:   "Searching alternative source...",
			Percentage:     0,
			Done:           false,
		}
	}

	// 找到最佳补充来源
	src, srcURL, _, err := e.FindBestSourceForFill(ctx, book.Title, book.SourceSite)
	if err != nil {
		logger.Errorf("[Crawler/FillMissing] 寻找补充来源失败: %v", err)
		if progressCh != nil {
			progressCh <- FillMissingProgress{
				Done:  true,
				Error: err,
			}
		}
		return nil, err
	}

	// 获取补充来源的章节列表
	if progressCh != nil {
		progressCh <- FillMissingProgress{
			CurrentChapter: 0,
			TotalChapters:  len(missingChapters),
			ChapterTitle:   fmt.Sprintf("Fetching chapter list from %s...", src.Name()),
			Percentage:     5,
			Done:           false,
		}
	}

	altChapters, err := src.FetchChapterList(ctx, srcURL)
	if err != nil {
		logger.Errorf("[Crawler/FillMissing] 获取补充来源章节列表失败: %v", err)
		if progressCh != nil {
			progressCh <- FillMissingProgress{
				Done:  true,
				Error: err,
			}
		}
		return nil, fmt.Errorf("fetch alternative chapter list failed: %w", err)
	}

	logger.Infof("[Crawler/FillMissing] 补充来源共有 %d 章", len(altChapters))

	// 建立标题到章节号的映射（用于快速查找）
	altTitleMap := make(map[string]int)
	for i, ch := range altChapters {
		// 存储章节标题到索引的映射
		altTitleMap[ch.Title] = i
		// 同时存储简化后的标题（去掉"第X章"前缀）
		simpleTitle := simplifyChapterTitle(ch.Title)
		if simpleTitle != "" && simpleTitle != ch.Title {
			altTitleMap[simpleTitle] = i
		}
	}

	// 获取本地已有的章节标题列表（用于匹配上一章）
	localChapters, _ := database.ListChapterTitles(bookID)
	localTitleMap := make(map[int]string)
	for _, ch := range localChapters {
		localTitleMap[ch.Num] = ch.Title
	}

	result := &FillMissingResult{
		SourceUsed:      src.Name(),
		MissingChapters: missingChapters,
	}

	// 逐个处理缺章
	for i, mc := range missingChapters {
		percentage := float64(i) / float64(len(missingChapters)) * 100
		if progressCh != nil {
			progressCh <- FillMissingProgress{
				CurrentChapter: i + 1,
				TotalChapters:  len(missingChapters),
				ChapterTitle:   fmt.Sprintf("第%d章 (prev: %s)", mc.ChapterNum, mc.PrevTitle),
				Percentage:     percentage,
				Done:           false,
			}
		}

		// 策略：用上一章标题在补充来源中定位
		// 如果上一章标题能在补充来源中匹配，则缺章就是匹配位置的下一章
		var targetChapter *ChapterInfo

		if mc.PrevTitle != "" && mc.PrevTitle != "不存在" {
			// 尝试匹配上一章标题
			prevSimple := simplifyChapterTitle(mc.PrevTitle)
			prevIndices := findMatchingChapters(altChapters, mc.PrevTitle, prevSimple)

			if len(prevIndices) > 0 {
				// 找到匹配，取第一个匹配的下一章
				idx := prevIndices[0]
				if idx+1 < len(altChapters) {
					targetChapter = &altChapters[idx+1]
					logger.Infof("[Crawler/FillMissing] 通过上一章匹配定位: 本地第%d章 ← 来源第%d章 '%s'", mc.ChapterNum, targetChapter.Num, targetChapter.Title)
				}
			}
		}

		// 如果上一章匹配失败，尝试用章节号直接对应
		if targetChapter == nil {
			if mc.ChapterNum <= len(altChapters) {
				targetChapter = &altChapters[mc.ChapterNum-1]
				logger.Infof("[Crawler/FillMissing] 通过章节号直接对应: 本地第%d章 ← 来源第%d章 '%s'", mc.ChapterNum, targetChapter.Num, targetChapter.Title)
			}
		}

		if targetChapter == nil {
			logger.Warnf("[Crawler/FillMissing] 无法在补充来源中找到第%d章", mc.ChapterNum)
			result.SkippedCount++
			continue
		}

		// 爬取目标章节内容
		content, err := src.FetchChapterContent(ctx, targetChapter.URL)
		if err != nil || content == "" {
			logger.Warnf("[Crawler/FillMissing] 爬取第%d章失败: %v", mc.ChapterNum, err)
			result.FailedCount++
			continue
		}

		// 验证章节标题格式
		if !isValidChapterTitle(extractChapterTitle(content)) {
			logger.Warnf("[Crawler/FillMissing] 第%d章内容标题格式异常，跳过", mc.ChapterNum)
			result.FailedCount++
			continue
		}

		// 更新数据库中的章节
		wordCount := len([]rune(content))
		chapter := db.Chapter{
			ChapterNum: mc.ChapterNum,
			Title:      extractChapterTitle(content),
			Content:    content,
			SourceURL:  targetChapter.URL,
			WordCount:  wordCount,
		}

		if err := database.UpdateChapter(bookID, chapter); err != nil {
			logger.Errorf("[Crawler/FillMissing] 更新第%d章失败: %v", mc.ChapterNum, err)
			result.FailedCount++
			continue
		}

		logger.Infof("[Crawler/FillMissing] 成功补充第%d章, title=%s, words=%d", mc.ChapterNum, chapter.Title, wordCount)
		result.FilledCount++
	}

	logger.Infof("[Crawler/FillMissing] 补充完成: filled=%d, failed=%d, skipped=%d, source=%s",
		result.FilledCount, result.FailedCount, result.SkippedCount, result.SourceUsed)

	if progressCh != nil {
		progressCh <- FillMissingProgress{
			CurrentChapter: len(missingChapters),
			TotalChapters:  len(missingChapters),
			ChapterTitle:   fmt.Sprintf("Done (filled %d, failed %d, skipped %d)", result.FilledCount, result.FailedCount, result.SkippedCount),
			Percentage:     100,
			Done:           true,
			Result:         result,
		}
	}

	return result, nil
}

// simplifyChapterTitle 简化章节标题，去掉"第X章"前缀
func simplifyChapterTitle(title string) string {
	title = strings.TrimSpace(title)
	if m := chapterTitleRe.FindStringSubmatch(title); m != nil {
		return strings.TrimSpace(m[1])
	}
	return title
}

// findMatchingChapters 在章节列表中查找匹配的章节索引
// 支持精确匹配和相似匹配
func findMatchingChapters(chapters []ChapterInfo, title, simpleTitle string) []int {
	var indices []int

	for i, ch := range chapters {
		// 精确匹配
		if ch.Title == title || ch.Title == simpleTitle {
			indices = append(indices, i)
			continue
		}

		// 简化标题匹配
		chSimple := simplifyChapterTitle(ch.Title)
		if chSimple == simpleTitle || chSimple == title {
			indices = append(indices, i)
			continue
		}

		// 相似度匹配（用于处理微小差异，如空格、标点等）
		if similarity(ch.Title, title) >= 85 || similarity(chSimple, simpleTitle) >= 85 {
			indices = append(indices, i)
		}
	}

	return indices
}

// FillMissingChaptersForAllBooks 为所有有缺章的书籍补充缺章
func (e *Engine) FillMissingChaptersForAllBooks(ctx context.Context, database *db.DB, progressCh chan<- FillMissingProgress) error {
	logger.Infof("[Crawler/FillMissing] 开始为所有书籍补充缺章")

	books, err := database.ListBooks()
	if err != nil {
		return fmt.Errorf("list books failed: %w", err)
	}

	var totalMissing int
	var bookMissingMap = make(map[int64][]MissingChapter)

	for _, book := range books {
		missing, err := e.FindMissingChapters(book.ID, database)
		if err != nil {
			logger.Warnf("[Crawler/FillMissing] 查找书籍 %s 缺章失败: %v", book.Title, err)
			continue
		}
		if len(missing) > 0 {
			bookMissingMap[book.ID] = missing
			totalMissing += len(missing)
		}
	}

	if totalMissing == 0 {
		logger.Infof("[Crawler/FillMissing] 所有书籍均无缺章")
		if progressCh != nil {
			progressCh <- FillMissingProgress{
				Done:   true,
				Result: &FillMissingResult{FilledCount: 0, FailedCount: 0, SourceUsed: ""},
			}
		}
		return nil
	}

	logger.Infof("[Crawler/FillMissing] 共有 %d 本书、%d 个缺章需要补充", len(bookMissingMap), totalMissing)

	processed := 0
	for bookID, missing := range bookMissingMap {
		// 为每本书单独补充
		bookProgressCh := make(chan FillMissingProgress, 10)
		var wg sync.WaitGroup
		wg.Add(1)

		go func() {
			defer wg.Done()
			for p := range bookProgressCh {
				if progressCh != nil {
					// 重新计算总体进度
					overallPercentage := float64(processed+p.CurrentChapter) / float64(totalMissing) * 100
					progressCh <- FillMissingProgress{
						CurrentChapter: processed + p.CurrentChapter,
						TotalChapters:  totalMissing,
						ChapterTitle:   p.ChapterTitle,
						Percentage:     overallPercentage,
						Done:           false,
					}
				}
			}
		}()

		_, err := e.FillMissingChapters(ctx, bookID, database, bookProgressCh)
		close(bookProgressCh)
		wg.Wait()

		if err != nil {
			logger.Warnf("[Crawler/FillMissing] 补充书籍 %d 缺章失败: %v", bookID, err)
		}

		processed += len(missing)
	}

	if progressCh != nil {
		progressCh <- FillMissingProgress{
			CurrentChapter: totalMissing,
			TotalChapters:  totalMissing,
			ChapterTitle:   "All done",
			Percentage:     100,
			Done:           true,
		}
	}

	return nil
}
