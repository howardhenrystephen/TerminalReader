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

<a href="https://www.star-history.com/?repos=howardhenrystephen%2FTerminalReader&type=timeline&logscale=&legend=bottom-right">
 <picture>
   <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/chart?repos=howardhenrystephen/TerminalReader&type=timeline&theme=dark&logscale&legend=bottom-right" />
   <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/chart?repos=howardhenrystephen/TerminalReader&type=timeline&logscale&legend=bottom-right" />
   <img alt="Star History Chart" src="https://api.star-history.com/chart?repos=howardhenrystephen/TerminalReader&type=timeline&logscale&legend=bottom-right" />
 </picture>
</a>

## Features

- **Bookshelf Management**: Fancy-list style book list with navigation, open, delete, refresh, pin/unpin, and book description view
- **Terminal Reader**: Full-screen reading view with vim-style keybindings for scrolling and paging, auto-removes duplicate chapter titles
- **Multi-source Search**: Concurrent search across multiple novel sites with availability markers
- **Auto Crawling**: One-click full book download with real-time progress display and background download support
- **Smart Updates**: Automatically append new chapters for existing books, avoiding duplicate downloads
- **Continue Download**: Resume incremental downloads from the last crawled chapter with a single key press (`c`)
- **Reading Position Save**: Auto-records chapter and character offset, resumes on next open
- **Chapter Picker**: Filterable chapter list for quick jumps to any chapter
- **Help**: Press `?` for program introduction and keybindings reference
- **Logging**: Full log levels (DEBUG/INFO/WARN/ERROR) with daily rotation, file-only output

## Installation

### Prerequisites

- Go >= 1.21
- Python 3 + cloudscraper (for crawler proxy)

### Build from Source

```bash
# Clone repository
git clone https://github.com/howardhenrystephen/TerminalReader.git
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
| `Enter` / `l` | Open book |
| `s` | Search new book |
| `c` | Continue download (resume from last chapter) |
| `d` | Delete book |
| `tab` | View book description |
| `p` | Pin/unpin book |
| `g` | Jump to top |
| `G` | Jump to bottom |
| `R` | Force redraw |
| `?` | Show help |
| `q` / `Ctrl+c` | Quit |

### Reader View

| Key | Action |
|-----|--------|
| `j` / `↓` | Scroll down one line |
| `k` / `↑` | Scroll up one line |
| `Space` / `f` / `PgDown` | Page down |
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

### Continue Download

For books that already exist in your bookshelf, press `c` to resume downloading from the last chapter:

- The app tracks the source URL and last crawled chapter in the `book_sources` table
- If the source has new chapters, only the missing ones are downloaded (smart incremental update)
- Downloads run in the background with a mini progress bar at the bottom of the bookshelf
- On completion, the bookshelf auto-refreshes to show the updated chapter count

## Database Design

SQLite storage, no extra configuration needed. Database file is auto-created on first run.

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
| pinned | Pinned status (0/1) |
| created_at / updated_at | Timestamps |

**book_sources** — Book source tracking table

| Column | Description |
|--------|-------------|
| book_id | FK to books.id |
| source_url | Source URL for resuming downloads |
| source_name | Source site name |
| last_crawled_chapter | Last successfully downloaded chapter |
| updated_at | Timestamp |

**chapters_{book_id}** — Per-book chapter table (auto-created on first download)

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
- [SQLite](https://sqlite.org/) — Local database (pure Go driver via modernc.org/sqlite, no CGO)
- Python cloudscraper — Anti-bot crawler proxy

## Project Structure

```
TerminalReader/
├── main.go              # Entry point
├── internal/
│   ├── db/              # Database layer
│   │   ├── database.go  # DB init & migration
│   │   ├── models.go    # Struct definitions
│   │   └── queries.go   # SQL queries
│   ├── tui/             # TUI view layer
│   │   ├── app.go       # Main state machine & message routing
│   │   ├── styles.go    # Color & style definitions
│   │   ├── keys.go      # Keybindings
│   │   ├── bookshelf.go # Bookshelf list view
│   │   ├── reader.go    # Reading view
│   │   ├── search.go    # Search input & results
│   │   ├── crawl.go     # Download progress dialog
│   │   ├── help.go      # Help/about page
│   │   ├── chapter_picker.go # Chapter jump dialog
│   │   └── toast.go     # Toast notification
│   └── crawler/         # Crawler engine
│       ├── crawler.go   # Engine & interfaces
│       ├── ixdzs8.go    # 爱下电子书 source implementation
│       └── ixdzs8_test.go
├── pkg/
│   └── logger/          # Logging package
│       └── logger.go    # Daily rotation file logger
├── script/
│   ├── spider.py        # Python crawler proxy (cloudscraper)
│   ├── fix_chapter_order.py
│   └── migrate_add_pinned.py
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
