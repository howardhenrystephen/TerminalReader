package crawler

import (
	"testing"
)

func TestSimplifyChapterTitle(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"第一章 测试标题", "测试标题"},
		{"第1章 测试标题", "测试标题"},
		{"第123章 测试标题", "测试标题"},
		{"测试标题", "测试标题"},
		{"  第一章 测试标题  ", "测试标题"},
		{"", ""},
	}

	for _, tt := range tests {
		result := simplifyChapterTitle(tt.input)
		if result != tt.expected {
			t.Errorf("simplifyChapterTitle(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestFindMatchingChapters(t *testing.T) {
	chapters := []ChapterInfo{
		{Num: 1, Title: "第一章 起始"},
		{Num: 2, Title: "第二章 发展"},
		{Num: 3, Title: "第三章 高潮"},
		{Num: 4, Title: "第四章 结局"},
		{Num: 5, Title: "第五章 尾声"},
	}

	tests := []struct {
		title       string
		simpleTitle string
		expected    []int
	}{
		{"第二章 发展", "发展", []int{1}},
		{"第三章 高潮", "高潮", []int{2}},
		{"不存在的标题", "不存在", []int{}},
		{"第一章 起始", "起始", []int{0}},
	}

	for _, tt := range tests {
		result := findMatchingChapters(chapters, tt.title, tt.simpleTitle)
		if len(result) != len(tt.expected) {
			t.Errorf("findMatchingChapters(%q, %q) = %v, want %v", tt.title, tt.simpleTitle, result, tt.expected)
			continue
		}
		for i := range result {
			if result[i] != tt.expected[i] {
				t.Errorf("findMatchingChapters(%q, %q)[%d] = %d, want %d", tt.title, tt.simpleTitle, i, result[i], tt.expected[i])
			}
		}
	}
}

func TestFindMatchingChaptersSimilarity(t *testing.T) {
	chapters := []ChapterInfo{
		{Num: 1, Title: "第一章 起始之地"},
		{Num: 2, Title: "第二章 发展之路"},
		{Num: 3, Title: "第三章 高潮之战"},
	}

	// 测试相似度匹配（微小差异）
	result := findMatchingChapters(chapters, "第二章 发展之路", "发展之路")
	if len(result) != 1 || result[0] != 1 {
		t.Errorf("相似度匹配失败: got %v, want [1]", result)
	}
}

func TestFindMatchingChaptersEmpty(t *testing.T) {
	result := findMatchingChapters([]ChapterInfo{}, "测试", "测试")
	if len(result) != 0 {
		t.Errorf("空列表应返回空结果: got %v", result)
	}
}

func TestFindMatchingChaptersMultipleMatches(t *testing.T) {
	chapters := []ChapterInfo{
		{Num: 1, Title: "第一章 测试"},
		{Num: 2, Title: "第二章 测试"},
		{Num: 3, Title: "第三章 其他"},
	}

	// 搜索 "测试" 应该匹配多个
	result := findMatchingChapters(chapters, "测试", "测试")
	if len(result) != 2 {
		t.Errorf("多匹配测试失败: got %v, want 2 matches", result)
	}
}
