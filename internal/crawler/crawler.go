package crawler

import (
	"context"
	"fmt"
	"net/http"
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
