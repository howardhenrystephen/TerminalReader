.PHONY: build build-all clean install test

BINARY := reader
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -s -w -X main.version=$(VERSION)

# 默认构建当前平台
build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./main.go

# 创建输出目录
dist:
	mkdir -p dist

# 交叉编译所有平台
build-all: dist
	@echo "Building $(VERSION)..."
	GOOS=darwin  GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-darwin-amd64 ./main.go
	GOOS=darwin  GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-darwin-arm64 ./main.go
	GOOS=linux   GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-linux-amd64 ./main.go
	GOOS=linux   GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-linux-arm64 ./main.go
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-windows-amd64.exe ./main.go
	@echo "Done. Binaries in dist/"

# 清理构建产物
clean:
	rm -rf dist/
	rm -f $(BINARY)

# 安装到 $GOPATH/bin
install:
	go install -ldflags "$(LDFLAGS)" ./main.go

# 运行测试
test:
	go test ./...
