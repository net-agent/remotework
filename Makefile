.PHONY: build build-all test vet fmt

# Go agent / server（注意：依赖 ../flex 本地 checkout，见 go.mod replace）

# 版本信息注入（sidecar 协议：version/commit 经 -ldflags -X 写入二进制）
VERSION ?= $(shell git describe --tags --always 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null)
LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT)

build:
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o dist/remote ./cmd

build-all:
	sh build.sh

test:
	go test ./...

vet:
	go vet ./...

fmt:
	go fmt ./...

