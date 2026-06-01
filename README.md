# TerminalReader

A terminal-based novel reader TUI app built with [Bubble Tea](https://github.com/charmbracelet/bubbletea). Search, download, manage and read web novels comfortably from the command line.

![Go Version](https://img.shields.io/badge/go-%3E%3D1.21-blue)
![License](https://img.shields.io/badge/license-MIT-green)
![Platform](https://img.shields.io/badge/platform-macOS%20%7C%20Linux%20%7C%20Windows-lightgrey)
![Bubble Tea](https://img.shields.io/badge/Bubble%20Tea-v0.25.0-ff69b4)
![Bubbles](https://img.shields.io/badge/Bubbles-v0.18.0-ff69b4)
![Lipgloss](https://img.shields.io/badge/Lipgloss-v1.1.1-ff69b4)
![SQLite](https://img.shields.io/badge/SQLite-v1.51.0-orange)
[![Release](https://img.shields.io/github/v/release/howardhenrystephen/TerminalReader)](https://github.com/howardhenrystephen/TerminalReader/releases)
[![Stars](https://img.shields.io/github/stars/howardhenrystephen/TerminalReader?style=social)](https://github.com/howardhenrystephen/TerminalReader/stargazers)

## Star History

[![Star History Chart](https://api.star-history.com/svg?repos=howardhenrystephen/TerminalReader&type=Date)](https://star-history.com/#howardhenrystephen/TerminalReader&Date)

## Features

- **Bookshelf Management**: Fancy-list style book list with navigation, open, delete, refresh, and pin support
- **Terminal Reader**: Full-screen reading view with vim-style keybindings for scrolling and paging, auto-removes duplicate chapter titles
- **Multi-source Search**: Concurrent search across multiple novel sites with availability markers
- **Auto Crawling**: One-click full book download with real-time progress display and background download support
- **Smart Updates**: Automatically append new chapters for existing books, avoiding duplicate downloads
- **Reading Position Save**: Auto-records chapter and character offset, resumes on next open
- **Chapter Picker**: Filterable chapter list for quick jumps to any chapter
- **Help**: Press `?` for program introduction
- **Logging**: Full分级日志 (DEBUG/INFO/WARN/ERROR) with daily rotation, file-only output

## Installation

### Prerequisites

- Go >= 1.21
- Python 3 + cloudscraper (for crawler proxy)

### Build from Source

```bash
# Clone repository
git clone https://github.com/howard/TerminalReader.git
cd TerminalReader

# Build
go build -o reader ./main.go

# Run
./reader
```

## Usage

### Launch

```bash
# Default: creates data/novels.db in current directory
./reader

# Specify database path
./reader -db /path/to/your/novels.db
```

### Bookshelf View

Launch to enter the bookshelf view showing your collection.

| Key | Action |
|-----|--------|
| `↑` / `k` | Move up |
| `↓` / `j` | Move down |
| `Enter` | Open book |
| `s` | Search new book |
| `d` | Delete book |
| `r` | Refresh bookshelf |
| `tab` | View book description |
| `p` | Pin/unpin book |
| `g` | Jump to top |
| `G` | Jump to bottom |
| `?` | Show help |
| `q` / `Ctrl+c` | Quit |

### Reader View

| Key | Action |
|-----|--------|
| `j` / `↓` | Scroll down one line |
| `k` / `↑` | Scroll up one line |
| `Space` / `f` | Page down |
| `b` / `PgUp` | Page up |
| `g` | Jump to chapter start |
| `G` | Jump to chapter end |
| `←` / `h` / `p` | Previous chapter |
| `→` / `l` / `n` | Next chapter |
| `c` | Open chapter picker |
| `Esc` / `q` | Return to bookshelf (auto-save position) |
| `?` | Show help |

### Chapter Picker

| Key | Action |
|-----|--------|
| `↑` / `k` | Move up |
| `↓` / `j` | Move down |
| `Enter` | Jump to selected chapter |
| `/` | Start filtering |
| `Esc` | Close picker |

### Search & Crawl

1. Press `s` in bookshelf to open search
2. Type novel name, press `Enter` to search
3. Use `↑/↓` to select an available source, press `Enter` for foreground download or `b` for background
4. Foreground shows real-time progress dialog, background shows mini progress bar at bookshelf bottom
5. Book auto-adds to bookshelf when complete

## Database Design

SQLite storage, no extra configuration needed.

### Schema

**books** — Master book table

| Column | Description |
|--------|-------------|
| id | Auto-increment PK |
| title | Book title |
| author | Author name |
| description | Book description |
| total_chapters | Total chapter count |
| current_chapter | Current reading chapter |
| current_offset | Character offset in current chapter |
| source_url | Crawl source URL |
| source_site | Source site name |
| pinned | Pinned status |
| created_at / updated_at | Timestamps |

**chapters_{book_id}** — Per-book chapter table

| Column | Description |
|--------|-------------|
| id | Auto-increment PK |
| chapter_num | Chapter number |
| title | Chapter title |
| content | Chapter text |
| word_count | Word count |
| created_at | Timestamp |

## Tech Stack

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) — Go TUI framework
- [Bubbles](https://github.com/charmbracelet/bubbles) — List, input, progress bar components
- [Lipgloss](https://github.com/charmbracelet/lipgloss) — Terminal styling
- [SQLite](https://sqlite.org/) — Local database (pure Go driver, no CGO)
- Python cloudscraper — Anti-bot crawler proxy

## Project Structure

```
TerminalReader/
├── main.go              # Entry point
├── internal/
│   ├── db/              # Database layer
│   │   ├── database.go
│   │   ├── models.go
│   │   └── queries.go
│   ├── tui/             # TUI view layer
│   │   ├── app.go
│   │   ├── styles.go
│   │   ├── keys.go
│   │   ├── bookshelf.go
│   │   ├── reader.go
│   │   ├── search.go
│   │   ├── crawl.go
│   │   ├── help.go
│   │   ├── chapter_picker.go
│   │   └── toast.go
│   └── crawler/         # Crawler engine
│       ├── crawler.go
│       ├── ixdzs8.go
│       └── ixdzs8_test.go
├── pkg/
│   └── logger/          # Logging package
│       └── logger.go
├── script/
│   ├── spider.py        # Python crawler proxy
│   ├── fix_chapter_order.py  # Fix chapter order script
│   └── migrate_add_pinned.py # DB migration script
├── data/
│   └── novels.db        # SQLite database (runtime generated)
├── log/                 # Log directory (daily rotation, runtime generated)
├── README.md
├── go.mod
└── go.sum
```

## Author

**Howard** — [HowardHenryStephen@gmail.com](mailto:HowardHenryStephen@gmail.com)

## License

MIT
