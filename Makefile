.PHONY: build build-all test vet fmt

# Go agent / server（flex 依赖已按 go.mod 中 pin 的版本拉取，无需本地 ../flex）

# 版本信息注入（sidecar 协议：version/commit 经 -ldflags -X 写入二进制）
VERSION ?= $(shell git describe --tags --always 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null)
LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT)

build:
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o dist/remote ./cmd

build-all:
	bash build.sh

test:
	go test ./...

vet:
	go vet ./...

fmt:
	go fmt ./...

