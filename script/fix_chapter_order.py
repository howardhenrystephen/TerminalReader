#!/usr/bin/env python3
"""
修复章节表顺序脚本

问题：并发爬取导致章节入库顺序错乱（按完成时间而非章节号顺序）
修复：按 chapter_num 排序后重建表，使 id 自增顺序与 chapter_num 一致

用法：
    python3 script/fix_chapter_order.py [数据库路径]

默认数据库路径：./data/novels.db
"""

import os
import sqlite3
import sys


def get_chapter_tables(conn):
    """获取所有章节表名"""
    cursor = conn.execute(
        "SELECT name FROM sqlite_master WHERE type='table' AND name LIKE 'chapters_%' ORDER BY name"
    )
    return [row[0] for row in cursor.fetchall()]


def get_book_info(conn, table_name):
    """根据章节表名获取书籍信息"""
    book_id = int(table_name.replace("chapters_", ""))
    cursor = conn.execute(
        "SELECT id, title, total_chapters FROM books WHERE id = ?", (book_id,)
    )
    row = cursor.fetchone()
    if row:
        return {"id": row[0], "title": row[1], "total_chapters": row[2]}
    return None


def analyze_table(conn, table_name):
    """分析章节表状态"""
    # 总记录数
    cursor = conn.execute(f"SELECT COUNT(*) FROM {table_name}")
    total = cursor.fetchone()[0]

    # 去重后的章节数
    cursor = conn.execute(f"SELECT COUNT(DISTINCT chapter_num) FROM {table_name}")
    distinct = cursor.fetchone()[0]

    # 查找缺失的章节号
    cursor = conn.execute(f"SELECT chapter_num FROM {table_name} ORDER BY chapter_num")
    existing = [row[0] for row in cursor.fetchall()]

    missing = []
    if existing:
        expected = set(range(1, max(existing) + 1))
        actual = set(existing)
        missing = sorted(expected - actual)

    # 查找重复的章节号
    cursor = conn.execute(
        f"SELECT chapter_num, COUNT(*) as cnt FROM {table_name} GROUP BY chapter_num HAVING cnt > 1"
    )
    duplicates = [(row[0], row[1]) for row in cursor.fetchall()]

    # 检查是否按 id 顺序与 chapter_num 一致
    cursor = conn.execute(
        f"SELECT id, chapter_num FROM {table_name} ORDER BY id LIMIT 20"
    )
    first_20 = cursor.fetchall()

    is_ordered = all(i + 1 == row[1] for i, row in enumerate(first_20))

    return {
        "total": total,
        "distinct": distinct,
        "missing": missing,
        "duplicates": duplicates,
        "is_ordered": is_ordered,
        "max_chapter": max(existing) if existing else 0,
    }


def fix_table(conn, table_name):
    """修复单张章节表：按 chapter_num 排序后重建，使 id 与 chapter_num 对齐"""
    temp_table = f"{table_name}_temp"

    # 获取原表结构
    cursor = conn.execute(f"PRAGMA table_info({table_name})")
    columns = cursor.fetchall()

    # 构建列定义（排除 id，因为它是自增主键）
    col_defs = []
    col_names = []
    for col in columns:
        cid, name, ctype, notnull, dflt_value, pk = col
        if name == "id":
            continue
        col_names.append(name)
        def_str = f"{name} {ctype}"
        if notnull:
            def_str += " NOT NULL"
        if dflt_value is not None:
            def_str += f" DEFAULT {dflt_value}"
        col_defs.append(def_str)

    # 创建临时表（id 自增，其他列与原表一致）
    create_sql = f"""CREATE TABLE {temp_table} (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        {", ".join(col_defs)}
    )"""
    conn.execute(create_sql)

    # 按 chapter_num 排序，将数据插入临时表（不指定 id，让自增生成）
    insert_cols = ", ".join(col_names)
    conn.execute(f"""
        INSERT INTO {temp_table} ({insert_cols})
        SELECT {insert_cols} FROM {table_name}
        ORDER BY chapter_num ASC
    """)

    # 删除原表，重命名临时表
    conn.execute(f"DROP TABLE {table_name}")
    conn.execute(f"ALTER TABLE {temp_table} RENAME TO {table_name}")


def main():
    db_path = sys.argv[1] if len(sys.argv) > 1 else os.path.join("data", "novels.db")

    if not os.path.exists(db_path):
        print(f"错误：数据库文件不存在: {db_path}")
        sys.exit(1)

    conn = sqlite3.connect(db_path)

    tables = get_chapter_tables(conn)
    if not tables:
        print("未找到章节表")
        conn.close()
        return

    print(f"发现 {len(tables)} 张章节表")
    print("=" * 60)

    # 先分析所有表
    need_fix = []
    for table in tables:
        book = get_book_info(conn, table)
        info = analyze_table(conn, table)

        title = book["title"] if book else "未知"
        book_id = book["id"] if book else "?"

        print(f"\n📚 {title} (ID: {book_id}, 表: {table})")
        print(f"   总记录数: {info['total']}, 去重章节数: {info['distinct']}")
        print(f"   最大章节号: {info['max_chapter']}")

        if info["missing"]:
            print(f"   ⚠️ 缺失章节: {info['missing']}")
        if info["duplicates"]:
            dup_str = ", ".join(f"第{ch}章({cnt}条)" for ch, cnt in info["duplicates"])
            print(f"   ⚠️ 重复章节: {dup_str}")

        if info["is_ordered"] and not info["duplicates"]:
            print(f"   ✅ 顺序正常，无需修复")
        else:
            print(f"   🔧 需要修复")
            need_fix.append((table, title, info))

    if not need_fix:
        print("\n所有表顺序正常，无需修复")
        conn.close()
        return

    print("\n" + "=" * 60)
    print(f"需要修复 {len(need_fix)} 张表")

    # 确认修复
    confirm = input("\n确认修复? [y/N]: ").strip().lower()
    if confirm not in ("y", "yes"):
        print("已取消")
        conn.close()
        return

    # 执行修复
    for table, title, info in need_fix:
        print(f"\n🔧 修复: {title} ({table})...")

        # 修复前记录数
        cursor = conn.execute(f"SELECT COUNT(*) FROM {table}")
        before_count = cursor.fetchone()[0]

        try:
            fix_table(conn, table)

            # 修复后记录数
            cursor = conn.execute(f"SELECT COUNT(*) FROM {table}")
            after_count = cursor.fetchone()[0]

            # 验证前几条
            cursor = conn.execute(
                f"SELECT id, chapter_num FROM {table} ORDER BY id LIMIT 5"
            )
            first_rows = cursor.fetchall()

            print(f"   修复前记录数: {before_count}, 修复后: {after_count}")
            print(f"   验证: {first_rows}")
            print(f"   ✅ 修复完成")

        except Exception as e:
            print(f"   ❌ 修复失败: {e}")
            conn.rollback()
            conn.close()
            sys.exit(1)

    conn.commit()
    conn.close()
    print("\n🎉 所有修复已完成并提交")


if __name__ == "__main__":
    main()
