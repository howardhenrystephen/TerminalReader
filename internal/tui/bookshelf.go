package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/henry/novel-reader/internal/db"
	"github.com/henry/novel-reader/pkg/logger"
)

// BookItem 列表项包装
type BookItem struct {
	book db.Book
}

// PinnedMarker 置顶标记
const PinnedMarker = "+ "

// FilterValue 返回过滤值
func (b BookItem) FilterValue() string { return b.book.Title }

// Title 返回标题
func (b BookItem) Title() string {
	if b.book.Pinned {
		return PinnedMarker + b.book.Title
	}
	return b.book.Title
}

// Description 返回描述（两行：作者+进度+时间，简介）
func (b BookItem) Description() string {
	progress := ""
	if b.book.TotalChapters > 0 {
		progress = fmt.Sprintf("Ch %d/%d", b.book.CurrentChapter, b.book.TotalChapters)
	} else {
		progress = fmt.Sprintf("Ch %d", b.book.CurrentChapter)
	}
	readAt := b.book.UpdatedAt.Format("2006-01-02")
	if b.book.UpdatedAt.IsZero() {
		readAt = "never"
	}
	line1 := progress
	if b.book.Author != "" {
		line1 = b.book.Author + " · " + line1
	}
	line1 += " · " + readAt

	desc := strings.TrimSpace(b.book.Description)
	if desc == "" {
		return line1
	}
	return line1 + "\n" + desc
}

// BookshelfModel 书架模型
type BookshelfModel struct {
	list         list.Model
	books        []db.Book
	db           *db.DB
	width        int
	height       int
	descViewport viewport.Model
}

// NewBookshelfModel 创建书架模型
func NewBookshelfModel(database *db.DB) BookshelfModel {
	logger.Debugf("[TUI/Bookshelf] 初始化书架模型")
	delegate := newBookDelegate()
	l := list.New([]list.Item{}, delegate, 0, 0)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.SetShowPagination(false)
	l.Title = "My Bookshelf"
	l.Styles.Title = TitleStyle

	m := BookshelfModel{
		list:         l,
		db:           database,
		descViewport: viewport.New(30, 10),
	}
	return m
}

// LoadBooks 从数据库加载书籍
func (m *BookshelfModel) LoadBooks() tea.Cmd {
	logger.Debugf("[TUI/Bookshelf] 加载书籍列表")
	return func() tea.Msg {
		books, err := m.db.ListBooks()
		if err != nil {
			logger.Errorf("[TUI/Bookshelf] 加载书籍失败: %v", err)
			return ShowToastMsg{Content: "Failed to load bookshelf: " + err.Error(), IsError: true}
		}
		logger.Debugf("[TUI/Bookshelf] 加载到 %d 本书", len(books))
		return bookshelfLoadedMsg{books: books}
	}
}

// Init 初始化
func (m BookshelfModel) Init() tea.Cmd {
	return m.LoadBooks()
}

// Update 更新
func (m BookshelfModel) Update(msg tea.Msg) (BookshelfModel, tea.Cmd) {
	switch msg := msg.(type) {
	case bookshelfLoadedMsg:
		m.books = msg.books
		items := make([]list.Item, len(m.books))
		for i, b := range m.books {
			items[i] = BookItem{book: b}
		}
		cmd := m.list.SetItems(items)
		return m, cmd
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width, msg.Height-4)
	case pinToggledMsg:
		// 置顶状态切换后，重新加载书籍列表以更新排序和标记
		return m, m.LoadBooks()
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// View 渲染书架
func (m BookshelfModel) View() string {
	return m.ViewWithMini("")
}

// ViewWithMini 渲染书架，支持在 footer 下方显示迷你进度
func (m BookshelfModel) ViewWithMini(miniView string) string {
	listView := m.list.View()

	// 构建 footer
	footerItems := []footerItem{
		{key: "↑/k", desc: "up"},
		{key: "↓/j", desc: "down"},
		{key: "enter", desc: "open"},
		{key: "s", desc: "search"},
		{key: "d", desc: "delete"},
		{key: "tab", desc: "desc"},
		{key: "p", desc: "pin"},
		{key: "?", desc: "help"},
		{key: "q", desc: "quit"},
	}
	footer := renderFooter(footerItems, m.width)

	// list 固定高度，让 footer 紧贴最底部
	listHeight := m.height - 2
	if miniView != "" {
		listHeight = m.height - 3
	}
	if listHeight < 1 {
		listHeight = 1
	}
	listView = lipgloss.NewStyle().Height(listHeight).Render(listView)

	// 如果有迷你进度条，放在 footer 下方单独一行
	var content string
	if miniView != "" {
		content = lipgloss.JoinVertical(lipgloss.Left, listView, footer, miniView)
	} else {
		content = lipgloss.JoinVertical(lipgloss.Left, listView, footer)
	}
	return lipgloss.NewStyle().
		Width(m.width).Height(m.height).
		Render(lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Top, content))
}

// ViewDescFull 渲染全屏书籍简介视图
func (m BookshelfModel) ViewDescFull(width, height int) string {
	book := m.SelectedBook()
	if book == nil {
		return ""
	}

	// 标题栏
	titleBar := TitleStyle.Width(width).Render(book.Title)

	// 元信息
	metaParts := []string{}
	if book.Author != "" {
		metaParts = append(metaParts, "Author: "+book.Author)
	}
	if book.TotalChapters > 0 {
		metaParts = append(metaParts, fmt.Sprintf("Chapters: %d", book.TotalChapters))
	}
	metaParts = append(metaParts, fmt.Sprintf("Current: Ch %d", book.CurrentChapter))
	if book.Pinned {
		metaParts = append(metaParts, "📌 Pinned")
	}
	metaLine := lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorSubtext)).
		Render(strings.Join(metaParts, "  ·  "))

	// 简介内容（使用 viewport）
	m.descViewport.Width = width - 8
	m.descViewport.Height = height - 10
	m.descViewport.SetContent(strings.TrimSpace(book.Description))
	descView := m.descViewport.View()

	// 组装内容
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		titleBar,
		"",
		metaLine,
		"",
		lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(ColorMuted)).
			Width(width-4).
			Height(height-10).
			Padding(0, 1).
			Render(descView),
		"",
		renderFooter([]footerItem{
			{key: "↑/k", desc: "scroll up"},
			{key: "↓/j", desc: "scroll down"},
			{key: "tab/esc", desc: "back"},
		}, width),
	)

	return lipgloss.NewStyle().
		Width(width).Height(height).
		Render(lipgloss.Place(width, height, lipgloss.Center, lipgloss.Top, content))
}

// SelectedBook 返回当前选中的书籍
func (m BookshelfModel) SelectedBook() *db.Book {
	if i, ok := m.list.SelectedItem().(BookItem); ok {
		return &i.book
	}
	return nil
}

// updateDescContent 更新简介面板内容
func (m *BookshelfModel) updateDescContent() {
	if book := m.SelectedBook(); book != nil {
		desc := strings.TrimSpace(book.Description)
		if desc == "" {
			desc = "暂无简介"
		}
		m.descViewport.SetContent(desc)
	}
}

// TogglePin 切换当前选中书籍的置顶状态
func (m *BookshelfModel) TogglePin() tea.Cmd {
	if book := m.SelectedBook(); book == nil {
		return nil
	}
	return func() tea.Msg {
		book := m.SelectedBook()
		if book == nil {
			return nil
		}
		newPin := !book.Pinned
		if err := m.db.UpdateBookPin(book.ID, newPin); err != nil {
			return ShowToastMsg{Content: "Failed to pin: " + err.Error(), IsError: true}
		}
		return pinToggledMsg{bookID: book.ID, pinned: newPin}
	}
}

// SetSize 设置尺寸
func (m *BookshelfModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.list.SetSize(width, height-2)
}

// bookshelfLoadedMsg 内部消息
type bookshelfLoadedMsg struct {
	books []db.Book
}

// pinToggledMsg 置顶状态切换消息
type pinToggledMsg struct {
	bookID int64
	pinned bool
}

func newBookDelegate() list.DefaultDelegate {
	d := list.NewDefaultDelegate()
	d.SetSpacing(0)
	d.Styles.NormalTitle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorText)).
		Padding(0, 0, 0, 2)
	d.Styles.NormalDesc = lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorMuted)).
		Padding(0, 0, 0, 2)
	d.Styles.SelectedTitle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorAccent)).
		Padding(0, 0, 0, 2)
	d.Styles.SelectedDesc = lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorSubtext)).
		Padding(0, 0, 0, 2)
	return d
}

// footerItem help 栏单项
type footerItem struct {
	key  string
	desc string
}

// renderFooter 渲染底部 help 栏
func renderFooter(items []footerItem, width int) string {
	var parts []string
	for _, item := range items {
		part := HelpKeyStyle.Render(item.key) + HelpSepStyle.Render(":") + HelpDescStyle.Render(item.desc)
		parts = append(parts, part)
	}
	line := strings.Join(parts, "  ")
	return FooterStyle.Width(width).Render(line)
}
