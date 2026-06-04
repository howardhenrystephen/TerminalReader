package crawler

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestIxdzs8Fetch(t *testing.T) {
	src := NewIxdzs8Source()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 1. 测试搜索
	t.Run("Search", func(t *testing.T) {
		results, err := src.Search(ctx, "剑颂")
		if err != nil {
			t.Fatalf("搜索失败: %v", err)
		}
		if len(results) == 0 {
			t.Fatal("搜索无结果")
		}
		for i, r := range results {
			fmt.Printf("[%d] Title=%s Author=%s URL=%s Available=%v\n", i, r.BookTitle, r.Author, r.SourceURL, r.Available)
		}
	})

	// 2. 测试获取书籍信息
	t.Run("FetchBookInfo", func(t *testing.T) {
		info, err := src.FetchBookInfo(ctx, "https://ixdzs8.com/read/233763/")
		if err != nil {
			t.Fatalf("获取书籍信息失败: %v", err)
		}
		fmt.Printf("Title=%s Author=%s TotalChapters=%d\n", info.Title, info.Author, info.TotalChapters)
	})

	// 3. 测试获取章节列表
	t.Run("FetchChapterList", func(t *testing.T) {
		chapters, err := src.FetchChapterList(ctx, "https://ixdzs8.com/read/233763/")
		if err != nil {
			t.Fatalf("获取章节列表失败: %v", err)
		}
		if len(chapters) == 0 {
			t.Fatal("章节列表为空")
		}
		fmt.Printf("Total chapters: %d\n", len(chapters))
		for i, ch := range chapters {
			if i < 5 || i >= len(chapters)-3 {
				fmt.Printf("  [%d] Num=%d Title=%s URL=%s\n", i, ch.Num, ch.Title, ch.URL)
			} else if i == 5 {
				fmt.Println("  ...")
			}
		}
	})

	// 4. 测试获取章节内容
	t.Run("FetchChapterContent", func(t *testing.T) {
		content, err := src.FetchChapterContent(ctx, "https://ixdzs8.com/read/233763/p2.html")
		if err != nil {
			t.Fatalf("获取章节内容失败: %v", err)
		}
		if content == "" {
			t.Fatal("章节内容为空")
		}
		// 只打印前 200 字符
		preview := content
		if len(preview) > 200 {
			preview = preview[:200] + "..."
		}
		fmt.Printf("Content preview:\n%s\n", preview)
	})
}
