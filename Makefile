BINARY := ledger
ifeq ($(OS),Windows_NT)
	BINARY := ledger.exe
endif

.PHONY: all build test vet demo plugin extension install clean

all: vet test build

build:
	go build -o $(BINARY) ./cmd/ledger

test:
	go test ./...

vet:
	go vet ./...

demo: build
	./demo.sh

## Build the Obsidian plugin (typecheck + bundle to main.js)
plugin:
	cd obsidian-plugin && npm install && npm run build

## Compile the VS Code extension to out/
extension:
	cd vscode-extension && npm install && npx tsc -p ./

## Put the binary on your PATH (override with PREFIX=...)
PREFIX ?= /usr/local/bin
install: build
	install -m 0755 $(BINARY) $(PREFIX)/$(BINARY)

clean:
	rm -f $(BINARY)
	rm -rf vscode-extension/out obsidian-plugin/main.js
