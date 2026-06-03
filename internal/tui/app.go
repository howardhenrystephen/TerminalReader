package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/henry/novel-reader/internal/crawler"
	"github.com/henry/novel-reader/internal/db"
	"github.com/henry/novel-reader/pkg/logger"
)

// ViewState 应用视图状态
type ViewState int

const (
	StateBookshelf ViewState = iota
	StateReader
	StateChapterPicker
	StateSearch
	StateConfirmCrawl
	StateCrawling
	StateBackgroundCrawling
	StateConfirmDelete
	StateHelp
	StateBookDesc
	StateConfirmNuke
)

// AppModel 应用主模型
type AppModel struct {
	state         ViewState
	prevState     ViewState
	db            *db.DB
	engine        *crawler.Engine
	bookshelf     BookshelfModel
	reader        *ReaderModel
	search        SearchModel
	chapterPicker ChapterPickerModel
	crawl         CrawlModel
	help          HelpModel
	toast         Toast
	toastTimer    *time.Timer
	width         int
	height        int
}

// NewApp 创建应用模型
func NewApp(database *db.DB, engine *crawler.Engine) AppModel {
	logger.Debugf("[TUI] NewApp: 初始化应用模型")
	return AppModel{
		db:            database,
		engine:        engine,
		bookshelf:     NewBookshelfModel(database),
		reader:        NewReaderModel(database),
		search:        NewSearchModel(engine),
		chapterPicker: NewChapterPickerModel(database),
		crawl:         *NewCrawlModel(database, engine),
		help:          NewHelpModel(),
	}
}

// Init 初始化
func (m AppModel) Init() tea.Cmd {
	return tea.Batch(
		m.bookshelf.Init(),
		m.search.Init(),
	)
}

// Update 更新状态机
func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.bookshelf.SetSize(msg.Width, msg.Height)
		m.reader.SetSize(msg.Width, msg.Height)
		m.search.SetSize(msg.Width, msg.Height)
		m.chapterPicker.SetSize(msg.Width, msg.Height)
		crawlModel, _ := m.crawl.Update(msg)
		m.crawl = crawlModel
		helpModel, _ := m.help.Update(msg)
		m.help = helpModel
		return m, nil

	case tea.KeyMsg:
		// 章节选择器打开时，优先处理它的按键
		if m.chapterPicker.IsActive() {
			return m.handleChapterPickerKeys(msg)
		}
		switch m.state {
		case StateBookshelf:
			return m.handleBookshelfKeys(msg)
		case StateReader:
			return m.handleReaderKeys(msg)
		case StateSearch:
			return m.handleSearchKeys(msg)
		case StateConfirmCrawl:
			return m.handleConfirmCrawlKeys(msg)
		case StateCrawling:
			if keyMatches(msg, keyWithKeys("esc", "enter")) {
				m.state = StateBookshelf
				return m, nil
			}
			if keyMatches(msg, GlobalKeys.Quit) {
				return m, tea.Quit
			}
			return m, nil
		case StateBackgroundCrawling:
			if keyMatches(msg, BookshelfKeys.StopCrawl) {
				logger.Infof("[TUI] 停止后台爬取")
				m.crawl.Cancel()
				m.state = StateBookshelf
				// 先发送 toast 给 bookshelf，再切换状态
				bookshelf, _ := m.bookshelf.Update(ShowToastMsg{Content: "Download stopped", IsError: false})
				m.bookshelf = bookshelf
				return m, m.bookshelf.LoadBooks()
			}
			if keyMatches(msg, GlobalKeys.Quit) {
				m.crawl.Cancel()
				return m, tea.Quit
			}
			if keyMatches(msg, GlobalKeys.Help) {
				return m, func() tea.Msg { return ShowHelpMsg{} }
			}
			if keyMatches(msg, BookshelfKeys.Search) {
				return m, func() tea.Msg { return OpenSearchMsg{} }
			}
			if keyMatches(msg, BookshelfKeys.Enter) {
				if book := m.bookshelf.SelectedBook(); book != nil {
					return m, func() tea.Msg { return OpenBookMsg{BookID: book.ID} }
				}
			}
			if keyMatches(msg, BookshelfKeys.Delete) {
				if m.bookshelf.SelectedBook() != nil {
					m.state = StateConfirmDelete
				}
				return m, nil
			}
			if keyMatches(msg, BookshelfKeys.Refresh) {
				return m, m.bookshelf.LoadBooks()
			}
			if keyMatches(msg, BookshelfKeys.Redraw) {
				logger.Debugf("[TUI] 强制刷新界面：清除 toast 和进度条")
				m.bookshelf.toast = ""
				m.bookshelf.toastIsError = false
				m.crawl.state = CrawlHidden
				return m, nil
			}
			bookshelf, bookshelfCmd := m.bookshelf.Update(msg)
			m.bookshelf = bookshelf
			crawl, crawlCmd := m.crawl.Update(msg)
			m.crawl = crawl
			return m, tea.Batch(bookshelfCmd, crawlCmd)
		case StateConfirmDelete:
			return m.handleConfirmDeleteKeys(msg)
		case StateHelp:
			if keyMatches(msg, GlobalKeys.Help) || keyMatches(msg, keyWithKeys("esc", "q")) {
				return m, func() tea.Msg { return CloseHelpMsg{} }
			}
			// 其他按键（上下滚动等）透传给 help 组件
			help, cmd := m.help.Update(msg)
			m.help = help
			return m, cmd
		case StateBookDesc:
			return m.handleBookDescKeys(msg)
		case StateConfirmNuke:
			return m.handleConfirmNukeKeys(msg)
		}

	case OpenBookMsg:
		logger.Infof("[TUI] 打开书籍 ID=%d", msg.BookID)
		m.state = StateReader
		cmd := m.reader.LoadBook(msg.BookID)
		return m, cmd

	case CloseReaderMsg:
		logger.Infof("[TUI] 关闭阅读器，保存进度")
		if err := m.reader.SaveProgress(); err != nil {
			logger.Warnf("[TUI] 保存进度失败: %v", err)
			cmds = append(cmds, showToast("Failed to save progress: "+err.Error(), true))
		}
		m.state = StateBookshelf
		cmds = append(cmds, m.bookshelf.LoadBooks())
		return m, tea.Batch(cmds...)

	case OpenSearchMsg:
		logger.Infof("[TUI] 打开搜索")
		m.prevState = m.state
		m.state = StateSearch
		m.search.Reset()
		m.search.Focus()
		return m, nil

	case CloseSearchMsg:
		logger.Debugf("[TUI] 关闭搜索")
		m.state = m.prevState
		if m.state == StateSearch {
			m.state = StateBookshelf
		}
		return m, nil

	case StartCrawlMsg:
		logger.Infof("[TUI] 开始爬取确认: title=%s source=%s", msg.BookTitle, msg.SourceName)
		m.prevState = m.state
		m.state = StateConfirmCrawl
		m.crawl.Start(msg.BookTitle, msg.SourceName, msg.SourceURL)
		return m, nil

	case crawlProgressMsg:
		logger.Debugf("[TUI] 爬取进度: %.1f%% %d/%d %s", msg.Progress, msg.CurrentCh, msg.TotalCh, msg.ChTitle)
		crawlModel, cmd := m.crawl.Update(msg)
		m.crawl = crawlModel
		return m, cmd

	case crawlDoneMsg:
		crawlModel, cmd := m.crawl.Update(msg)
		m.crawl = crawlModel
		wasBackground := m.state == StateBackgroundCrawling
		var cmds []tea.Cmd
		if msg.Progress.Error != nil {
			logger.Errorf("[TUI] 爬取失败: %v", msg.Progress.Error)
			if wasBackground {
				bookshelf, _ := m.bookshelf.Update(ShowToastMsg{Content: "Download failed: " + msg.Progress.Error.Error(), IsError: true})
				m.bookshelf = bookshelf
				m.state = StateBookshelf
				cmds = append(cmds, m.bookshelf.LoadBooks())
			} else {
				m.state = StateBookshelf
				bookshelf, _ := m.bookshelf.Update(ShowToastMsg{Content: "Download failed: " + msg.Progress.Error.Error(), IsError: true})
				m.bookshelf = bookshelf
				cmds = append(cmds, m.bookshelf.LoadBooks())
			}
		} else {
			logger.Infof("[TUI] 爬取完成")
			if wasBackground {
				bookshelf, _ := m.bookshelf.Update(ShowToastMsg{Content: "Download complete", IsError: false})
				m.bookshelf = bookshelf
				m.state = StateBookshelf
				cmds = append(cmds, m.bookshelf.LoadBooks())
			} else {
				m.state = StateBookshelf
				bookshelf, _ := m.bookshelf.Update(ShowToastMsg{Content: "Download complete", IsError: false})
				m.bookshelf = bookshelf
				cmds = append(cmds, m.bookshelf.LoadBooks())
			}
		}
		return m, tea.Batch(append(cmds, cmd)...)

	case ShowHelpMsg:
		logger.Debugf("[TUI] 显示帮助")
		m.prevState = m.state
		m.state = StateHelp
		return m, nil

	case CloseHelpMsg:
		logger.Debugf("[TUI] 关闭帮助")
		m.state = m.prevState
		if m.state == StateHelp {
			m.state = StateBookshelf
		}
		return m, nil

	case ShowBookDescMsg:
		logger.Debugf("[TUI] 显示书籍简介")
		m.prevState = m.state
		m.state = StateBookDesc
		m.bookshelf.updateDescContent()
		return m, nil

	case ContinueCrawlMsg:
		logger.Infof("[TUI] 继续下载: bookID=%d, source=%s", msg.BookID, msg.SourceName)
		m.state = StateBackgroundCrawling
		m.crawl.Start(msg.BookTitle, msg.SourceName, msg.SourceURL)
		return m, m.crawl.crawlCmd()

	case CloseBookDescMsg:
		logger.Debugf("[TUI] 关闭书籍简介")
		m.state = m.prevState
		if m.state == StateBookDesc {
			m.state = StateBookshelf
		}
		return m, nil

	case ShowToastMsg:
		if msg.IsError {
			logger.Warnf("[TUI] Toast错误: %s", msg.Content)
		} else {
			logger.Debugf("[TUI] Toast: %s", msg.Content)
		}
		// 书架状态下，toast 由 bookshelf 自己渲染，不通过 AppModel 的 toast
		if m.state == StateBookshelf || m.state == StateBackgroundCrawling {
			bookshelf, _ := m.bookshelf.Update(msg)
			m.bookshelf = bookshelf
			return m, nil
		}
		m.toast = NewToast(msg.Content, msg.IsError)
		if m.toastTimer != nil {
			m.toastTimer.Stop()
		}
		m.toastTimer = time.AfterFunc(3*time.Second, func() {
			m.toast.Clear()
		})
		return m, nil
	}

	// 透传给子组件
	// 后台爬取任务可能在任何状态下运行，crawl 组件始终需要接收消息
	var crawlCmd tea.Cmd
	crawl, crawlCmd := m.crawl.Update(msg)
	m.crawl = crawl

	switch m.state {
	case StateBookshelf:
		bookshelf, cmd := m.bookshelf.Update(msg)
		m.bookshelf = bookshelf
		return m, tea.Batch(cmd, crawlCmd)
	case StateReader:
		reader, cmd := m.reader.Update(msg)
		m.reader = &reader
		return m, tea.Batch(cmd, crawlCmd)
	case StateChapterPicker:
		picker, cmd := m.chapterPicker.Update(msg)
		m.chapterPicker = picker
		return m, tea.Batch(cmd, crawlCmd)
	case StateSearch:
		search, cmd := m.search.Update(msg)
		m.search = search
		return m, tea.Batch(cmd, crawlCmd)
	case StateConfirmCrawl, StateCrawling:
		return m, crawlCmd
	case StateBackgroundCrawling:
		bookshelf, cmd := m.bookshelf.Update(msg)
		m.bookshelf = bookshelf
		return m, tea.Batch(cmd, crawlCmd)
	case StateHelp:
		help, cmd := m.help.Update(msg)
		m.help = help
		return m, tea.Batch(cmd, crawlCmd)
	case StateBookDesc:
		bookshelf, cmd := m.bookshelf.Update(msg)
		m.bookshelf = bookshelf
		return m, tea.Batch(cmd, crawlCmd)
	}

	return m, crawlCmd
}

// View 渲染主视图
func (m AppModel) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	var content string
	switch m.state {
	case StateBookshelf:
		// 书架页：如果有后台下载，在 help 下方显示迷你进度
		miniView := ""
		if m.crawl.state == CrawlProgressing || m.crawl.state == CrawlFinished {
			miniView = m.crawl.MiniView()
		}
		content = m.bookshelf.ViewWithMini(miniView)
	case StateReader:
		content = m.reader.View()
	case StateChapterPicker:
		content = m.chapterPicker.View()
	case StateSearch:
		content = m.search.View()
	case StateConfirmCrawl, StateCrawling:
		content = m.crawl.View()
	case StateBackgroundCrawling:
		// 后台下载状态：显示书架 + help 下方的迷你进度
		miniView := m.crawl.MiniView()
		content = m.bookshelf.ViewWithMini(miniView)
	case StateConfirmDelete:
		content = m.viewConfirmDelete()
	case StateHelp:
		content = m.help.View()
	case StateBookDesc:
		content = m.bookshelf.ViewDescFull(m.width, m.height)
	case StateConfirmNuke:
		content = m.viewConfirmNuke()
	}

	// 叠加 Toast（非书架状态下）
	if m.toast.Visible && m.state != StateBookshelf && m.state != StateBackgroundCrawling {
		toastView := m.toast.View()
		content = lipgloss.JoinVertical(lipgloss.Left, content, toastView)
	}

	return content
}

func (m AppModel) handleBookshelfKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if keyMatches(msg, GlobalKeys.Quit) {
		logger.Infof("[TUI] 用户退出程序")
		if m.crawl.state == CrawlProgressing {
			logger.Infof("[TUI] 取消后台爬取")
			m.crawl.Cancel()
		}
		return m, tea.Quit
	}
	if keyMatches(msg, GlobalKeys.Help) {
		return m, func() tea.Msg { return ShowHelpMsg{} }
	}
	if keyMatches(msg, BookshelfKeys.Search) {
		return m, func() tea.Msg { return OpenSearchMsg{} }
	}
	if keyMatches(msg, BookshelfKeys.Enter) {
		if book := m.bookshelf.SelectedBook(); book != nil {
			return m, func() tea.Msg { return OpenBookMsg{BookID: book.ID} }
		}
	}
	if keyMatches(msg, BookshelfKeys.Delete) {
		if book := m.bookshelf.SelectedBook(); book != nil {
			logger.Infof("[TUI] 确认删除书籍: %s", book.Title)
			m.state = StateConfirmDelete
		}
		return m, nil
	}
	if keyMatches(msg, BookshelfKeys.Nuke) {
		logger.Infof("[TUI] 用户请求清除所有数据")
		m.state = StateConfirmNuke
		return m, nil
	}
	if keyMatches(msg, BookshelfKeys.Refresh) {
		logger.Debugf("[TUI] 刷新书架")
		return m, m.bookshelf.LoadBooks()
	}
	if keyMatches(msg, BookshelfKeys.Desc) {
		if m.bookshelf.SelectedBook() != nil {
			logger.Debugf("[TUI] 显示书籍简介")
			return m, func() tea.Msg { return ShowBookDescMsg{} }
		}
		return m, nil
	}
	if keyMatches(msg, BookshelfKeys.Pin) {
		logger.Debugf("[TUI] 切换置顶")
		return m, m.bookshelf.TogglePin()
	}
	if keyMatches(msg, BookshelfKeys.Redraw) {
		logger.Debugf("[TUI] 强制刷新界面：清除 toast 和进度条")
		// 清除 bookshelf toast
		m.bookshelf.toast = ""
		m.bookshelf.toastIsError = false
		// 强制隐藏 crawl 进度条
		m.crawl.state = CrawlHidden
		return m, nil
	}
	if keyMatches(msg, BookshelfKeys.Continue) {
		book := m.bookshelf.SelectedBook()
		if book == nil {
			return m, nil
		}
		// 优先从 book_sources 表获取来源信息
		bs, err := m.db.GetBookSource(book.ID)
		if err != nil {
			logger.Warnf("[TUI] 获取书籍来源失败: %v", err)
			bookshelf, _ := m.bookshelf.Update(ShowToastMsg{Content: "Failed to get source info", IsError: true})
			m.bookshelf = bookshelf
			return m, nil
		}
		if bs != nil && bs.SourceURL != "" {
			logger.Infof("[TUI] 继续下载(来自book_sources): %s", book.Title)
			return m, func() tea.Msg {
				return ContinueCrawlMsg{
					BookID:     book.ID,
					BookTitle:  book.Title,
					SourceName: bs.SourceName,
					SourceURL:  bs.SourceURL,
				}
			}
		}
		// 回退到 books 表的 source_url/source_site
		if book.SourceURL != "" && book.SourceSite != "" {
			logger.Infof("[TUI] 继续下载(来自books表): %s", book.Title)
			return m, func() tea.Msg {
				return ContinueCrawlMsg{
					BookID:     book.ID,
					BookTitle:  book.Title,
					SourceName: book.SourceSite,
					SourceURL:  book.SourceURL,
				}
			}
		}
		bookshelf, _ := m.bookshelf.Update(ShowToastMsg{Content: "No source URL available for this book", IsError: true})
		m.bookshelf = bookshelf
		return m, nil
	}

	bookshelf, cmd := m.bookshelf.Update(msg)
	m.bookshelf = bookshelf
	return m, cmd
}

func (m AppModel) handleBookDescKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if keyMatches(msg, BookshelfKeys.Desc) || keyMatches(msg, keyWithKeys("esc", "q")) {
		return m, func() tea.Msg { return CloseBookDescMsg{} }
	}
	// 上下滚动简介
	if keyMatches(msg, keyWithKeys("up", "k")) {
		m.bookshelf.descViewport.LineUp(1)
		return m, nil
	}
	if keyMatches(msg, keyWithKeys("down", "j")) {
		m.bookshelf.descViewport.LineDown(1)
		return m, nil
	}
	if keyMatches(msg, keyWithKeys("pgup")) {
		m.bookshelf.descViewport.LineUp(5)
		return m, nil
	}
	if keyMatches(msg, keyWithKeys("pgdown", " ")) {
		m.bookshelf.descViewport.LineDown(5)
		return m, nil
	}
	return m, nil
}

func (m AppModel) handleReaderKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if keyMatches(msg, GlobalKeys.Help) {
		return m, func() tea.Msg { return ShowHelpMsg{} }
	}
	if keyMatches(msg, ReaderKeys.Back) {
		logger.Debugf("[TUI] 阅读器返回书架")
		return m, func() tea.Msg { return CloseReaderMsg{} }
	}
	if keyMatches(msg, ReaderKeys.ChapterPicker) {
		logger.Debugf("[TUI] 打开章节选择器 bookID=%d currentCh=%d", m.reader.bookID, m.reader.chapterNum)
		m.state = StateChapterPicker
		return m, m.chapterPicker.Open(m.reader.bookID, m.reader.chapterNum)
	}
	if keyMatches(msg, ReaderKeys.NextChapter) {
		logger.Debugf("[TUI] 下一章")
		return m, m.reader.NextChapter()
	}
	if keyMatches(msg, ReaderKeys.PrevChapter) {
		logger.Debugf("[TUI] 上一章")
		return m, m.reader.PrevChapter()
	}

	reader, cmd := m.reader.Update(msg)
	m.reader = &reader
	return m, cmd
}

func (m AppModel) handleSearchKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// 1. 关闭/返回
	if keyMatches(msg, SearchKeys.Close) {
		if m.search.IsSelectingSource() || m.search.isSearching {
			// 在来源选择阶段或搜索中 -> 直接关闭搜索
			logger.Debugf("[TUI] 关闭搜索(取消)")
			return m, func() tea.Msg { return CloseSearchMsg{Cancelled: true} }
		}
		// 在搜索结果阶段 -> 返回到来源选择
		logger.Debugf("[TUI] 返回来源选择")
		m.search.GoBack()
		return m, nil
	}

	// 2. 确认/Enter
	if keyMatches(msg, SearchKeys.Confirm) {
		if m.search.isSearching {
			return m, nil
		}
		// 步骤1：在来源选择阶段
		if m.search.IsSelectingSource() {
			// 1a: 输入为空 -> 忽略
			if m.search.Value() == "" {
				return m, nil
			}
			// 1b: 有来源列表但没选择 -> 显示来源列表（首次按Enter）
			if !m.search.HasResults() {
				return m, m.search.StartSearch()
			}
			// 1c: 已有来源列表 -> 选择来源并开始搜索
			return m, m.search.SelectSource()
		}
		// 步骤2：在搜索结果中按 Enter 开始爬取
		if result := m.search.SelectedResult(); result != nil && result.Available {
			return m, func() tea.Msg {
				return StartCrawlMsg{
					BookTitle:  result.BookTitle,
					SourceName: result.SourceName,
					SourceURL:  result.SourceURL,
				}
			}
		}
		return m, nil
	}

	// 3. 后台下载（仅在搜索结果页面，且输入框失焦时可用）
	if keyMatches(msg, SearchKeys.Background) {
		if m.search.isSearching || m.search.IsSelectingSource() {
			return m, nil
		}
		if result := m.search.SelectedResult(); result != nil && result.Available {
			logger.Infof("[TUI] 后台下载: %s", result.BookTitle)
			m.state = StateBackgroundCrawling
			m.crawl.Start(result.BookTitle, result.SourceName, result.SourceURL)
			return m, tea.Batch(
				func() tea.Msg { return CloseSearchMsg{} },
				m.crawl.crawlCmd(),
			)
		}
		return m, nil
	}

	// 4. 上下移动（仅在列表有项目时）
	if keyMatches(msg, SearchKeys.Up) {
		if m.search.HasResults() {
			m.search.CursorUp()
		}
		return m, nil
	}
	if keyMatches(msg, SearchKeys.Down) {
		if m.search.HasResults() {
			m.search.CursorDown()
		}
		return m, nil
	}

	// 5. 透传给 search 组件（输入框、列表）
	search, cmd := m.search.Update(msg)
	m.search = search
	return m, cmd
}

func (m AppModel) handleConfirmCrawlKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if keyMatches(msg, keyWithKeys("enter")) {
		logger.Infof("[TUI] 确认前台爬取")
		m.state = StateCrawling
		return m, m.crawl.crawlCmd()
	}
	if keyMatches(msg, keyWithKeys("b")) {
		logger.Infof("[TUI] 确认后台爬取")
		m.state = StateBackgroundCrawling
		return m, m.crawl.crawlCmd()
	}
	if keyMatches(msg, keyWithKeys("esc")) {
		m.state = m.prevState
		return m, nil
	}
	return m, nil
}

func (m AppModel) handleConfirmDeleteKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if keyMatches(msg, keyWithKeys("enter", "y")) {
		if book := m.bookshelf.SelectedBook(); book != nil {
			logger.Infof("[TUI] 删除书籍: %s (ID=%d)", book.Title, book.ID)
			if err := m.db.DeleteBook(book.ID); err != nil {
				logger.Errorf("[TUI] 删除书籍失败: %v", err)
				return m, showToast("Delete failed: "+err.Error(), true)
			}
		}
		m.state = StateBookshelf
		return m, tea.Batch(
			showToast("Deleted", false),
			m.bookshelf.LoadBooks(),
		)
	}
	if keyMatches(msg, keyWithKeys("esc", "n")) {
		m.state = StateBookshelf
		return m, nil
	}
	return m, nil
}

func (m AppModel) handleConfirmNukeKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if keyMatches(msg, keyWithKeys("enter", "y")) {
		logger.Infof("[TUI] 用户确认清除所有数据")
		appDir := appDataDir()
		// 关闭数据库连接以便删除文件
		m.db.Close()
		// 删除数据目录
		if err := os.RemoveAll(appDir); err != nil {
			logger.Errorf("[TUI] 清除数据失败: %v", err)
			return m, tea.Batch(
				showToast("Clear failed: "+err.Error(), true),
				tea.Quit,
			)
		}
		logger.Infof("[TUI] 数据已清除: %s", appDir)
		return m, tea.Batch(
			showToast("All data cleared. Goodbye!", false),
			tea.Quit,
		)
	}
	if keyMatches(msg, keyWithKeys("esc", "n")) {
		m.state = StateBookshelf
		return m, nil
	}
	return m, nil
}

// appDataDir 返回应用数据目录 (与 main.go 保持一致)
func appDataDir() string {
	var base string
	switch runtime.GOOS {
	case "windows":
		base = os.Getenv("APPDATA")
		if base == "" {
			base = os.Getenv("USERPROFILE")
		}
	case "darwin", "linux":
		base = os.Getenv("HOME")
		if base != "" {
			base = filepath.Join(base, ".config")
		}
	}
	if base == "" {
		base, _ = os.Getwd()
	}
	return filepath.Join(base, "terminalreader")
}

func (m AppModel) handleChapterPickerKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// 如果 list 正在过滤中，esc 先取消过滤，而不是关闭选择器
	if m.chapterPicker.list.FilterState() == list.FilterApplied {
		if keyMatches(msg, keyWithKeys("esc")) {
			m.chapterPicker.list.ResetFilter()
			return m, nil
		}
	}
	if keyMatches(msg, ChapterPickerKeys.Close) {
		logger.Debugf("[TUI] 关闭章节选择器")
		m.state = StateReader
		m.chapterPicker.Close()
		return m, nil
	}
	if keyMatches(msg, ChapterPickerKeys.Confirm) {
		chNum := m.chapterPicker.SelectedChapter()
		if chNum > 0 {
			logger.Infof("[TUI] 跳转章节: %d", chNum)
			m.state = StateReader
			m.chapterPicker.Close()
			return m, m.reader.loadChapterCmd(chNum)
		}
	}
	picker, cmd := m.chapterPicker.Update(msg)
	m.chapterPicker = picker
	return m, cmd
}

func (m AppModel) viewConfirmDelete() string {
	book := m.bookshelf.SelectedBook()
	name := ""
	if book != nil {
		name = book.Title
	}
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		TitleStyle.Render("Confirm Delete"),
		"",
		fmt.Sprintf("Delete %q?", name),
		"",
		renderFooter([]footerItem{
			{key: "enter/y", desc: "confirm"},
			{key: "esc/n", desc: "cancel"},
		}, 48),
	)
	box := DialogBoxStyle.Width(50).Render(content)
	return lipgloss.NewStyle().
		Width(m.width).Height(m.height).
		Render(lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box))
}

func (m AppModel) viewConfirmNuke() string {
	appDir := appDataDir()
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		TitleStyle.Render("⚠️  Clear All Data"),
		"",
		"This will delete EVERYTHING:",
		fmt.Sprintf("  %s", appDir),
		"",
		"Including all books, chapters, logs, and settings.",
		"This action CANNOT be undone.",
		"",
		renderFooter([]footerItem{
			{key: "enter/y", desc: "confirm"},
			{key: "esc/n", desc: "cancel"},
		}, 48),
	)
	box := DialogBoxStyle.Width(56).Render(content)
	return lipgloss.NewStyle().
		Width(m.width).Height(m.height).
		Render(lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box))
}

// 消息类型

// OpenBookMsg 打开书籍
type OpenBookMsg struct {
	BookID int64
}

// CloseReaderMsg 关闭阅读器
type CloseReaderMsg struct{}

// OpenSearchMsg 打开搜索
type OpenSearchMsg struct{}

// CloseSearchMsg 关闭搜索
type CloseSearchMsg struct {
	Cancelled bool
}

// StartCrawlMsg 开始爬取
type StartCrawlMsg struct {
	SourceName string
	SourceURL  string
	BookTitle  string
}

// ShowHelpMsg 显示帮助
type ShowHelpMsg struct{}

// CloseHelpMsg 关闭帮助
type CloseHelpMsg struct{}

// ShowBookDescMsg 显示书籍简介
type ShowBookDescMsg struct{}

// CloseBookDescMsg 关闭书籍简介
type CloseBookDescMsg struct{}

// ShowToastMsg 显示 Toast
type ShowToastMsg struct {
	Content string
	IsError bool
}

// ContinueCrawlMsg 继续下载消息
type ContinueCrawlMsg struct {
	BookID     int64
	BookTitle  string
	SourceName string
	SourceURL  string
}

func showToast(content string, isError bool) tea.Cmd {
	return func() tea.Msg {
		return ShowToastMsg{Content: content, IsError: isError}
	}
}

// keyMatches 判断按键是否匹配绑定
type simpleBinding struct {
	keys []string
}

func keyWithKeys(keys ...string) simpleBinding {
	return simpleBinding{keys: keys}
}

// WaitForBackgroundCrawl 等待后台爬取任务结束
func (m AppModel) WaitForBackgroundCrawl() {
	logger.Infof("[TUI] 等待后台爬取任务结束")
	m.crawl.Wait()
	logger.Infof("[TUI] 后台爬取任务已结束")
}

func keyMatches(msg tea.KeyMsg, binding interface{}) bool {
	var keys []string
	switch b := binding.(type) {
	case simpleBinding:
		keys = b.keys
	case interface{ Keys() []string }:
		keys = b.Keys()
	default:
		return false
	}
	for _, k := range keys {
		if msg.String() == k {
			return true
		}
	}
	return false
}
