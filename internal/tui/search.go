package tui

import (
	"context"
	"sort"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/henry/novel-reader/internal/crawler"
	"github.com/henry/novel-reader/internal/db"
	"github.com/henry/novel-reader/pkg/logger"
)

// searchStep 搜索步骤
type searchStep int

const (
	stepSelectSource searchStep = iota // 选择来源
	stepShowResults                    // 显示搜索结果
)

// SourceItem 来源列表项
type SourceItem struct {
	name       string
	matchScore int
}

func (s SourceItem) FilterValue() string { return s.name }
func (s SourceItem) Title() string {
	if s.matchScore > 0 {
		return s.name + "  " + lipgloss.NewStyle().Foreground(lipgloss.Color(ColorMuted)).Render("score: "+itoa(s.matchScore))
	}
	return s.name
}
func (s SourceItem) Description() string { return "" }

// SearchResultItem 搜索结果列表项
type SearchResultItem struct {
	result  db.SearchResult
	keyword string
	score   int
}

func (s SearchResultItem) FilterValue() string { return s.result.BookTitle }
func (s SearchResultItem) Title() string       { return s.result.BookTitle }
func (s SearchResultItem) Description() string {
	return ""
}

// SearchModel 搜索模型 — 使用 bubbles/list 支持翻页
type SearchModel struct {
	input       textinput.Model
	results     list.Model
	isSearching bool
	engine      *crawler.Engine
	width       int
	height      int

	step           searchStep // 当前步骤
	keyword        string     // 当前搜索关键词
	selectedSource string     // 选中的来源名称
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
		step:    stepSelectSource,
	}
}

func (m *SearchModel) searchCmd(sourceName, keyword string) tea.Cmd {
	logger.Infof("[TUI/Search] 开始搜索: %s, 来源: %s", keyword, sourceName)
	return func() tea.Msg {
		var results []db.SearchResult
		if sourceName == "" {
			results = m.engine.SearchAll(context.Background(), keyword)
		} else {
			results = m.engine.SearchBySource(context.Background(), sourceName, keyword)
		}
		logger.Debugf("[TUI/Search] 搜索完成: %s from %s, %d 个结果", keyword, sourceName, len(results))
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
		m.step = stepShowResults
		// 计算每条结果的匹配度并排序
		scored := make([]SearchResultItem, 0, len(msg.Results))
		for _, r := range msg.Results {
			if r.Error != nil {
				continue
			}
			scored = append(scored, SearchResultItem{
				result:  r,
				keyword: m.keyword,
				score:   crawler.CalcItemMatchScore(m.keyword, r.BookTitle),
			})
		}
		// 按匹配度从高到低排序
		sort.Slice(scored, func(i, j int) bool {
			return scored[i].score > scored[j].score
		})
		items := make([]list.Item, 0, len(scored))
		for _, s := range scored {
			items = append(items, s)
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
	if len(items) == 0 && !m.isSearching {
		if m.step == stepSelectSource {
			resultView = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorMuted)).Render("  select a source")
		} else if m.input.Value() != "" {
			resultView = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorMuted)).Render("  no results")
		}
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

	// 根据步骤显示不同标题
	title := "Search Books"
	if m.step == stepSelectSource && m.input.Value() != "" {
		title = "Select Source"
	} else if m.step == stepShowResults {
		title = "Search Results"
	}

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		TitleStyle.Render(title),
		"",
		inputLine,
		"",
		resultView,
	)

	// 根据步骤显示不同 footer
	var footerItems []footerItem
	if m.step == stepSelectSource {
		footerItems = []footerItem{
			{key: "↑", desc: "up"},
			{key: "↓", desc: "down"},
			{key: "enter", desc: "select source"},
			{key: "esc", desc: "close"},
		}
	} else {
		footerItems = []footerItem{
			{key: "↑", desc: "up"},
			{key: "↓", desc: "down"},
			{key: "enter", desc: "confirm"},
			{key: "b", desc: "background"},
			{key: "esc", desc: "back"},
		}
	}
	footer := renderFooter(footerItems, 68)
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
	m.step = stepSelectSource
	m.keyword = ""
	m.selectedSource = ""
}

// Focus 聚焦输入框
func (m *SearchModel) Focus() {
	m.input.Focus()
}

// Blur 失焦
func (m *SearchModel) Blur() {
	m.input.Blur()
}

// StartSearch 开始搜索流程
// 步骤1：输入关键词后，显示来源选择列表
func (m *SearchModel) StartSearch() tea.Cmd {
	m.keyword = m.input.Value()
	logger.Debugf("[TUI/Search] 用户输入关键词: %s", m.keyword)

	// 并发计算每个来源的匹配度
	type sourceMatch struct {
		name  string
		score int
	}
	sourceNames := m.engine.GetSourceNames()
	matchCh := make(chan sourceMatch, len(sourceNames))
	for _, name := range sourceNames {
		go func(srcName string) {
			score := m.engine.SourceMatchScore(context.Background(), srcName, m.keyword)
			matchCh <- sourceMatch{name: srcName, score: score}
		}(name)
	}

	// 收集匹配度结果
	matchMap := make(map[string]int, len(sourceNames))
	for i := 0; i < len(sourceNames); i++ {
		m := <-matchCh
		matchMap[m.name] = m.score
	}

	// 显示来源选择列表（带匹配度）
	sourceItems := make([]list.Item, 0)
	for _, name := range sourceNames {
		displayName := name
		if name == "爱下电子书" {
			displayName = "源A"
		} else if name == "笔趣阁" {
			displayName = "源B"
		}
		sourceItems = append(sourceItems, SourceItem{name: displayName, matchScore: matchMap[name]})
	}
	m.step = stepSelectSource
	m.results.SetItems(sourceItems)
	return nil
}

// SelectSource 选择来源并开始搜索
func (m *SearchModel) SelectSource() tea.Cmd {
	sel := m.results.SelectedItem()
	if sel == nil {
		return nil
	}
	item, ok := sel.(SourceItem)
	if !ok {
		return nil
	}

	// 将显示名称映射回原始来源名称
	m.selectedSource = item.name
	if item.name == "源A" {
		m.selectedSource = "爱下电子书"
	} else if item.name == "源B" {
		m.selectedSource = "笔趣阁"
	}

	logger.Debugf("[TUI/Search] 用户选择来源: %s", m.selectedSource)
	m.isSearching = true
	m.results.SetItems([]list.Item{})
	return m.searchCmd(m.selectedSource, m.keyword)
}

// GoBack 从搜索结果返回到来源选择
func (m *SearchModel) GoBack() {
	m.step = stepSelectSource
	m.results.SetItems([]list.Item{})
	m.StartSearch()
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

// IsSelectingSource 是否正在选择来源
func (m SearchModel) IsSelectingSource() bool {
	return m.step == stepSelectSource
}

// SearchResultsMsg 搜索结果消息
type SearchResultsMsg struct {
	Results []db.SearchResult
}

// renderMatchBar 将匹配度渲染为可视化条
func renderMatchBar(score int) string {
	if score >= 100 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#4ade80")).Render("██████ 100%")
	}
	if score >= 80 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#4ade80")).Render("█████░ " + itoa(score) + "%")
	}
	if score >= 60 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAccent)).Render("████░░ " + itoa(score) + "%")
	}
	if score >= 40 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAccent)).Render("███░░░ " + itoa(score) + "%")
	}
	if score >= 20 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(ColorMuted)).Render("██░░░░ " + itoa(score) + "%")
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(ColorMuted)).Render("█░░░░░ " + itoa(score) + "%")
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [4]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

func newSearchDelegate() list.DefaultDelegate {
	d := list.NewDefaultDelegate()
	d.SetSpacing(0)
	d.Styles.NormalTitle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorText)).
		Padding(0, 0, 0, 2)
	d.Styles.NormalDesc = lipgloss.NewStyle().
		Height(0).
		Padding(0, 0, 0, 2)
	d.Styles.SelectedTitle = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(lipgloss.Color(ColorAccent)).
		Foreground(lipgloss.Color(ColorAccent)).
		Padding(0, 0, 0, 1)
	d.Styles.SelectedDesc = lipgloss.NewStyle().
		Height(0).
		Padding(0, 0, 0, 1)
	return d
}
