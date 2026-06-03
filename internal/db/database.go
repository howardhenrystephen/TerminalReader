package db

import (
	"database/sql"
	"fmt"

	"github.com/henry/novel-reader/pkg/logger"

	_ "modernc.org/sqlite"
)

// DB 封装数据库连接
type DB struct {
	conn *sql.DB
}

// InitDB 初始化数据库连接并执行迁移
func InitDB(dbPath string) (*DB, error) {
	logger.Infof("[DB] 打开数据库: %s", dbPath)
	// 启用 WAL 模式和 busy timeout，避免并发读写时的 SQLITE_BUSY 错误
	conn, err := sql.Open("sqlite", dbPath+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		logger.Errorf("[DB] 打开数据库失败: %v", err)
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := conn.Ping(); err != nil {
		logger.Errorf("[DB] 数据库 Ping 失败: %v", err)
		return nil, fmt.Errorf("ping db: %w", err)
	}
	logger.Infof("[DB] 数据库连接成功")
	db := &DB{conn: conn}
	if err := db.Migrate(); err != nil {
		logger.Errorf("[DB] 数据库迁移失败: %v", err)
		return nil, fmt.Errorf("migrate: %w", err)
	}
	logger.Infof("[DB] 数据库迁移完成")
	return db, nil
}

// Close 关闭数据库连接
func (d *DB) Close() error {
	return d.conn.Close()
}

// Migrate 创建基础表结构
func (d *DB) Migrate() error {
	if _, err := d.exec(sqlCreateBooksTable); err != nil {
		return fmt.Errorf("create books table: %w", err)
	}
	if _, err := d.exec(sqlCreateBooksTrigger); err != nil {
		return fmt.Errorf("create trigger: %w", err)
	}
	if _, err := d.exec(sqlCreateBookSourcesTable); err != nil {
		return fmt.Errorf("create book_sources table: %w", err)
	}
	// 迁移：为旧数据库添加 pinned 列
	if _, err := d.exec(`ALTER TABLE books ADD COLUMN pinned INTEGER DEFAULT 0;`); err != nil {
		// 列已存在时会报错，忽略
		logger.Debugf("[DB] pinned 列可能已存在: %v", err)
	}
	return nil
}

// CreateChapterTable 为指定书籍创建章节表
func (d *DB) CreateChapterTable(bookID int64) error {
	table := chapterTableName(bookID)
	logger.Infof("[DB] 创建章节表: %s", table)
	sql := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		chapter_num INTEGER NOT NULL,
		title TEXT NOT NULL,
		content TEXT NOT NULL,
		source_url TEXT,
		word_count INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`, table)
	if _, err := d.exec(sql); err != nil {
		logger.Errorf("[DB] 创建章节表失败: %v", err)
		return err
	}
	// 迁移：为旧表添加 source_url 列
	if _, err := d.exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN source_url TEXT;", table)); err != nil {
		logger.Debugf("[DB] source_url 列可能已存在: %v", err)
	}
	return nil
}

// DropChapterTable 删除指定书籍的章节表
func (d *DB) DropChapterTable(bookID int64) error {
	logger.Infof("[DB] 删除章节表: %s", chapterTableName(bookID))
	sql := fmt.Sprintf("DROP TABLE IF EXISTS %s;", chapterTableName(bookID))
	_, err := d.exec(sql)
	if err != nil {
		logger.Errorf("[DB] 删除章节表失败: %v", err)
	}
	return err
}

func (d *DB) exec(query string, args ...interface{}) (sql.Result, error) {
	return d.conn.Exec(query, args...)
}

func (d *DB) query(query string, args ...interface{}) (*sql.Rows, error) {
	return d.conn.Query(query, args...)
}

func (d *DB) queryRow(query string, args ...interface{}) *sql.Row {
	return d.conn.QueryRow(query, args...)
}
