.PHONY: build build-all clean install install-deps test build-spider build-release

BINARY := reader
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -s -w -X main.version=$(VERSION)

# 默认构建当前平台
build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./main.go

# 创建输出目录
dist:
	mkdir -p dist

# 打包 spider.py 为独立二进制（需要 pyinstaller）
# 打包后用户无需安装 Python 和 cloudscraper
build-spider:
	@echo "Building spider binary..."
	@which pyinstaller >/dev/null 2>&1 || (echo "❌ pyinstaller not found. Install: pip install pyinstaller" && exit 1)
	pyinstaller --onefile --name spider --distpath dist script/spider.py
	@rm -rf build/ spider.spec
	@echo "✅ Spider binary built: dist/spider"

# 交叉编译所有平台（仅 Go 二进制，spider 需单独打包）
build-all: dist
	@echo "Building $(VERSION)..."
	GOOS=darwin  GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-darwin-amd64 ./main.go
	GOOS=darwin  GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-darwin-arm64 ./main.go
	GOOS=linux   GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-linux-amd64 ./main.go
	GOOS=linux   GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-linux-arm64 ./main.go
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-windows-amd64.exe ./main.go
	@echo "Done. Binaries in dist/"

# 完整发布包：Go 二进制 + spider 二进制（当前平台）
build-release: build build-spider
	@echo "✅ Release build complete: $(BINARY) + dist/spider"

# 清理构建产物
clean:
	rm -rf dist/
	rm -f $(BINARY)

# 安装 Go 程序到 $GOPATH/bin
install:
	go install -ldflags "$(LDFLAGS)" ./main.go

# 安装 Python 依赖（cloudscraper + beautifulsoup4）
# 仅在使用 spider.py 时需要，打包后的 spider 二进制不需要
install-deps:
	@echo "Installing Python dependencies..."
	@python3 -m pip install cloudscraper beautifulsoup4 2>/dev/null || \
	 python -m pip install cloudscraper beautifulsoup4 2>/dev/null || \
	 (echo "❌ pip install failed. Please run manually:" && \
	  echo "   python3 -m pip install cloudscraper beautifulsoup4" && \
	  echo "   or" && \
	  echo "   python -m pip install cloudscraper beautifulsoup4" && \
	  exit 1)
	@echo "✅ Python dependencies installed"

# 运行测试
test:
	go test ./...
