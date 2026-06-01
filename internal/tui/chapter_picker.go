package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/henry/novel-reader/internal/db"
	"github.com/henry/novel-reader/pkg/logger"
)

// ChapterItem 章节列表项
type ChapterItem struct {
	num     int
	title   string
	preview string
}

func (c ChapterItem) FilterValue() string { return c.title }
func (c ChapterItem) Title() string       { return fmt.Sprintf("Ch %d: %s", c.num, c.title) }
func (c ChapterItem) Description() string {
	if c.preview == "" {
		return ""
	}
	return truncate(c.preview, 50)
}

// ChapterPickerModel 章节选择弹窗模型
type ChapterPickerModel struct {
	list      list.Model
	db        *db.DB
	bookID    int64
	width     int
	height    int
	active    bool
	currentCh int
	filtering bool
}

// NewChapterPickerModel 创建章节选择模型
func NewChapterPickerModel(database *db.DB) ChapterPickerModel {
	delegate := newChapterDelegate()
	l := list.New([]list.Item{}, delegate, 0, 0)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(false)
	l.SetShowTitle(false)
	l.SetShowPagination(false)
	// 默认过滤关闭，按 / 开启
	l.ResetFilter()

	return ChapterPickerModel{
		db:   database,
		list: l,
	}
}

// Open 打开章节选择器，加载指定书籍的章节列表
func (m *ChapterPickerModel) Open(bookID int64, currentCh int) tea.Cmd {
	logger.Debugf("[TUI/ChapterPicker] 打开章节选择器 bookID=%d currentCh=%d", bookID, currentCh)
	m.bookID = bookID
	m.currentCh = currentCh
	m.active = true
	m.filtering = false
	m.list.ResetFilter()
	return m.loadChaptersCmd()
}

// Close 关闭章节选择器
func (m *ChapterPickerModel) Close() {
	logger.Debugf("[TUI/ChapterPicker] 关闭章节选择器")
	m.active = false
	m.filtering = false
	m.list.ResetFilter()
	m.list.SetItems([]list.Item{})
}

// IsActive 是否处于打开状态
func (m ChapterPickerModel) IsActive() bool {
	return m.active
}

// SelectedChapter 返回当前选中的章节号
func (m ChapterPickerModel) SelectedChapter() int {
	if item, ok := m.list.SelectedItem().(ChapterItem); ok {
		return item.num
	}
	return 0
}

func (m *ChapterPickerModel) loadChaptersCmd() tea.Cmd {
	return func() tea.Msg {
		logger.Debugf("[TUI/ChapterPicker] 加载章节列表 bookID=%d", m.bookID)
		chapters, err := m.db.ListChaptersWithPreview(m.bookID)
		if err != nil {
			logger.Errorf("[TUI/ChapterPicker] 加载章节失败: %v", err)
			return ShowToastMsg{Content: "Failed to load chapters: " + err.Error(), IsError: true}
		}
		logger.Debugf("[TUI/ChapterPicker] 加载到 %d 个章节", len(chapters))
		items := make([]list.Item, len(chapters))
		for i, ch := range chapters {
			items[i] = ChapterItem{num: ch.Num, title: ch.Title, preview: ch.Preview}
		}
		return chapterListLoadedMsg{items: items, currentCh: m.currentCh}
	}
}

// Init 初始化
func (m ChapterPickerModel) Init() tea.Cmd {
	return nil
}

// Update 更新
func (m ChapterPickerModel) Update(msg tea.Msg) (ChapterPickerModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(58, msg.Height-10)
	case chapterListLoadedMsg:
		cmd := m.list.SetItems(msg.items)
		// 滚动到当前章节
		for i, item := range msg.items {
			if ch, ok := item.(ChapterItem); ok && ch.num == msg.currentCh {
				m.list.Select(i)
				break
			}
		}
		return m, cmd
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// View 渲染章节选择弹窗
func (m ChapterPickerModel) View() string {
	if !m.active {
		return ""
	}

	boxHeight := m.height - 4
	if boxHeight < 10 {
		boxHeight = 10
	}
	fixedTopHeight := 2 // Title(1) + footer(1)
	listHeight := boxHeight - fixedTopHeight
	if listHeight < 3 {
		listHeight = 3
	}
	m.list.SetSize(58, listHeight)
	listView := m.list.View()

	// footer 和标题放在同一行
	footer := renderFooter([]footerItem{
		{key: "↑/k", desc: "up"},
		{key: "↓/j", desc: "down"},
		{key: "enter", desc: "jump"},
		{key: "/", desc: "filter"},
		{key: "esc", desc: "close"},
	}, 58)

	filterHint := ""
	if m.list.FilterState() == list.Filtering {
		filterHint = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAccent)).Render("[filtering]")
	}

	titleLine := lipgloss.JoinHorizontal(
		lipgloss.Top,
		TitleStyle.Render("Select Chapter"),
		"  ",
		filterHint,
	)

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		titleLine,
		listView,
		footer,
	)

	return lipgloss.NewStyle().
		Width(m.width).Height(m.height).
		Render(lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, DialogBoxStyle.Width(60).Render(content)))
}

// SetSize 设置尺寸
func (m *ChapterPickerModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.list.SetSize(58, height-10)
}

func newChapterDelegate() list.DefaultDelegate {
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

func firstLine(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}

type chapterListLoadedMsg struct {
	items     []list.Item
	currentCh int
}
