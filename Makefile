BINARY := modemux
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION)"

.PHONY: build build-dev build-arm64 build-amd64 generate test lint clean install

generate:
	templ generate ./internal/web/templates/

build: generate
	CGO_ENABLED=0 go build $(LDFLAGS) -o bin/$(BINARY) ./cmd/modemux

build-dev: generate
	CGO_ENABLED=0 go build -tags dev $(LDFLAGS) -o bin/$(BINARY)-dev ./cmd/modemux

build-arm64: generate
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o bin/$(BINARY)-linux-arm64 ./cmd/modemux

build-amd64: generate
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY)-linux-amd64 ./cmd/modemux

test:
	go test -tags dev -v -race -count=1 ./...

lint:
	golangci-lint run ./...

clean:
	rm -rf bin/
	go clean -cache

install: build
	sudo cp bin/$(BINARY) /usr/local/bin/
	sudo cp scripts/systemd/modemux.service /etc/systemd/system/
	sudo systemctl daemon-reload
