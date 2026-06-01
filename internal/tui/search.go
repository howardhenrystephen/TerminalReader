package tui

import (
	"context"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/henry/novel-reader/internal/crawler"
	"github.com/henry/novel-reader/internal/db"
	"github.com/henry/novel-reader/pkg/logger"
)

// SearchResultItem 列表项包装
type SearchResultItem struct {
	result db.SearchResult
}

func (s SearchResultItem) FilterValue() string { return s.result.BookTitle }
func (s SearchResultItem) Title() string       { return s.result.BookTitle }
func (s SearchResultItem) Description() string {
	desc := s.result.SourceName
	if s.result.Author != "" {
		desc += " · " + s.result.Author
	}
	return desc
}

// SearchModel 搜索模型 — 使用 bubbles/list 支持翻页
type SearchModel struct {
	input       textinput.Model
	results     list.Model
	isSearching bool
	engine      *crawler.Engine
	width       int
	height      int
}

// NewSearchModel 创建搜索模型
func NewSearchModel(engine *crawler.Engine) SearchModel {
	ti := textinput.New()
	ti.Placeholder = "Search for a book..."
	ti.Focus()
	ti.CharLimit = 50
	ti.Width = 60

	delegate := newSearchDelegate()
	l := list.New([]list.Item{}, delegate, 0, 0)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.SetShowTitle(false)
	l.SetShowPagination(false)

	return SearchModel{
		input:   ti,
		results: l,
		engine:  engine,
	}
}

func (m *SearchModel) searchCmd(keyword string) tea.Cmd {
	logger.Infof("[TUI/Search] 开始搜索: %s", keyword)
	return func() tea.Msg {
		results := m.engine.SearchAll(context.Background(), keyword)
		logger.Debugf("[TUI/Search] 搜索完成: %s, %d 个结果", keyword, len(results))
		return SearchResultsMsg{Results: results}
	}
}

// Init 初始化
func (m SearchModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update 更新
func (m SearchModel) Update(msg tea.Msg) (SearchModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.results.SetSize(68, msg.Height-10)
	case SearchResultsMsg:
		m.isSearching = false
		items := make([]list.Item, 0, len(msg.Results))
		for _, r := range msg.Results {
			if r.Error != nil {
				continue
			}
			items = append(items, SearchResultItem{result: r})
		}
		cmd := m.results.SetItems(items)
		return m, cmd
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)

	var listCmd tea.Cmd
	m.results, listCmd = m.results.Update(msg)

	return m, tea.Batch(cmd, listCmd)
}

// View 渲染搜索弹层
func (m SearchModel) View() string {
	inputLine := m.input.View()
	if m.isSearching {
		inputLine += "  searching..."
	}

	var resultView string
	items := m.results.Items()
	if len(items) == 0 && !m.isSearching && m.input.Value() != "" {
		resultView = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorMuted)).Render("  no results")
	} else {
		resultView = m.results.View()
	}

	// 计算结果区域高度：固定弹窗高度，减去标题、输入框、footer 占用的空间
	boxHeight := m.height - 4
	if boxHeight < 10 {
		boxHeight = 10
	}
	fixedTopHeight := 6 // Title(1) + gap(1) + input(1) + gap(1) + footer(1) + padding(2)
	resultHeight := boxHeight - fixedTopHeight
	if resultHeight < 3 {
		resultHeight = 3
	}
	resultView = lipgloss.NewStyle().Height(resultHeight).Render(resultView)

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		TitleStyle.Render("Search Books"),
		"",
		inputLine,
		"",
		resultView,
	)

	footer := renderFooter([]footerItem{
		{key: "↑/k", desc: "up"},
		{key: "↓/j", desc: "down"},
		{key: "enter", desc: "search/confirm"},
		{key: "b", desc: "background"},
		{key: "esc", desc: "close"},
	}, 68)
	content = lipgloss.JoinVertical(lipgloss.Left, content, "", footer)

	box := DialogBoxStyle.Width(70).Height(boxHeight).Render(content)
	// 使用全屏背景填充，确保关闭搜索后不留残留
	return lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		Render(lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box))
}

// SetSize 设置尺寸
func (m *SearchModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.results.SetSize(68, height-10)
}

// Value 返回输入框内容
func (m SearchModel) Value() string {
	return m.input.Value()
}

// SetValue 设置输入框内容
func (m *SearchModel) SetValue(s string) {
	m.input.SetValue(s)
}

// Reset 重置搜索状态（清空输入、结果、光标）
func (m *SearchModel) Reset() {
	m.input.SetValue("")
	m.input.Reset()
	m.results.SetItems([]list.Item{})
	m.isSearching = false
}

// Focus 聚焦输入框
func (m *SearchModel) Focus() {
	m.input.Focus()
}

// Blur 失焦
func (m *SearchModel) Blur() {
	m.input.Blur()
}

// StartSearch 开始搜索
func (m *SearchModel) StartSearch() tea.Cmd {
	keyword := m.input.Value()
	logger.Debugf("[TUI/Search] 用户触发搜索: %s", keyword)
	m.isSearching = true
	m.results.SetItems([]list.Item{})
	return m.searchCmd(keyword)
}

// CursorUp 上移光标
func (m *SearchModel) CursorUp() {
	m.results.CursorUp()
}

// CursorDown 下移光标
func (m *SearchModel) CursorDown() {
	m.results.CursorDown()
}

// SelectedResult 返回选中的结果
func (m SearchModel) SelectedResult() *db.SearchResult {
	if i, ok := m.results.SelectedItem().(SearchResultItem); ok {
		return &i.result
	}
	return nil
}

// HasResults 返回是否有搜索结果
func (m SearchModel) HasResults() bool {
	return len(m.results.Items()) > 0
}

// SearchResultsMsg 搜索结果消息
type SearchResultsMsg struct {
	Results []db.SearchResult
}

func newSearchDelegate() list.DefaultDelegate {
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
