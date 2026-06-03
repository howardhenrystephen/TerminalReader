package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/henry/novel-reader/internal/crawler"
	"github.com/henry/novel-reader/internal/db"
	"github.com/henry/novel-reader/internal/tui"
	"github.com/henry/novel-reader/pkg/logger"
)

func checkPrerequisites() error {
	// 检查 Python
	pythonCmd := "python3"
	if _, err := exec.LookPath(pythonCmd); err != nil {
		pythonCmd = "python"
		if _, err := exec.LookPath(pythonCmd); err != nil {
			return fmt.Errorf("未找到 Python，请先安装 Python 3")
		}
	}

	// 检查 cloudscraper
	cmd := exec.Command(pythonCmd, "-c", "import cloudscraper")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("未找到 cloudscraper，请运行: %s -m pip install cloudscraper", pythonCmd)
	}

	// 检查 spider.py
	candidates := []string{
		filepath.Join("script", "spider.py"),
		filepath.Join("..", "script", "spider.py"),
	}
	found := false
	for _, p := range candidates {
		if abs, err := filepath.Abs(p); err == nil {
			if _, err := os.Stat(abs); err == nil {
				found = true
				break
			}
		}
	}
	if !found {
		return fmt.Errorf("未找到 script/spider.py，请确保从正确的目录运行，或重新下载完整 release 包")
	}

	return nil
}

func main() {
	// 初始化日志系统
	logDir := filepath.Join(".", "log")
	if err := logger.Init(logDir); err != nil {
		fmt.Fprintf(os.Stderr, "初始化日志失败: %v\n", err)
		os.Exit(1)
	}
	logger.Infof("程序启动")

	// 检查运行环境
	if err := checkPrerequisites(); err != nil {
		fmt.Fprintf(os.Stderr, "\n❌ 环境检查失败: %v\n\n", err)
		fmt.Fprintln(os.Stderr, "💡 提示:")
		fmt.Fprintln(os.Stderr, "   1. 确保已安装 Python 3")
		fmt.Fprintln(os.Stderr, "   2. 运行: pip install cloudscraper")
		fmt.Fprintln(os.Stderr, "   3. 从 release 包解压后的目录运行程序")
		fmt.Fprintln(os.Stderr, "")
		os.Exit(1)
	}

	var dbPath string
	flag.StringVar(&dbPath, "db", "", "数据库文件路径")
	flag.Parse()

	if dbPath == "" {
		dataDir := filepath.Join(".", "data")
		if err := os.MkdirAll(dataDir, 0755); err != nil {
			logger.Errorf("创建数据目录失败: %v", err)
			os.Exit(1)
		}
		dbPath = filepath.Join(dataDir, "novels.db")
	}

	database, err := db.InitDB(dbPath)
	if err != nil {
		logger.Errorf("初始化数据库失败: %v", err)
		os.Exit(1)
	}
	defer func() {
		database.Close()
		logger.Infof("程序退出")
	}()

	engine := crawler.NewEngine(database)
	// 注册来源
	engine.RegisterSource(crawler.NewIxdzs8Source())
	engine.RegisterSource(crawler.NewBoqugeSource())
	logger.Infof("爬虫引擎初始化完成，注册来源: %v", engine.GetSourceNames())

	app := tui.NewApp(database, engine)
	p := tea.NewProgram(app, tea.WithAltScreen())
	logger.Infof("TUI 程序启动")
	finalModel, err := p.Run()
	if err != nil {
		logger.Errorf("运行失败: %v", err)
		os.Exit(1)
	}
	// 等待后台爬取任务结束
	if m, ok := finalModel.(tui.AppModel); ok {
		logger.Infof("等待后台爬取任务结束...")
		m.WaitForBackgroundCrawl()
		logger.Infof("后台爬取任务已结束")
	}
}
