package crawler

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/henry/novel-reader/internal/db"
	"github.com/henry/novel-reader/pkg/logger"
)

const ixdzs8BaseURL = "https://ixdzs8.com"

// Ixdzs8Source 爱下电子书来源 — 通过 Python cloudscraper 代理访问
type Ixdzs8Source struct {
	spiderPath string
}

// NewIxdzs8Source 创建爱下电子书来源
func NewIxdzs8Source() *Ixdzs8Source {
	return &Ixdzs8Source{
		spiderPath: findSpiderPath(),
	}
}

func (i *Ixdzs8Source) Name() string {
	return "源B"
}

// findSpiderPath 查找 spider 路径
// 优先查找打包后的二进制（spider/spider.exe），找不到再回退到 spider.py
func findSpiderPath() string {
	// 1. 优先查找打包后的二进制（不需要 Python 环境）
	binaryCandidates := []string{
		"spider",
		filepath.Join("script", "spider"),
		filepath.Join("..", "script", "spider"),
		filepath.Join("..", "..", "script", "spider"),
	}
	if runtime.GOOS == "windows" {
		for i := range binaryCandidates {
			binaryCandidates[i] += ".exe"
		}
	}
	for _, p := range binaryCandidates {
		if abs, err := filepath.Abs(p); err == nil {
			if _, err := os.Stat(abs); err == nil {
				return abs
			}
		}
	}

	// 2. 回退到 Python 脚本
	pyCandidates := []string{
		filepath.Join("script", "spider.py"),
		filepath.Join("..", "script", "spider.py"),
		filepath.Join("..", "..", "script", "spider.py"),
	}
	for _, p := range pyCandidates {
		if abs, err := filepath.Abs(p); err == nil {
			if _, err := os.Stat(abs); err == nil {
				return abs
			}
		}
	}
	abs, _ := filepath.Abs(pyCandidates[0])
	return abs
}

// pythonCmd 返回 Python 命令
func pythonCmd() string {
	if runtime.GOOS == "windows" {
		return "python"
	}
	return "python3"
}

// isBinarySpider 判断 spider 路径是否是打包后的二进制
func isBinarySpider(path string) bool {
	return !strings.HasSuffix(path, ".py")
}

// spiderRequest 向爬虫发送请求（支持打包后的二进制和 Python 脚本）
func (i *Ixdzs8Source) spiderRequest(ctx context.Context, req map[string]interface{}) (map[string]interface{}, error) {
	logger.Debugf("[Crawler/ixdzs8] spiderRequest: cmd=%v _reqId=%v", req["cmd"], req["_reqId"])

	var cmd *exec.Cmd
	if isBinarySpider(i.spiderPath) {
		cmd = exec.CommandContext(ctx, i.spiderPath)
		logger.Debugf("[Crawler/ixdzs8] spiderRequest: exec binary=%s", i.spiderPath)
	} else {
		cmd = exec.CommandContext(ctx, pythonCmd(), i.spiderPath)
		logger.Debugf("[Crawler/ixdzs8] spiderRequest: exec=%s %s", pythonCmd(), i.spiderPath)
	}

	setProcessGroup(cmd)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		logger.Errorf("[Crawler/ixdzs8] spiderRequest: stdin pipe error: %v", err)
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		logger.Errorf("[Crawler/ixdzs8] spiderRequest: stdout pipe error: %v", err)
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		logger.Errorf("[Crawler/ixdzs8] spiderRequest: stderr pipe error: %v", err)
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		logger.Errorf("[Crawler/ixdzs8] spiderRequest: start error: %v", err)
		return nil, fmt.Errorf("start spider: %w", err)
	}
	logger.Debugf("[Crawler/ixdzs8] spiderRequest: process started, pid=%d", cmd.Process.Pid)

	// 发送请求
	data, _ := json.Marshal(req)
	data = append(data, '\n')
	logger.Debugf("[Crawler/ixdzs8] spiderRequest: sending request: %s", string(data))
	if _, err := stdin.Write(data); err != nil {
		cmd.Process.Kill()
		logger.Errorf("[Crawler/ixdzs8] spiderRequest: write error: %v", err)
		return nil, fmt.Errorf("write request: %w", err)
	}
	stdin.Close()
	logger.Debugf("[Crawler/ixdzs8] spiderRequest: request sent, stdin closed")

	// 读取响应（只读第一行 JSON）
	scanner := bufio.NewScanner(stdout)
	// 增大缓冲区以支持大响应（如笔趣阁章节列表可能超过 64KB）
	scanBuf := make([]byte, 0, 64*1024)
	scanner.Buffer(scanBuf, 1024*1024)
	var respLine string
	if scanner.Scan() {
		respLine = scanner.Text()
		logger.Debugf("[Crawler/ixdzs8] spiderRequest: got response line, len=%d", len(respLine))
	} else {
		logger.Debugf("[Crawler/ixdzs8] spiderRequest: scanner.Scan() returned false")
	}

	// 读取 stderr
	stderrScanner := bufio.NewScanner(stderr)
	var stderrLines []string
	for stderrScanner.Scan() {
		line := stderrScanner.Text()
		stderrLines = append(stderrLines, line)
		logger.Debugf("[Crawler/ixdzs8] spiderRequest: stderr: %s", line)
	}

	// 等待进程结束
	waitErr := cmd.Wait()
	if waitErr != nil {
		logger.Debugf("[Crawler/ixdzs8] spiderRequest: wait error: %v", waitErr)
		if respLine == "" {
			return nil, fmt.Errorf("spider exited with error: %w, stderr: %s", waitErr, strings.Join(stderrLines, "; "))
		}
	} else {
		logger.Debugf("[Crawler/ixdzs8] spiderRequest: process exited normally")
	}

	if respLine == "" {
		logger.Errorf("[Crawler/ixdzs8] spiderRequest: empty response, stderr: %s", strings.Join(stderrLines, "; "))
		return nil, fmt.Errorf("empty response from spider, stderr: %s", strings.Join(stderrLines, "; "))
	}

	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(respLine), &resp); err != nil {
		logger.Errorf("[Crawler/ixdzs8] spiderRequest: json unmarshal error: %v, raw=%s", err, respLine[:min(len(respLine), 200)])
		return nil, fmt.Errorf("parse response: %w, raw: %s", err, respLine[:min(len(respLine), 200)])
	}

	if success, _ := resp["success"].(bool); !success {
		errMsg, _ := resp["error"].(string)
		logger.Errorf("[Crawler/ixdzs8] spiderRequest: spider reported error: %s", errMsg)
		return nil, fmt.Errorf("spider error: %s", errMsg)
	}

	logger.Debugf("[Crawler/ixdzs8] spiderRequest: success")
	return resp, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Search 搜索小说
func (i *Ixdzs8Source) Search(ctx context.Context, keyword string) ([]db.SearchResult, error) {
	logger.Debugf("[Crawler/ixdzs8] Search: keyword=%s", keyword)
	searchURL := fmt.Sprintf("%s/bsearch?q=%s", ixdzs8BaseURL, keyword)
	resp, err := i.spiderRequest(ctx, map[string]interface{}{
		"cmd":    "fetch",
		"url":    searchURL,
		"_reqId": 1,
	})
	if err != nil {
		logger.Errorf("[Crawler/ixdzs8] Search: spiderRequest error: %v", err)
		return nil, err
	}

	html, _ := resp["html"].(string)
	logger.Debugf("[Crawler/ixdzs8] Search: got html, len=%d", len(html))
	results := i.parseSearchHTML(html)
	logger.Debugf("[Crawler/ixdzs8] Search: parsed %d results", len(results))
	return results, nil
}

// parseSearchHTML 解析搜索结果 HTML
func (i *Ixdzs8Source) parseSearchHTML(html string) []db.SearchResult {
	var results []db.SearchResult
	re := regexp.MustCompile(`<a href="(/read/\d+/)"[^>]*>([^<]+)</a>`)
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

		author := ""
		authorRe := regexp.MustCompile(`<a href="` + regexp.QuoteMeta(href) + `"[^>]*>[^<]*</a>.*?作者[:：\s]*<[^>]*>([^<]+)</`)
		am := authorRe.FindStringSubmatch(html)
		if len(am) > 1 {
			author = strings.TrimSpace(am[1])
		}

		results = append(results, db.SearchResult{
			SourceName: i.Name(),
			SourceURL:  ixdzs8BaseURL + href,
			BookTitle:  title,
			Author:     author,
			Available:  true,
		})
	}

	return results
}

// FetchBookInfo 获取书籍基本信息
func (i *Ixdzs8Source) FetchBookInfo(ctx context.Context, bookURL string) (BookInfo, error) {
	logger.Debugf("[Crawler/ixdzs8] FetchBookInfo: url=%s", bookURL)
	resp, err := i.spiderRequest(ctx, map[string]interface{}{
		"cmd":    "info",
		"url":    bookURL,
		"_reqId": 2,
	})
	if err != nil {
		logger.Errorf("[Crawler/ixdzs8] FetchBookInfo: spiderRequest error: %v", err)
		return BookInfo{}, err
	}

	infoMap, _ := resp["info"].(map[string]interface{})
	title, _ := infoMap["title"].(string)
	author, _ := infoMap["author"].(string)
	intro, _ := infoMap["intro"].(string)
	maxPageF, _ := infoMap["maxPage"].(float64)
	maxPage := int(maxPageF)

	logger.Debugf("[Crawler/ixdzs8] FetchBookInfo: title=%s author=%s maxPage=%d", title, author, maxPage)
	return BookInfo{
		Title:         title,
		Author:        author,
		Description:   intro,
		TotalChapters: maxPage,
	}, nil
}

// FetchChapterList 获取章节列表
func (i *Ixdzs8Source) FetchChapterList(ctx context.Context, bookURL string) ([]ChapterInfo, error) {
	logger.Debugf("[Crawler/ixdzs8] FetchChapterList: url=%s", bookURL)
	resp, err := i.spiderRequest(ctx, map[string]interface{}{
		"cmd":    "fetch",
		"url":    bookURL,
		"_reqId": 3,
	})
	if err != nil {
		logger.Errorf("[Crawler/ixdzs8] FetchChapterList: spiderRequest error: %v", err)
		return nil, err
	}

	html, _ := resp["html"].(string)
	maxPage := extractMaxPage(html)
	logger.Debugf("[Crawler/ixdzs8] FetchChapterList: maxPage=%d", maxPage)
	if maxPage == 0 {
		return nil, fmt.Errorf("无法获取章节数量")
	}

	var chapters []ChapterInfo
	for page := 1; page <= maxPage; page++ {
		chapters = append(chapters, ChapterInfo{
			Num:   page,
			Title: fmt.Sprintf("第%d章", page),
			URL:   fmt.Sprintf("%s/p%d.html", strings.TrimSuffix(bookURL, "/"), page),
		})
	}
	return chapters, nil
}

// FetchChapterContent 获取章节内容
func (i *Ixdzs8Source) FetchChapterContent(ctx context.Context, chapterURL string) (string, error) {
	logger.Debugf("[Crawler/ixdzs8] FetchChapterContent: url=%s", chapterURL)
	resp, err := i.spiderRequest(ctx, map[string]interface{}{
		"cmd":     "chapter",
		"url":     chapterURL,
		"referer": strings.TrimSuffix(chapterURL, "/"),
		"_reqId":  4,
	})
	if err != nil {
		logger.Errorf("[Crawler/ixdzs8] FetchChapterContent: spiderRequest error: %v", err)
		return "", err
	}

	chapterMap, _ := resp["chapter"].(map[string]interface{})
	title, _ := chapterMap["title"].(string)
	content, _ := chapterMap["content"].(string)

	logger.Debugf("[Crawler/ixdzs8] FetchChapterContent: title=%s contentLen=%d", title, len(content))
	if title != "" && content != "" {
		fullContent := title + "\n\n" + content
		// 检测标题是否符合 "第X章" 格式，不符合说明是错误页面（如跳转到小说主页）
		if !chapterTitleRe.MatchString(strings.TrimSpace(title)) {
			logger.Warnf("[Crawler/ixdzs8] 章节标题格式异常，可能是错误页面: title=%s url=%s", title, chapterURL)
			return "", fmt.Errorf("invalid chapter title: %s", title)
		}
		return fullContent, nil
	}
	return content, nil
}

// chapterResult 单章爬取结果
type chapterResult struct {
	idx     int
	ch      ChapterInfo
	content string
	err     error
}

// CrawlIxdzs8Book 爬取整本书并入库（智能更新：同名书追加新章节，并发下载）
func (e *Engine) CrawlIxdzs8Book(ctx context.Context, sourceURL string, database *db.DB, progressCh chan<- CrawlProgress) (int64, error) {
	src := NewIxdzs8Source()
	logger.Infof("[Crawler/ixdzs8] CrawlIxdzs8Book: start, sourceURL=%s", sourceURL)

	// 1. 获取书籍信息
	if progressCh != nil {
		progressCh <- CrawlProgress{CurrentChapter: 0, TotalChapters: 0, ChapterTitle: "Fetching book info...", Percentage: 0, Done: false}
	}

	bookInfo, err := src.FetchBookInfo(ctx, sourceURL)
	if err != nil {
		logger.Errorf("[Crawler/ixdzs8] CrawlIxdzs8Book: FetchBookInfo error: %v", err)
		return 0, fmt.Errorf("fetch book info failed: %w", err)
	}
	logger.Debugf("[Crawler/ixdzs8] CrawlIxdzs8Book: bookInfo=%+v", bookInfo)

	// 2. 获取章节列表
	if progressCh != nil {
		progressCh <- CrawlProgress{CurrentChapter: 0, TotalChapters: 0, ChapterTitle: "Fetching chapter list...", Percentage: 0, Done: false}
	}

	chapters, err := src.FetchChapterList(ctx, sourceURL)
	if err != nil {
		logger.Errorf("[Crawler/ixdzs8] CrawlIxdzs8Book: FetchChapterList error: %v", err)
		return 0, fmt.Errorf("fetch chapter list failed: %w", err)
	}

	if len(chapters) == 0 {
		return 0, fmt.Errorf("no chapters found")
	}
	logger.Debugf("[Crawler/ixdzs8] CrawlIxdzs8Book: total chapters=%d", len(chapters))

	// 3. 检查是否已存在同名书籍
	existingBook, err := database.GetBookByTitle(bookInfo.Title)
	if err != nil {
		logger.Errorf("[Crawler/ixdzs8] GetBookByTitle error: %v", err)
		return 0, fmt.Errorf("query existing book failed: %w", err)
	}

	var bookID int64
	var startChapter int
	if existingBook != nil {
		// 已存在：复用旧书 ID，从已有章节数+1 开始爬取
		bookID = existingBook.ID
		existingCount, _ := database.GetChapterCount(bookID)
		startChapter = existingCount
		logger.Infof("[Crawler/ixdzs8] 已存在书籍, id=%d, 已有章节=%d, 新总数=%d", bookID, existingCount, len(chapters))
		if err := database.UpdateBookTotalChapters(bookID, len(chapters)); err != nil {
			logger.Warnf("[Crawler/ixdzs8] UpdateBookTotalChapters error: %v", err)
		}
		// 更新来源信息
		if err := database.UpsertBookSource(bookID, sourceURL, src.Name(), startChapter); err != nil {
			logger.Warnf("[Crawler/ixdzs8] UpsertBookSource error: %v", err)
		}
	} else {
		// 不存在：创建新书
		bookID, err = database.AddBook(bookInfo.Title, bookInfo.Author, bookInfo.Description, len(chapters), sourceURL, src.Name())
		if err != nil {
			logger.Errorf("[Crawler/ixdzs8] AddBook error: %v", err)
			return 0, fmt.Errorf("create book record failed: %w", err)
		}
		logger.Infof("[Crawler/ixdzs8] 新书创建成功, bookID=%d", bookID)
		if err := database.CreateChapterTable(bookID); err != nil {
			logger.Errorf("[Crawler/ixdzs8] CreateChapterTable error: %v", err)
			return 0, fmt.Errorf("create chapter table failed: %w", err)
		}
		// 保存来源信息
		if err := database.UpsertBookSource(bookID, sourceURL, src.Name(), 0); err != nil {
			logger.Warnf("[Crawler/ixdzs8] UpsertBookSource error: %v", err)
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
	resultCh := make(chan chapterResult, len(toFetch))
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
					resultCh <- chapterResult{idx: work.idx, ch: work.ch, err: ctx.Err()}
					return
				default:
				}

				content, err := src.FetchChapterContent(ctx, work.ch.URL)
				if err != nil {
					logger.Warnf("[Crawler/ixdzs8] worker %d chapter %d fetch error: %v", workerID, work.ch.Num, err)
				}
				resultCh <- chapterResult{idx: work.idx, ch: work.ch, content: content, err: err}
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
	results := make([]*chapterResult, len(toFetch))
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
				logger.Errorf("[Crawler/ixdzs8] chapter %d placeholder insert error: %v", res.ch.Num, err)
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
			logger.Errorf("[Crawler/ixdzs8] chapter %d insert error: %v", res.ch.Num, err)
			failed++
			continue
		}
		fetched++
		logger.Debugf("[Crawler/ixdzs8] chapter %d saved, wordCount=%d", res.ch.Num, wordCount)
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
		logger.Warnf("[Crawler/ixdzs8] 更新来源最后章节失败: %v", err)
	}

	logger.Infof("[Crawler/ixdzs8] 爬取完成, bookID=%d, skipped=%d, fetched=%d, failed=%d, finalCount=%d", bookID, skipped, fetched, failed, finalCount)

	return bookID, nil
}

// extractMaxPage 从 HTML 中提取最大页码
func extractMaxPage(html string) int {
	re := regexp.MustCompile(`/read/\d+/p(\d+)\.html`)
	matches := re.FindAllStringSubmatch(html, -1)
	maxPage := 0
	for _, m := range matches {
		if len(m) > 1 {
			page, _ := strconv.Atoi(m[1])
			if page > maxPage {
				maxPage = page
			}
		}
	}
	return maxPage
}
