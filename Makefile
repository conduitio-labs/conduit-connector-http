.PHONY: build test test-integration generate install-paramgen

VERSION=$(shell git describe --tags --dirty --always)

build:
	go build -ldflags "-X 'github.com/conduitio-labs/conduit-connector-discord.version=${VERSION}'" -o conduit-connector-discord cmd/connector/main.go

test:
	go test $(GOTEST_FLAGS) -race ./...

generate:
	go generate ./...

install-paramgen:
	go install github.com/conduitio/conduit-connector-sdk/cmd/paramgen@latest
