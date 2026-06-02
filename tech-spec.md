# novel-reader 技术规格文档（Agent 实施指南）

## 1. 项目总览

### 1.1 架构模式

采用 **MVC 分层架构**：
- **Model 层**（`db/` 包）：SQLite 数据访问，纯 Go 结构体 + SQL
- **Controller 层**（`crawler/` 包）：HTTP 爬取引擎，接口驱动的多来源解析
- **View 层**（`tui/` 包）：Bubble Tea Model 实现，状态机驱动的视图路由
- **入口**（`main.go`）：依赖注入，组装各层

### 1.2 关键技术决策

| 决策点 | 选择 | 理由 |
|--------|------|------|
| SQLite 驱动 | `modernc.org/sqlite` | 纯 Go 实现，零 CGO 依赖，交叉编译友好 |
| HTML 解析 | `golang.org/x/net/html` | 标准库扩展，无需额外依赖 |
| HTTP 客户端 | `net/http` + 自定义 Transport | 标准库，可配置超时和重试 |
| 并发模型 | goroutine + channel | Go 原生，适合多来源并发搜索 |
| 配置文件 | 无，存 SQLite | 减少依赖，数据库本身即配置存储 |

### 1.3 模块依赖关系

```
main.go
  ├── db/          ← 无外部项目依赖
  ├── crawler/
  │     └── db/    ← 爬取完成后写入数据库
  └── tui/
        ├── db/    ← 查询书籍/章节/保存进度
        └── crawler/  ← 触发搜索和爬取
```

**禁止循环依赖**。`tui` 调用 `crawler` 的接口，`crawler` 通过回调/channel 向 `tui` 报告进度，不直接依赖 `tui`。

---

## 2. 数据库层（`db/` 包）

### 2.1 文件清单

```
db/
├── database.go   # 连接初始化 + 迁移 + 表生命周期管理
├── models.go     # 所有结构体定义
└── queries.go    # 所有数据访问函数
```

### 2.2 `models.go` — 完整结构体定义

```go
package db

import "time"

// Book 对应 books 表
type Book struct {
    ID             int
    Title          string
    Author         string
    Description    string
    TotalChapters  int
    CurrentChapter int    // 默认 1
    CurrentOffset  int    // 默认 0
    SourceURL      string
    SourceSite     string
    CreatedAt      time.Time
    UpdatedAt      time.Time
}

// Chapter 对应 chapters_{book_id} 表
type Chapter struct {
    ID          int
    ChapterNum  int
    Title       string
    Content     string
    WordCount   int
    CreatedAt   time.Time
}

// SearchResult 搜索结果项（非数据库表，业务结构体）
type SearchResult struct {
    SourceName string
    SourceURL  string
    BookTitle  string
    Author     string
    Available  bool
    Error      string // 空字符串表示无错误
}

// CrawlTask 爬取任务参数
type CrawlTask struct {
    BookID    int
    SourceURL string
    SourceSite string
}
```

### 2.3 `database.go` — 实现细节

```go
package db

import (
    "database/sql"
    "fmt"
    "os"
    "path/filepath"
    
    _ "modernc.org/sqlite"
)

// DB 包装 sql.DB，提供项目专用的数据库操作
type DB struct {
    conn *sql.DB
}

// InitDB 初始化数据库连接，如果数据库文件不存在则创建
// 逻辑：
// 1. 确保数据库文件所在目录存在（os.MkdirAll）
// 2. sql.Open("sqlite", dbPath) 建立连接
// 3. 设置连接池参数：conn.SetMaxOpenConns(1)（SQLite 单连接避免锁竞争）
// 4. 调用 Migrate() 执行建表
// 5. 返回 *DB 包装器
func InitDB(dbPath string) (*DB, error) { ... }

// Close 关闭数据库连接
func (d *DB) Close() error { ... }

// Migrate 执行数据库迁移
// 执行以下 SQL：
//   1. CREATE TABLE IF NOT EXISTS books (...)
//   2. CREATE TRIGGER IF NOT EXISTS update_books_timestamp ...
// 注意：不创建 chapters 表，chapters 表在 AddBook 时动态创建
func (d *DB) Migrate() error { ... }

// CreateChapterTable 为指定书籍创建章节表
// 表名：fmt.Sprintf("chapters_%d", bookID)
// 执行 CREATE TABLE IF NOT EXISTS chapters_%d (...)
// 字段：id/chapter_num/title/content/word_count/created_at
// chapter_num 加 UNIQUE 约束
func (d *DB) CreateChapterTable(bookID int) error { ... }

// DropChapterTable 删除指定书籍的章节表（删除书籍时调用）
func (d *DB) DropChapterTable(bookID int) error { ... }

// Exec / Query 底层方法（供 queries.go 使用）
func (d *DB) exec(query string, args ...interface{}) (sql.Result, error) { ... }
func (d *DB) query(query string, args ...interface{}) (*sql.Rows, error) { ... }
func (d *DB) queryRow(query string, args ...interface{}) *sql.Row { ... }
```

**关键实现约束**：
- SQLite 连接池必须设为 `MaxOpenConns(1)`，SQLite 不支持同一进程多写入并发
- 所有 SQL 语句使用 `?` 占位符（modernc-sqlite 兼容）
- 数据库路径支持相对路径和绝对路径
- 数据目录不存在时自动创建（`os.MkdirAll(filepath.Dir(dbPath), 0755)`）

### 2.4 `queries.go` — 所有查询函数

```go
package db

import (
    "database/sql"
    "fmt"
    "strings"
    "time"
)

// ==================== Book CRUD ====================

// ListBooks 查询所有书籍，按 updated_at 降序（最近阅读的在前）
// SQL: SELECT id, title, author, description, total_chapters, current_chapter, 
//             current_offset, source_url, source_site, created_at, updated_at
//      FROM books ORDER BY updated_at DESC
func (d *DB) ListBooks() ([]Book, error) { ... }

// GetBook 根据 ID 查询单本书
// SQL: SELECT ... FROM books WHERE id = ?
// 不存在返回 sql.ErrNoRows
func (d *DB) GetBook(id int) (*Book, error) { ... }

// AddBook 添加书籍到 books 表，然后创建对应的 chapters 表
// 逻辑：
//   1. INSERT INTO books (title, author, description, total_chapters, source_url, source_site)
//      VALUES (?, ?, ?, ?, ?, ?)
//   2. 通过 sql.Result.LastInsertId() 获取 bookID
//   3. 调用 d.CreateChapterTable(bookID)
//   4. 返回 bookID
// 事务：使用 sql.Tx 包裹步骤 1-3，任一失败回滚
func (d *DB) AddBook(title, author, description string, totalChapters int, sourceURL, sourceSite string) (int, error) { ... }

// UpdateBookProgress 更新阅读进度
// SQL: UPDATE books SET current_chapter = ?, current_offset = ? WHERE id = ?
// updated_at 由触发器自动更新
func (d *DB) UpdateBookProgress(bookID, chapterNum, offset int) error { ... }

// UpdateBookTotalChapters 爬取过程中更新总章节数
// SQL: UPDATE books SET total_chapters = ? WHERE id = ?
func (d *DB) UpdateBookTotalChapters(bookID, total int) error { ... }

// DeleteBook 删除书籍及其章节表
// 逻辑：
//   1. BEGIN TRANSACTION
//   2. DELETE FROM books WHERE id = ?
//   3. DROP TABLE IF EXISTS chapters_?
//   4. COMMIT
func (d *DB) DeleteBook(bookID int) error { ... }

// ==================== Chapter CRUD ====================

// chapterTableName 辅助函数：生成章节表名
func chapterTableName(bookID int) string {
    return fmt.Sprintf("chapters_%d", bookID)
}

// InsertChapter 插入单章内容
// SQL: INSERT INTO chapters_? (chapter_num, title, content, word_count) 
//      VALUES (?, ?, ?, ?)
// word_count = utf8.RuneCountInString(content)
func (d *DB) InsertChapter(bookID int, chapterNum int, title, content string) error { ... }

// BulkInsertChapters 批量插入章节（事务优化）
// 逻辑：
//   1. BEGIN
//   2. PREPARE 语句
//   3. 循环执行 EXECUTE
//   4. COMMIT
// 用于爬取完成后一次性写入大量章节
func (d *DB) BulkInsertChapters(bookID int, chapters []Chapter) error { ... }

// GetChapter 查询单章内容
// SQL: SELECT id, chapter_num, title, content, word_count, created_at 
//      FROM chapters_? WHERE chapter_num = ?
func (d *DB) GetChapter(bookID, chapterNum int) (*Chapter, error) { ... }

// GetChapterCount 获取某书的章节总数
// SQL: SELECT COUNT(*) FROM chapters_?
func (d *DB) GetChapterCount(bookID int) (int, error) { ... }

// ListChapterTitles 获取所有章节标题（用于目录跳转）
// SQL: SELECT chapter_num, title FROM chapters_? ORDER BY chapter_num
func (d *DB) ListChapterTitles(bookID int) ([]struct{ Num int; Title string }, error) { ... }
```

**SQL 语句速查表**（所有语句必须在 `queries.go` 中以常量形式定义）：

```go
const (
    sqlCreateBooksTable = `CREATE TABLE IF NOT EXISTS books (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        title TEXT NOT NULL,
        author TEXT,
        description TEXT,
        total_chapters INTEGER DEFAULT 0,
        current_chapter INTEGER DEFAULT 1,
        current_offset INTEGER DEFAULT 0,
        source_url TEXT,
        source_site TEXT,
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
        updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
    )`

    sqlCreateBooksTrigger = `CREATE TRIGGER IF NOT EXISTS update_books_timestamp
        AFTER UPDATE ON books
        BEGIN
            UPDATE books SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
        END`

    sqlListBooks = `SELECT id, title, author, description, total_chapters, 
        current_chapter, current_offset, source_url, source_site, 
        created_at, updated_at FROM books ORDER BY updated_at DESC`

    sqlGetBook = `SELECT id, title, author, description, total_chapters,
        current_chapter, current_offset, source_url, source_site,
        created_at, updated_at FROM books WHERE id = ?`

    sqlInsertBook = `INSERT INTO books (title, author, description, total_chapters,
        source_url, source_site) VALUES (?, ?, ?, ?, ?, ?)`

    sqlUpdateProgress = `UPDATE books SET current_chapter = ?, current_offset = ? WHERE id = ?`

    sqlUpdateTotalChapters = `UPDATE books SET total_chapters = ? WHERE id = ?`

    sqlDeleteBook = `DELETE FROM books WHERE id = ?`
)
```

---

## 3. 爬虫引擎（`crawler/` 包）

### 3.1 文件清单

```
crawler/
├── crawler.go    # 爬取引擎主控、接口定义、并发控制
├── sources.go    # 来源配置注册中心
├── biquge.go     # 笔趣阁解析器
├── x23us.go      # 顶点小说解析器
└── qidian.go     # 起点中文网解析器
```

### 3.2 `crawler.go` — 引擎核心

```go
package crawler

import (
    "context"
    "net/http"
    "time"
    
    "novel-reader/db"
)

// Source 定义爬取来源接口，每个来源（笔趣阁、起点等）实现此接口
type Source interface {
    // Name 返回来源名称（如"笔趣阁"）
    Name() string
    
    // Search 在来源站点搜索小说，返回搜索结果
    // keyword: 用户输入的书名
    // 返回 (*SearchResult, error)，找不到返回 nil, nil
    Search(ctx context.Context, keyword string) (*db.SearchResult, error)
    
    // FetchBookInfo 获取书籍基本信息（总章节数、简介等）
    // bookURL: Search 返回的 SourceURL
    FetchBookInfo(ctx context.Context, bookURL string) (*BookInfo, error)
    
    // FetchChapterList 获取章节列表（不含正文）
    FetchChapterList(ctx context.Context, bookURL string) ([]ChapterInfo, error)
    
    // FetchChapterContent 获取单章正文
    // chapterURL: 章节页面的完整 URL
    FetchChapterContent(ctx context.Context, chapterURL string) (string, error)
}

// BookInfo 书籍基本信息
type BookInfo struct {
    Title       string
    Author      string
    Description string
    TotalChapters int
    CoverURL    string // 可忽略
}

// ChapterInfo 章节列表项（不含正文）
type ChapterInfo struct {
    Num     int
    Title   string
    URL     string // 章节页面的完整 URL
}

// Engine 爬取引擎
type Engine struct {
    httpClient *http.Client
    sources    map[string]Source // sourceName -> Source
}

// NewEngine 创建引擎实例
// httpClient 配置：
//   - Timeout: 30s per request
//   - Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: false}}
//   - 自定义 User-Agent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"
// 注册所有内置来源
func NewEngine() *Engine { ... }

// SearchAll 并发搜索所有来源
// 逻辑：
//   1. 创建一个 buffered channel，缓冲区大小 = len(engine.sources)
//   2. 对每个来源启动一个 goroutine
//   3. 每个 goroutine 内：
//        - 设置 10s 超时的 context
//        - 调用 source.Search(ctx, keyword)
//        - 结果发送到 channel（即使出错也发送，Error 字段记录错误）
//   4. 主 goroutine 从 channel 收集所有结果
//   5. 返回 []db.SearchResult
// 注意：函数返回时机——所有 goroutine 完成后才返回，使用 sync.WaitGroup
func (e *Engine) SearchAll(ctx context.Context, keyword string) []db.SearchResult { ... }

// CrawlBook 爬取整本书
// 参数：
//   ctx: 支持取消的 context
//   sourceName: 来源名（如"笔趣阁"）
//   bookURL: 书籍页面 URL
//   progressCh: 进度报告 channel（可选，传 nil 则不报告）
//   dbConn: 数据库连接，用于实时写入
//   bookID: books 表中的 ID
//
// 逻辑：
//   1. 根据 sourceName 获取 Source 实例
//   2. 调用 source.FetchBookInfo(bookURL) 获取书籍信息
//   3. 调用 source.FetchChapterList(bookURL) 获取所有章节 URL
//   4. 更新 books 表 total_chapters
//   5. 并发爬取章节（最多 3 个并发 goroutine）：
//        - 使用信号量 channel 控制并发：sem := make(chan struct{}, 3)
//        - 每个 goroutine：获取 sem -> 爬取章节 -> 写入数据库 -> 释放 sem
//        - 向 progressCh 发送 CrawlProgress 更新
//   6. 完成后关闭 progressCh
//   7. 返回 error（如果有）
// 
// 错误处理：
//   - 单章爬取失败记录日志，继续后续章节
//   - 最后返回累计的错误信息
func (e *Engine) CrawlBook(ctx context.Context, sourceName, bookURL string, 
    progressCh chan<- db.CrawlProgress, dbConn *db.DB, bookID int) error { ... }

// CrawlProgress 爬取进度（用于 TUI 进度条更新）
type CrawlProgress struct {
    CurrentChapter int
    TotalChapters  int
    ChapterTitle   string
    Percentage     float64  // 0.0 ~ 100.0
    Done           bool
    Error          error
}
```

### 3.3 `sources.go` — 来源注册中心

```go
package crawler

// RegisterSource 注册一个爬取来源
// 在 NewEngine 中调用：
//   e.RegisterSource(&BiqugeSource{...})
//   e.RegisterSource(&X23usSource{...})
//   e.RegisterSource(&QidianSource{...})
func (e *Engine) RegisterSource(s Source) { ... }

// GetSourceNames 返回所有已注册来源的名称列表
func (e *Engine) GetSourceNames() []string { ... }
```

### 3.4 `biquge.go` — 笔趣阁解析器（示例模板）

所有解析器遵循相同结构，仅 HTML 选择器和 URL 模板不同。

```go
package crawler

import (
    "context"
    "fmt"
    "io"
    "net/http"
    "net/url"
    "strings"
    
    "golang.org/x/net/html"
    "novel-reader/db"
)

// BiqugeSource 笔趣阁来源
type BiqugeSource struct {
    client *http.Client
}

// Name 返回 "笔趣阁"
func (b *BiqugeSource) Name() string { return "笔趣阁" }

// Search 搜索逻辑：
//   1. 构造搜索 URL: fmt.Sprintf("https://www.biquge.co/search.html?keyword=%s", url.QueryEscape(keyword))
//   2. GET 请求，10s 超时
//   3. 解析 HTML：
//        - 查找 class="result-item" 的 div
//        - 提取 .result-game-item-title a 的文本（书名）
//        - 提取 .result-game-item-info p 中的作者
//        - 提取 .result-game-item-title a 的 href（书籍 URL）
//   4. 模糊匹配：使用 levenshtein 距离或 strings.Contains 判断书名是否匹配
//   5. 返回 &db.SearchResult{Available: true, ...} 或 nil
// 错误：超时返回 Error="请求超时"，解析失败返回 Error="解析失败"
func (b *BiqugeSource) Search(ctx context.Context, keyword string) (*db.SearchResult, error) { ... }

// FetchBookInfo 逻辑：
//   1. GET bookURL
//   2. 解析 HTML 提取：书名(h1)、作者(meta 或 p)、简介(#intro p)
//   3. 提取总章节数（从章节列表统计或 meta 信息）
func (b *BiqugeSource) FetchBookInfo(ctx context.Context, bookURL string) (*BookInfo, error) { ... }

// FetchChapterList 逻辑：
//   1. GET bookURL
//   2. 解析 HTML 查找章节列表（通常 #list dl dd a）
//   3. 每个 a 标签：提取 href（相对 URL 需要拼接为绝对 URL）和文本（章节名）
//   4. 按顺序编号（从 1 开始）
//   5. 返回 []ChapterInfo
// 注意：笔趣阁章节目录可能有重复（最新章节 + 正文章节），需要去重
func (b *BiqugeSource) FetchChapterList(ctx context.Context, bookURL string) ([]ChapterInfo, error) { ... }

// FetchChapterContent 逻辑：
//   1. GET chapterURL
//   2. 查找 id="content" 或 class="showtxt" 的 div
//   3. 提取文本内容
//   4. 清理广告文本和无用标记（常见："笔趣阁", "www.biquge.co", "请记住域名" 等）
//   5. 将 HTML 中的 <br>, <p> 等转为换行符
//   6. 返回纯文本字符串
func (b *BiqugeSource) FetchChapterContent(ctx context.Context, chapterURL string) (string, error) { ... }

// ==================== HTML 解析辅助函数（包内共享）====================

// fetchHTML 发送 GET 请求并返回 HTML 文档的根节点
// 配置：
//   - User-Agent: Mozilla/5.0...
//   - Accept: text/html
//   - 超时从 ctx 控制
// 返回 *html.Node（golang.org/x/net/html 的节点类型）
func fetchHTML(ctx context.Context, client *http.Client, url string) (*html.Node, error) { ... }

// findNode 按条件查找节点（类似 querySelector）
// 参数：node *html.Node, tag string, attrKey string, attrVal string
// 递归查找第一个匹配的节点
func findNode(node *html.Node, tag, attrKey, attrVal string) *html.Node { ... }

// findAllNodes 查找所有匹配节点（类似 querySelectorAll）
func findAllNodes(node *html.Node, tag, attrKey, attrVal string) []*html.Node { ... }

// getTextContent 提取节点下的所有文本内容
func getTextContent(node *html.Node) string { ... }

// getAttr 获取节点的指定属性值
func getAttr(node *html.Node, key string) string { ... }

// absURL 将相对 URL 转为绝对 URL
func absURL(base, ref string) string { ... }
```

### 3.5 其他解析器（`x23us.go`, `qidian.go`）

与 `biquge.go` 结构完全一致，仅需修改：
1. 结构体名（`X23usSource`, `QidianSource`）
2. `Name()` 返回值
3. 搜索 URL 模板
4. HTML 选择器（根据各站点 DOM 结构调整）
5. 正文内容区域的 CSS 选择器

---

## 4. TUI 视图层（`tui/` 包）

### 4.1 文件清单

```
tui/
├── app.go           # Bubble Tea Model 主控 + 状态机
├── styles.go        # Lipgloss 全局样式定义
├── keys.go          # 快捷键定义（bubbles/key 包）
├── bookshelf.go     # 书架视图 Model
├── reader.go        # 阅读器视图 Model
├── search.go        # 搜索弹层 Model
├── crawl.go         # 爬取确认 + 进度弹窗 Model
├── help.go          # 帮助弹窗 Model
└── toast.go         # Toast 通知组件
```

### 4.2 `styles.go` — 全局样式

```go
package tui

import "github.com/charmbracelet/lipgloss"

// 颜色常量
const (
    ColorBg       = "#1a1b26" // 背景
    ColorText     = "#c0caf5" // 主文本
    ColorAccent   = "#7aa2f7" // 强调蓝
    ColorHighlight= "#e0af68" // 琥珀高亮
    ColorSuccess  = "#9ece6a" // 成功绿
    ColorError    = "#f7768e" // 错误红
    ColorMuted    = "#565f89" // 分隔线
    ColorSubtext  = "#a9b1d6" // 副文本
)

var (
    // 基础样式
    BaseStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorText))
    
    // 标题样式
    TitleStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color(ColorAccent)).
        Bold(true).
        MarginLeft(1)
    
    // 副标题/描述样式
    DescStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color(ColorSubtext)).
        MarginLeft(1)
    
    // 帮助栏样式
    HelpKeyStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color(ColorHighlight)).
        Bold(true)
    HelpDescStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color(ColorMuted))
    HelpSepStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color(ColorMuted))
    
    // 弹窗样式
    DialogBoxStyle = lipgloss.NewStyle().
        Border(lipgloss.RoundedBorder()).
        BorderForeground(lipgloss.Color(ColorAccent)).
        Padding(1, 2).
        Width(50)
    
    // Toast 样式
    ToastSuccessStyle = lipgloss.NewStyle().
        Background(lipgloss.Color(ColorSuccess)).
        Foreground(lipgloss.Color(ColorBg)).
        Padding(0, 1).
        Bold(true)
    
    ToastErrorStyle = lipgloss.NewStyle().
        Background(lipgloss.Color(ColorError)).
        Foreground(lipgloss.Color(ColorBg)).
        Padding(0, 1).
        Bold(true)
    
    // 阅读器样式
    ReaderTextStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color(ColorText))
    
    ReaderHeaderStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color(ColorMuted)).
        Height(1)
    
    // 列表 delegate 样式（在 bookshelf.go 中设置给 list.Model）
    // 通过 list.NewDefaultDelegate() 自定义
)

// WindowSize 全局终端尺寸（由 app.go 在 WindowSizeMsg 时更新）
var WindowSize struct {
    Width  int
    Height int
}
```

### 4.3 `keys.go` — 快捷键定义

使用 `github.com/charmbracelet/bubbles/key` 包定义所有快捷键：

```go
package tui

import "github.com/charmbracelet/bubbles/key"

// ==================== 全局快捷键 ====================
type GlobalKeyMap struct {
    Quit key.Binding
    Help key.Binding
}

var GlobalKeys = GlobalKeyMap{
    Quit: key.NewBinding(
        key.WithKeys("q", "ctrl+c"),
        key.WithHelp("q", "退出"),
    ),
    Help: key.NewBinding(
        key.WithKeys("?"),
        key.WithHelp("?", "帮助"),
    ),
}

// ==================== 书架快捷键 ====================
type BookshelfKeyMap struct {
    Up       key.Binding
    Down     key.Binding
    Enter    key.Binding
    Search   key.Binding
    Delete   key.Binding
    Refresh  key.Binding
    GoTop    key.Binding
    GoBottom key.Binding
}

var BookshelfKeys = BookshelfKeyMap{
    Up: key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "上移")),
    Down: key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "下移")),
    Enter: key.NewBinding(key.WithKeys("enter", "l"), key.WithHelp("enter/l", "阅读")),
    Search: key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "搜索")),
    Delete: key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "删除")),
    Refresh: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "刷新")),
    GoTop: key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "顶部")),
    GoBottom: key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "底部")),
}

// ==================== 阅读器快捷键 ====================
type ReaderKeyMap struct {
    ScrollUp      key.Binding
    ScrollDown    key.Binding
    PageUp        key.Binding
    PageDown      key.Binding
    PrevChapter   key.Binding
    NextChapter   key.Binding
    GoStart       key.Binding
    GoEnd         key.Binding
    Back          key.Binding
    Bookmark      key.Binding
}

var ReaderKeys = ReaderKeyMap{
    ScrollUp: key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "上滚")),
    ScrollDown: key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "下滚")),
    PageUp: key.NewBinding(key.WithKeys("b"), key.WithHelp("b", "上翻页")),
    PageDown: key.NewBinding(key.WithKeys(" ", "f"), key.WithHelp("space/f", "下翻页")),
    PrevChapter: key.NewBinding(key.WithKeys("left", "h", "p"), key.WithHelp("←/h/p", "上一章")),
    NextChapter: key.NewBinding(key.WithKeys("right", "l", "n"), key.WithHelp("→/l/n", "下一章")),
    GoStart: key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "章首")),
    GoEnd: key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "章尾")),
    Back: key.NewBinding(key.WithKeys("esc", "b"), key.WithHelp("esc/b", "书架")),
    Bookmark: key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "书签")),
}

// ==================== 搜索弹层快捷键 ====================
type SearchKeyMap struct {
    Close   key.Binding
    Confirm key.Binding
    Up      key.Binding
    Down    key.Binding
    Tab     key.Binding
}

var SearchKeys = SearchKeyMap{
    Close: key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "取消")),
    Confirm: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "确认")),
    Up: key.NewBinding(key.WithKeys("up"), key.WithHelp("↑", "上选")),
    Down: key.NewBinding(key.WithKeys("down"), key.WithHelp("↓", "下选")),
    Tab: key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "切换焦点")),
}
```

### 4.4 `app.go` — 主 Model 和状态机

```go
package tui

import (
    "fmt"
    
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
    
    "novel-reader/db"
    "novel-reader/crawler"
)

// ==================== 状态定义 ====================

// ViewState 表示当前视图状态
type ViewState int

const (
    StateBookshelf ViewState = iota
    StateReader
    StateSearch
    StateConfirmCrawl
    StateCrawling
    StateConfirmDelete
    StateHelp
)

// ==================== 主 Model ====================

type AppModel struct {
    // 状态机
    state ViewState
    prevState ViewState // 用于帮助弹窗返回
    
    // 依赖（通过构造函数注入）
    db      *db.DB
    engine  *crawler.Engine
    
    // 子视图 Model
    bookshelf BookshelfModel
    reader    ReaderModel
    search    SearchModel
    crawl     CrawlModel
    help      HelpModel
    
    // Toast
    toast     *Toast
    toastTimer int // 帧计数器，用于自动消失
    
    // 窗口尺寸
    width     int
    height    int
}

// NewApp 创建应用主 Model
// 参数：
//   database: 已初始化的 *db.DB
//   engine: 已初始化的 *crawler.Engine
// 逻辑：
//   1. 初始化所有子 Model
//   2. 从数据库加载书籍列表到 bookshelf
//   3. 设置初始状态为 StateBookshelf
//   4. 返回 *AppModel
func NewApp(database *db.DB, engine *crawler.Engine) *AppModel { ... }

// Init 实现 tea.Model
// 返回所有子 Model 的 Init() 命令的 tea.Batch
func (m AppModel) Init() tea.Cmd { ... }

// Update 实现 tea.Model —— 核心状态机
// 逻辑流程：
//   1. 处理全局消息：
//        - tea.KeyMsg: 检查 q/Ctrl+c（需确认是否正在爬取）
//        - tea.WindowSizeMsg: 更新 WindowSize 全局变量，传递给所有子 Model
//        - Toast 消失定时
//   2. 根据 m.state 分发到对应子 Model 的 Update
//   3. 处理子 Model 返回的导航消息（自定义 Msg 类型）：
//        - OpenBookMsg: 切换到 StateReader，加载指定书籍
//        - CloseReaderMsg: 切换到 StateBookshelf，保存阅读位置
//        - OpenSearchMsg: 切换到 StateSearch
//        - CloseSearchMsg: 切换到 StateBookshelf
//        - StartCrawlMsg: 切换到 StateCrawling
//        - CrawlDoneMsg: 切换到 StateBookshelf，显示 Toast
//        - ShowHelpMsg: 保存 prevState，切换到 StateHelp
//        - CloseHelpMsg: 恢复到 prevState
//        - ShowToastMsg: 设置 toast 内容
//   4. 返回更新后的 Model 和 tea.Cmd
func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { ... }

// View 实现 tea.Model
// 逻辑：
//   1. 根据 m.state 选择主视图渲染
//   2. 如果 m.state != StateHelp，在顶层叠加 Help 弹窗（半透明背景）
//   3. 如果有 toast，在最顶层叠加 Toast
//   4. 返回组合后的字符串
func (m AppModel) View() string { ... }

// ==================== 自定义消息类型 ====================

// OpenBookMsg 打开书籍
type OpenBookMsg struct {
    BookID int
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
}

// CrawlProgressMsg 爬取进度（从 goroutine 通过 channel 发送到 Update）
type CrawlProgressMsg crawler.CrawlProgress

// CrawlDoneMsg 爬取完成
type CrawlDoneMsg struct {
    BookID int
    Error  error
}

// ShowHelpMsg 显示帮助
type ShowHelpMsg struct{}

// CloseHelpMsg 关闭帮助
type CloseHelpMsg struct{}

// ShowToastMsg 显示 Toast
type ShowToastMsg struct {
    Content string
    IsError bool
}
```

### 4.5 `bookshelf.go` — 书架视图

```go
package tui

import (
    "fmt"
    "strconv"
    
    "github.com/charmbracelet/bubbles/list"
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
    
    "novel-reader/db"
)

// BookItem 实现 list.Item 接口，用于 fancy-list 展示
type BookItem struct {
    book db.Book
}

// FilterValue 实现 list.Item —— 用于搜索过滤（书架内搜索）
func (b BookItem) FilterValue() string { return b.book.Title }

// Title 实现 list.DefaultItem —— 显示书名和作者
func (b BookItem) Title() string {
    return fmt.Sprintf("%s  %s", b.book.Title, 
        lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSubtext)).Render(b.book.Author))
}

// Description 实现 list.DefaultItem —— 显示简介和阅读进度
func (b BookItem) Description() string {
    progress := fmt.Sprintf("%d/%d", b.book.CurrentChapter, b.book.TotalChapters)
    if b.book.TotalChapters == 0 {
        progress = "未开始"
    }
    desc := b.book.Description
    if len(desc) > 60 {
        desc = desc[:60] + "..."
    }
    return fmt.Sprintf("%s  |  %s", desc, 
        lipgloss.NewStyle().Foreground(lipgloss.Color(ColorHighlight)).Render(progress))
}

// ==================== 书架 Model ====================

type BookshelfModel struct {
    list     list.Model
    books    []db.Book
    db       *db.DB
    width    int
    height   int
}

// NewBookshelfModel 创建书架 Model
// 参数：width, height 为可用空间尺寸
// 逻辑：
//   1. 创建 list.Model：
//        list.New([]list.Item{}, list.NewDefaultDelegate(), width, height-2)
//   2. 设置 list 样式：
//        - list.Title = "📚 我的书架"
//        - list.Styles.Title = TitleStyle
//        - 设置 NoItems 的显示内容（空书架提示）
//   3. 加载书籍列表（调用 LoadBooks）
//   4. 设置自定义 delegate 样式（左侧蓝色竖条）
func NewBookshelfModel(database *db.DB, width, height int) BookshelfModel { ... }

// LoadBooks 从数据库加载书籍并刷新列表
// 调用 db.ListBooks()，将 []db.Book 转为 []list.Item
// 使用 m.list.SetItems(items)
func (m *BookshelfModel) LoadBooks() error { ... }

// Init 实现 tea.Model
func (m BookshelfModel) Init() tea.Cmd { return nil }

// Update 处理书架视图的输入
// 按键处理逻辑：
//   - up/k: m.list.CursorUp()
//   - down/j: m.list.CursorDown()
//   - enter/l: 获取当前选中项（m.list.SelectedItem()），发送 OpenBookMsg
//   - s: 发送 OpenSearchMsg
//   - d: 获取当前选中项，发送 ShowConfirmDelete 消息（由 app.go 处理状态切换）
//   - r: 调用 LoadBooks() 刷新
//   - g: m.list.Select(0) 跳到顶部
//   - G: m.list.Select(len(items)-1) 跳到底部
//   - 1-9: 如果数字在范围内，m.list.Select(n-1) 然后发送 OpenBookMsg
//   - ?: 发送 ShowHelpMsg
func (m BookshelfModel) Update(msg tea.Msg) (BookshelfModel, tea.Cmd) { ... }

// View 渲染书架视图
// 返回 m.list.View() 的包装
// 空书架时：在列表区域居中显示 "书架为空，按 s 搜索并添加书籍"
func (m BookshelfModel) View() string { ... }

// SelectedBook 返回当前选中的书籍 ID
func (m BookshelfModel) SelectedBook() (db.Book, bool) { ... }

// SetSize 更新尺寸（响应窗口变化）
func (m *BookshelfModel) SetSize(width, height int) { ... }
```

**自定义 Delegate 实现**：

要实现 `fancy-list` 风格的左侧蓝色竖条，需要自定义 `list.ItemDelegate`：

```go
// 创建自定义 delegate
func newBookDelegate() list.DefaultDelegate {
    d := list.NewDefaultDelegate()
    
    // 设置选中项样式：左侧蓝色竖条
    d.Styles.SelectedTitle = lipgloss.NewStyle().
        Border(lipgloss.NormalBorder(), false, false, false, true).
        BorderForeground(lipgloss.Color(ColorAccent)).
        Foreground(lipgloss.Color(ColorText)).
        Bold(true).
        Padding(0, 0, 0, 1)
    
    d.Styles.SelectedDesc = lipgloss.NewStyle().
        Border(lipgloss.NormalBorder(), false, false, false, true).
        BorderForeground(lipgloss.Color(ColorAccent)).
        Foreground(lipgloss.Color(ColorSubtext)).
        Padding(0, 0, 0, 1)
    
    d.Styles.NormalTitle = lipgloss.NewStyle().
        Foreground(lipgloss.Color(ColorText)).
        Padding(0, 0, 0, 2)
    
    d.Styles.NormalDesc = lipgloss.NewStyle().
        Foreground(lipgloss.Color(ColorMuted)).
        Padding(0, 0, 0, 2)
    
    return d
}
```

### 4.6 `reader.go` — 阅读器视图

这是整个项目中最复杂的组件。

```go
package tui

import (
    "fmt"
    "strings"
    "unicode/utf8"
    
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
    
    "novel-reader/db"
)

// ReaderModel 阅读器状态
type ReaderModel struct {
    // 当前书籍
    bookID    int
    bookTitle string
    
    // 当前章节
    chapterNum   int
    chapterTitle string
    chapterContent string
    
    // 阅读位置（字符偏移量）
    // offset 表示从章节开头算起，已经向上滚动了多少个字符
    // 0 表示在章节开头，len(content) 表示在章节结尾
    offset int
    
    // 总行数（用于计算百分比）
    totalLines int
    
    // 排版后的行（软换行后的文本行）
    lines []string
    
    // 依赖
    db *db.DB
    
    // 尺寸
    width  int
    height int
}

// NewReaderModel 创建阅读器
func NewReaderModel(database *db.DB) ReaderModel {
    return ReaderModel{db: database}
}

// LoadBook 加载书籍和章节
// 逻辑：
//   1. 调用 db.GetBook(bookID) 获取书籍信息
//   2. 设置 bookID, bookTitle
//   3. 调用 db.GetChapter(bookID, book.CurrentChapter) 加载当前章节
//   4. 设置 chapterNum, chapterTitle, chapterContent
//   5. 设置 offset = book.CurrentOffset
//   6. 调用 reflow() 重新排版
//   7. 返回 tea.Cmd（可以是 nil）
func (m *ReaderModel) LoadBook(bookID int) tea.Cmd { ... }

// LoadChapter 加载指定章节
// 逻辑：
//   1. 调用 db.GetChapter(m.bookID, chapterNum)
//   2. 更新 chapterNum, chapterTitle, chapterContent
//   3. offset = 0（新章节从头开始）
//   4. 调用 reflow()
//   5. 更新数据库当前章节：db.UpdateBookProgress(m.bookID, chapterNum, 0)
func (m *ReaderModel) LoadChapter(chapterNum int) error { ... }

// reflow 重新排版文本
// 将 chapterContent 按终端宽度进行软换行，生成 lines 数组
// 算法：
//   1. 阅读区宽度 = m.width - 4（左右各留 2 字符边距）
//   2. 将 content 按 rune 遍历，每行最多 width 个 rune
//   3. 优先在标点或空格处断行（中文不在字中间断）
//   4. 结果存入 m.lines
//   5. m.totalLines = len(m.lines)
func (m *ReaderModel) reflow() { ... }

// visibleLines 计算当前 offset 对应屏幕上的可见行
// 返回要显示的文本字符串（已用 \n 连接）
// 算法：
//   1. 计算每页可显示行数 = m.height - 3（顶部信息栏 1 + 底部 help 栏 1 + 预留 1）
//   2. offsetLine = m.offset / (m.width - 4)  // 粗略估计所在行
//   3. startLine = offsetLine
//   4. endLine = min(startLine+pageHeight, m.totalLines)
//   5. 返回 strings.Join(m.lines[startLine:endLine], "\n")
func (m *ReaderModel) visibleLines() string { ... }

// scrollDown 向下滚动
// delta: 滚动的字符数（行模式为 width，页模式为 pageSize*width）
func (m *ReaderModel) scrollDown(delta int) { ... }

// scrollUp 向上滚动
func (m *ReaderModel) scrollUp(delta int) { ... }

// readingPercentage 计算阅读百分比
func (m *ReaderModel) readingPercentage() float64 { ... }

// ==================== tea.Model 实现 ====================

func (m ReaderModel) Init() tea.Cmd { return nil }

func (m ReaderModel) Update(msg tea.Msg) (ReaderModel, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch {
        // 下滚
        case key.Matches(msg, ReaderKeys.ScrollDown):
            m.scrollDown(m.width - 4) // 一行
        case key.Matches(msg, ReaderKeys.PageDown):
            m.scrollDown((m.height - 3) * (m.width - 4)) // 一页
        // 上滚
        case key.Matches(msg, ReaderKeys.ScrollUp):
            m.scrollUp(m.width - 4)
        case key.Matches(msg, ReaderKeys.PageUp):
            m.scrollUp((m.height - 3) * (m.width - 4))
        // 上一章
        case key.Matches(msg, ReaderKeys.PrevChapter):
            if m.chapterNum > 1 {
                m.LoadChapter(m.chapterNum - 1)
            }
        // 下一章
        case key.Matches(msg, ReaderKeys.NextChapter):
            m.LoadChapter(m.chapterNum + 1)
        // 章首
        case key.Matches(msg, ReaderKeys.GoStart):
            m.offset = 0
        // 章尾
        case key.Matches(msg, ReaderKeys.GoEnd):
            m.offset = max(0, len(m.chapterContent)-(m.height-3)*(m.width-4))
        // 返回书架
        case key.Matches(msg, ReaderKeys.Back):
            // 保存阅读位置
            m.db.UpdateBookProgress(m.bookID, m.chapterNum, m.offset)
            return m, func() tea.Msg { return CloseReaderMsg{} }
        }
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
        m.reflow()
    }
    return m, nil
}

func (m ReaderModel) View() string {
    if m.bookID == 0 {
        return "加载中..."
    }
    
    // 顶部信息栏
    header := fmt.Sprintf("%s > 第%d章 %s",
        m.bookTitle, m.chapterNum, m.chapterTitle)
    pct := fmt.Sprintf("%.0f%%", m.readingPercentage())
    headerLine := lipgloss.JoinHorizontal(lipgloss.Top,
        ReaderHeaderStyle.Render(header),
        lipgloss.NewStyle().Width(m.width-lipgloss.Width(header)-4).Align(lipgloss.Right).Render(pct),
    )
    
    // 正文区域
    content := ReaderTextStyle.Render(m.visibleLines())
    
    // 底部帮助栏
    help := fmt.Sprintf("←/h/p 上一章  →/l/n 下一章  [b]书架  j/k 滚行  space/f/b 翻页  g/G 首尾  ?帮助")
    helpLine := lipgloss.NewStyle().
        Foreground(lipgloss.Color(ColorMuted)).
        Height(1).
        Render(help)
    
    return lipgloss.JoinVertical(lipgloss.Left,
        headerLine,
        content,
        helpLine,
    )
}

// SetSize 更新尺寸
func (m *ReaderModel) SetSize(width, height int) { ... }
```

### 4.7 `search.go` — 搜索弹层

```go
package tui

import (
    "context"
    
    "github.com/charmbracelet/bubbles/list"
    "github.com/charmbracelet/bubbles/spinner"
    "github.com/charmbracelet/bubbles/textinput"
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
    
    "novel-reader/crawler"
    "novel-reader/db"
)

// SearchResultItem 搜索结果列表项（实现 list.Item）
type SearchResultItem struct {
    result db.SearchResult
}

func (s SearchResultItem) FilterValue() string { return s.result.BookTitle }
func (s SearchResultItem) Title() string { 
    if s.result.Available {
        return lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccess)).Render("✓ ") +
            s.result.BookTitle + "  " + s.result.Author + "  [" + s.result.SourceName + "]"
    }
    return lipgloss.NewStyle().Foreground(lipgloss.Color(ColorMuted)).Render("✗ ") +
        s.result.BookTitle + "  [" + s.result.SourceName + "]"
}
func (s SearchResultItem) Description() string { return "" }

// SearchModel 搜索弹层
type SearchModel struct {
    input       textinput.Model    // 搜索输入框
    resultList  list.Model         // 结果列表
    spinner     spinner.Model      // 搜索中动画
    isSearching bool               // 是否正在搜索
    engine      *crawler.Engine
    width       int
    height      int
}

// NewSearchModel 创建搜索弹层
// 逻辑：
//   1. 创建 textinput.Model：
//        ti := textinput.New()
//        ti.Placeholder = "输入小说名..."
//        ti.Focus()
//        ti.CharLimit = 50
//        ti.Width = 40
//   2. 创建空的 list.Model（搜索结果）
//   3. 创建 spinner.Model，设置样式
//   4. 返回 SearchModel
func NewSearchModel(engine *crawler.Engine) SearchModel { ... }

// searchCmd 返回执行搜索的命令（tea.Cmd）
// 内部启动 goroutine 调用 engine.SearchAll，结果通过 channel 传回
func (m *SearchModel) searchCmd(keyword string) tea.Cmd { ... }

// Init 实现 tea.Model
func (m SearchModel) Init() tea.Cmd {
    return textinput.Blink
}

// Update 处理搜索弹层输入
// 状态：输入中 -> 搜索中 -> 结果展示
// 
// 输入阶段：
//   - 字母/数字：传递给 textinput.Update
//   - Enter：提交搜索，设置 isSearching=true，调用 searchCmd
//   - Esc：发送 CloseSearchMsg
// 
// 搜索阶段：
//   - 接收 SearchResultsMsg（自定义），更新 resultList
//   - isSearching = false
// 
// 结果阶段：
//   - up/down：resultList.CursorUp/Down
//   - Enter：获取选中项，发送 StartCrawlMsg
//   - Esc：发送 CloseSearchMsg
func (m SearchModel) Update(msg tea.Msg) (SearchModel, tea.Cmd) { ... }

// View 渲染搜索弹层
// 布局：
//   居中浮动窗口，宽度 80%，最大 80 字符
//   顶部：标题 + 输入框
//   中间：搜索结果列表（或 spinner）
//   底部：帮助提示
func (m SearchModel) View() string { ... }

// SetSize 更新尺寸
func (m *SearchModel) SetSize(width, height int) { ... }

// ==================== 自定义消息 ====================

// SearchResultsMsg 搜索结果返回
type SearchResultsMsg struct {
    Results []db.SearchResult
}
```

### 4.8 `crawl.go` — 爬取弹窗

```go
package tui

import (
    "context"
    
    "github.com/charmbracelet/bubbles/progress"
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
    
    "novel-reader/crawler"
    "novel-reader/db"
)

// CrawlModel 爬取状态管理
type CrawlModel struct {
    state       CrawlDialogState // confirm / crawling / done
    bookTitle   string
    sourceName  string
    sourceURL   string
    progress    progress.Model
    currentCh   int
    totalCh     int
    chTitle     string
    db          *db.DB
    engine      *crawler.Engine
    bookID      int // 爬取完成后的书籍 ID
}

type CrawlDialogState int

const (
    CrawlConfirm CrawlDialogState = iota
    CrawlProgressing
    CrawlFinished
)

// NewCrawlModel 创建爬取 Model
func NewCrawlModel(database *db.DB, engine *crawler.Engine) CrawlModel { ... }

// Start 设置爬取参数并切换到确认状态
func (m *CrawlModel) Start(bookTitle, sourceName, sourceURL string) { ... }

// crawlCmd 返回执行爬取的 tea.Cmd
// 逻辑：
//   1. 创建 progress channel：ch := make(chan crawler.CrawlProgress, 10)
//   2. 创建 context：ctx, cancel := context.WithCancel(context.Background())
//   3. 启动 goroutine：
//        - 调用 engine.CrawlBook(ctx, m.sourceName, m.sourceURL, ch, m.db, bookID)
//        - 完成后向 channel 发送 Done 信号
//   4. 返回 tea.Cmd：
//        - 循环从 ch 读取进度，每次读取返回 CrawlProgressMsg
//        - 最后返回 CrawlDoneMsg
//   5. 支持取消：外部发送取消消息时调用 cancel()
func (m *CrawlModel) crawlCmd() tea.Cmd { ... }

// Init 实现 tea.Model
func (m CrawlModel) Init() tea.Cmd { return nil }

// Update 处理爬取弹窗交互
// 确认阶段：
//   - Enter：切换到 CrawlProgressing，调用 crawlCmd
//   - Esc：发送 CloseSearchMsg（取消）
// 
// 爬取阶段：
//   - CrawlProgressMsg：更新进度条和当前章节信息
//   - CrawlDoneMsg：切换到 CrawlFinished，如果成功添加书籍到书架
//   - Esc：发送取消信号
// 
// 完成阶段：
//   - Enter/Esc：发送 CloseSearchMsg，回到书架
func (m CrawlModel) Update(msg tea.Msg) (CrawlModel, tea.Cmd) { ... }

// View 渲染爬取弹窗
// 确认阶段：显示书名、来源、提示
// 爬取阶段：显示进度条 "第 X/Y 章 · 章节名 [====>    ] XX%"
// 完成阶段：显示成功/失败信息
func (m CrawlModel) View() string { ... }
```

### 4.9 `help.go` — 帮助弹窗

```go
package tui

import (
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
)

// HelpContext 帮助内容的上下文
type HelpContext int

const (
    HelpBookshelf HelpContext = iota
    HelpReader
    HelpSearch
)

type HelpModel struct {
    context HelpContext
    width   int
    height  int
}

// NewHelpModel 创建帮助弹窗
func NewHelpModel() HelpModel { ... }

// SetContext 设置帮助内容上下文
func (m *HelpModel) SetContext(ctx HelpContext) { ... }

// Init 实现 tea.Model
func (m HelpModel) Init() tea.Cmd { return nil }

// Update 处理帮助弹窗输入
// - Esc / ? / q：发送 CloseHelpMsg
func (m HelpModel) Update(msg tea.Msg) (HelpModel, tea.Cmd) { ... }

// View 渲染帮助弹窗
// 根据 context 显示不同的快捷键说明
// 布局：居中浮动窗口，半透明背景（通过在底层叠加 dimmed 内容实现）
func (m HelpModel) View() string { ... }
```

### 4.10 `toast.go` — Toast 通知

```go
package tui

import "github.com/charmbracelet/lipgloss"

// Toast 临时通知
type Toast struct {
    Content string
    IsError bool
    Visible bool
}

// NewToast 创建 Toast
func NewToast(content string, isError bool) *Toast {
    return &Toast{Content: content, IsError: isError, Visible: true}
}

// View 渲染 Toast
// 顶部居中悬浮，3 秒后自动消失
func (t *Toast) View(width int) string {
    if !t.Visible {
        return ""
    }
    style := ToastSuccessStyle
    if t.IsError {
        style = ToastErrorStyle
    }
    return lipgloss.Place(width, 1, lipgloss.Center, lipgloss.Top,
        style.Render(" "+t.Content+" "),
    )
}
```

Toast 的消失逻辑在 `app.go` 的 Update 中实现：设置一个 3 秒（180 帧 @ 60fps）的定时器，到时设置 `toast.Visible = false`。

---

## 5. 主入口（`main.go`）

```go
package main

import (
    "flag"
    "fmt"
    "os"
    
    tea "github.com/charmbracelet/bubbletea"
    
    "novel-reader/crawler"
    "novel-reader/db"
    "novel-reader/tui"
)

func main() {
    // 1. 解析命令行参数
    dbPath := flag.String("db", "data/novels.db", "数据库文件路径")
    flag.Parse()
    
    // 2. 初始化数据库
    database, err := db.InitDB(*dbPath)
    if err != nil {
        fmt.Fprintf(os.Stderr, "数据库初始化失败: %v\n", err)
        os.Exit(1)
    }
    defer database.Close()
    
    // 3. 运行迁移
    if err := database.Migrate(); err != nil {
        fmt.Fprintf(os.Stderr, "数据库迁移失败: %v\n", err)
        os.Exit(1)
    }
    
    // 4. 初始化爬取引擎
    engine := crawler.NewEngine()
    
    // 5. 创建 TUI 应用
    app := tui.NewApp(database, engine)
    
    // 6. 启动 Bubble Tea
    p := tea.NewProgram(app, 
        tea.WithAltScreen(),       // 使用备用屏幕缓冲区
        tea.WithMouseCellMotion(), // 可选：支持鼠标
    )
    
    if _, err := p.Run(); err != nil {
        fmt.Fprintf(os.Stderr, "运行失败: %v\n", err)
        os.Exit(1)
    }
}
```

---

## 6. 实现路线图（按优先级排序）

### Phase 1：基础设施（必须先完成）

1. **`go.mod`** — 初始化模块，添加所有依赖
2. **`db/models.go`** — 所有数据结构体
3. **`db/database.go`** — 数据库连接 + 迁移 + 建表
4. **`db/queries.go`** — 所有 CRUD 查询函数
5. **`tui/styles.go`** — 全局颜色 + Lipgloss 样式
6. **`tui/keys.go`** — 所有快捷键绑定

### Phase 2：核心 TUI（独立完成，不依赖爬虫）

7. **`tui/bookshelf.go`** — 书架列表（用 mock 数据测试）
8. **`tui/reader.go`** — 阅读器（文本排版 + 滚动是难点，需重点测试）
9. **`tui/help.go`** — 帮助弹窗
10. **`tui/toast.go`** — Toast 通知
11. **`tui/app.go`** — 主 Model + 状态机（集成以上子视图）
12. **`main.go`** — 入口（此时只能用书架 + 空阅读器测试）

**Phase 2 验证点**：
- 启动后看到书架列表（含 mock 书籍数据）
- 按 Enter 进入阅读器，能看到文本内容
- hjkl/space 翻页滚动正常
- `?` 呼出帮助，`q` 退出
- 按 `b` 返回书架

### Phase 3：搜索界面（不依赖真实爬虫，用 mock）

13. **`tui/search.go`** — 搜索弹层（输入框 + 结果列表）
14. **`tui/crawl.go`** — 爬取确认 + 进度弹窗（用 mock 进度测试）
15. 更新 `app.go` 集成搜索状态切换

**Phase 3 验证点**：
- 按 `s` 打开搜索弹层
- 输入文字能看到输入框内容
- 按 Enter "搜索"（mock 延迟 2 秒后返回假结果）
- 能看到 spinner 动画
- 结果显示后可用上下键选择
- 选择后进入确认弹窗
- 确认后显示进度条（mock 进度更新）
- 完成后返回书架

### Phase 4：爬虫引擎（独立开发）

16. **`crawler/crawler.go`** — 接口定义 + 引擎主控
17. **`crawler/sources.go`** — 来源注册
18. **`crawler/biquge.go`** — 笔趣阁解析器（最难，作为模板）
19. **`crawler/x23us.go`** — 顶点小说（复制笔趣阁改选择器）
20. **`crawler/qidian.go`** — 起点中文网（复制笔趣阁改选择器）

**Phase 4 验证点**：
- 单元测试：Search 函数能正确解析 HTML 返回结果
- 单元测试：FetchChapterList 能提取章节列表
- 单元测试：FetchChapterContent 能提取正文
- 集成测试：SearchAll 并发搜索多个来源

### Phase 5：集成与打磨

21. 连接爬虫和 TUI：搜索调用真实 SearchAll，爬取调用真实 CrawlBook
22. 添加真实书籍数据（搜索一本小说完整走通流程）
23. 边界情况处理：空书架、网络错误、爬取中断、大章节
24. 性能优化：大数据库查询、大章节内存管理
25. 最终测试：完整用户流程验证

---

## 7. 关键技术难点与解决方案

### 7.1 阅读器文本排版（`reader.go` 的 `reflow`）

**难点**：中文字符和英文字符混合时的软换行，需要正确处理 rune 宽度。

**解决方案**：
- 使用 `unicode/utf8` 包处理 rune
- 中文字符宽度 = 2 终端列，ASCII = 1 列（使用 `github.com/mattn/go-runewidth` 或手动判断）
- 换行策略：优先在空格或标点处断行，如果一行全是中文则在字符边界硬断
- 预先将 content 转为 []rune，按视觉宽度累计，超过阈值时断行

```go
// reflow 伪代码
func (m *ReaderModel) reflow() {
    maxWidth := m.width - 4 // 左右边距
    runes := []rune(m.chapterContent)
    var lines []string
    var line []rune
    var lineWidth int
    
    for i := 0; i < len(runes); i++ {
        w := runeWidth(runes[i])
        if lineWidth+w > maxWidth && len(line) > 0 {
            lines = append(lines, string(line))
            line = nil
            lineWidth = 0
        }
        line = append(line, runes[i])
        lineWidth += w
    }
    if len(line) > 0 {
        lines = append(lines, string(line))
    }
    m.lines = lines
    m.totalLines = len(lines)
}
```

### 7.2 跨 goroutine 的 Bubble Tea 消息传递

**难点**：爬虫在后台 goroutine 中运行，需要将进度实时传递给 Bubble Tea 的 Update 循环。

**解决方案**：使用 `tea.Cmd` 返回 channel 读取函数。

```go
// 在 crawl.go 中
func (m *CrawlModel) crawlCmd() tea.Cmd {
    progressCh := make(chan crawler.CrawlProgress, 10)
    ctx, cancel := context.WithCancel(context.Background())
    m.cancelFunc = cancel // 保存以便取消
    
    // 在后台 goroutine 执行爬取
    go func() {
        defer close(progressCh)
        bookID, _ := m.db.AddBook(m.bookTitle, "", "", 0, m.sourceURL, m.sourceName)
        m.bookID = bookID
        m.engine.CrawlBook(ctx, m.sourceName, m.sourceURL, progressCh, m.db, bookID)
    }()
    
    // 返回 tea.Cmd：从 channel 读取并转为 tea.Msg
    return func() tea.Msg {
        progress := <-progressCh
        return CrawlProgressMsg(progress)
    }
}

// 在 Update 中处理进度消息后，如果未 Done，再次调用 crawlCmd 继续读取
func (m CrawlModel) Update(msg tea.Msg) (CrawlModel, tea.Cmd) {
    switch msg := msg.(type) {
    case CrawlProgressMsg:
        m.currentCh = msg.CurrentChapter
        m.totalCh = msg.TotalChapters
        m.chTitle = msg.ChapterTitle
        if msg.Done {
            return m, func() tea.Msg { return CrawlDoneMsg{BookID: m.bookID} }
        }
        // 继续读取下一个进度
        return m, m.crawlCmdContinue()
    }
}
```

### 7.3 SQLite 并发写入

**难点**：Bubble Tea 的 Update 在主 goroutine 中运行，而爬虫在后台 goroutine 中写入数据库。

**解决方案**：
- 数据库连接池设为 `MaxOpenConns(1)`，SQLite 会自动串行化写入
- 或者：爬虫只通过 channel 报告进度，所有数据库写入操作都在主 goroutine 的 Update 中执行
- **推荐方案**：爬虫 goroutine 只负责 HTTP 请求和 HTML 解析，通过 channel 将章节内容传回，主 goroutine 负责写入数据库。这样避免并发写入问题。

### 7.4 终端尺寸变化

所有子 Model 必须实现 `SetSize(width, height int)` 方法。在 `app.go` 的 `WindowSizeMsg` 处理中同步更新所有子 Model 的尺寸。

---

## 8. 依赖清单（`go.mod`）

```
module novel-reader

go 1.21

require (
    github.com/charmbracelet/bubbletea v0.25.0
    github.com/charmbracelet/bubbles v0.18.0
    github.com/charmbracelet/lipgloss v0.9.1
    golang.org/x/net v0.20.0
    modernc.org/sqlite v1.28.0
)
```

安装命令：
```bash
go mod init novel-reader
go get github.com/charmbracelet/bubbletea@v0.25.0
go get github.com/charmbracelet/bubbles@v0.18.0
go get github.com/charmbracelet/lipgloss@v0.9.1
go get golang.org/x/net@v0.20.0
go get modernc.org/sqlite@v1.28.0
```

---

## 9. 测试策略

### 9.1 单元测试

每个文件对应一个 `_test.go` 文件：

```
db/queries_test.go       # 数据库 CRUD 测试（使用内存 SQLite :memory:）
crawler/biquge_test.go   # 笔趣阁解析器测试（使用本地 HTML fixture）
tui/reader_test.go       # 阅读器排版测试
```

### 9.2 集成测试

```bash
# 测试完整搜索+爬取流程（使用真实网络请求）
go test -v ./crawler/ -run TestIntegration -timeout 60s

# 测试完整 TUI 流程（发送 tea.Msg 模拟用户操作）
go test -v ./tui/ -run TestAppFlow
```

### 9.3 手动测试清单

1. 首次启动：空数据库，显示空书架提示
2. 搜索一本书："诡秘之主"，验证多个来源返回结果
3. 爬取一本书：验证进度条更新，完成后书架显示新书
4. 打开书阅读：验证文本显示、翻页、章节切换
5. 退出重进：验证阅读位置保存和恢复
6. 删除书籍：验证确认弹窗，删除后书架更新
7. 终端缩放：调整终端大小，验证布局自适应
8. 大章节测试：验证大文本不卡顿
