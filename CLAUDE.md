# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Remotework is a Go traffic relay tool for remote control assistance. Two machines running agents connect through a central server to proxy port traffic (RDP, SSH, etc.). It is not a remote control tool itself — it relays traffic between endpoints.

## Build & Test Commands

```bash
# Build for current platform
CGO_ENABLED=0 go build -o dist/remote ./cmd

# Cross-platform build (all targets)
sh build.sh

# Run all tests
go test ./...

# Run a single package's tests
go test ./utils -v

# Format and vet
go fmt ./...
go vet ./...
```

Entry point is `./cmd` (not root). The build scripts disable CGO for static binaries.

## Architecture

Three operational modes selected via `-mode` flag (default: `agent`):

- **agent** — Connects to server, runs services (portproxy, socks5, rdp). Entry: `cmd/main.go`
- **server** — Relay server accepting Flex (TCP) and WebSocket connections. Entry: `server/main.go`
- **cli** — Ping utility for connectivity testing

### Key Packages

- **`agent/`** — Hub-based orchestrator. `Hub` manages multiple virtual networks and services. Networks implement the `Network` interface; services implement `ServiceController`. URL scheme determines network type (`tcp://`, `vtcp://`, `ws://`).
- **`server/`** — Uses `flex/v2/switcher` with `mixlisten` to multiplex Flex protocol and HTTP/WebSocket on a single port.
- **`utils/`** — Shared utilities: async logging (`NamedLogger`), encrypted connections (`SecretListener` via `cipherconn`), bidirectional relay (`LinkReadWriteCloser`), exponential backoff (`Cooldown`), pprof server.
- **`cmd/`** — CLI flags, config loading, signal handling, platform-specific system tray (`systray_*.go`).

### Core Flow (Agent Mode)

1. Parse flags → load config (JSON with `//` comments or TOML)
2. Create `Hub`, add networks (Flex virtual networks + local TCP)
3. Start services — each runs in its own goroutine
4. Networks auto-reconnect with exponential backoff (3s → 1min)
5. Encryption is optional, enabled via `?secret=` query parameter on URLs

### URL Scheme Convention

All connections use URL-based addressing: `scheme://domain:port[?secret=key]`
- `tcp://` — standard TCP (local network)
- `vtcp://` — Flex virtual TCP (through relay server)
- `ws://` — WebSocket transport

### Config Format

Supports JSON (with `//` line comments) and TOML. Key sections: `agent` (network connections), `portproxy`/`pipe` (port forwarding), `socks5`/`sox` (SOCKS5 proxy), `rdp`, `pprof`.

## Key Dependencies

- `github.com/net-agent/flex/v3` — Virtual network protocol (core transport layer)
- `github.com/net-agent/cipherconn` — Pre-shared key encryption
- `github.com/net-agent/socks` — SOCKS5 implementation
- `github.com/getlantern/systray` — System tray UI

## Language

Code comments and documentation are primarily in Chinese.
