"""
Python 爬虫服务（支持多线程并发下载）
用于处理 Node.js 无法连接的 TLS 问题
通过 stdio 与 Node.js 通信
"""

import json
import os
import re
import sys
import time
from concurrent.futures import ThreadPoolExecutor, as_completed
from urllib.parse import urljoin, urlparse

# 尝试导入 cloudscraper
try:
    import cloudscraper

    CLOUDSCRAPER_AVAILABLE = True
except ImportError:
    CLOUDSCRAPER_AVAILABLE = False

# 线程安全的 scraper 存储（每个线程一个）
_thread_scrapers = {}


def get_scraper():
    """获取当前线程的 scraper 实例"""
    import threading

    tid = threading.current_thread().ident
    if tid not in _thread_scrapers:
        _thread_scrapers[tid] = cloudscraper.create_scraper(
            browser={"browser": "chrome", "platform": "windows", "desktop": True}
        )
    return _thread_scrapers[tid]


def fetch_html(url: str, referer: str = None) -> str:
    """获取页面 HTML，自动处理 JS 挑战"""
    scraper = get_scraper()
    headers = {
        "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
        "Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
        "Accept-Language": "zh-CN,zh;q=0.9",
    }
    if referer:
        headers["Referer"] = referer

    resp = scraper.get(url, headers=headers, timeout=30)
    resp.raise_for_status()
    html = resp.text

    # 处理 JS 挑战
    if "正在验证浏览器" in html or "challenge" in html.lower():
        token_match = re.search(r'token\s*=\s*["\']([^"\']+)["\']', html)
        if token_match:
            token = token_match.group(1)
            separator = "&" if "?" in url else "?"
            challenge_url = f"{url}{separator}challenge={token}"
            time.sleep(1.5)
            resp = scraper.get(challenge_url, headers=headers, timeout=30)
            html = resp.text

    return html


def extract_book_id(url: str) -> str:
    """从 URL 提取 book_id"""
    match = re.search(r"/read/(\d+)", url)
    return match.group(1) if match else None


def extract_info(html: str, url: str) -> dict:
    """提取小说信息"""
    from bs4 import BeautifulSoup

    soup = BeautifulSoup(html, "html.parser")

    # 提取标题
    title_el = soup.select_one(".n-text h1")
    title = title_el.get_text(strip=True) if title_el else "未知书名"

    # 提取作者
    author_el = soup.select_one(".bauthor")
    author = author_el.get_text(strip=True) if author_el else "未知作者"
    author = re.sub(r"^(作者|作家)[：:\s]*", "", author).strip()

    # 提取简介
    intro_el = soup.select_one(".pintro")
    intro = ""
    if intro_el:
        intro = intro_el.get_text(separator="\n", strip=True)
        intro = re.sub(r"\n{3,}", "\n\n", intro)

    # 提取封面
    cover_url = None
    img_el = soup.select_one(".n-img img")
    if img_el:
        cover_url = img_el.get("src") or img_el.get("data-src")
        if cover_url:
            parsed = urlparse(url)
            domain = f"{parsed.scheme}://{parsed.netloc}"
            if cover_url.startswith("//"):
                cover_url = "https:" + cover_url
            elif cover_url.startswith("/"):
                cover_url = domain + cover_url

    # 检测最大页码
    max_page = 0
    matches = re.findall(r"/read/\d+/p(\d+)\.html", html)
    if matches:
        max_page = max(int(m) for m in matches)

    return {
        "title": title,
        "author": author,
        "intro": intro,
        "coverUrl": cover_url,
        "maxPage": max_page,
    }


def extract_chapter(html: str) -> dict:
    """提取章节内容"""
    from bs4 import BeautifulSoup

    soup = BeautifulSoup(html, "html.parser")

    # 提取标题
    title_match = re.search(r"<title>(.*?)</title>", html, re.IGNORECASE)
    title = title_match.group(1).strip() if title_match else "未知章节"
    title = re.split(r"[_\-|]", title)[0].strip()

    # 提取 page-content
    content_el = soup.select_one(".page-content")
    if content_el:
        paragraphs = content_el.find_all("p")
        texts = [p.get_text(strip=True) for p in paragraphs if p.get_text(strip=True)]
        content = "\n\n".join(texts)
    else:
        # 兜底
        paragraphs = soup.find_all("p")
        texts = [
            p.get_text(strip=True)
            for p in paragraphs
            if len(p.get_text(strip=True)) > 10
        ]
        content = "\n\n".join(texts)

    return {"title": title, "content": content}


def download_image(image_url: str, output_path: str) -> bool:
    """下载图片"""
    try:
        scraper = get_scraper()
        resp = scraper.get(
            image_url, timeout=30, headers={"Referer": "https://ixdzs8.com/"}
        )
        with open(output_path, "wb") as f:
            f.write(resp.content)
        return True
    except Exception as e:
        print(f"下载图片失败: {e}", file=sys.stderr)
        return False


def fetch_single_chapter(
    page_num: int, base_url: str, book_id: str, output_dir: str
) -> dict:
    """
    获取单章内容并保存（用于多线程）

    Returns:
        {"pageNum": int, "success": bool, "title": str, "error": str or None}
    """
    chapter_url = f"{base_url}/p{page_num}.html"

    try:
        html = fetch_html(chapter_url, base_url)
        chapter = extract_chapter(html)

        chapter_data = {
            "pageNum": page_num,
            "title": chapter["title"],
            "url": chapter_url,
            "content": chapter["content"],
        }

        # 保存单章 JSON
        chapter_path = os.path.join(output_dir, f"p{page_num}.json")
        with open(chapter_path, "w", encoding="utf-8") as f:
            json.dump(chapter_data, f, ensure_ascii=False, indent=2)

        return {
            "pageNum": page_num,
            "success": True,
            "title": chapter["title"],
            "error": None,
        }
    except Exception as e:
        return {
            "pageNum": page_num,
            "success": False,
            "title": None,
            "error": str(e),
        }


def crawl_chapters(
    url: str, start: int, end: int, output_dir: str, max_workers: int = 5
) -> dict:
    """
    多线程并发爬取章节

    Args:
        url: 目录页 URL
        start: 起始页码
        end: 结束页码
        output_dir: 输出目录
        max_workers: 并发线程数

    Returns:
        {"total": int, "success": int, "failed": int, "results": list}
    """
    base_url = re.sub(r"/p\d+\.html$", "", url.rstrip("/"))
    if not base_url:
        base_url = url.rstrip("/")

    book_id = extract_book_id(url)

    # 清空或创建 txt 文件
    txt_path = os.path.join(output_dir, f"{book_id}.txt")
    with open(txt_path, "w", encoding="utf-8") as f:
        f.write("")

    results = []
    success_count = 0
    failed_count = 0

    print(
        f"开始多线程爬取: {start} 到 {end} 章, 线程数: {max_workers}", file=sys.stderr
    )

    with ThreadPoolExecutor(max_workers=max_workers) as executor:
        # 提交所有任务
        future_to_page = {
            executor.submit(
                fetch_single_chapter, page, base_url, book_id, output_dir
            ): page
            for page in range(start, end + 1)
        }

        # 处理完成的任务
        for future in as_completed(future_to_page):
            page = future_to_page[future]
            try:
                result = future.result()
                results.append(result)

                if result["success"]:
                    success_count += 1
                    # 追加到 txt
                    chapter_data = {
                        "pageNum": result["pageNum"],
                        "title": result["title"],
                        "content": "",  # 从文件读取
                    }
                    chapter_path = os.path.join(
                        output_dir, f"p{result['pageNum']}.json"
                    )
                    with open(chapter_path, "r", encoding="utf-8") as f:
                        data = json.load(f)
                        chapter_data["content"] = data["content"]

                    txt_content = f"\n{'=' * 50}\n{chapter_data['title']}\n{'=' * 50}\n\n{chapter_data['content']}\n\n"
                    with open(txt_path, "a", encoding="utf-8") as f:
                        f.write(txt_content)

                    print(f"  ✅ p{page}: {result['title']}", file=sys.stderr)
                else:
                    failed_count += 1
                    print(f"  ❌ p{page}: {result['error']}", file=sys.stderr)

            except Exception as e:
                failed_count += 1
                print(f"  ❌ p{page}: {e}", file=sys.stderr)

    # 保存 chapters.json
    chapters_summary = []
    for r in sorted(results, key=lambda x: x["pageNum"]):
        if r["success"]:
            chapters_summary.append(
                {
                    "pageNum": r["pageNum"],
                    "title": r["title"],
                }
            )

    chapters_json = {
        "bookId": book_id,
        "total": len(chapters_summary),
        "chapters": chapters_summary,
    }
    with open(os.path.join(output_dir, "chapters.json"), "w", encoding="utf-8") as f:
        json.dump(chapters_json, f, ensure_ascii=False, indent=2)

    print(f"爬取完成: 成功 {success_count}, 失败 {failed_count}", file=sys.stderr)

    return {
        "total": end - start + 1,
        "success": success_count,
        "failed": failed_count,
        "results": results,
    }


def handle_command(command: dict) -> dict:
    """处理命令"""
    cmd = command.get("cmd")
    req_id = command.get("_reqId")

    result = {"_reqId": req_id}

    try:
        if cmd == "fetch":
            url = command["url"]
            referer = command.get("referer")
            html = fetch_html(url, referer)
            result.update({"success": True, "html": html})

        elif cmd == "info":
            url = command["url"]
            html = fetch_html(url)
            info = extract_info(html, url)
            result.update({"success": True, "info": info})

        elif cmd == "chapter":
            url = command["url"]
            referer = command.get("referer")
            html = fetch_html(url, referer)
            chapter = extract_chapter(html)
            result.update({"success": True, "chapter": chapter})

        elif cmd == "download":
            url = command["url"]
            path = command["path"]
            success = download_image(url, path)
            result.update({"success": success})

        elif cmd == "crawl":
            # 多线程爬取
            url = command["url"]
            start = command.get("start", 1)
            end = command.get("end", 1)
            output_dir = command["outputDir"]
            max_workers = command.get("maxWorkers", 5)

            crawl_result = crawl_chapters(url, start, end, output_dir, max_workers)
            result.update({"success": True, "crawl": crawl_result})

        else:
            result.update({"success": False, "error": f"未知命令: {cmd}"})

    except Exception as e:
        result.update({"success": False, "error": str(e)})

    return result


def main():
    """主循环：从 stdin 读取命令，输出到 stdout"""
    print("Python spider ready (multi-threaded)", file=sys.stderr)

    for line in sys.stdin:
        line = line.strip()
        if not line:
            continue

        try:
            command = json.loads(line)
            result = handle_command(command)
            print(json.dumps(result), flush=True)
        except json.JSONDecodeError as e:
            print(
                json.dumps({"success": False, "error": f"JSON 解析错误: {e}"}),
                flush=True,
            )
        except Exception as e:
            print(json.dumps({"success": False, "error": str(e)}), flush=True)


if __name__ == "__main__":
    main()
