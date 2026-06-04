package crawler

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/henry/novel-reader/internal/db"
	"github.com/henry/novel-reader/pkg/logger"
)

const boqugeBaseURL = "https://m.boquge.com"

// BoqugeSource 笔趣阁来源 — 通过 Python cloudscraper 代理访问
type BoqugeSource struct {
	spiderPath string
}

// NewBoqugeSource 创建笔趣阁来源
func NewBoqugeSource() *BoqugeSource {
	return &BoqugeSource{
		spiderPath: findSpiderPath(),
	}
}

func (b *BoqugeSource) Name() string {
	return "源C"
}

// spiderRequest 向 Python 爬虫发送请求（复用 ixdzs8 的辅助函数）
func (b *BoqugeSource) spiderRequest(ctx context.Context, req map[string]interface{}) (map[string]interface{}, error) {
	logger.Debugf("[Crawler/boquge] spiderRequest: cmd=%v _reqId=%v", req["cmd"], req["_reqId"])

	src := NewIxdzs8Source()
	resp, err := src.spiderRequest(ctx, req)
	if err != nil {
		logger.Errorf("[Crawler/boquge] spiderRequest error: %v", err)
		return nil, err
	}
	logger.Debugf("[Crawler/boquge] spiderRequest: success")
	return resp, nil
}

// Search 搜索小说
func (b *BoqugeSource) Search(ctx context.Context, keyword string) ([]db.SearchResult, error) {
	logger.Debugf("[Crawler/boquge] Search: keyword=%s", keyword)
	searchURL := fmt.Sprintf("%s/search.htm?keyword=%s", boqugeBaseURL, keyword)
	resp, err := b.spiderRequest(ctx, map[string]interface{}{
		"cmd":    "fetch",
		"url":    searchURL,
		"_reqId": 1,
	})
	if err != nil {
		logger.Errorf("[Crawler/boquge] Search: spiderRequest error: %v", err)
		return nil, err
	}

	html, _ := resp["html"].(string)
	logger.Debugf("[Crawler/boquge] Search: got html, len=%d", len(html))
	results := b.parseSearchHTML(html)
	logger.Debugf("[Crawler/boquge] Search: parsed %d results", len(results))
	return results, nil
}

// parseSearchHTML 解析搜索结果 HTML
func (b *BoqugeSource) parseSearchHTML(html string) []db.SearchResult {
	var results []db.SearchResult

	// 匹配搜索结果列表项
	// 格式: <li><i class="tag-blue">奇幻</i><a href="/wapbook/76671.html">伏天氏</a>　/　净无痕　<span class="time">04-24 20:26</span><br/><a href="/wapbook/76671_180008774.html">番外（6）</a></li>
	re := regexp.MustCompile(`<a href="(/wapbook/\d+\.html)"[^>]*>([^<]+)</a>`)
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

		// 提取作者：在 <a> 标签后的文本中
		author := ""
		// 构造正则，匹配该 href 后的作者信息
		authorRe := regexp.MustCompile(`<a href="` + regexp.QuoteMeta(href) + `"[^>]*>[^<]*</a>\s*[/／]\s*([^<\s][^<\n]*?)(?:\s*<span|\s*<br|$)`)
		am := authorRe.FindStringSubmatch(html)
		if len(am) > 1 {
			author = strings.TrimSpace(am[1])
			// 去除可能的 HTML 标签残留
			author = regexp.MustCompile(`<[^>]+>`).ReplaceAllString(author, "")
			author = strings.TrimSpace(author)
		}

		results = append(results, db.SearchResult{
			SourceName: b.Name(),
			SourceURL:  boqugeBaseURL + href,
			BookTitle:  title,
			Author:     author,
			Available:  true,
		})
	}

	return results
}

// FetchBookInfo 获取书籍基本信息
func (b *BoqugeSource) FetchBookInfo(ctx context.Context, bookURL string) (BookInfo, error) {
	logger.Debugf("[Crawler/boquge] FetchBookInfo: url=%s", bookURL)
	resp, err := b.spiderRequest(ctx, map[string]interface{}{
		"cmd":    "info",
		"url":    bookURL,
		"site":   "boquge",
		"_reqId": 2,
	})
	if err != nil {
		logger.Errorf("[Crawler/boquge] FetchBookInfo: spiderRequest error: %v", err)
		return BookInfo{}, err
	}

	infoMap, _ := resp["info"].(map[string]interface{})
	title, _ := infoMap["title"].(string)
	author, _ := infoMap["author"].(string)
	intro, _ := infoMap["intro"].(string)
	coverURL, _ := infoMap["coverUrl"].(string)

	logger.Debugf("[Crawler/boquge] FetchBookInfo: title=%s author=%s", title, author)
	return BookInfo{
		Title:       title,
		Author:      author,
		Description: intro,
		CoverURL:    coverURL,
	}, nil
}

// FetchChapterList 获取章节列表（通过 spider.py 并发获取）
func (b *BoqugeSource) FetchChapterList(ctx context.Context, bookURL string) ([]ChapterInfo, error) {
	logger.Debugf("[Crawler/boquge] FetchChapterList: url=%s", bookURL)

	resp, err := b.spiderRequest(ctx, map[string]interface{}{
		"cmd":        "chapter_list",
		"url":        bookURL,
		"site":       "boquge",
		"maxWorkers": 100,
		"_reqId":     3,
	})
	if err != nil {
		logger.Errorf("[Crawler/boquge] FetchChapterList: spiderRequest error: %v", err)
		return nil, err
	}

	chaptersRaw, _ := resp["chapters"].([]interface{})
	var allChapters []ChapterInfo
	chapterNum := 1

	for _, item := range chaptersRaw {
		chMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		title, _ := chMap["title"].(string)
		url, _ := chMap["url"].(string)
		if title == "" || url == "" {
			continue
		}
		allChapters = append(allChapters, ChapterInfo{
			Num:   chapterNum,
			Title: title,
			URL:   url,
		})
		chapterNum++
	}

	logger.Debugf("[Crawler/boquge] FetchChapterList: total chapters=%d", len(allChapters))
	return allChapters, nil
}

// FetchChapterContent 获取章节内容
func (b *BoqugeSource) FetchChapterContent(ctx context.Context, chapterURL string) (string, error) {
	logger.Debugf("[Crawler/boquge] FetchChapterContent: url=%s", chapterURL)
	resp, err := b.spiderRequest(ctx, map[string]interface{}{
		"cmd":     "chapter",
		"url":     chapterURL,
		"site":    "boquge",
		"referer": strings.TrimSuffix(chapterURL, "/"),
		"_reqId":  4,
	})
	if err != nil {
		logger.Errorf("[Crawler/boquge] FetchChapterContent: spiderRequest error: %v", err)
		return "", err
	}

	chapterMap, _ := resp["chapter"].(map[string]interface{})
	title, _ := chapterMap["title"].(string)
	content, _ := chapterMap["content"].(string)

	logger.Debugf("[Crawler/boquge] FetchChapterContent: title=%s contentLen=%d", title, len(content))
	if title != "" && content != "" {
		fullContent := title + "\n\n" + content
		// 检测标题是否符合 "第X章" 格式，不符合说明是错误页面
		if !chapterTitleRe.MatchString(strings.TrimSpace(title)) {
			logger.Warnf("[Crawler/boquge] 章节标题格式异常，可能是错误页面: title=%s url=%s", title, chapterURL)
			return "", fmt.Errorf("invalid chapter title: %s", title)
		}
		return fullContent, nil
	}
	return content, nil
}

// chapterResult 单章爬取结果
type boqugeChapterResult struct {
	idx     int
	ch      ChapterInfo
	content string
	err     error
}

// CrawlBoqugeBook 爬取整本书并入库（智能更新：同名书追加新章节，并发下载）
func (e *Engine) CrawlBoqugeBook(ctx context.Context, sourceURL string, database *db.DB, progressCh chan<- CrawlProgress) (int64, error) {
	src := NewBoqugeSource()
	logger.Infof("[Crawler/boquge] CrawlBoqugeBook: start, sourceURL=%s", sourceURL)

	// 1. 获取书籍信息
	if progressCh != nil {
		progressCh <- CrawlProgress{CurrentChapter: 0, TotalChapters: 0, ChapterTitle: "Fetching book info...", Percentage: 0, Done: false}
	}

	bookInfo, err := src.FetchBookInfo(ctx, sourceURL)
	if err != nil {
		logger.Errorf("[Crawler/boquge] CrawlBoqugeBook: FetchBookInfo error: %v", err)
		return 0, fmt.Errorf("fetch book info failed: %w", err)
	}
	logger.Debugf("[Crawler/boquge] CrawlBoqugeBook: bookInfo=%+v", bookInfo)

	// 2. 获取章节列表
	if progressCh != nil {
		progressCh <- CrawlProgress{CurrentChapter: 0, TotalChapters: 0, ChapterTitle: "Fetching chapter list...", Percentage: 0, Done: false}
	}

	chapters, err := src.FetchChapterList(ctx, sourceURL)
	if err != nil {
		logger.Errorf("[Crawler/boquge] CrawlBoqugeBook: FetchChapterList error: %v", err)
		return 0, fmt.Errorf("fetch chapter list failed: %w", err)
	}

	if len(chapters) == 0 {
		return 0, fmt.Errorf("no chapters found")
	}
	logger.Debugf("[Crawler/boquge] CrawlBoqugeBook: total chapters=%d", len(chapters))

	// 3. 检查是否已存在同名书籍
	existingBook, err := database.GetBookByTitle(bookInfo.Title)
	if err != nil {
		logger.Errorf("[Crawler/boquge] GetBookByTitle error: %v", err)
		return 0, fmt.Errorf("query existing book failed: %w", err)
	}

	var bookID int64
	var startChapter int
	if existingBook != nil {
		// 已存在：复用旧书 ID，从已有章节数+1 开始爬取
		bookID = existingBook.ID
		existingCount, _ := database.GetChapterCount(bookID)
		startChapter = existingCount
		logger.Infof("[Crawler/boquge] 已存在书籍, id=%d, 已有章节=%d, 新总数=%d", bookID, existingCount, len(chapters))
		if err := database.UpdateBookTotalChapters(bookID, len(chapters)); err != nil {
			logger.Warnf("[Crawler/boquge] UpdateBookTotalChapters error: %v", err)
		}
		// 更新来源信息
		if err := database.UpsertBookSource(bookID, sourceURL, src.Name(), startChapter); err != nil {
			logger.Warnf("[Crawler/boquge] UpsertBookSource error: %v", err)
		}
	} else {
		// 不存在：创建新书
		bookID, err = database.AddBook(bookInfo.Title, bookInfo.Author, bookInfo.Description, len(chapters), sourceURL, src.Name())
		if err != nil {
			logger.Errorf("[Crawler/boquge] AddBook error: %v", err)
			return 0, fmt.Errorf("create book record failed: %w", err)
		}
		logger.Infof("[Crawler/boquge] 新书创建成功, bookID=%d", bookID)
		if err := database.CreateChapterTable(bookID); err != nil {
			logger.Errorf("[Crawler/boquge] CreateChapterTable error: %v", err)
			return 0, fmt.Errorf("create chapter table failed: %w", err)
		}
		// 保存来源信息
		if err := database.UpsertBookSource(bookID, sourceURL, src.Name(), 0); err != nil {
			logger.Warnf("[Crawler/boquge] UpsertBookSource error: %v", err)
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
	const maxWorkers = 8
	resultCh := make(chan boqugeChapterResult, len(toFetch))
	workCh := make(chan struct {
		idx int
		ch  ChapterInfo
	}, len(toFetch))

	for _, w := range toFetch {
		workCh <- w
	}
	close(workCh)

	var wg sync.WaitGroup
	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for work := range workCh {
				select {
				case <-ctx.Done():
					resultCh <- boqugeChapterResult{idx: work.idx, ch: work.ch, err: ctx.Err()}
					return
				default:
				}

				content, err := src.FetchChapterContent(ctx, work.ch.URL)
				if err != nil {
					logger.Warnf("[Crawler/boquge] worker %d chapter %d fetch error: %v", workerID, work.ch.Num, err)
				}
				resultCh <- boqugeChapterResult{idx: work.idx, ch: work.ch, content: content, err: err}
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
	results := make([]*boqugeChapterResult, len(toFetch))
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
				logger.Errorf("[Crawler/boquge] chapter %d placeholder insert error: %v", res.ch.Num, err)
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
			logger.Errorf("[Crawler/boquge] chapter %d insert error: %v", res.ch.Num, err)
			failed++
			continue
		}
		fetched++
		logger.Debugf("[Crawler/boquge] chapter %d saved, wordCount=%d", res.ch.Num, wordCount)
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
		logger.Warnf("[Crawler/boquge] 更新来源最后章节失败: %v", err)
	}

	logger.Infof("[Crawler/boquge] 爬取完成, bookID=%d, skipped=%d, fetched=%d, failed=%d, finalCount=%d", bookID, skipped, fetched, failed, finalCount)

	return bookID, nil
}
