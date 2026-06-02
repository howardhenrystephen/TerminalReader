package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/henry/novel-reader/internal/crawler"
	"github.com/henry/novel-reader/internal/db"
	"github.com/henry/novel-reader/internal/tui"
	"github.com/henry/novel-reader/pkg/logger"
)

func main() {
	// 初始化日志系统
	logDir := filepath.Join(".", "log")
	if err := logger.Init(logDir); err != nil {
		fmt.Fprintf(os.Stderr, "初始化日志失败: %v\n", err)
		os.Exit(1)
	}
	logger.Infof("程序启动")

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
