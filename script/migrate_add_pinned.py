#!/usr/bin/env python3
"""
数据库迁移脚本：为 books 表添加 pinned 列

用法：
    python3 script/migrate_add_pinned.py [数据库路径]

如果不指定路径，默认使用 data/novels.db
"""

import os
import sqlite3
import sys


def migrate(db_path):
    if not os.path.exists(db_path):
        print(f"数据库不存在: {db_path}")
        sys.exit(1)

    conn = sqlite3.connect(db_path)
    cursor = conn.cursor()

    # 检查 books 表是否存在
    cursor.execute(
        "SELECT name FROM sqlite_master WHERE type='table' AND name='books';"
    )
    if not cursor.fetchone():
        print(f"books 表不存在于 {db_path}")
        conn.close()
        sys.exit(1)

    # 检查 pinned 列是否已存在
    cursor.execute("PRAGMA table_info(books);")
    columns = [row[1] for row in cursor.fetchall()]

    if "pinned" in columns:
        print(f"✓ pinned 列已存在于 {db_path}，无需迁移")
        conn.close()
        return

    # 添加 pinned 列
    cursor.execute("ALTER TABLE books ADD COLUMN pinned INTEGER DEFAULT 0;")
    conn.commit()
    print(f"✓ 成功添加 pinned 列到 {db_path}")

    # 验证
    cursor.execute("PRAGMA table_info(books);")
    columns = [row[1] for row in cursor.fetchall()]
    if "pinned" in columns:
        print(f"✓ 验证通过: pinned 列已确认添加")
    else:
        print(f"✗ 验证失败: pinned 列未找到")

    conn.close()


if __name__ == "__main__":
    if len(sys.argv) > 1:
        db_path = sys.argv[1]
    else:
        db_path = os.path.join("data", "novels.db")

    migrate(db_path)
