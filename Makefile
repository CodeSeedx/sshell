BINARY  := sshell-go
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION)"
BUILD   := go build $(LDFLAGS)

.PHONY: all build clean install test lint vet fmt fmtcheck version help

all: build

build:
	$(BUILD) -o $(BINARY) .

install:
	$(BUILD) -o $(BINARY) .
	@echo "Binary built: ./$(BINARY)"
	@echo "To install: cp $(BINARY) /usr/local/bin/"

clean:
	rm -f $(BINARY)

test:
	go test -short -race -v ./...

vet:
	go vet ./...

fmtcheck:
	@test -z "$$(gofmt -l .)" || { echo "Files need formatting:"; gofmt -l .; exit 1; }

lint: vet fmtcheck test

version:
	@echo $(VERSION)

help:
	@echo "Targets:"
	@echo "  build    - 编译二进制文件 (默认)"
	@echo "  install  - 编译并提示安装"
	@echo "  clean    - 清理编译产物"
	@echo "  test     - 运行测试 (含 race 检测)"
	@echo "  vet      - 静态分析"
	@echo "  fmtcheck - 检查格式化"
	@echo "  lint     - vet + fmtcheck + test"
	@echo "  version  - 显示版本号"
