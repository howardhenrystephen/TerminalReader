package db

import (
	"os"
	"path/filepath"
	"testing"
)

func setupTestDB(t *testing.T) *DB {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	return db
}

func TestUpdateChapter(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// 添加一本书
	bookID, err := db.AddBook("测试书", "测试作者", "测试描述", 5, "http://test.com", "测试源")
	if err != nil {
		t.Fatalf("AddBook failed: %v", err)
	}

	// 创建章节表
	if err := db.CreateChapterTable(bookID); err != nil {
		t.Fatalf("CreateChapterTable failed: %v", err)
	}

	// 插入一个占位章节（模拟缺章）
	placeholder := Chapter{
		ChapterNum: 3,
		Title:      "不存在",
		Content:    "第3章 不存在",
		SourceURL:  "",
		WordCount:  0,
	}
	if err := db.InsertChapter(bookID, placeholder); err != nil {
		t.Fatalf("InsertChapter failed: %v", err)
	}

	// 验证插入成功
	ch, err := db.GetChapter(bookID, 3)
	if err != nil {
		t.Fatalf("GetChapter failed: %v", err)
	}
	if ch == nil {
		t.Fatal("Chapter not found")
	}
	if ch.Title != "不存在" {
		t.Errorf("Expected title '不存在', got %q", ch.Title)
	}

	// 更新章节内容
	updated := Chapter{
		ChapterNum: 3,
		Title:      "第三章 真实内容",
		Content:    "第三章 真实内容\n\n这是真实的内容。",
		SourceURL:  "http://test.com/ch3",
		WordCount:  20,
	}
	if err := db.UpdateChapter(bookID, updated); err != nil {
		t.Fatalf("UpdateChapter failed: %v", err)
	}

	// 验证更新成功
	ch, err = db.GetChapter(bookID, 3)
	if err != nil {
		t.Fatalf("GetChapter after update failed: %v", err)
	}
	if ch == nil {
		t.Fatal("Chapter not found after update")
	}
	if ch.Title != "第三章 真实内容" {
		t.Errorf("Expected title '第三章 真实内容', got %q", ch.Title)
	}
	if ch.Content != "第三章 真实内容\n\n这是真实的内容。" {
		t.Errorf("Expected content updated, got %q", ch.Content)
	}
	if ch.WordCount != 20 {
		t.Errorf("Expected wordCount 20, got %d", ch.WordCount)
	}
	if ch.SourceURL != "http://test.com/ch3" {
		t.Errorf("Expected sourceURL 'http://test.com/ch3', got %q", ch.SourceURL)
	}
}

func TestGetChaptersWithPlaceholder(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	bookID, err := db.AddBook("测试书2", "作者", "描述", 5, "http://test.com", "测试源")
	if err != nil {
		t.Fatalf("AddBook failed: %v", err)
	}

	if err := db.CreateChapterTable(bookID); err != nil {
		t.Fatalf("CreateChapterTable failed: %v", err)
	}

	// 插入正常章节和占位章节
	chapters := []Chapter{
		{ChapterNum: 1, Title: "第一章", Content: "内容1", WordCount: 10},
		{ChapterNum: 2, Title: "不存在", Content: "第2章 不存在", WordCount: 0},
		{ChapterNum: 3, Title: "第三章", Content: "内容3", WordCount: 10},
		{ChapterNum: 4, Title: "不存在", Content: "第4章 不存在", WordCount: 0},
		{ChapterNum: 5, Title: "第五章", Content: "内容5", WordCount: 10},
	}

	for _, ch := range chapters {
		if err := db.InsertChapter(bookID, ch); err != nil {
			t.Fatalf("InsertChapter failed: %v", err)
		}
	}

	// 获取占位章节
	placeholders, err := db.GetChaptersWithPlaceholder(bookID)
	if err != nil {
		t.Fatalf("GetChaptersWithPlaceholder failed: %v", err)
	}

	if len(placeholders) != 2 {
		t.Errorf("Expected 2 placeholders, got %d", len(placeholders))
	}

	expectedNums := []int{2, 4}
	for i, p := range placeholders {
		if p.Num != expectedNums[i] {
			t.Errorf("Expected placeholder %d at index %d, got %d", expectedNums[i], i, p.Num)
		}
		if p.Title != "不存在" {
			t.Errorf("Expected title '不存在', got %q", p.Title)
		}
	}
}

func TestFindMissingChapterNums(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	bookID, err := db.AddBook("测试书3", "作者", "描述", 10, "http://test.com", "测试源")
	if err != nil {
		t.Fatalf("AddBook failed: %v", err)
	}

	if err := db.CreateChapterTable(bookID); err != nil {
		t.Fatalf("CreateChapterTable failed: %v", err)
	}

	// 插入不连续的章节：1, 2, 4, 5, 7（缺少 3, 6）
	chapters := []Chapter{
		{ChapterNum: 1, Title: "第一章", Content: "内容1", WordCount: 10},
		{ChapterNum: 2, Title: "第二章", Content: "内容2", WordCount: 10},
		{ChapterNum: 4, Title: "第四章", Content: "内容4", WordCount: 10},
		{ChapterNum: 5, Title: "第五章", Content: "内容5", WordCount: 10},
		{ChapterNum: 7, Title: "第七章", Content: "内容7", WordCount: 10},
	}

	for _, ch := range chapters {
		if err := db.InsertChapter(bookID, ch); err != nil {
			t.Fatalf("InsertChapter failed: %v", err)
		}
	}

	// 查找缺失的章节号
	missing, err := db.FindMissingChapterNums(bookID)
	if err != nil {
		t.Fatalf("FindMissingChapterNums failed: %v", err)
	}

	// 注意：这个查询找的是 chapter_num 不连续的地方
	// 1->2 连续，2->4 缺少3，4->5 连续，5->7 缺少6
	// 所以应该找到 3 和 6
	expected := []int{3, 6}
	if len(missing) != len(expected) {
		t.Errorf("Expected %d missing chapters, got %d: %v", len(expected), len(missing), missing)
	}

	for i, num := range missing {
		if num != expected[i] {
			t.Errorf("Expected missing chapter %d at index %d, got %d", expected[i], i, num)
		}
	}
}

func TestFindMissingChapterNumsNoMissing(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	bookID, err := db.AddBook("测试书4", "作者", "描述", 3, "http://test.com", "测试源")
	if err != nil {
		t.Fatalf("AddBook failed: %v", err)
	}

	if err := db.CreateChapterTable(bookID); err != nil {
		t.Fatalf("CreateChapterTable failed: %v", err)
	}

	// 插入连续的章节
	chapters := []Chapter{
		{ChapterNum: 1, Title: "第一章", Content: "内容1", WordCount: 10},
		{ChapterNum: 2, Title: "第二章", Content: "内容2", WordCount: 10},
		{ChapterNum: 3, Title: "第三章", Content: "内容3", WordCount: 10},
	}

	for _, ch := range chapters {
		if err := db.InsertChapter(bookID, ch); err != nil {
			t.Fatalf("InsertChapter failed: %v", err)
		}
	}

	missing, err := db.FindMissingChapterNums(bookID)
	if err != nil {
		t.Fatalf("FindMissingChapterNums failed: %v", err)
	}

	if len(missing) != 0 {
		t.Errorf("Expected no missing chapters, got %v", missing)
	}
}

func TestFindMissingChapterNumsEmpty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	bookID, err := db.AddBook("测试书5", "作者", "描述", 0, "http://test.com", "测试源")
	if err != nil {
		t.Fatalf("AddBook failed: %v", err)
	}

	if err := db.CreateChapterTable(bookID); err != nil {
		t.Fatalf("CreateChapterTable failed: %v", err)
	}

	missing, err := db.FindMissingChapterNums(bookID)
	if err != nil {
		t.Fatalf("FindMissingChapterNums failed: %v", err)
	}

	if len(missing) != 0 {
		t.Errorf("Expected no missing chapters for empty table, got %v", missing)
	}
}

// TestMain 确保测试使用临时目录
func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
