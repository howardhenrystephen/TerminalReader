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

// showToastCmd 创建显示 Toast 的命令
func showToastCmd(content string, isError bool) tea.Cmd {
	return func() tea.Msg {
		return ShowToastMsg{Content: content, IsError: isError}
	}
}

// ReaderModel 阅读器模型
type ReaderModel struct {
	bookID          int64
	bookTitle       string
	chapterNum      int
	chapterTitle    string
	chapterContent  string
	totalChapters   int // 来源声称的总章节数
	downloadedCount int // 实际已下载的章节数
	offset          int
	totalLines      int
	lines           []string
	db              *db.DB
	width           int
	height          int
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
		m.totalChapters = book.TotalChapters
		m.downloadedCount, _ = m.db.GetChapterCount(book.ID)
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
			return chapterLoadFailedMsg{chapterNum: chapterNum, reason: "Failed to load chapter: " + err.Error()}
		}
		if ch == nil {
			logger.Warnf("[TUI/Reader] 章节不存在: bookID=%d chapterNum=%d", m.bookID, chapterNum)
			return chapterLoadFailedMsg{chapterNum: chapterNum, reason: "Chapter not downloaded yet"}
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
// 匹配模式：第X章... / 第X集... / 第X集 第X章... 等
func stripChapterHeaders(content string) string {
	// 正则匹配 "第XXX章" 或 "第XXX集" 开头的行，支持中文数字和阿拉伯数字
	// 匹配模式如：第1章、第一章、第100章、第一百章、第十一集 等
	re := regexp.MustCompile(`(?m)^[第][零一二三四五六七八九十百千万亿\d]+[章集].*$`)

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
	contentHeight := m.height - 4 // header(1) + body + footer(1) + toast(1), body少一行
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
	contentHeight := m.height - 4
	maxOffset := m.totalLines - contentHeight
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.offset < maxOffset {
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
	case chapterLoadFailedMsg:
		// 恢复章节号到加载前的值
		m.chapterNum = msg.chapterNum
		return m, showToastCmd(msg.reason, true)
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.reflow()
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			m.scrollUp()
		case "down", "j":
			if m.atEndOfLastChapter() {
				return m, showToastCmd("Already at the last chapter", false)
			}
			m.scrollDown()
		case " ", "pgdown":
			if m.atEndOfLastChapter() {
				return m, showToastCmd("Already at the last chapter", false)
			}
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
		fmt.Sprintf("%s · 第%d章 %s (%d%%)", m.bookTitle, m.chapterNum, m.chapterTitle, int(m.readingPercentage())),
	)

	visible := m.visibleLines()
	content := strings.Join(visible, "\n")
	if content == "" {
		content = "No content"
	}
	body := ReaderTextStyle.Width(m.width).Render(content)

	footer := renderFooter([]footerItem{
		{key: "↑", desc: "up"},
		{key: "↓", desc: "down"},
		{key: "space", desc: "page"},
		{key: "b", desc: "page up"},
		{key: "←", desc: "prev"},
		{key: "→", desc: "next"},
		{key: "c", desc: "chapters"},
		{key: "g", desc: "start"},
		{key: "G", desc: "end"},
		{key: "f", desc: "fill"},
		{key: "esc", desc: "back"},
		{key: "?", desc: "help"},
	}, m.width)

	// body 固定高度，header + body + footer 共占 m.height-2 行，预留2行（footer上移1行 + toast 1行）
	bodyHeight := m.height - 4
	if bodyHeight < 1 {
		bodyHeight = 1
	}
	body = lipgloss.NewStyle().Height(bodyHeight).Render(body)

	viewContent := lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
	// reader 自身只渲染 m.height-2 行，留2行给 app.go（footer上移1行 + toast 1行）
	return lipgloss.NewStyle().
		Width(m.width).Height(m.height - 2).
		Render(lipgloss.Place(m.width, m.height-2, lipgloss.Center, lipgloss.Top, viewContent))
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
func (m *ReaderModel) NextChapter(totalChapters int) tea.Cmd {
	// 以实际下载的章节数为准，避免 total_chapters 与实际情况不符
	limit := m.downloadedCount
	if limit == 0 {
		limit = totalChapters
	}
	if m.chapterNum >= limit {
		logger.Debugf("[TUI/Reader] 已经是最后一章 (已下载 %d 章)", limit)
		return showToastCmd("Already at the last chapter", false)
	}
	m.chapterNum++
	m.offset = 0
	logger.Debugf("[TUI/Reader] 下一章: %d", m.chapterNum)
	return m.loadChapterCmd(m.chapterNum)
}

// PrevChapter 上一章
func (m *ReaderModel) PrevChapter() tea.Cmd {
	if m.chapterNum <= 1 {
		logger.Debugf("[TUI/Reader] 已经是第一章")
		return showToastCmd("Already at the first chapter", false)
	}
	m.chapterNum--
	m.offset = 0
	logger.Debugf("[TUI/Reader] 上一章: %d", m.chapterNum)
	return m.loadChapterCmd(m.chapterNum)
}

// PageDown 向下翻页
func (m *ReaderModel) PageDown() {
	contentHeight := m.height - 4
	if contentHeight < 1 {
		contentHeight = 1
	}
	maxOffset := m.totalLines - contentHeight
	if maxOffset < 0 {
		maxOffset = 0
	}
	m.offset += contentHeight
	if m.offset > maxOffset {
		m.offset = maxOffset
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
	contentHeight := m.height - 4
	m.offset = m.totalLines - contentHeight
	if m.offset < 0 {
		m.offset = 0
	}
}

// atEndOfLastChapter 判断是否在最后一章的最后一页
func (m *ReaderModel) atEndOfLastChapter() bool {
	limit := m.downloadedCount
	if limit == 0 {
		limit = m.totalChapters
	}
	if m.chapterNum < limit {
		return false
	}
	contentHeight := m.height - 4
	if contentHeight < 1 {
		contentHeight = 1
	}
	maxOffset := m.totalLines - contentHeight
	if maxOffset < 0 {
		maxOffset = 0
	}
	return m.offset >= maxOffset
}

type chapterLoadedMsg struct {
	chapterNum   int
	chapterTitle string
	content      string
}

// chapterLoadFailedMsg 章节加载失败消息
// 用于在缺章时恢复正确的章节号并显示提示
type chapterLoadFailedMsg struct {
	chapterNum int
	reason     string
}
