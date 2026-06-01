package tui

import "github.com/henry/novel-reader/pkg/logger"

// Toast 通知组件
type Toast struct {
	Content string
	IsError bool
	Visible bool
}

// NewToast 创建 Toast
func NewToast(content string, isError bool) Toast {
	logger.Debugf("[TUI/Toast] 创建Toast: %s", content)
	return Toast{
		Content: content,
		IsError: isError,
		Visible: true,
	}
}

// View 渲染 Toast
func (t *Toast) View() string {
	if !t.Visible || t.Content == "" {
		return ""
	}
	if t.IsError {
		return ToastErrorStyle.Render(t.Content)
	}
	return ToastSuccessStyle.Render(t.Content)
}

// ToastHeight Toast 占用高度
func (t *Toast) ToastHeight() int {
	if !t.Visible || t.Content == "" {
		return 0
	}
	return 1
}

// Clear 清空 Toast
func (t *Toast) Clear() {
	t.Visible = false
	t.Content = ""
}
