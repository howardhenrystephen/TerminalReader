#!/usr/bin/env python3
"""
从所有 chapters_* 表的 content 字段第一行提取真实章节标题，更新 title 字段。

content 第一行格式如：
  "第3章 一日三境"
  "第3章一日三境"

title 字段当前存储的是：
  笔趣阁：章节列表标题（可能不准确）
  爱下电子书：固定 "第X章"（无真实标题）

修复后 title 字段将存储 content 中提取的真实章节标题（不含 "第X章" 前缀）。
"""

import re
import sqlite3
import sys


def extract_title_from_content(content: str) -> str:
    """从 content 第一行提取章节标题（去掉 '第X章' 前缀）。"""
    if not content:
        return ""
    lines = content.strip().split("\n")
    for line in lines:
        line = line.strip()
        if not line:
            continue
        # 匹配 "第X章 标题" 或 "第X章标题"
        m = re.match(r"^第[零一二三四五六七八九十百千万亿\d]+章\s*(.*)$", line)
        if m:
            return m.group(1).strip()
        # 如果第一行不是 "第X章" 格式，直接返回第一行作为标题
        return line
    return ""


def migrate(db_path: str):
    conn = sqlite3.connect(db_path)
    cursor = conn.cursor()

    # 获取所有 chapters_* 表
    cursor.execute(
        "SELECT name FROM sqlite_master WHERE type='table' AND name LIKE 'chapters_%'"
    )
    tables = [row[0] for row in cursor.fetchall()]

    if not tables:
        print("没有找到 chapters_* 表，无需处理。")
        return

    total_updated = 0
    for table in tables:
        cursor.execute(f"SELECT chapter_num, title, content FROM {table}")
        rows = cursor.fetchall()
        updated = 0
        for chapter_num, old_title, content in rows:
            real_title = extract_title_from_content(content)
            if real_title and real_title != old_title:
                cursor.execute(
                    f"UPDATE {table} SET title = ? WHERE chapter_num = ?",
                    (real_title, chapter_num),
                )
                updated += 1
        conn.commit()
        total_updated += updated
        print(f"  {table}: 更新了 {updated} 条记录")

    print(f"\n总计更新: {total_updated} 条记录")
    conn.close()


if __name__ == "__main__":
    db_path = sys.argv[1] if len(sys.argv) > 1 else "data/novels.db"
    print(f"处理数据库: {db_path}")
    migrate(db_path)
