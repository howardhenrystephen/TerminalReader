package crawler

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/henry/novel-reader/internal/db"
	"github.com/henry/novel-reader/pkg/logger"
)

// Source 定义小说来源接口
type Source interface {
	Name() string
	Search(ctx context.Context, keyword string) ([]db.SearchResult, error)
	FetchBookInfo(ctx context.Context, url string) (BookInfo, error)
	FetchChapterList(ctx context.Context, url string) ([]ChapterInfo, error)
	FetchChapterContent(ctx context.Context, url string) (string, error)
}

// BookInfo 书籍基本信息
type BookInfo struct {
	Title         string
	Author        string
	Description   string
	TotalChapters int
	CoverURL      string
}

// ChapterInfo 章节信息
type ChapterInfo struct {
	Num   int
	Title string
	URL   string
}

// Engine 爬虫引擎
type Engine struct {
	httpClient *http.Client
	sources    []Source
	db         *db.DB
}

// NewEngine 创建爬虫引擎
func NewEngine(database *db.DB) *Engine {
	logger.Infof("[Crawler] 创建爬虫引擎")
	return &Engine{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		sources:    []Source{},
		db:         database,
	}
}

// RegisterSource 注册一个来源
func (e *Engine) RegisterSource(src Source) {
	logger.Infof("[Crawler] 注册来源: %s", src.Name())
	e.sources = append(e.sources, src)
}

// GetSourceNames 返回所有已注册来源名称
func (e *Engine) GetSourceNames() []string {
	names := make([]string, len(e.sources))
	for i, s := range e.sources {
		names[i] = s.Name()
	}
	return names
}

// SourceMatchScore 计算某个来源对关键词的综合匹配度 (0-100)
// 综合考虑：搜索结果数量、完全匹配、最佳匹配相似度
func (e *Engine) SourceMatchScore(ctx context.Context, sourceName, keyword string) int {
	for _, src := range e.sources {
		if src.Name() == sourceName {
			res, err := src.Search(ctx, keyword)
			if err != nil || len(res) == 0 {
				return 0
			}
			return calcSourceScore(keyword, res)
		}
	}
	return 0
}

// SearchAll 并发搜索所有来源
func (e *Engine) SearchAll(ctx context.Context, keyword string) []db.SearchResult {
	logger.Infof("[Crawler] 开始搜索关键词: %s", keyword)
	if len(e.sources) == 0 {
		logger.Warnf("[Crawler] 没有可用的搜索来源")
		return nil
	}

	results := make([]db.SearchResult, 0)
	resCh := make(chan []db.SearchResult, len(e.sources))

	for _, src := range e.sources {
		go func(s Source) {
			res, err := s.Search(ctx, keyword)
			if err != nil {
				resCh <- []db.SearchResult{{
					SourceName: s.Name(),
					Error:      err,
				}}
				return
			}
			resCh <- res
		}(src)
	}

	for i := 0; i < len(e.sources); i++ {
		select {
		case res := <-resCh:
			results = append(results, res...)
		case <-ctx.Done():
			logger.Warnf("[Crawler] 搜索被取消")
			break
		}
	}
	logger.Infof("[Crawler] 搜索完成 '%s', 共 %d 条结果", keyword, len(results))
	return results
}

// SearchBySource 按指定来源搜索
func (e *Engine) SearchBySource(ctx context.Context, sourceName, keyword string) []db.SearchResult {
	logger.Infof("[Crawler] 开始搜索关键词: %s, 来源: %s", keyword, sourceName)
	for _, src := range e.sources {
		if src.Name() == sourceName {
			res, err := src.Search(ctx, keyword)
			if err != nil {
				logger.Errorf("[Crawler] 搜索失败: %v", err)
				return []db.SearchResult{{
					SourceName: src.Name(),
					Error:      err,
				}}
			}
			logger.Infof("[Crawler] 搜索完成 '%s' from '%s', 共 %d 条结果", keyword, sourceName, len(res))
			return res
		}
	}
	logger.Warnf("[Crawler] 未找到来源: %s", sourceName)
	return nil
}

// CrawlBook 爬取整本书
func (e *Engine) CrawlBook(ctx context.Context, sourceName, sourceURL string, progressCh chan<- CrawlProgress) (int64, error) {
	logger.Infof("[Crawler] 开始爬取书籍: source=%s, url=%s", sourceName, sourceURL)
	switch sourceName {
	case "爱下电子书":
		bookID, err := e.CrawlIxdzs8Book(ctx, sourceURL, e.db, progressCh)
		if err != nil {
			logger.Errorf("[Crawler] 爬取失败: %v", err)
		} else {
			logger.Infof("[Crawler] 爬取完成, bookID=%d", bookID)
		}
		return bookID, err
	case "笔趣阁":
		bookID, err := e.CrawlBoqugeBook(ctx, sourceURL, e.db, progressCh)
		if err != nil {
			logger.Errorf("[Crawler] 爬取失败: %v", err)
		} else {
			logger.Infof("[Crawler] 爬取完成, bookID=%d", bookID)
		}
		return bookID, err
	default:
		logger.Errorf("[Crawler] 未实现的来源: %s", sourceName)
		if progressCh != nil {
			progressCh <- CrawlProgress{CurrentChapter: 0, TotalChapters: 0, ChapterTitle: "准备中...", Percentage: 0, Done: false}
		}
		return 0, fmt.Errorf("crawl not implemented for source: %s", sourceName)
	}
}

// CrawlProgress 爬取进度
type CrawlProgress struct {
	CurrentChapter int
	TotalChapters  int
	ChapterTitle   string
	Percentage     float64
	Done           bool
	Error          error
}

// CalcItemMatchScore 计算单条搜索结果的匹配度 (0-100)
// 用于结果列表排序
func CalcItemMatchScore(keyword, title string) int {
	kw := strings.ToLower(strings.TrimSpace(keyword))
	t := strings.ToLower(strings.TrimSpace(title))
	if kw == "" || t == "" {
		return 0
	}
	if kw == t {
		return 100
	}
	if strings.Contains(t, kw) {
		// 被包含方越短分数越高
		return 80 + (100-len(kw)*100/len(t))*20/100
	}
	if strings.Contains(kw, t) {
		return 70 + (100-len(t)*100/len(kw))*15/100
	}
	return similarity(kw, t)
}

// calcSourceScore 计算来源综合匹配度 (0-100)
// 权重：完全匹配(40) + 最佳相似度(35) + 结果数量(25)
func calcSourceScore(keyword string, results []db.SearchResult) int {
	kw := strings.ToLower(strings.TrimSpace(keyword))
	if kw == "" || len(results) == 0 {
		return 0
	}

	// 1. 结果数量分 (0-25)
	// 有结果就给基础分，结果越多分越高，封顶25
	countScore := 5
	if len(results) >= 3 {
		countScore = 10
	}
	if len(results) >= 5 {
		countScore = 15
	}
	if len(results) >= 10 {
		countScore = 20
	}
	if len(results) >= 20 {
		countScore = 25
	}

	// 2. 完全匹配分 (0-40)
	exactScore := 0
	bestSimilarity := 0

	for _, r := range results {
		if r.Error != nil || r.BookTitle == "" {
			continue
		}
		title := strings.ToLower(strings.TrimSpace(r.BookTitle))

		// 完全匹配
		if title == kw {
			exactScore = 40
			break // 找到完全匹配，直接满分
		}

		// 计算当前最佳相似度
		sim := similarity(kw, title)
		if sim > bestSimilarity {
			bestSimilarity = sim
		}
	}

	// 3. 最佳相似度分 (0-35)
	simScore := bestSimilarity * 35 / 100

	total := countScore + exactScore + simScore
	if total > 100 {
		total = 100
	}
	return total
}

// similarity 计算两个字符串的相似度 (0-100)
// 基于最长公共子序列(LCS)的改进算法
func similarity(a, b string) int {
	if a == "" || b == "" {
		return 0
	}
	if a == b {
		return 100
	}

	// 包含关系
	if strings.Contains(b, a) {
		return 80 + (100-len(a)*100/len(b))*20/100
	}
	if strings.Contains(a, b) {
		return 80 + (100-len(b)*100/len(a))*20/100
	}

	// 计算LCS长度
	lcsLen := lcs([]rune(a), []rune(b))
	maxLen := len([]rune(a))
	if len([]rune(b)) > maxLen {
		maxLen = len([]rune(b))
	}
	if maxLen == 0 {
		return 0
	}

	baseScore := lcsLen * 100 / maxLen

	// 额外加分：连续匹配的子串
	consecutiveBonus := consecutiveMatchBonus(a, b)

	score := baseScore + consecutiveBonus
	if score > 100 {
		score = 100
	}
	return score
}

// lcs 计算最长公共子序列长度
func lcs(a, b []rune) int {
	m, n := len(a), len(b)
	if m == 0 || n == 0 {
		return 0
	}
	// 使用一维数组优化空间
	prev := make([]int, n+1)
	curr := make([]int, n+1)
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if a[i-1] == b[j-1] {
				curr[j] = prev[j-1] + 1
			} else if prev[j] > curr[j-1] {
				curr[j] = prev[j]
			} else {
				curr[j] = curr[j-1]
			}
		}
		prev, curr = curr, prev
	}
	return prev[n]
}

// consecutiveMatchBonus 计算连续匹配 bonus (0-20)
func consecutiveMatchBonus(a, b string) int {
	maxConsecutive := 0
	currConsecutive := 0

	// 简单滑动窗口找最长连续匹配
	ra := []rune(a)
	rb := []rune(b)

	for i := 0; i < len(ra); i++ {
		for j := 0; j < len(rb); j++ {
			currConsecutive = 0
			for k := 0; i+k < len(ra) && j+k < len(rb) && ra[i+k] == rb[j+k]; k++ {
				currConsecutive++
			}
			if currConsecutive > maxConsecutive {
				maxConsecutive = currConsecutive
			}
		}
	}

	if maxConsecutive >= 4 {
		return 20
	}
	if maxConsecutive >= 3 {
		return 15
	}
	if maxConsecutive >= 2 {
		return 8
	}
	return 0
}
