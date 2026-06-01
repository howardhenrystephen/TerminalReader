package tui

import (
	"fmt"
	"regexp"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/wrap"

	"github.com/henry/novel-reader/internal/db"
	"github.com/henry/novel-reader/pkg/logger"
)

// ReaderModel 阅读器模型
type ReaderModel struct {
	bookID         int64
	bookTitle      string
	chapterNum     int
	chapterTitle   string
	chapterContent string
	offset         int
	totalLines     int
	lines          []string
	db             *db.DB
	width          int
	height         int
}

// NewReaderModel 创建阅读器模型
func NewReaderModel(database *db.DB) *ReaderModel {
	return &ReaderModel{
		db: database,
	}
}

// LoadBook 加载书籍和当前章节
func (m *ReaderModel) LoadBook(bookID int64) tea.Cmd {
	logger.Debugf("[TUI/Reader] 加载书籍 ID=%d", bookID)
	return func() tea.Msg {
		book, err := m.db.GetBook(bookID)
		if err != nil || book == nil {
			logger.Errorf("[TUI/Reader] 加载书籍失败 ID=%d: %v", bookID, err)
			return ShowToastMsg{Content: "Failed to load book", IsError: true}
		}
		m.bookID = book.ID
		m.bookTitle = book.Title
		m.chapterNum = book.CurrentChapter
		if m.chapterNum < 1 {
			m.chapterNum = 1
		}
		logger.Infof("[TUI/Reader] 打开书籍: %s 第%d章", m.bookTitle, m.chapterNum)
		return m.loadChapterCmd(m.chapterNum)()
	}
}

func (m *ReaderModel) loadChapterCmd(chapterNum int) tea.Cmd {
	return func() tea.Msg {
		logger.Debugf("[TUI/Reader] 加载章节 bookID=%d chapterNum=%d", m.bookID, chapterNum)
		ch, err := m.db.GetChapter(m.bookID, chapterNum)
		if err != nil {
			logger.Errorf("[TUI/Reader] 加载章节失败: %v", err)
			return ShowToastMsg{Content: "Failed to load chapter: " + err.Error(), IsError: true}
		}
		if ch == nil {
			logger.Warnf("[TUI/Reader] 章节不存在: bookID=%d chapterNum=%d", m.bookID, chapterNum)
			return ShowToastMsg{Content: "Chapter not found", IsError: true}
		}
		logger.Debugf("[TUI/Reader] 章节加载完成: %s (字数=%d)", ch.Title, len([]rune(ch.Content)))
		return chapterLoadedMsg{
			chapterNum:   ch.ChapterNum,
			chapterTitle: ch.Title,
			content:      ch.Content,
		}
	}
}

// reflow 重新排版文本
func (m *ReaderModel) reflow() {
	if m.width <= 0 {
		return
	}
	contentWidth := m.width - 4
	if contentWidth < 10 {
		contentWidth = 10
	}

	cleaned := stripChapterHeaders(m.chapterContent)
	wrapped := wrap.String(cleaned, contentWidth)
	m.lines = strings.Split(wrapped, "\n")
	m.totalLines = len(m.lines)
}

// stripChapterHeaders 删除内容开头的章节标题重复行
// 匹配模式：第X章... 或 第X章...（中间有无空格或换行）
func stripChapterHeaders(content string) string {
	// 正则匹配 "第XXX章" 开头的行，支持中文数字和阿拉伯数字
	// 匹配模式如：第1章、第一章、第100章、第一百章 等
	re := regexp.MustCompile(`(?m)^[第][零一二三四五六七八九十百千万亿\d]+[章].*$`)

	lines := strings.Split(content, "\n")
	startIdx := 0
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			// 空行跳过，但如果已经找到了标题行，空行也算作标题区域的一部分
			if startIdx > 0 && i == startIdx {
				startIdx++
			}
			continue
		}
		if re.MatchString(trimmed) {
			startIdx = i + 1
		} else {
			// 非标题行且非空行，停止扫描
			break
		}
	}

	if startIdx >= len(lines) {
		return ""
	}
	return strings.Join(lines[startIdx:], "\n")
}

// visibleLines 返回当前可见行
func (m *ReaderModel) visibleLines() []string {
	if m.totalLines == 0 {
		return nil
	}
	contentHeight := m.height - 3 // header(1) + footer(1), no bottom padding
	if contentHeight < 1 {
		contentHeight = 1
	}
	end := m.offset + contentHeight
	if end > m.totalLines {
		end = m.totalLines
	}
	return m.lines[m.offset:end]
}

func (m *ReaderModel) scrollDown() {
	contentHeight := m.height - 3
	if m.offset < m.totalLines-contentHeight {
		m.offset++
	}
}

func (m *ReaderModel) scrollUp() {
	if m.offset > 0 {
		m.offset--
	}
}

func (m *ReaderModel) readingPercentage() float64 {
	if m.totalLines == 0 {
		return 0
	}
	return float64(m.offset) / float64(m.totalLines) * 100
}

// Init 初始化
func (m ReaderModel) Init() tea.Cmd {
	return nil
}

// Update 更新
func (m ReaderModel) Update(msg tea.Msg) (ReaderModel, tea.Cmd) {
	switch msg := msg.(type) {
	case chapterLoadedMsg:
		m.chapterNum = msg.chapterNum
		m.chapterTitle = msg.chapterTitle
		m.chapterContent = msg.content
		m.offset = 0
		m.reflow()
		return m, nil
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.reflow()
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			m.scrollUp()
		case "down", "j":
			m.scrollDown()
		case " ", "f", "pgdown":
			m.PageDown()
		case "b", "pgup":
			m.PageUp()
		case "g":
			m.GoStart()
		case "G":
			m.GoEnd()
		}
	}
	return m, nil
}

// View 渲染阅读器
func (m ReaderModel) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	header := ReaderHeaderStyle.Width(m.width).Render(
		fmt.Sprintf("%s · %s (%d%%)", m.bookTitle, m.chapterTitle, int(m.readingPercentage())),
	)

	visible := m.visibleLines()
	content := strings.Join(visible, "\n")
	if content == "" {
		content = "No content"
	}
	body := ReaderTextStyle.Width(m.width).Render(content)

	footer := renderFooter([]footerItem{
		{key: "↑/k", desc: "scroll up"},
		{key: "↓/j", desc: "scroll down"},
		{key: "space/f", desc: "page down"},
		{key: "b", desc: "page up"},
		{key: "←/h", desc: "prev chapter"},
		{key: "→/l", desc: "next chapter"},
		{key: "c", desc: "chapters"},
		{key: "g/G", desc: "start/end"},
		{key: "esc", desc: "back"},
		{key: "?", desc: "help"},
	}, m.width)

	// body 固定高度，让 footer 始终贴在最底部，无下方padding
	bodyHeight := m.height - 3
	if bodyHeight < 1 {
		bodyHeight = 1
	}
	body = lipgloss.NewStyle().Height(bodyHeight).Render(body)

	viewContent := lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
	return lipgloss.NewStyle().
		Width(m.width).Height(m.height).
		Render(lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Top, viewContent))
}

// SetSize 设置尺寸
func (m *ReaderModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.reflow()
}

// SaveProgress 保存阅读进度
func (m *ReaderModel) SaveProgress() error {
	logger.Debugf("[TUI/Reader] 保存进度: bookID=%d chapter=%d offset=%d", m.bookID, m.chapterNum, m.offset)
	return m.db.UpdateBookProgress(m.bookID, m.chapterNum, m.offset)
}

// NextChapter 下一章
func (m *ReaderModel) NextChapter() tea.Cmd {
	m.chapterNum++
	m.offset = 0
	logger.Debugf("[TUI/Reader] 下一章: %d", m.chapterNum)
	return m.loadChapterCmd(m.chapterNum)
}

// PrevChapter 上一章
func (m *ReaderModel) PrevChapter() tea.Cmd {
	if m.chapterNum > 1 {
		m.chapterNum--
		m.offset = 0
		logger.Debugf("[TUI/Reader] 上一章: %d", m.chapterNum)
		return m.loadChapterCmd(m.chapterNum)
	}
	return nil
}

// PageDown 向下翻页
func (m *ReaderModel) PageDown() {
	contentHeight := m.height - 3
	if contentHeight < 1 {
		contentHeight = 1
	}
	m.offset += contentHeight
	if m.offset >= m.totalLines {
		m.offset = m.totalLines - 1
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

// PageUp 向上翻页
func (m *ReaderModel) PageUp() {
	contentHeight := m.height - 3
	if contentHeight < 1 {
		contentHeight = 1
	}
	m.offset -= contentHeight
	if m.offset < 0 {
		m.offset = 0
	}
}

// GoStart 跳到章节开头
func (m *ReaderModel) GoStart() {
	m.offset = 0
}

// GoEnd 跳到章节结尾
func (m *ReaderModel) GoEnd() {
	contentHeight := m.height - 3
	m.offset = m.totalLines - contentHeight
	if m.offset < 0 {
		m.offset = 0
	}
}

type chapterLoadedMsg struct {
	chapterNum   int
	chapterTitle string
	content      string
}
