package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// HelpModel 帮助弹窗 — 程序介绍
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

// renderContent 渲染带颜色的帮助内容
func (m *HelpModel) renderContent() {
	if m.width == 0 || m.height == 0 {
		return
	}

	highlight := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorHighlight)).Bold(true).Render
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorMuted)).Render

	content := highlight("  TerminalReader v1.0") + "\n\n"
	content += "  一款基于终端的小说阅读器，在命令行中搜索、下载、管理和阅读网络小说。\n\n"

	content += highlight("技术栈") + "\n"
	content += "  " + muted("Bubble Tea · Bubbles · Lipgloss · SQLite") + "\n\n"

	content += highlight("作者") + "\n"
	content += "  Howard <HowardHenryStephen@gmail.com>"

	m.content = content
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
	title := TitleStyle.Render("About TerminalReader")
	content := lipgloss.JoinVertical(lipgloss.Left, title, "", body)
	content = DialogBoxStyle.Render(content)

	// Footer help bar
	footer := renderFooter([]footerItem{
		{key: "↑/k", desc: "scroll up"},
		{key: "↓/j", desc: "scroll down"},
		{key: "pgup/pgdown", desc: "page"},
		{key: "g/G", desc: "top/bottom"},
		{key: "?/esc", desc: "back"},
	}, m.width)

	// 将 footer 放在内容下方，紧贴内容无空行
	contentWithFooter := lipgloss.JoinVertical(lipgloss.Left, content, footer)

	// 使用 lipgloss.Place 全屏居中
	return lipgloss.NewStyle().
		Width(m.width).Height(m.height).
		Render(lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, contentWithFooter))
}
