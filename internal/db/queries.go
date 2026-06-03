package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/henry/novel-reader/pkg/logger"
)

const sqlCreateBooksTable = `CREATE TABLE IF NOT EXISTS books (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	title TEXT NOT NULL,
	author TEXT,
	description TEXT,
	total_chapters INTEGER DEFAULT 0,
	current_chapter INTEGER DEFAULT 1,
	current_offset INTEGER DEFAULT 0,
	source_url TEXT,
	source_site TEXT,
	pinned INTEGER DEFAULT 0,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);`

const sqlMigrateChapterSourceURL = `ALTER TABLE %s ADD COLUMN source_url TEXT;`

const sqlCreateBooksTrigger = `CREATE TRIGGER IF NOT EXISTS update_books_updated_at
AFTER UPDATE ON books
FOR EACH ROW
BEGIN
	UPDATE books SET updated_at = CURRENT_TIMESTAMP WHERE id = OLD.id;
END;`

const sqlCreateBookSourcesTable = `CREATE TABLE IF NOT EXISTS book_sources (
	book_id INTEGER PRIMARY KEY,
	source_url TEXT NOT NULL,
	source_name TEXT,
	last_crawled_chapter INTEGER DEFAULT 0,
	updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (book_id) REFERENCES books(id) ON DELETE CASCADE
);`

const sqlUpsertBookSource = `INSERT INTO book_sources (book_id, source_url, source_name, last_crawled_chapter) VALUES (?, ?, ?, ?)
ON CONFLICT(book_id) DO UPDATE SET source_url=excluded.source_url, source_name=excluded.source_name, last_crawled_chapter=excluded.last_crawled_chapter, updated_at=CURRENT_TIMESTAMP;`

const sqlGetBookSource = `SELECT book_id, source_url, source_name, last_crawled_chapter, updated_at FROM book_sources WHERE book_id = ?;`

const sqlUpdateLastCrawledChapter = `UPDATE book_sources SET last_crawled_chapter = ? WHERE book_id = ?;`

const sqlListBooks = `SELECT id, title, author, description, total_chapters, current_chapter, current_offset, source_url, source_site, pinned, created_at, updated_at FROM books ORDER BY pinned DESC, updated_at DESC;`

const sqlGetBook = `SELECT id, title, author, description, total_chapters, current_chapter, current_offset, source_url, source_site, pinned, created_at, updated_at FROM books WHERE id = ?;`

const sqlGetBookByTitle = `SELECT id, title, author, description, total_chapters, current_chapter, current_offset, source_url, source_site, pinned, created_at, updated_at FROM books WHERE title = ? ORDER BY id DESC LIMIT 1;`

const sqlInsertBook = `INSERT INTO books (title, author, description, total_chapters, source_url, source_site) VALUES (?, ?, ?, ?, ?, ?);`

const sqlUpdateProgress = `UPDATE books SET current_chapter = ?, current_offset = ? WHERE id = ?;`

const sqlUpdateTotalChapters = `UPDATE books SET total_chapters = ? WHERE id = ?;`

const sqlUpdatePin = `UPDATE books SET pinned = ? WHERE id = ?;`

const sqlDeleteBook = `DELETE FROM books WHERE id = ?;`

const sqlDeleteBookSource = `DELETE FROM book_sources WHERE book_id = ?;`

// ListBooks 返回所有书籍
func (d *DB) ListBooks() ([]Book, error) {
	logger.Debugf("[DB] 查询所有书籍")
	rows, err := d.query(sqlListBooks)
	if err != nil {
		logger.Errorf("[DB] 查询所有书籍失败: %v", err)
		return nil, err
	}
	defer rows.Close()

	var books []Book
	for rows.Next() {
		var b Book
		var createdAt, updatedAt string
		var pinned int
		err := rows.Scan(&b.ID, &b.Title, &b.Author, &b.Description, &b.TotalChapters, &b.CurrentChapter, &b.CurrentOffset, &b.SourceURL, &b.SourceSite, &pinned, &createdAt, &updatedAt)
		if err != nil {
			return nil, err
		}
		b.Pinned = pinned != 0
		b.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		b.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		// 查询已下载章节数
		b.DownloadedChapters, _ = d.GetChapterCount(b.ID)
		books = append(books, b)
	}
	logger.Debugf("[DB] 查询到 %d 本书", len(books))
	return books, rows.Err()
}

// GetBook 根据 ID 获取书籍
func (d *DB) GetBook(id int64) (*Book, error) {
	logger.Debugf("[DB] 查询书籍 ID=%d", id)
	var b Book
	var createdAt, updatedAt string
	var pinned int
	err := d.queryRow(sqlGetBook, id).Scan(&b.ID, &b.Title, &b.Author, &b.Description, &b.TotalChapters, &b.CurrentChapter, &b.CurrentOffset, &b.SourceURL, &b.SourceSite, &pinned, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			logger.Debugf("[DB] 书籍 ID=%d 不存在", id)
			return nil, nil
		}
		logger.Errorf("[DB] 查询书籍 ID=%d 失败: %v", id, err)
		return nil, err
	}
	b.Pinned = pinned != 0
	b.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	b.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &b, nil
}

// GetBookByTitle 根据书名获取最新一本书
func (d *DB) GetBookByTitle(title string) (*Book, error) {
	logger.Debugf("[DB] 根据书名查询: %s", title)
	var b Book
	var createdAt, updatedAt string
	var pinned int
	err := d.queryRow(sqlGetBookByTitle, title).Scan(&b.ID, &b.Title, &b.Author, &b.Description, &b.TotalChapters, &b.CurrentChapter, &b.CurrentOffset, &b.SourceURL, &b.SourceSite, &pinned, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			logger.Debugf("[DB] 书名 '%s' 不存在", title)
			return nil, nil
		}
		logger.Errorf("[DB] 查询书名 '%s' 失败: %v", title, err)
		return nil, err
	}
	b.Pinned = pinned != 0
	b.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	b.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &b, nil
}

// UpdateBookPin 更新书籍置顶状态
func (d *DB) UpdateBookPin(id int64, pinned bool) error {
	logger.Infof("[DB] 更新置顶状态: bookID=%d, pinned=%v", id, pinned)
	var pinVal int
	if pinned {
		pinVal = 1
	}
	_, err := d.exec(sqlUpdatePin, pinVal, id)
	if err != nil {
		logger.Errorf("[DB] 更新置顶状态失败: bookID=%d, %v", id, err)
	}
	return err
}

// AddBook 添加新书
func (d *DB) AddBook(title, author, description string, totalChapters int, sourceURL, sourceSite string) (int64, error) {
	logger.Infof("[DB] 添加新书: %s (作者: %s, 总章节: %d)", title, author, totalChapters)
	res, err := d.exec(sqlInsertBook, title, author, description, totalChapters, sourceURL, sourceSite)
	if err != nil {
		logger.Errorf("[DB] 添加新书 '%s' 失败: %v", title, err)
		return 0, err
	}
	id, _ := res.LastInsertId()
	logger.Infof("[DB] 新书 '%s' 添加成功, ID=%d", title, id)
	return id, nil
}

// UpdateBookProgress 更新阅读进度
func (d *DB) UpdateBookProgress(id int64, chapter, offset int) error {
	logger.Debugf("[DB] 更新阅读进度: bookID=%d, chapter=%d, offset=%d", id, chapter, offset)
	_, err := d.exec(sqlUpdateProgress, chapter, offset, id)
	if err != nil {
		logger.Errorf("[DB] 更新阅读进度失败: bookID=%d, %v", id, err)
	}
	return err
}

// UpdateBookTotalChapters 更新总章节数
func (d *DB) UpdateBookTotalChapters(id int64, total int) error {
	logger.Debugf("[DB] 更新总章节数: bookID=%d, total=%d", id, total)
	_, err := d.exec(sqlUpdateTotalChapters, total, id)
	if err != nil {
		logger.Errorf("[DB] 更新总章节数失败: bookID=%d, %v", id, err)
	}
	return err
}

// DeleteBook 删除书籍
func (d *DB) DeleteBook(id int64) error {
	logger.Infof("[DB] 删除书籍: ID=%d", id)
	_, err := d.exec(sqlDeleteBook, id)
	if err != nil {
		logger.Errorf("[DB] 删除书籍 ID=%d 失败: %v", id, err)
		return err
	}
	return d.DropChapterTable(id)
}

func chapterTableName(bookID int64) string {
	return fmt.Sprintf("chapters_%d", bookID)
}

// InsertChapter 插入单章（忽略已存在的章节）
func (d *DB) InsertChapter(bookID int64, ch Chapter) error {
	logger.Debugf("[DB] 插入章节: bookID=%d, chapterNum=%d, title=%s", bookID, ch.ChapterNum, ch.Title)
	sql := fmt.Sprintf("INSERT OR IGNORE INTO %s (chapter_num, title, content, source_url, word_count) VALUES (?, ?, ?, ?, ?);", chapterTableName(bookID))
	_, err := d.exec(sql, ch.ChapterNum, ch.Title, ch.Content, ch.SourceURL, ch.WordCount)
	if err != nil {
		logger.Errorf("[DB] 插入章节失败: bookID=%d, chapterNum=%d, %v", bookID, ch.ChapterNum, err)
	}
	return err
}

// BulkInsertChapters 批量插入章节
func (d *DB) BulkInsertChapters(bookID int64, chapters []Chapter) error {
	logger.Infof("[DB] 批量插入章节: bookID=%d, 数量=%d", bookID, len(chapters))
	table := chapterTableName(bookID)
	tx, err := d.conn.Begin()
	if err != nil {
		logger.Errorf("[DB] 批量插入章节开启事务失败: %v", err)
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(fmt.Sprintf("INSERT INTO %s (chapter_num, title, content, source_url, word_count) VALUES (?, ?, ?, ?, ?);", table))
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, ch := range chapters {
		if _, err := stmt.Exec(ch.ChapterNum, ch.Title, ch.Content, ch.SourceURL, ch.WordCount); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		logger.Errorf("[DB] 批量插入章节提交事务失败: %v", err)
		return err
	}
	logger.Infof("[DB] 批量插入章节完成: bookID=%d", bookID)
	return nil
}

// GetChapter 获取指定章节
func (d *DB) GetChapter(bookID int64, chapterNum int) (*Chapter, error) {
	logger.Debugf("[DB] 获取章节: bookID=%d, chapterNum=%d", bookID, chapterNum)
	query := fmt.Sprintf("SELECT id, chapter_num, title, content, source_url, word_count, created_at FROM %s WHERE chapter_num = ?;", chapterTableName(bookID))
	var ch Chapter
	var createdAt string
	err := d.queryRow(query, chapterNum).Scan(&ch.ID, &ch.ChapterNum, &ch.Title, &ch.Content, &ch.SourceURL, &ch.WordCount, &createdAt)
	if err != nil {
		if err == sql.ErrNoRows {
			logger.Debugf("[DB] 章节不存在: bookID=%d, chapterNum=%d", bookID, chapterNum)
			return nil, nil
		}
		logger.Errorf("[DB] 获取章节失败: bookID=%d, chapterNum=%d, %v", bookID, chapterNum, err)
		return nil, err
	}
	ch.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	return &ch, nil
}

// GetChapterCount 获取章节总数
func (d *DB) GetChapterCount(bookID int64) (int, error) {
	logger.Debugf("[DB] 获取章节总数: bookID=%d", bookID)
	sql := fmt.Sprintf("SELECT COUNT(*) FROM %s;", chapterTableName(bookID))
	var count int
	err := d.queryRow(sql).Scan(&count)
	if err != nil {
		logger.Errorf("[DB] 获取章节总数失败: bookID=%d, %v", bookID, err)
	}
	return count, err
}

// ListChapterTitles 返回章节序号和标题列表
func (d *DB) ListChapterTitles(bookID int64) ([]struct {
	Num   int
	Title string
}, error) {
	logger.Debugf("[DB] 获取章节标题列表: bookID=%d", bookID)
	sql := fmt.Sprintf("SELECT chapter_num, title FROM %s ORDER BY chapter_num;", chapterTableName(bookID))
	rows, err := d.query(sql)
	if err != nil {
		logger.Errorf("[DB] 获取章节标题列表失败: bookID=%d, %v", bookID, err)
		return nil, err
	}
	defer rows.Close()

	var list []struct {
		Num   int
		Title string
	}
	for rows.Next() {
		var item struct {
			Num   int
			Title string
		}
		if err := rows.Scan(&item.Num, &item.Title); err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	return list, rows.Err()
}

// ChapterPreview 章节预览信息
type ChapterPreview struct {
	Num     int
	Title   string
	Preview string
}

// ListChaptersWithPreview 返回章节列表，包含内容第一行作为预览
func (d *DB) ListChaptersWithPreview(bookID int64) ([]ChapterPreview, error) {
	logger.Debugf("[DB] 获取章节预览列表: bookID=%d", bookID)
	sql := fmt.Sprintf("SELECT chapter_num, title, content FROM %s ORDER BY chapter_num;", chapterTableName(bookID))
	rows, err := d.query(sql)
	if err != nil {
		logger.Errorf("[DB] 获取章节预览列表失败: bookID=%d, %v", bookID, err)
		return nil, err
	}
	defer rows.Close()

	var list []ChapterPreview
	for rows.Next() {
		var item ChapterPreview
		var content string
		if err := rows.Scan(&item.Num, &item.Title, &content); err != nil {
			return nil, err
		}
		item.Preview = firstLineOfContent(content)
		list = append(list, item)
	}
	return list, rows.Err()
}

func firstLineOfContent(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}

// UpsertBookSource 插入或更新书籍来源信息
func (d *DB) UpsertBookSource(bookID int64, sourceURL, sourceName string, lastCrawledChapter int) error {
	logger.Debugf("[DB] 更新书籍来源: bookID=%d, source=%s, lastChapter=%d", bookID, sourceName, lastCrawledChapter)
	_, err := d.exec(sqlUpsertBookSource, bookID, sourceURL, sourceName, lastCrawledChapter)
	if err != nil {
		logger.Errorf("[DB] 更新书籍来源失败: bookID=%d, %v", bookID, err)
	}
	return err
}

// GetBookSource 获取书籍来源信息
func (d *DB) GetBookSource(bookID int64) (*BookSource, error) {
	logger.Debugf("[DB] 获取书籍来源: bookID=%d", bookID)
	var bs BookSource
	var updatedAt string
	err := d.queryRow(sqlGetBookSource, bookID).Scan(&bs.BookID, &bs.SourceURL, &bs.SourceName, &bs.LastCrawledChapter, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			logger.Debugf("[DB] 书籍来源不存在: bookID=%d", bookID)
			return nil, nil
		}
		logger.Errorf("[DB] 获取书籍来源失败: bookID=%d, %v", bookID, err)
		return nil, err
	}
	bs.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &bs, nil
}

// UpdateLastCrawledChapter 更新最后爬取章节
func (d *DB) UpdateLastCrawledChapter(bookID int64, chapter int) error {
	logger.Debugf("[DB] 更新最后爬取章节: bookID=%d, chapter=%d", bookID, chapter)
	_, err := d.exec(sqlUpdateLastCrawledChapter, chapter, bookID)
	if err != nil {
		logger.Errorf("[DB] 更新最后爬取章节失败: bookID=%d, %v", bookID, err)
	}
	return err
}
