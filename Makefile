VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS = -s -w \
	-X github.com/FrankFMY/burrow/internal/shared.Version=$(VERSION) \
	-X github.com/FrankFMY/burrow/internal/shared.Commit=$(COMMIT) \
	-X github.com/FrankFMY/burrow/internal/shared.BuildDate=$(DATE)

TAGS = with_utls,with_quic,with_gvisor,with_wireguard

.PHONY: all server client clean test

all: server client

server:
	go build -tags "$(TAGS)" -ldflags "$(LDFLAGS)" -o bin/burrow-server ./cmd/burrow-server

client:
	go build -tags "$(TAGS)" -ldflags "$(LDFLAGS)" -o bin/burrow ./cmd/burrow

clean:
	rm -rf bin/

test:
	go test -tags "$(TAGS)" ./...

tidy:
	go mod tidy

fmt:
	gofmt -s -w .

vet:
	go vet -tags "$(TAGS)" ./...
