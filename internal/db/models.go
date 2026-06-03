package db

import "time"

// Book 表示一本书的元信息
type Book struct {
	ID                 int64
	Title              string
	Author             string
	Description        string
	TotalChapters      int
	CurrentChapter     int
	CurrentOffset      int
	DownloadedChapters int
	SourceURL          string
	SourceSite         string
	Pinned             bool
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// Chapter 表示单章内容
type Chapter struct {
	ID         int64
	ChapterNum int
	Title      string
	Content    string
	SourceURL  string // 章节原始URL，如 p35.html
	WordCount  int
	CreatedAt  time.Time
}

// SearchResult 表示搜索返回的结果
type SearchResult struct {
	SourceName string
	SourceURL  string
	BookTitle  string
	Author     string
	Available  bool
	Error      error
}

// CrawlTask 表示一次爬取任务
type CrawlTask struct {
	BookID     int64
	SourceURL  string
	SourceSite string
}

// BookSource 表示书籍来源跟踪信息
type BookSource struct {
	BookID             int64
	SourceURL          string
	SourceName         string
	LastCrawledChapter int
	UpdatedAt          time.Time
}
