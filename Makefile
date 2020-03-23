.PHONY: all mod build install

OUTPUT = bin/gtd-ng
BUILD_CMD = go build -ldflags '-w -s -extldflags "-static"'
VERSION = v0.1.0

all: mac linux

mod: go.mod
	go mod download

mac: mod
	GOOS=darwin GOARCH=amd64 $(BUILD_CMD) -o $(OUTPUT)
	tar -czvf packages/gtd-ng_${VERSION}_darwin_amd64.tgz $(OUTPUT)

linux: mod
	GOOS=linux GOARCH=amd64 $(BUILD_CMD) -o $(OUTPUT)
	tar -cjvf packages/gtd-ng_${VERSION}_linux_amd64.zip $(OUTPUT)

install: mac
	cp bin/gtd-ng $(HOME)/bin/gtd-ng
