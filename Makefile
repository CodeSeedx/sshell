APP_NAME = sshell
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS = -ldflags "-s -w -X main.version=$(VERSION)"

.PHONY: all build clean build-all release

all: build

build:
	go build $(LDFLAGS) -o $(APP_NAME) .

clean:
	rm -f $(APP_NAME)
	rm -rf release/

build-all: clean
	mkdir -p release
	GOOS=linux   GOARCH=amd64 go build $(LDFLAGS) -o release/$(APP_NAME)-linux-amd64 .
	GOOS=linux   GOARCH=arm64 go build $(LDFLAGS) -o release/$(APP_NAME)-linux-arm64 .
	GOOS=darwin  GOARCH=amd64 go build $(LDFLAGS) -o release/$(APP_NAME)-darwin-amd64 .
	GOOS=darwin  GOARCH=arm64 go build $(LDFLAGS) -o release/$(APP_NAME)-darwin-arm64 .
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o release/$(APP_NAME)-windows-amd64.exe .

release: build-all
	tar czf release/$(APP_NAME)-$(VERSION)-linux-amd64.tar.gz   -C release $(APP_NAME)-linux-amd64
	tar czf release/$(APP_NAME)-$(VERSION)-linux-arm64.tar.gz   -C release $(APP_NAME)-linux-arm64
	tar czf release/$(APP_NAME)-$(VERSION)-darwin-amd64.tar.gz  -C release $(APP_NAME)-darwin-amd64
	tar czf release/$(APP_NAME)-$(VERSION)-darwin-arm64.tar.gz  -C release $(APP_NAME)-darwin-arm64
	tar czf release/$(APP_NAME)-$(VERSION)-windows-amd64.tar.gz -C release $(APP_NAME)-windows-amd64.exe
