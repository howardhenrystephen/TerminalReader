package crawler

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/henry/novel-reader/internal/db"
)

// TestFindMissingChaptersIntegration 集成测试：查找缺章
func TestFindMissingChaptersIntegration(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	database, err := db.InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer database.Close()

	engine := NewEngine(database)

	// 添加一本书
	bookID, err := database.AddBook("测试书", "作者", "描述", 5, "http://test.com", "测试源")
	if err != nil {
		t.Fatalf("AddBook failed: %v", err)
	}

	if err := database.CreateChapterTable(bookID); err != nil {
		t.Fatalf("CreateChapterTable failed: %v", err)
	}

	// 插入章节，其中第2章和第4章是占位符（模拟缺章）
	chapters := []db.Chapter{
		{ChapterNum: 1, Title: "第一章 起始", Content: "内容1", WordCount: 10},
		{ChapterNum: 2, Title: "不存在", Content: "第2章 不存在", WordCount: 0},
		{ChapterNum: 3, Title: "第三章 发展", Content: "内容3", WordCount: 10},
		{ChapterNum: 4, Title: "不存在", Content: "第4章 不存在", WordCount: 0},
		{ChapterNum: 5, Title: "第五章 结局", Content: "内容5", WordCount: 10},
	}

	for _, ch := range chapters {
		if err := database.InsertChapter(bookID, ch); err != nil {
			t.Fatalf("InsertChapter failed: %v", err)
		}
	}

	// 测试 FindMissingChapters
	missing, err := engine.FindMissingChapters(bookID, database)
	if err != nil {
		t.Fatalf("FindMissingChapters failed: %v", err)
	}

	if len(missing) != 2 {
		t.Fatalf("Expected 2 missing chapters, got %d", len(missing))
	}

	// 验证第2章的上一章标题
	if missing[0].ChapterNum != 2 {
		t.Errorf("Expected missing chapter 2 first, got %d", missing[0].ChapterNum)
	}
	if missing[0].PrevTitle != "第一章 起始" {
		t.Errorf("Expected prev title '第一章 起始', got %q", missing[0].PrevTitle)
	}

	// 验证第4章的上一章标题
	if missing[1].ChapterNum != 4 {
		t.Errorf("Expected missing chapter 4 second, got %d", missing[1].ChapterNum)
	}
	if missing[1].PrevTitle != "第三章 发展" {
		t.Errorf("Expected prev title '第三章 发展', got %q", missing[1].PrevTitle)
	}
}

// TestFindMissingChaptersNoMissing 测试没有缺章的情况
func TestFindMissingChaptersNoMissing(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	database, err := db.InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer database.Close()

	engine := NewEngine(database)

	bookID, err := database.AddBook("完整书", "作者", "描述", 3, "http://test.com", "测试源")
	if err != nil {
		t.Fatalf("AddBook failed: %v", err)
	}

	if err := database.CreateChapterTable(bookID); err != nil {
		t.Fatalf("CreateChapterTable failed: %v", err)
	}

	// 插入完整章节
	chapters := []db.Chapter{
		{ChapterNum: 1, Title: "第一章", Content: "内容1", WordCount: 10},
		{ChapterNum: 2, Title: "第二章", Content: "内容2", WordCount: 10},
		{ChapterNum: 3, Title: "第三章", Content: "内容3", WordCount: 10},
	}

	for _, ch := range chapters {
		if err := database.InsertChapter(bookID, ch); err != nil {
			t.Fatalf("InsertChapter failed: %v", err)
		}
	}

	missing, err := engine.FindMissingChapters(bookID, database)
	if err != nil {
		t.Fatalf("FindMissingChapters failed: %v", err)
	}

	if len(missing) != 0 {
		t.Errorf("Expected 0 missing chapters, got %d", len(missing))
	}
}

// TestFindMissingChaptersFirstChapterMissing 测试第一章就是缺章的情况
func TestFindMissingChaptersFirstChapterMissing(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	database, err := db.InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer database.Close()

	engine := NewEngine(database)

	bookID, err := database.AddBook("缺首章", "作者", "描述", 3, "http://test.com", "测试源")
	if err != nil {
		t.Fatalf("AddBook failed: %v", err)
	}

	if err := database.CreateChapterTable(bookID); err != nil {
		t.Fatalf("CreateChapterTable failed: %v", err)
	}

	chapters := []db.Chapter{
		{ChapterNum: 1, Title: "不存在", Content: "第1章 不存在", WordCount: 0},
		{ChapterNum: 2, Title: "第二章", Content: "内容2", WordCount: 10},
	}

	for _, ch := range chapters {
		if err := database.InsertChapter(bookID, ch); err != nil {
			t.Fatalf("InsertChapter failed: %v", err)
		}
	}

	missing, err := engine.FindMissingChapters(bookID, database)
	if err != nil {
		t.Fatalf("FindMissingChapters failed: %v", err)
	}

	if len(missing) != 1 {
		t.Fatalf("Expected 1 missing chapter, got %d", len(missing))
	}

	if missing[0].ChapterNum != 1 {
		t.Errorf("Expected missing chapter 1, got %d", missing[0].ChapterNum)
	}

	// 第一章没有上一章，所以 PrevTitle 应该是空
	if missing[0].PrevTitle != "" {
		t.Errorf("Expected empty prev title for first chapter, got %q", missing[0].PrevTitle)
	}
}

// TestFillMissingChaptersNoSource 测试没有可用来源的情况
func TestFillMissingChaptersNoSource(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	database, err := db.InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer database.Close()

	// 创建一个没有注册任何来源的 engine
	engine := NewEngine(database)

	bookID, err := database.AddBook("无来源书", "作者", "描述", 3, "http://test.com", "测试源")
	if err != nil {
		t.Fatalf("AddBook failed: %v", err)
	}

	if err := database.CreateChapterTable(bookID); err != nil {
		t.Fatalf("CreateChapterTable failed: %v", err)
	}

	// 插入缺章
	chapters := []db.Chapter{
		{ChapterNum: 1, Title: "第一章", Content: "内容1", WordCount: 10},
		{ChapterNum: 2, Title: "不存在", Content: "第2章 不存在", WordCount: 0},
	}

	for _, ch := range chapters {
		if err := database.InsertChapter(bookID, ch); err != nil {
			t.Fatalf("InsertChapter failed: %v", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	progressCh := make(chan FillMissingProgress, 10)
	go func() {
		for range progressCh {
		}
	}()

	result, err := engine.FillMissingChapters(ctx, bookID, database, progressCh)
	close(progressCh)

	// 应该返回错误，因为没有可用的替代来源
	if err == nil {
		t.Error("Expected error when no alternative source available")
	}
	if result != nil {
		t.Error("Expected nil result when no alternative source available")
	}
}
