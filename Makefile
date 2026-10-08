BINARY_NAME=duplex
VERSION=1.0.0
LDFLAGS=-s -w

.PHONY: all build test clean install uninstall cross-compile

all: build test

build:
	go build -ldflags="$(LDFLAGS)" -o $(BINARY_NAME) cmd/duplex/main.go

test:
	go test -v -race ./pkg/...

clean:
	rm -f $(BINARY_NAME)
	rm -rf dist/
	rm -rf *-duplex/ *-split/

install:
	./scripts/install.sh

uninstall:
	./scripts/uninstall.sh

cross-compile:
	mkdir -p dist
	GOOS=darwin GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY_NAME)-darwin-arm64 cmd/duplex/main.go
	GOOS=darwin GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY_NAME)-darwin-amd64 cmd/duplex/main.go
	GOOS=linux GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY_NAME)-linux-amd64 cmd/duplex/main.go
	GOOS=linux GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY_NAME)-linux-arm64 cmd/duplex/main.go
	GOOS=windows GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY_NAME)-windows-amd64.exe cmd/duplex/main.go
	@echo "Cross-compilation complete! Binaries located in dist/"
