package crawler

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/henry/novel-reader/internal/db"
	"github.com/henry/novel-reader/pkg/logger"
)

const yqxscBaseURL = "https://m.1qxs.com"

// YqxscSource 1qxs.com 来源 — 通过 Python cloudscraper 代理访问
type YqxscSource struct {
	spiderPath string
}

// NewYqxscSource 创建 1qxs.com 来源
func NewYqxscSource() *YqxscSource {
	return &YqxscSource{
		spiderPath: findSpiderPath(),
	}
}

func (y *YqxscSource) Name() string {
	return "源A"
}

// spiderRequest 向 Python 爬虫发送请求（复用 ixdzs8 的辅助函数）
func (y *YqxscSource) spiderRequest(ctx context.Context, req map[string]interface{}) (map[string]interface{}, error) {
	logger.Debugf("[Crawler/yqxsc] spiderRequest: cmd=%v _reqId=%v", req["cmd"], req["_reqId"])

	src := NewIxdzs8Source()
	resp, err := src.spiderRequest(ctx, req)
	if err != nil {
		logger.Errorf("[Crawler/yqxsc] spiderRequest error: %v", err)
		return nil, err
	}
	logger.Debugf("[Crawler/yqxsc] spiderRequest: success")
	return resp, nil
}

// Search 搜索小说
func (y *YqxscSource) Search(ctx context.Context, keyword string) ([]db.SearchResult, error) {
	logger.Debugf("[Crawler/yqxsc] Search: keyword=%s", keyword)
	searchURL := fmt.Sprintf("%s/search?kw=%s", yqxscBaseURL, keyword)
	resp, err := y.spiderRequest(ctx, map[string]interface{}{
		"cmd":    "fetch",
		"url":    searchURL,
		"_reqId": 1,
	})
	if err != nil {
		logger.Errorf("[Crawler/yqxsc] Search: spiderRequest error: %v", err)
		return nil, err
	}

	html, _ := resp["html"].(string)
	logger.Debugf("[Crawler/yqxsc] Search: got html, len=%d", len(html))
	results := y.parseSearchHTML(html)
	logger.Debugf("[Crawler/yqxsc] Search: parsed %d results", len(results))
	return results, nil
}

// parseSearchHTML 解析搜索结果 HTML
func (y *YqxscSource) parseSearchHTML(html string) []db.SearchResult {
	var results []db.SearchResult

	// 匹配搜索结果列表项
	// 格式: <a href="/xs_1/91943"><div class="name line_1">戾天子</div></a>
	// 使用 (?s) 让 . 匹配换行符
	re := regexp.MustCompile(`(?s)<a[^>]*href="(/xs_\d+/\d+)"[^>]*>.*?<div[^>]*class="name[^"]*"[^>]*>([^<]+)</div>.*?</a>`)
	matches := re.FindAllStringSubmatch(html, -1)

	seen := make(map[string]bool)
	for _, m := range matches {
		if len(m) < 3 {
			continue
		}
		href := m[1]
		title := strings.TrimSpace(m[2])
		if title == "" || seen[href] {
			continue
		}
		seen[href] = true

		results = append(results, db.SearchResult{
			SourceName: y.Name(),
			SourceURL:  yqxscBaseURL + href,
			BookTitle:  title,
			Author:     "",
			Available:  true,
		})
	}

	return results
}

// FetchBookInfo 获取书籍基本信息
func (y *YqxscSource) FetchBookInfo(ctx context.Context, bookURL string) (BookInfo, error) {
	logger.Debugf("[Crawler/yqxsc] FetchBookInfo: url=%s", bookURL)
	resp, err := y.spiderRequest(ctx, map[string]interface{}{
		"cmd":    "info",
		"url":    bookURL,
		"site":   "yqxsc",
		"_reqId": 2,
	})
	if err != nil {
		logger.Errorf("[Crawler/yqxsc] FetchBookInfo: spiderRequest error: %v", err)
		return BookInfo{}, err
	}

	infoMap, _ := resp["info"].(map[string]interface{})
	title, _ := infoMap["title"].(string)
	author, _ := infoMap["author"].(string)
	intro, _ := infoMap["intro"].(string)
	totalChaptersF, _ := infoMap["totalChapters"].(float64)
	totalChapters := int(totalChaptersF)

	logger.Debugf("[Crawler/yqxsc] FetchBookInfo: title=%s author=%s totalChapters=%d", title, author, totalChapters)
	return BookInfo{
		Title:         title,
		Author:        author,
		Description:   intro,
		TotalChapters: totalChapters,
	}, nil
}

// FetchChapterList 获取章节列表
// 1qxs 的章节 URL 格式为 /xs_1/91943/N，N 从 1 到 totalChapters
func (y *YqxscSource) FetchChapterList(ctx context.Context, bookURL string) ([]ChapterInfo, error) {
	logger.Debugf("[Crawler/yqxsc] FetchChapterList: url=%s", bookURL)

	// 先获取书籍信息以得到总章节数
	bookInfo, err := y.FetchBookInfo(ctx, bookURL)
	if err != nil {
		logger.Errorf("[Crawler/yqxsc] FetchChapterList: FetchBookInfo error: %v", err)
		return nil, err
	}

	if bookInfo.TotalChapters == 0 {
		return nil, fmt.Errorf("无法获取章节数量")
	}

	var chapters []ChapterInfo
	for i := 1; i <= bookInfo.TotalChapters; i++ {
		chapters = append(chapters, ChapterInfo{
			Num:   i,
			Title: fmt.Sprintf("第%d章", i),
			URL:   fmt.Sprintf("%s/%d", strings.TrimSuffix(bookURL, "/"), i),
		})
	}

	logger.Debugf("[Crawler/yqxsc] FetchChapterList: total chapters=%d", len(chapters))
	return chapters, nil
}

// FetchChapterContent 获取章节内容
// 1qxs 的章节可能有分页，如 /xs_1/91943/1/1, /xs_1/91943/1/2, ...
// 需要获取 /xs_1/91943/1 的 keyword meta 作为标题
// 内容取 class=content 的 div 内所有 p 包裹的内容，不取第一个 p 和最后两个 p
func (y *YqxscSource) FetchChapterContent(ctx context.Context, chapterURL string) (string, error) {
	logger.Debugf("[Crawler/yqxsc] FetchChapterContent: url=%s", chapterURL)
	resp, err := y.spiderRequest(ctx, map[string]interface{}{
		"cmd":     "chapter",
		"url":     chapterURL,
		"site":    "yqxsc",
		"referer": strings.TrimSuffix(chapterURL, "/"),
		"_reqId":  4,
	})
	if err != nil {
		logger.Errorf("[Crawler/yqxsc] FetchChapterContent: spiderRequest error: %v", err)
		return "", err
	}

	chapterMap, _ := resp["chapter"].(map[string]interface{})
	title, _ := chapterMap["title"].(string)
	content, _ := chapterMap["content"].(string)

	logger.Debugf("[Crawler/yqxsc] FetchChapterContent: title=%s contentLen=%d", title, len(content))
	if title != "" && content != "" {
		fullContent := title + "\n\n" + content
		return fullContent, nil
	}
	return content, nil
}

// yqxscChapterResult 单章爬取结果
type yqxscChapterResult struct {
	idx     int
	ch      ChapterInfo
	content string
	err     error
}

// yqxscStopFlag 用于优雅停止源A爬取（当前章节完成后再停止）
type yqxscStopFlag struct {
	mu   sync.Mutex
	stop bool
}

func (f *yqxscStopFlag) ShouldStop() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.stop
}

func (f *yqxscStopFlag) SetStop() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stop = true
}

// CrawlYqxscBook 爬取整本书并入库（智能更新：同名书追加新章节，并发下载）
func (e *Engine) CrawlYqxscBook(ctx context.Context, sourceURL string, database *db.DB, progressCh chan<- CrawlProgress) (int64, error) {
	src := NewYqxscSource()
	logger.Infof("[Crawler/yqxsc] CrawlYqxscBook: start, sourceURL=%s", sourceURL)

	// 1. 获取书籍信息
	if progressCh != nil {
		progressCh <- CrawlProgress{CurrentChapter: 0, TotalChapters: 0, ChapterTitle: "Fetching book info...", Percentage: 0, Done: false}
	}

	bookInfo, err := src.FetchBookInfo(ctx, sourceURL)
	if err != nil {
		logger.Errorf("[Crawler/yqxsc] CrawlYqxscBook: FetchBookInfo error: %v", err)
		return 0, fmt.Errorf("fetch book info failed: %w", err)
	}
	logger.Debugf("[Crawler/yqxsc] CrawlYqxscBook: bookInfo=%+v", bookInfo)

	// 2. 获取章节列表
	if progressCh != nil {
		progressCh <- CrawlProgress{CurrentChapter: 0, TotalChapters: 0, ChapterTitle: "Fetching chapter list...", Percentage: 0, Done: false}
	}

	chapters, err := src.FetchChapterList(ctx, sourceURL)
	if err != nil {
		logger.Errorf("[Crawler/yqxsc] CrawlYqxscBook: FetchChapterList error: %v", err)
		return 0, fmt.Errorf("fetch chapter list failed: %w", err)
	}

	if len(chapters) == 0 {
		return 0, fmt.Errorf("no chapters found")
	}
	logger.Debugf("[Crawler/yqxsc] CrawlYqxscBook: total chapters=%d", len(chapters))

	// 3. 检查是否已存在同名书籍
	existingBook, err := database.GetBookByTitle(bookInfo.Title)
	if err != nil {
		logger.Errorf("[Crawler/yqxsc] GetBookByTitle error: %v", err)
		return 0, fmt.Errorf("query existing book failed: %w", err)
	}

	var bookID int64
	var startChapter int
	if existingBook != nil {
		// 已存在：复用旧书 ID，从已有章节数+1 开始爬取
		bookID = existingBook.ID
		existingCount, _ := database.GetChapterCount(bookID)
		startChapter = existingCount
		logger.Infof("[Crawler/yqxsc] 已存在书籍, id=%d, 已有章节=%d, 新总数=%d", bookID, existingCount, len(chapters))
		if err := database.UpdateBookTotalChapters(bookID, len(chapters)); err != nil {
			logger.Warnf("[Crawler/yqxsc] UpdateBookTotalChapters error: %v", err)
		}
		// 更新来源信息
		if err := database.UpsertBookSource(bookID, sourceURL, src.Name(), startChapter); err != nil {
			logger.Warnf("[Crawler/yqxsc] UpsertBookSource error: %v", err)
		}
	} else {
		// 不存在：创建新书
		bookID, err = database.AddBook(bookInfo.Title, bookInfo.Author, bookInfo.Description, len(chapters), sourceURL, src.Name())
		if err != nil {
			logger.Errorf("[Crawler/yqxsc] AddBook error: %v", err)
			return 0, fmt.Errorf("create book record failed: %w", err)
		}
		logger.Infof("[Crawler/yqxsc] 新书创建成功, bookID=%d", bookID)
		if err := database.CreateChapterTable(bookID); err != nil {
			logger.Errorf("[Crawler/yqxsc] CreateChapterTable error: %v", err)
			return 0, fmt.Errorf("create chapter table failed: %w", err)
		}
		// 保存来源信息
		if err := database.UpsertBookSource(bookID, sourceURL, src.Name(), 0); err != nil {
			logger.Warnf("[Crawler/yqxsc] UpsertBookSource error: %v", err)
		}
	}

	// 4. 收集需要爬取的章节
	total := len(chapters)
	var toFetch []struct {
		idx int
		ch  ChapterInfo
	}
	for idx, ch := range chapters {
		if ch.Num <= startChapter {
			continue
		}
		toFetch = append(toFetch, struct {
			idx int
			ch  ChapterInfo
		}{idx: idx, ch: ch})
	}

	if len(toFetch) == 0 {
		if progressCh != nil {
			progressCh <- CrawlProgress{
				CurrentChapter: total,
				TotalChapters:  total,
				ChapterTitle:   "Done (all chapters exist)",
				Percentage:     100,
				Done:           true,
			}
		}
		return bookID, nil
	}

	// 5. 并发爬取章节（worker pool）
	// 1qxs 网站对并发敏感，worker 数限制为 1，并在请求间增加延迟
	// 使用优雅停止：context 取消时完成当前章节，不再开始新章节
	const maxWorkers = 1
	resultCh := make(chan yqxscChapterResult, len(toFetch))
	workCh := make(chan struct {
		idx int
		ch  ChapterInfo
	}, len(toFetch))

	for _, w := range toFetch {
		workCh <- w
	}
	close(workCh)

	stopFlag := &yqxscStopFlag{}
	// 监听 context 取消，设置停止标志（不立即退出，让当前章节完成）
	go func() {
		<-ctx.Done()
		logger.Infof("[Crawler/yqxsc] 收到停止信号，当前章节完成后停止")
		stopFlag.SetStop()
	}()

	var wg sync.WaitGroup
	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for work := range workCh {
				// 检查是否需要停止（在开始新章节前检查）
				if stopFlag.ShouldStop() {
					logger.Debugf("[Crawler/yqxsc] worker %d 跳过 chapter %d（已停止）", workerID, work.ch.Num)
					continue
				}

				content, err := src.FetchChapterContent(ctx, work.ch.URL)
				if err != nil {
					logger.Warnf("[Crawler/yqxsc] worker %d chapter %d fetch error: %v", workerID, work.ch.Num, err)
				}
				resultCh <- yqxscChapterResult{idx: work.idx, ch: work.ch, content: content, err: err}
				// 请求间隔，避免触发反爬
				time.Sleep(500 * time.Millisecond)
			}
		}(i)
	}

	// 等待所有 worker 完成，关闭 resultCh
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	// 6. 收集并发结果并按顺序入库
	skipped := total - len(toFetch)
	fetched := 0
	failed := 0

	// 预分配结果槽位，按原始章节顺序保存
	results := make([]*yqxscChapterResult, len(toFetch))
	received := 0
	for res := range resultCh {
		results[res.idx] = &res
		received++
		percentage := float64(received) / float64(len(toFetch)) * 100
		if progressCh != nil {
			progressCh <- CrawlProgress{
				CurrentChapter: received,
				TotalChapters:  len(toFetch),
				ChapterTitle:   res.ch.Title,
				Percentage:     percentage,
				Done:           false,
			}
		}
	}

	// 按原始章节顺序（idx）排序后入库
	for i := 0; i < len(toFetch); i++ {
		res := results[i]
		if res == nil {
			continue
		}

		// 爬取失败或内容为空，插入占位记录
		if res.err != nil || res.content == "" {
			placeholder := db.Chapter{
				ChapterNum: res.ch.Num,
				Title:      "不存在",
				Content:    fmt.Sprintf("第%d章 不存在", res.ch.Num),
				SourceURL:  res.ch.URL,
				WordCount:  0,
			}
			if err := database.InsertChapter(bookID, placeholder); err != nil {
				logger.Errorf("[Crawler/yqxsc] chapter %d placeholder insert error: %v", res.ch.Num, err)
			}
			failed++
			continue
		}

		wordCount := len([]rune(res.content))
		chapter := db.Chapter{
			ChapterNum: res.ch.Num,
			Title:      extractChapterTitle(res.content),
			Content:    res.content,
			SourceURL:  res.ch.URL,
			WordCount:  wordCount,
		}

		if err := database.InsertChapter(bookID, chapter); err != nil {
			logger.Errorf("[Crawler/yqxsc] chapter %d insert error: %v", res.ch.Num, err)
			failed++
			continue
		}
		fetched++
		logger.Debugf("[Crawler/yqxsc] chapter %d saved, wordCount=%d", res.ch.Num, wordCount)
	}

	if progressCh != nil {
		progressCh <- CrawlProgress{
			CurrentChapter: total,
			TotalChapters:  total,
			ChapterTitle:   fmt.Sprintf("Done (skipped %d, fetched %d, failed %d)", skipped, fetched, failed),
			Percentage:     100,
			Done:           true,
		}
	}
	// 更新最后爬取章节数
	finalCount, _ := database.GetChapterCount(bookID)
	if err := database.UpsertBookSource(bookID, sourceURL, src.Name(), finalCount); err != nil {
		logger.Warnf("[Crawler/yqxsc] 更新来源最后章节失败: %v", err)
	}

	logger.Infof("[Crawler/yqxsc] 爬取完成, bookID=%d, skipped=%d, fetched=%d, failed=%d, finalCount=%d", bookID, skipped, fetched, failed, finalCount)

	return bookID, nil
}
