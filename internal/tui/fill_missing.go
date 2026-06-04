package tui

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/stopwatch"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/henry/novel-reader/internal/crawler"
	"github.com/henry/novel-reader/internal/db"
	"github.com/henry/novel-reader/pkg/logger"
)

// FillMissingState 补充缺章状态
type FillMissingState int

const (
	FillMissingIdle FillMissingState = iota
	FillMissingConfirm
	FillMissingProgressing
	FillMissingFinished
)

// FillMissingModel 补充缺章模型
type FillMissingModel struct {
	state       FillMissingState
	book        *db.Book
	progress    float64
	currentCh   int
	totalCh     int
	chTitle     string
	logs        []crawler.LogMessage
	db          *db.DB
	engine      *crawler.Engine
	result      *crawler.FillMissingResult
	err         error
	progressCh  chan crawler.FillMissingProgress
	done        bool
	width       int
	height      int
	progressBar progress.Model
	stopwatch   stopwatch.Model
	cancelFunc  context.CancelFunc
}

// NewFillMissingModel 创建补充缺章模型
func NewFillMissingModel(database *db.DB, engine *crawler.Engine) *FillMissingModel {
	pb := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(120),
	)
	pb.ShowPercentage = false

	return &FillMissingModel{
		db:          database,
		engine:      engine,
		state:       FillMissingIdle,
		progressBar: pb,
		stopwatch:   stopwatch.NewWithInterval(time.Second),
	}
}

// Start 设置补充缺章任务（书架整书补充）
func (m *FillMissingModel) Start(book *db.Book) {
	logger.Debugf("[TUI/FillMissing] 设置补充任务: %s", book.Title)
	m.book = book
	m.state = FillMissingConfirm
	m.progress = 0
	m.currentCh = 0
	m.totalCh = 0
	m.chTitle = ""
	m.logs = nil
	m.result = nil
	m.err = nil
	m.progressCh = nil
	m.done = false
	m.stopwatch = stopwatch.NewWithInterval(time.Second)
}

// StartSingle 设置单章补充任务（阅读器触发）
func (m *FillMissingModel) StartSingle(bookID int64, chapterNum int) {
	logger.Debugf("[TUI/FillMissing] 设置单章补充任务: bookID=%d, chapter=%d", bookID, chapterNum)
	m.book = &db.Book{ID: bookID}
	m.state = FillMissingProgressing
	m.progress = 0
	m.currentCh = 0
	m.totalCh = 0
	m.chTitle = ""
	m.logs = nil
	m.result = nil
	m.err = nil
	m.progressCh = nil
	m.done = false
	m.stopwatch = stopwatch.NewWithInterval(time.Second)
}

// fillSingleMissingCmd 启动单章补充（异步，带进度）
func (m *FillMissingModel) fillSingleMissingCmd(bookID int64, chapterNum int) tea.Cmd {
	logger.Infof("[TUI/FillMissing] 启动单章补充: bookID=%d, chapter=%d", bookID, chapterNum)
	m.progressCh = make(chan crawler.FillMissingProgress, 100)

	ctx, cancel := context.WithCancel(context.Background())
	m.cancelFunc = cancel

	go func() {
		defer close(m.progressCh)
		result, err := m.engine.FillSingleMissingChapter(ctx, bookID, chapterNum, m.db, m.progressCh)
		if err != nil {
			logger.Errorf("[TUI/FillMissing] 单章补充失败: %v", err)
			select {
			case m.progressCh <- crawler.FillMissingProgress{Done: true, Error: err}:
			case <-ctx.Done():
			}
		} else {
			select {
			case m.progressCh <- crawler.FillMissingProgress{Done: true, Result: result}:
			case <-ctx.Done():
			}
		}
	}()

	return tea.Batch(m.stopwatch.Start(), m.listenProgressCmd())
}

// fillMissingCmd 启动补充缺章
func (m *FillMissingModel) fillMissingCmd() tea.Cmd {
	if m.book == nil {
		return nil
	}
	logger.Infof("[TUI/FillMissing] 启动补充缺章: %s", m.book.Title)
	m.progressCh = make(chan crawler.FillMissingProgress, 100)

	ctx, cancel := context.WithCancel(context.Background())
	m.cancelFunc = cancel

	go func() {
		defer close(m.progressCh)
		result, err := m.engine.FillMissingChapters(ctx, m.book.ID, m.db, m.progressCh)
		if err != nil {
			logger.Errorf("[TUI/FillMissing] 补充缺章失败: %v", err)
			select {
			case m.progressCh <- crawler.FillMissingProgress{Done: true, Error: err}:
			case <-ctx.Done():
			}
		} else {
			select {
			case m.progressCh <- crawler.FillMissingProgress{Done: true, Result: result}:
			case <-ctx.Done():
			}
		}
	}()

	return tea.Batch(m.stopwatch.Start(), m.listenProgressCmd())
}

// Cancel 取消补充
func (m *FillMissingModel) Cancel() {
	if m.cancelFunc != nil {
		logger.Infof("[TUI/FillMissing] 取消补充缺章")
		m.cancelFunc()
		m.cancelFunc = nil
	}
}

// listenProgressCmd 监听进度通道
func (m *FillMissingModel) listenProgressCmd() tea.Cmd {
	return func() tea.Msg {
		select {
		case p, ok := <-m.progressCh:
			if !ok {
				m.done = true
				return fillMissingDoneMsg{}
			}
			if p.Done || p.Error != nil {
				m.done = true
				return fillMissingDoneMsg{Progress: p}
			}
			return fillMissingProgressMsg{
				Progress:    p.Percentage,
				CurrentCh:   p.CurrentChapter,
				TotalCh:     p.TotalChapters,
				ChTitle:     p.ChapterTitle,
				LogMessages: p.LogMessages,
			}
		case <-time.After(200 * time.Millisecond):
			return fillMissingTickMsg{}
		}
	}
}

// Init 初始化
func (m FillMissingModel) Init() tea.Cmd {
	return nil
}

// Update 更新
func (m FillMissingModel) Update(msg tea.Msg) (FillMissingModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.progressBar.Width = 20
	case fillMissingProgressMsg:
		m.progress = msg.Progress
		m.currentCh = msg.CurrentCh
		m.totalCh = msg.TotalCh
		m.chTitle = msg.ChTitle
		m.state = FillMissingProgressing
		if len(msg.LogMessages) > 0 {
			m.logs = msg.LogMessages
		}
		cmds = append(cmds, m.progressBar.SetPercent(msg.Progress/100))
		cmds = append(cmds, m.listenProgressCmd())
	case fillMissingTickMsg:
		if !m.done {
			cmds = append(cmds, m.listenProgressCmd())
		}
	case fillMissingDoneMsg:
		m.state = FillMissingFinished
		m.done = true
		if len(msg.Progress.LogMessages) > 0 {
			m.logs = msg.Progress.LogMessages
		}
		if msg.Progress.Error != nil {
			m.err = msg.Progress.Error
			m.progress = msg.Progress.Percentage
			m.currentCh = msg.Progress.CurrentChapter
			m.totalCh = msg.Progress.TotalChapters
		} else {
			m.progress = 100
			if m.totalCh == 0 {
				m.totalCh = m.currentCh
			}
			m.result = msg.Progress.Result
		}
		swCmd := m.stopwatch.Stop()
		cmds = append(cmds, swCmd)
		cmds = append(cmds, func() tea.Msg {
			time.Sleep(1500 * time.Millisecond)
			return fillMissingHideMsg{}
		})
		cmds = append(cmds, func() tea.Msg {
			time.Sleep(1500 * time.Millisecond)
			return forceRefreshMsg{}
		})
		return m, tea.Batch(cmds...)
	case fillMissingHideMsg:
		if m.state == FillMissingFinished {
			m.state = FillMissingIdle
		}
		m.book = nil
		m.result = nil
		m.err = nil
		m.progress = 0
		m.currentCh = 0
		m.totalCh = 0
		m.chTitle = ""
		m.logs = nil
		m.done = false
	}

	var swCmd tea.Cmd
	m.stopwatch, swCmd = m.stopwatch.Update(msg)
	if swCmd != nil {
		cmds = append(cmds, swCmd)
	}

	pbModel, pbCmdTemp := m.progressBar.Update(msg)
	if mdl, ok := pbModel.(progress.Model); ok {
		m.progressBar = mdl
	}
	if pbCmdTemp != nil {
		cmds = append(cmds, pbCmdTemp)
	}

	return m, tea.Batch(cmds...)
}

// View 渲染补充缺章弹窗
func (m FillMissingModel) View() string {
	switch m.state {
	case FillMissingConfirm:
		bookName := ""
		missingCount := 0
		if m.book != nil {
			bookName = m.book.Title
			missing, _ := m.engine.FindMissingChapters(m.book.ID, m.db)
			missingCount = len(missing)
		}
		content := lipgloss.JoinVertical(
			lipgloss.Left,
			TitleStyle.Render("Fill Missing Chapters"),
			"",
			fmt.Sprintf("Book: %s", bookName),
			fmt.Sprintf("Missing chapters: %d", missingCount),
			"",
			"This will search alternative sources and try to fill missing chapters.",
			"The previous chapter's title will be used to locate the correct position.",
			"",
			renderFooter([]footerItem{
				{key: "enter", desc: "start"},
				{key: "esc", desc: "cancel"},
			}, 56),
		)
		return lipgloss.NewStyle().
			Width(m.width).Height(m.height).
			Render(lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Top, DialogBoxStyle.Width(60).Render(content)))

	case FillMissingProgressing:
		bar := m.progressBar.ViewAs(m.progress / 100)
		info := fmt.Sprintf("%d/%d %s", m.currentCh, m.totalCh, m.chTitle)
		logView := m.renderLogs()
		content := lipgloss.JoinVertical(
			lipgloss.Left,
			TitleStyle.Render("Filling Missing Chapter"),
			"",
			bar,
			fmt.Sprintf("%.1f%% %s", m.progress, info),
			"",
			logView,
			"",
			renderFooter([]footerItem{
				{key: "esc", desc: "cancel"},
			}, 48),
		)
		return lipgloss.NewStyle().
			Width(m.width).Height(m.height).
			Render(lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Top, DialogBoxStyle.Width(60).Render(content)))

	case FillMissingFinished:
		status := "Done"
		style := ToastSuccessStyle
		if m.err != nil {
			status = "Failed: " + m.err.Error()
			style = ToastErrorStyle
		} else if m.result != nil {
			status = fmt.Sprintf("Filled %d, Failed %d, Skipped %d (from %s)",
				m.result.FilledCount, m.result.FailedCount, m.result.SkippedCount, m.result.SourceUsed)
			if m.result.FilledCount > 0 {
				style = ToastSuccessStyle
			} else {
				style = ToastErrorStyle
			}
		}
		logView := m.renderLogs()
		content := lipgloss.JoinVertical(
			lipgloss.Left,
			TitleStyle.Render("Fill Result"),
			"",
			style.Render(status),
			"",
			logView,
			"",
			renderFooter([]footerItem{
				{key: "enter", desc: "close"},
				{key: "esc", desc: "close"},
			}, 48),
		)
		return lipgloss.NewStyle().
			Width(m.width).Height(m.height).
			Render(lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Top, DialogBoxStyle.Width(60).Render(content)))
	}

	return ""
}

// renderLogs 渲染固定高度日志区域（旧日志被新日志顶上去，UI不变形）
func (m FillMissingModel) renderLogs() string {
	const maxLines = 6
	const lineHeight = 1

	var lines []string
	start := 0
	if len(m.logs) > maxLines {
		start = len(m.logs) - maxLines
	}

	for i := start; i < len(m.logs); i++ {
		log := m.logs[i]
		var prefix string
		var color string
		switch log.Level {
		case crawler.LogSuccess:
			prefix = "✓"
			color = ColorSuccess
		case crawler.LogError:
			prefix = "✗"
			color = ColorError
		case crawler.LogWarning:
			prefix = "!"
			color = ColorHighlight
		default:
			prefix = "•"
			color = ColorSubtext
		}

		msg := log.Message
		if log.URL != "" {
			msg = msg + " " + log.URL
		}
		// 截断过长内容，保证每行固定宽度
		maxWidth := 56
		runes := []rune(msg)
		if len(runes) > maxWidth {
			msg = string(runes[:maxWidth-3]) + "..."
		}

		line := lipgloss.NewStyle().
			Foreground(lipgloss.Color(color)).
			Height(lineHeight).
			Render(fmt.Sprintf("%s %s", prefix, msg))
		lines = append(lines, line)
	}

	// 补空行保持固定高度
	for len(lines) < maxLines {
		lines = append(lines, lipgloss.NewStyle().Height(lineHeight).Render(""))
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// MiniView 返回迷你进度条
func (m FillMissingModel) MiniView() string {
	if m.state != FillMissingProgressing && m.state != FillMissingFinished {
		return ""
	}
	if m.state == FillMissingIdle {
		return ""
	}

	bar := m.progressBar.ViewAs(m.progress / 100)
	elapsed := m.stopwatch.View()

	var status string
	if m.state == FillMissingFinished {
		if m.err != nil {
			status = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorError)).Render("Failed")
		} else {
			status = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccess)).Render("Done")
		}
	} else {
		status = fmt.Sprintf("%.0f%%", m.progress)
	}

	indent := " "
	line := lipgloss.JoinHorizontal(
		lipgloss.Top,
		bar,
		" ",
		lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSubtext)).Render(fmt.Sprintf("%s · %s · %d/%d", status, elapsed, m.currentCh, m.totalCh)),
	)

	return indent + line
}

// IsActive 是否处于活动状态
func (m FillMissingModel) IsActive() bool {
	return m.state == FillMissingConfirm || m.state == FillMissingProgressing || m.state == FillMissingFinished
}

// Wait 等待任务结束
func (m *FillMissingModel) Wait() {
	logger.Debugf("[TUI/FillMissing] 等待任务结束")
	m.Cancel()
	if m.progressCh != nil {
		for range m.progressCh {
		}
	}
	logger.Debugf("[TUI/FillMissing] 任务已结束")
}

// 消息类型
type fillMissingProgressMsg struct {
	Progress    float64
	CurrentCh   int
	TotalCh     int
	ChTitle     string
	LogMessages []crawler.LogMessage
}

type fillMissingDoneMsg struct {
	Progress crawler.FillMissingProgress
}

type fillMissingTickMsg struct{}
type fillMissingHideMsg struct{}

// StartFillMissingMsg 开始补充缺章消息
type StartFillMissingMsg struct {
	Book *db.Book
}

// StartFillSingleMissingMsg 开始补充单章消息
type StartFillSingleMissingMsg struct {
	BookID     int64
	ChapterNum int
}
