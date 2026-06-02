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

// CrawlDialogState 爬取弹窗状态
type CrawlDialogState int

const (
	CrawlConfirm CrawlDialogState = iota
	CrawlProgressing
	CrawlFinished
	CrawlHidden // 1.5s后隐藏进度条
)

// CrawlModel 爬取弹窗模型
type CrawlModel struct {
	state      CrawlDialogState
	bookTitle  string
	sourceName string
	sourceURL  string
	progress   float64
	currentCh  int
	totalCh    int
	chTitle    string
	db         *db.DB
	engine     *crawler.Engine
	bookID     int64
	err        error
	// 用于在 Update 和 Cmd 之间共享进度通道
	progressCh chan crawler.CrawlProgress
	done       bool
	width      int
	height     int
	// bubbles 组件
	progressBar progress.Model
	stopwatch   stopwatch.Model
	// 取消爬取
	cancelFunc context.CancelFunc
}

// NewCrawlModel 创建爬取弹窗模型
func NewCrawlModel(database *db.DB, engine *crawler.Engine) *CrawlModel {
	pb := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(120),
	)
	pb.ShowPercentage = false

	return &CrawlModel{
		db:          database,
		engine:      engine,
		state:       CrawlConfirm,
		progressBar: pb,
		stopwatch:   stopwatch.NewWithInterval(time.Second),
	}
}

// Start 设置爬取任务信息
func (m *CrawlModel) Start(bookTitle, sourceName, sourceURL string) {
	logger.Debugf("[TUI/Crawl] 设置爬取任务: %s [%s]", bookTitle, sourceName)
	m.bookTitle = bookTitle
	m.sourceName = sourceName
	m.sourceURL = sourceURL
	m.state = CrawlConfirm
	m.progress = 0
	m.currentCh = 0
	m.totalCh = 0
	m.chTitle = ""
	m.err = nil
	m.progressCh = nil
	m.done = false
	m.stopwatch = stopwatch.NewWithInterval(time.Second)
}

// crawlCmd 启动爬取
func (m *CrawlModel) crawlCmd() tea.Cmd {
	logger.Infof("[TUI/Crawl] 启动爬取: %s [%s]", m.bookTitle, m.sourceName)
	// 创建进度通道
	m.progressCh = make(chan crawler.CrawlProgress, 100)

	// 创建可取消的 context
	ctx, cancel := context.WithCancel(context.Background())
	m.cancelFunc = cancel

	// 启动爬取 goroutine
	go func() {
		defer close(m.progressCh)
		bookID, err := m.engine.CrawlBook(ctx, m.sourceName, m.sourceURL, m.progressCh)
		m.bookID = bookID
		if err != nil {
			logger.Errorf("[TUI/Crawl] 爬取失败: %v", err)
			select {
			case m.progressCh <- crawler.CrawlProgress{Done: true, Error: err}:
			case <-ctx.Done():
			}
		}
	}()

	// 启动秒表
	return tea.Batch(m.stopwatch.Start(), m.listenProgressCmd())
}

// Cancel 取消爬取
func (m *CrawlModel) Cancel() {
	if m.cancelFunc != nil {
		logger.Infof("[TUI/Crawl] 取消爬取")
		m.cancelFunc()
		m.cancelFunc = nil
	}
}

// listenProgressCmd 监听进度通道的 Cmd
func (m *CrawlModel) listenProgressCmd() tea.Cmd {
	return func() tea.Msg {
		// 等待进度或超时（用于刷新 UI）
		select {
		case p, ok := <-m.progressCh:
			if !ok {
				// 通道关闭，爬取完成
				m.done = true
				return crawlDoneMsg{}
			}
			if p.Done || p.Error != nil {
				m.done = true
				return crawlDoneMsg{Progress: p}
			}
			return crawlProgressMsg{
				Progress:  p.Percentage,
				CurrentCh: p.CurrentChapter,
				TotalCh:   p.TotalChapters,
				ChTitle:   p.ChapterTitle,
			}
		case <-time.After(200 * time.Millisecond):
			// 超时，返回 tick 让 UI 刷新
			return crawlTickMsg{}
		}
	}
}

// Init 初始化
func (m CrawlModel) Init() tea.Cmd {
	return nil
}

// Update 更新
func (m CrawlModel) Update(msg tea.Msg) (CrawlModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.progressBar.Width = 20
	case crawlProgressMsg:
		m.progress = msg.Progress
		m.currentCh = msg.CurrentCh
		m.totalCh = msg.TotalCh
		m.chTitle = msg.ChTitle
		m.state = CrawlProgressing
		// 更新 progress bar
		cmds = append(cmds, m.progressBar.SetPercent(msg.Progress/100))
		// 继续监听下一个进度
		cmds = append(cmds, m.listenProgressCmd())
	case crawlTickMsg:
		// UI 刷新 tick，如果还没完成则继续监听
		if !m.done {
			cmds = append(cmds, m.listenProgressCmd())
		}
	case crawlDoneMsg:
		m.state = CrawlFinished
		m.done = true
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
		}
		// 停止秒表
		swCmd := m.stopwatch.Stop()
		cmds = append(cmds, swCmd)
		// 1.5秒后隐藏进度条，并触发强制刷新
		cmds = append(cmds, func() tea.Msg {
			time.Sleep(1500 * time.Millisecond)
			return crawlHideMsg{}
		})
		cmds = append(cmds, func() tea.Msg {
			time.Sleep(1500 * time.Millisecond)
			return forceRefreshMsg{}
		})
		return m, tea.Batch(cmds...)
	case crawlHideMsg:
		m.state = CrawlHidden
	case forceRefreshMsg:
		// 强制刷新，触发重新渲染
	}

	// 更新 bubbles 组件
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

// View 渲染爬取弹窗
func (m CrawlModel) View() string {
	switch m.state {
	case CrawlConfirm:
		content := lipgloss.JoinVertical(
			lipgloss.Left,
			TitleStyle.Render("Confirm Download"),
			"",
			fmt.Sprintf("Title: %s", m.bookTitle),
			fmt.Sprintf("Source: %s", m.sourceName),
			"",
			renderFooter([]footerItem{
				{key: "enter", desc: "start"},
				{key: "b", desc: "background"},
				{key: "esc", desc: "cancel"},
			}, 48),
		)
		return lipgloss.NewStyle().
			Width(m.width).Height(m.height).
			Render(lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Top, DialogBoxStyle.Width(50).Render(content)))
	case CrawlProgressing:
		bar := m.progressBar.ViewAs(m.progress / 100)
		info := fmt.Sprintf("%d/%d %s", m.currentCh, m.totalCh, m.chTitle)
		content := lipgloss.JoinVertical(
			lipgloss.Left,
			TitleStyle.Render("Downloading"),
			"",
			bar,
			fmt.Sprintf("%.1f%% %s", m.progress, info),
			"",
			renderFooter([]footerItem{
				{key: "esc", desc: "cancel"},
			}, 48),
		)
		return lipgloss.NewStyle().
			Width(m.width).Height(m.height).
			Render(lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Top, DialogBoxStyle.Width(50).Render(content)))
	case CrawlFinished:
		status := "Done"
		style := ToastSuccessStyle
		if m.err != nil {
			status = "Failed: " + m.err.Error()
			style = ToastErrorStyle
		}
		content := lipgloss.JoinVertical(
			lipgloss.Left,
			TitleStyle.Render("Download Result"),
			"",
			style.Render(status),
			"",
			renderFooter([]footerItem{
				{key: "enter/esc", desc: "close"},
			}, 48),
		)
		return lipgloss.NewStyle().
			Width(m.width).Height(m.height).
			Render(lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Top, DialogBoxStyle.Width(50).Render(content)))
	}
	return ""
}

// MiniView 返回后台下载的迷你进度条（显示在 footer 下方）
func (m CrawlModel) MiniView() string {
	if m.state != CrawlProgressing && m.state != CrawlFinished {
		return ""
	}
	// 如果已经完成超过1.5秒（状态变为CrawlHidden），不显示
	if m.state == CrawlHidden {
		return ""
	}

	bar := m.progressBar.ViewAs(m.progress / 100)
	elapsed := m.stopwatch.View()

	var status string
	if m.state == CrawlFinished {
		if m.err != nil {
			status = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorError)).Render("Failed")
		} else {
			status = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccess)).Render("Done")
		}
	} else {
		status = fmt.Sprintf("%.0f%%", m.progress)
	}

	// 缩进显示，进度条更长更细
	indent := " "
	line := lipgloss.JoinHorizontal(
		lipgloss.Top,
		bar,
		" ",
		lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSubtext)).Render(fmt.Sprintf("%s · %s · %d/%d · x:stop", status, elapsed, m.currentCh, m.totalCh)),
	)

	return indent + line
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

type crawlProgressMsg struct {
	Progress  float64
	CurrentCh int
	TotalCh   int
	ChTitle   string
}

type crawlDoneMsg struct {
	Progress crawler.CrawlProgress
}

type crawlTickMsg struct{}

type crawlHideMsg struct{}

type forceRefreshMsg struct{}

// Wait 等待爬取 goroutine 结束
func (m *CrawlModel) Wait() {
	logger.Debugf("[TUI/Crawl] 等待爬取 goroutine 结束")
	// 先取消爬取，避免无限等待
	m.Cancel()
	// 通过持续监听进度通道直到关闭来等待 goroutine 结束
	if m.progressCh != nil {
		for range m.progressCh {
		}
	}
	logger.Debugf("[TUI/Crawl] 爬取 goroutine 已结束")
}
