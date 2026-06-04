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

# 全局请求时间记录（用于控制请求间隔）
_last_request_time = 0
_request_lock = None


def _ensure_lock():
    global _request_lock
    if _request_lock is None:
        import threading

        _request_lock = threading.Lock()


def _throttle_request(min_interval: float = 1.0):
    """控制请求间隔，避免触发反爬"""
    global _last_request_time
    _ensure_lock()
    with _request_lock:
        now = time.time()
        elapsed = now - _last_request_time
        if elapsed < min_interval:
            time.sleep(min_interval - elapsed)
        _last_request_time = time.time()


def get_scraper():
    """获取当前线程的 scraper 实例"""
    import threading

    tid = threading.current_thread().ident
    if tid not in _thread_scrapers:
        _thread_scrapers[tid] = cloudscraper.create_scraper(
            browser={"browser": "chrome", "platform": "windows", "desktop": True}
        )
    return _thread_scrapers[tid]


def fetch_html(
    url: str, referer: str = None, retry: int = 3, delay: float = 0.5
) -> str:
    """获取页面 HTML，自动处理 JS 挑战和反爬重试"""
    scraper = get_scraper()
    headers = {
        "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
        "Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
        "Accept-Language": "zh-CN,zh;q=0.9",
    }
    if referer:
        headers["Referer"] = referer

    # 1qxs 网站对频率敏感，增加请求节流
    if "1qxs.com" in url:
        _throttle_request(min_interval=1.5)

    last_error = None
    for attempt in range(retry):
        try:
            if attempt > 0:
                # 重试前增加延迟，指数退避
                sleep_time = delay * (2 ** (attempt - 1))
                time.sleep(sleep_time)
                # 1qxs 重试时额外增加间隔
                if "1qxs.com" in url:
                    time.sleep(2.0)

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

            # 检查是否是错误/跳转页面（1qxs 的反爬机制）
            if "window.location.href" in html and "出错了" in html:
                last_error = (
                    f"Anti-crawl redirect page detected (attempt {attempt + 1})"
                )
                continue

            return html

        except Exception as e:
            last_error = str(e)
            continue

    raise Exception(f"fetch_html failed after {retry} retries: {last_error}")


def extract_book_id(url: str) -> str:
    """从 URL 提取 book_id"""
    match = re.search(r"/read/(\d+)", url)
    return match.group(1) if match else None


def extract_info(html: str, url: str) -> dict:
    """提取小说信息（默认 ixdzs8 站点）"""
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


def extract_boquge_info(html: str, url: str) -> dict:
    """提取笔趣阁小说信息"""
    from bs4 import BeautifulSoup

    soup = BeautifulSoup(html, "html.parser")

    # 提取标题
    title = "未知书名"
    title_el = soup.select_one("dl.info dt")
    if title_el:
        title = title_el.get_text(strip=True)

    # 提取作者
    author = "未知作者"
    for p in soup.select("dl.info dd p"):
        text = p.get_text(strip=True)
        if text.startswith("作者"):
            author = text.replace("作者：", "").strip()
            break

    # 提取简介（优先取 #all，其次 #shot）
    intro = ""
    intro_el = soup.select_one("p.summary#all")
    if intro_el:
        for a in intro_el.select("a.unfold"):
            a.decompose()
        intro = intro_el.get_text(strip=True)
        intro = intro.replace("简介：", "").strip()
    else:
        intro_el = soup.select_one("p.summary#shot")
        if intro_el:
            for a in intro_el.select("a.unfold"):
                a.decompose()
            intro = intro_el.get_text(strip=True)
            intro = intro.replace("简介：", "").strip()

    # 提取封面
    cover_url = None
    img_el = soup.select_one("div.novel-cover img")
    if img_el:
        cover_url = img_el.get("src") or img_el.get("data-src")
        if cover_url:
            parsed = urlparse(url)
            domain = f"{parsed.scheme}://{parsed.netloc}"
            if cover_url.startswith("//"):
                cover_url = "https:" + cover_url
            elif cover_url.startswith("/"):
                cover_url = domain + cover_url

    return {
        "title": title,
        "author": author,
        "intro": intro,
        "coverUrl": cover_url,
        "maxPage": 0,
    }


def extract_boquge_chapter(html: str) -> dict:
    """提取笔趣阁章节内容"""
    from bs4 import BeautifulSoup

    # 提取标题
    title_match = re.search(r"<title>(.*?)</title>", html, re.IGNORECASE)
    title = title_match.group(1).strip() if title_match else "未知章节"
    title = re.split(r"[_\-|]", title)[0].strip()

    # 提取内容
    soup = BeautifulSoup(html, "html.parser")
    content_el = soup.select_one("div#cContent")
    if content_el:
        paragraphs = content_el.find_all("p")
        texts = []
        for p in paragraphs:
            text = p.get_text(strip=True)
            if (
                text
                and not text.startswith("请记住本书首发域名")
                and not text.startswith("顶点小说网")
                and not text.startswith("网页版章节内容慢")
                and not text.startswith("请退出转码页面")
                and not text.startswith("新笔趣阁为你提供最快的")
                and not text.startswith("由于各种问题地址更改为")
                and not text.startswith("请收藏新地址避免迷路")
            ):
                texts.append(text)
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


def extract_yqxsc_info(html: str, url: str) -> dict:
    """提取 1qxs.com 小说信息"""
    from bs4 import BeautifulSoup

    soup = BeautifulSoup(html, "html.parser")

    # 提取标题: 从 <title> 取 "XXX - YYY - ZZZZZ" 的 XXX
    title = "未知书名"
    title_match = re.search(r"<title>(.*?)</title>", html, re.IGNORECASE)
    if title_match:
        title_text = title_match.group(1).strip()
        parts = title_text.split(" - ")
        if parts:
            title = parts[0].strip()

    # 提取作者: 从 <title> 取 "XXX - YYY - ZZZZZ" 的 YYY
    author = "未知作者"
    if title_match:
        title_text = title_match.group(1).strip()
        parts = title_text.split(" - ")
        if len(parts) >= 2:
            author = parts[1].strip()

    # 提取简介: 从 meta name=description content="xxx:yyy" 取 ":" 之后的内容
    intro = ""
    desc_meta = soup.find("meta", attrs={"name": "description"})
    if desc_meta:
        desc_content = desc_meta.get("content", "")
        # 取 ":" 之后的内容作为简介（":" 之前是网站宣传语）
        if ":" in desc_content:
            intro = desc_content.split(":", 1)[1].strip()
        else:
            intro = desc_content.strip()

    # 提取总章节数: 从书籍主页的章节链接中提取最大章节号
    # 书籍主页上有最新章节链接如 /xs_1/87331/725
    total_chapters = 0
    chapter_links = re.findall(r'href="/xs_\d+/\d+/(\d+)"', html)
    if chapter_links:
        total_chapters = max(int(x) for x in chapter_links)

    # 如果上面没取到，尝试从 catalog 页面获取
    if total_chapters == 0:
        catalog_match = re.search(r"/xs_(\d+)/(\d+)", url)
        if catalog_match:
            site_num = catalog_match.group(1)
            book_id = catalog_match.group(2)
            catalog_url = f"https://m.1qxs.com/catalog_{site_num}/{book_id}"
            try:
                catalog_html = fetch_html(catalog_url)
                catalog_soup = BeautifulSoup(catalog_html, "html.parser")
                pagelist = catalog_soup.select_one("select.pagelist")
                if pagelist:
                    options = pagelist.find_all("option")
                    if options:
                        last_option = options[-1].get_text(strip=True)
                        # 格式如 "1-100章"，提取 yyy
                        match = re.search(r"-(\d+)", last_option)
                        if match:
                            total_chapters = int(match.group(1))
            except Exception as e:
                print(f"获取 catalog 失败: {e}", file=sys.stderr)

    return {
        "title": title,
        "author": author,
        "intro": intro,
        "coverUrl": None,
        "totalChapters": total_chapters,
    }


import base64


def _extract_yqxsc_page_content(html: str) -> str:
    """从单页 HTML 提取内容（包括可见内容和 p_key 隐藏的 base64 内容）"""
    from bs4 import BeautifulSoup

    texts = []

    # 1. 提取可见内容
    soup = BeautifulSoup(html, "html.parser")
    content_el = soup.select_one("div.content")
    if content_el:
        paragraphs = content_el.find_all("p")
        for i, p in enumerate(paragraphs):
            # 跳过第一个和最后两个
            if i == 0 or i >= len(paragraphs) - 2:
                continue
            text = p.get_text(strip=True)
            # 跳过阅读模式提示和加载更多按钮
            if text and "阅|读|模|式" not in text and "加|载|更|多" not in text:
                texts.append(text)

    # 2. 提取 p_key 隐藏的 base64 内容
    p_key_match = re.search(r"p_key=['\"]([^'\"]+)['\"]", html)
    if p_key_match:
        try:
            decoded = base64.b64decode(p_key_match.group(1)).decode("utf-8")
            decoded_soup = BeautifulSoup(decoded, "html.parser")
            for p in decoded_soup.find_all("p"):
                text = p.get_text(strip=True)
                if text:
                    texts.append(text)
        except Exception:
            pass

    return "\n\n".join(texts)


def extract_yqxsc_chapter(html: str, url: str) -> dict:
    """提取 1qxs.com 章节内容（自动获取所有分页，每页包含 p_key 隐藏内容）"""
    from bs4 import BeautifulSoup

    # 提取标题: 从 meta name=keywords content="xxx" 取 "," 前的字符
    title = "未知章节"
    keywords_match = re.search(
        r'<meta[^>]*name=["\']keywords["\'][^>]*content=["\']([^"\']+)["\']',
        html,
        re.IGNORECASE,
    )
    if not keywords_match:
        keywords_match = re.search(
            r'<meta[^>]*content=["\']([^"\']+)["\'][^>]*name=["\']keywords["\']',
            html,
            re.IGNORECASE,
        )
    if keywords_match:
        keywords_content = keywords_match.group(1).strip()
        parts = keywords_content.split(",")
        if parts:
            title = parts[0].strip()

    # 提取第一页内容（包含 p_key 解码内容）
    content = _extract_yqxsc_page_content(html)

    # 检测是否有分页: 查找 "下一页" 链接
    next_page_match = re.search(r'href="(/xs_\d+/\d+/\d+/\d+)"[^>]*>下一页', html)
    if next_page_match:
        # 有分页，循环获取所有后续页面
        parsed = urlparse(url)
        base_url = f"{parsed.scheme}://{parsed.netloc}"
        current_url = url

        # 安全限制：最多获取 50 个分页，防止无限循环
        for _ in range(50):
            next_page_match = re.search(
                r'href="(/xs_\d+/\d+/\d+/\d+)"[^>]*>下一页', html
            )
            if not next_page_match:
                break

            next_page_path = next_page_match.group(1)
            next_page_url = base_url + next_page_path
            try:
                page_html = fetch_html(next_page_url, current_url)
                page_content = _extract_yqxsc_page_content(page_html)
                if page_content:
                    content += "\n\n" + page_content
                html = page_html  # 更新 html 用于检测下一页
                current_url = next_page_url
            except Exception as e:
                print(f"获取分页 {next_page_url} 失败: {e}", file=sys.stderr)
                break

    return {"title": title, "content": content}


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


def fetch_boquge_chapter_list(
    base_url: str, book_id: str, max_workers: int = 8
) -> list:
    """并发获取笔趣阁章节列表"""
    from bs4 import BeautifulSoup

    # 先获取第一页，提取尾页数字作为上界
    first_page_html = fetch_html(f"{base_url}/wapbook/{book_id}-1.html")
    last_page_match = re.search(
        rf'href="{book_id}-(\d+)\.html"[^>]*>尾页', first_page_html
    )
    upper_bound = int(last_page_match.group(1)) if last_page_match else 1000

    # 二分查找确定实际最后一页
    def has_chapters(page: int) -> bool:
        try:
            html = fetch_html(f"{base_url}/wapbook/{book_id}-{page}.html")
            return bool(re.search(rf'<a href="/wapbook/{book_id}_\d+\.html"', html))
        except Exception:
            return False

    low, high = 1, upper_bound
    while low < high:
        mid = (low + high + 1) // 2
        if has_chapters(mid):
            low = mid
        else:
            high = mid - 1
    actual_last = low

    # 并发获取所有页面
    def fetch_page(page: int):
        try:
            html = fetch_html(f"{base_url}/wapbook/{book_id}-{page}.html")
            soup = BeautifulSoup(html, "html.parser")
            chapters = []
            for a in soup.select("ul#chapterlist li a"):
                href = a.get("href", "")
                if href.startswith("/wapbook/") and "_" in href:
                    chapters.append(
                        {"title": a.get_text(strip=True), "url": base_url + href}
                    )
            return page, chapters
        except Exception as e:
            return page, []

    all_chapters = []
    with ThreadPoolExecutor(max_workers=max_workers) as executor:
        futures = {executor.submit(fetch_page, p): p for p in range(1, actual_last + 1)}
        page_results = {}
        for future in as_completed(futures):
            page, chapters = future.result()
            page_results[page] = chapters

    # 按页码顺序合并
    for page in sorted(page_results.keys()):
        all_chapters.extend(page_results[page])

    return all_chapters


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
            site = command.get("site", "ixdzs8")
            html = fetch_html(url)
            if site == "boquge":
                info = extract_boquge_info(html, url)
            elif site == "yqxsc":
                info = extract_yqxsc_info(html, url)
            else:
                info = extract_info(html, url)
            result.update({"success": True, "info": info})

        elif cmd == "chapter":
            url = command["url"]
            referer = command.get("referer")
            site = command.get("site", "ixdzs8")
            html = fetch_html(url, referer)
            if site == "boquge":
                chapter = extract_boquge_chapter(html)
            elif site == "yqxsc":
                chapter = extract_yqxsc_chapter(html, url)
            else:
                chapter = extract_chapter(html)
            result.update({"success": True, "chapter": chapter})

        elif cmd == "chapter_list":
            # 笔趣阁章节列表（并发获取）
            url = command["url"]
            site = command.get("site", "boquge")
            if site == "boquge":
                book_id_match = re.search(r"/wapbook/(\d+)\.html", url)
                if not book_id_match:
                    raise ValueError(f"无法从 URL 提取书籍 ID: {url}")
                book_id = book_id_match.group(1)
                base_url = "https://m.boquge.com"
                max_workers = command.get("maxWorkers", 8)
                chapters = fetch_boquge_chapter_list(base_url, book_id, max_workers)
                result.update({"success": True, "chapters": chapters})
            else:
                result.update(
                    {"success": False, "error": f"chapter_list 不支持站点: {site}"}
                )

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
