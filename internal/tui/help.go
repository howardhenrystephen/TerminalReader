package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// HelpModel 帮助弹窗 — 完整使用指南
type HelpModel struct {
	width    int
	height   int
	offset   int
	maxLines int
	content  string
}

// NewHelpModel 创建帮助弹窗
func NewHelpModel() HelpModel {
	return HelpModel{}
}

// Init 初始化
func (m HelpModel) Init() tea.Cmd {
	return nil
}

// Update 更新
func (m HelpModel) Update(msg tea.Msg) (HelpModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.renderContent()
	case ShowHelpMsg:
		m.offset = 0
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			m.scrollUp()
		case "down", "j":
			m.scrollDown()
		case "pgup":
			m.scrollPageUp()
		case "pgdown", " ":
			m.scrollPageDown()
		case "g":
			m.offset = 0
		case "G":
			m.offset = m.maxLines - m.visibleLines()
			if m.offset < 0 {
				m.offset = 0
			}
		}
	}
	return m, nil
}

func (m *HelpModel) scrollUp() {
	if m.offset > 0 {
		m.offset--
	}
}

func (m *HelpModel) scrollDown() {
	if m.offset < m.maxLines-m.visibleLines() {
		m.offset++
	}
}

func (m *HelpModel) scrollPageUp() {
	page := m.visibleLines()
	m.offset -= page
	if m.offset < 0 {
		m.offset = 0
	}
}

func (m *HelpModel) scrollPageDown() {
	page := m.visibleLines()
	m.offset += page
	max := m.maxLines - m.visibleLines()
	if max < 0 {
		max = 0
	}
	if m.offset > max {
		m.offset = max
	}
}

func (m HelpModel) visibleLines() int {
	v := m.height - 4
	if v < 3 {
		v = 3
	}
	return v
}

// renderContent 渲染帮助内容
func (m *HelpModel) renderContent() {
	if m.width == 0 || m.height == 0 {
		return
	}

	accent := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAccent)).Bold(true).Render
	highlight := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorHighlight)).Bold(true).Render
	success := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccess)).Render
	text := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorText)).Render
	sub := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSubtext)).Render

	_ = text
	_ = sub

	var b strings.Builder

	b.WriteString(accent("  TerminalReader") + "\n\n")
	b.WriteString("  一款基于终端的小说阅读器，在命令行中搜索、下载、管理和阅读网络小说。\n\n")

	b.WriteString(highlight("  页面说明") + "\n\n")

	b.WriteString(accent("  Bookshelf（书架）") + "\n")
	b.WriteString("    书籍列表页，显示所有已下载的书籍。每本书展示标题、作者、阅读进度、\n")
	b.WriteString("    已下载章节数和更新时间。支持置顶排序和查看简介。\n\n")

	b.WriteString(accent("  Reader（阅读器）") + "\n")
	b.WriteString("    全屏阅读界面，自动去除重复章节标题。退出时自动保存阅读位置，\n")
	b.WriteString("    下次打开自动恢复到上次阅读位置。\n\n")

	b.WriteString(accent("  Search（搜索）") + "\n")
	b.WriteString("    输入书名搜索网络小说，支持多来源并发搜索。选择可用来源后，\n")
	b.WriteString("    按 Enter 前台下载，按 b 后台下载。\n\n")

	b.WriteString(accent("  Chapter Picker（章节选择）") + "\n")
	b.WriteString("    阅读器中按 c 打开，可快速跳转到任意章节，支持 / 过滤搜索。\n\n")

	b.WriteString(accent("  Book Description（书籍简介）") + "\n")
	b.WriteString("    书架中按 tab 打开，展示书籍的完整简介和元信息。\n\n")

	b.WriteString(highlight("  书架快捷键") + "\n\n")
	b.WriteString(success("    ↑") + "        上移\n")
	b.WriteString(success("    ↓") + "        下移\n")
	b.WriteString(success("    enter") + "    打开书籍\n")
	b.WriteString(success("    s") + "        搜索新书\n")
	b.WriteString(success("    c") + "        继续下载（从上次进度增量下载）\n")
	b.WriteString(success("    d") + "        删除书籍\n")
	b.WriteString(success("    r") + "        刷新书架\n")
	b.WriteString(success("    tab") + "      查看书籍简介\n")
	b.WriteString(success("    p") + "        置顶 / 取消置顶\n")
	b.WriteString(success("    g") + "        跳到顶部\n")
	b.WriteString(success("    G") + "        跳到底部\n")
	b.WriteString(success("    r") + "        强制重绘（清除残留 toast / 进度条）\n")
	b.WriteString(success("    ?") + "        显示帮助\n")
	b.WriteString(success("    q") + "        退出程序\n\n")

	b.WriteString(highlight("  阅读器快捷键") + "\n\n")
	b.WriteString(success("    ↑") + "        向上滚动一行\n")
	b.WriteString(success("    ↓") + "        向下滚动一行\n")
	b.WriteString(success("    space") + "    向下翻页\n")
	b.WriteString(success("    b") + "        向上翻页\n")
	b.WriteString(success("    g") + "        跳到章节开头\n")
	b.WriteString(success("    G") + "        跳到章节结尾\n")
	b.WriteString(success("    ←") + "        上一章\n")
	b.WriteString(success("    →") + "        下一章\n")
	b.WriteString(success("    c") + "        打开章节选择器\n")
	b.WriteString(success("    esc") + "      返回书架（自动保存进度）\n")
	b.WriteString(success("    ?") + "        显示帮助\n\n")

	b.WriteString(highlight("  章节选择器快捷键") + "\n\n")
	b.WriteString(success("    ↑") + "        上移\n")
	b.WriteString(success("    ↓") + "        下移\n")
	b.WriteString(success("    enter") + "    跳转到选中章节\n")
	b.WriteString(success("    /") + "        开始过滤\n")
	b.WriteString(success("    esc") + "      关闭选择器\n\n")

	b.WriteString(highlight("  搜索与下载") + "\n\n")
	b.WriteString("    1. 书架按 s 打开搜索，输入书名后按 Enter 搜索\n")
	b.WriteString("    2. 用 ↑/↓ 选择可用来源（绿色标记）\n")
	b.WriteString("    3. 按 Enter 前台下载，显示实时进度弹窗\n")
	b.WriteString("    4. 按 b 后台下载，书架底部显示迷你进度条\n\n")
	b.WriteString("    5. 后台下载时按 x 可停止，完成后自动刷新书架\n")
	b.WriteString("    6. 已存在的书籍会自动增量下载，跳过已有章节\n\n")

	b.WriteString(highlight("  技术栈") + "\n")
	b.WriteString("    Bubble Tea · Bubbles · Lipgloss · SQLite (WAL) · Python cloudscraper\n\n")

	b.WriteString(highlight("  作者") + "\n")
	b.WriteString("    Howard <HowardHenryStephen@gmail.com>")

	m.content = b.String()
	m.maxLines = len(strings.Split(m.content, "\n"))
}

// View 渲染帮助弹窗
func (m HelpModel) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	if m.content == "" {
		m.renderContent()
	}

	lines := strings.Split(m.content, "\n")
	visible := m.visibleLines() - 2

	end := m.offset + visible
	if end > len(lines) {
		end = len(lines)
	}

	var visibleLines []string
	if m.offset < len(lines) {
		visibleLines = lines[m.offset:end]
	}

	// body 高度固定为 visible，不足时底部留白
	body := lipgloss.NewStyle().Height(visible).Render(strings.Join(visibleLines, "\n"))

	// 标题 + 内容，用 DialogBox 包裹（上下各1行padding）
	title := TitleStyle.Render("Help & Guide")
	content := lipgloss.JoinVertical(lipgloss.Left, title, "", body)
	content = DialogBoxStyle.Render(content)

	// Footer help bar
	footer := renderFooter([]footerItem{
		{key: "↑", desc: "scroll up"},
		{key: "↓", desc: "scroll down"},
		{key: "pgup", desc: "page up"},
		{key: "pgdown", desc: "page down"},
		{key: "g", desc: "top"},
		{key: "G", desc: "bottom"},
		{key: "?", desc: "back"},
	}, m.width)

	// 将 footer 放在内容下方，紧贴内容无空行
	contentWithFooter := lipgloss.JoinVertical(lipgloss.Left, content, footer)

	// 使用 lipgloss.Place 全屏居中
	return lipgloss.NewStyle().
		Width(m.width).Height(m.height).
		Render(lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, contentWithFooter))
}
