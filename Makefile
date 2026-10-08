.PHONY: build build-all test vet fmt sidecar sidecar-all desktop-dev desktop-build

# Go agent / server（注意：依赖 ../flex 本地 checkout，见 go.mod replace）

build:
	CGO_ENABLED=0 go build -o dist/remote ./cmd

build-all:
	sh build.sh

test:
	go test ./...

vet:
	go vet ./...

fmt:
	go fmt ./...

# Desktop 客户端（Tauri 2 + React，详见 desktop/README.md）

sidecar:
	sh desktop/build-sidecar.sh current

sidecar-all:
	sh desktop/build-sidecar.sh all

desktop-dev:
	cd desktop && pnpm tauri dev

desktop-build:
	cd desktop && pnpm tauri build
