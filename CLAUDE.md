# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Remotework is a Go traffic relay tool for remote control assistance. Two machines running agents connect through a central server to proxy port traffic (RDP, SSH, etc.). It is not a remote control tool itself — it relays traffic between endpoints.

The desktop client (Tauri 2 + React, hosting the agent as a sidecar via the `api/` control plane) lives in the separate `remotework-desktop` repository; it was split out of this repo's former `desktop/` directory.

## Build & Test Commands

```bash
make build        # Build for current platform (CGO_ENABLED=0 go build -o dist/remote ./cmd)
make build-all    # Cross-platform build (sh build.sh, outputs to dist/<date><suffix>/)
make test         # go test ./...
make vet          # go vet ./...
make fmt          # go fmt ./...
```

Entry point is `./cmd` (not root). The build scripts disable CGO for static binaries.

## Local Dependency Requirement

`go.mod` contains `replace github.com/net-agent/flex/v3 => ../flex`. This is **required**: remotework uses flex APIs (e.g. `flex/v3/packet/ws`) that only exist in commits after the published v3.0.1. Building requires a sibling checkout of the flex repo at `../flex`. Do not remove the replace directive until a newer flex version is published and pinned in `go.mod`.

## Architecture

Single binary, three run modes selected via `-mode` flag (default: `agent`):

- **agent** — Connects to relay server, runs tunnels (port forwarding / socks5). Entry: `cmd/main.go`
- **server** — Relay server multiplexing Flex protocol and HTTP/WebSocket on a single port. Implementation: `server/` package (`server/server.go`), uses `flex/v3/switcher`
- **validate** — Validate a config file and exit

### Key Packages

- **`kernel/`** — Plugin runtime (microkernel): plugin lifecycle state machine, dependency-injection capability container (`Context` + `Inject`/`Provide`), ordered startup/shutdown orchestration, side-effect cleanup on unload. Developer manual: `kernel/README.md`
- **`plugins/`** — Loadable plugins on kernel: `nettcp` (builtin TCP networks), `netflex` (Flex virtual networks), `tunnel` (portproxy/socks5 services), `api` (control plane observer). Naming/capability conventions: `plugins/doc.go`
- **`vnet/`** — `Network` interface implementations (Flex virtual network, local TCP) plus state reporting (`ReportingNetwork`); consumed by network plugins
- **`vservice/`** — Service controllers (portproxy, socks5) and `ServiceState`; consumed by the tunnel plugin
- **`configv2/`** — Config parsing/normalization/validation (JSON with `//` comments, TOML); consumed by `cmd` and plugins
- **`api/`** — Control plane only: HTTP REST + WebSocket, driven by the `Backend` interface (`api/backend.go`), implemented by `plugins/api`. No data traffic. Runtime options come from its own `api.Config` (not `configv2.APIInfo`); binds synchronously in `Start()` and reports the actual address via `Addr()`. Security: Bearer token (HTTP) / `?token=` (WS), Host-header whitelist, CORS/Origin allow-list. Full reference: `api/README.md`
- **`server/`** — Relay server internals: config, mux, `Server` lifecycle
- **`utils/`** — Shared utilities: logging (`NewModuleLogger`, `NamedLogger`, `SetFormat` — `json` writes slog JSON to stdout for sidecar managed mode, `text` writes console format to stderr), encrypted connections (`SecretListener` via cipherconn), bidirectional relay (`LinkReadWriteCloser`), exponential backoff (`Cooldown`), pprof server, listening-port probe
- **`cmd/`** — CLI flags (`agent/server/validate`, `-log-format text|json`), config loading, kernel host assembly, signal handling, and the sidecar process contract (C3 stdout control lines: `ready`/`fatal`, exit codes 0/1/2, `version`/`commit` injected via `-ldflags -X` by the build scripts). Protocol spec: `docs/tauri-sidecar-protocol.md`. Sample configs: `cmd/config_example/`

### Core Flow (Agent Mode)

1. Parse flags → load config (JSON with `//` comments or TOML)
2. Create `kernel.Host`, load plugins: builtin networks → link networks (`netflex`) → tunnels → api
3. The kernel scheduler initializes plugins in dependency order (tunnel plugins inject network capabilities; unmet deps park in `Pending` until provided)
4. Networks auto-reconnect with exponential backoff (3s → 1min)
5. Config loading is fault-tolerant per item: invalid links/tunnels are skipped with logged errors (`configv2.LoadAndCheck`) while valid ones load normally — one bad tunnel never blocks others. Degraded mode (API only) only triggers when the config file itself cannot be parsed, so the desktop UI can fix it
6. Transport encryption is optional, enabled via `?authcode=` on tunnel endpoint URLs
7. Shutdown (OS signal or `POST /api/v1/stop`) unloads all plugins in reverse dependency order

### URL Scheme Convention

All connections use URL-based addressing:
- `tcp://host:port` — standard TCP (local network)
- `vtcp://vhostname.linkalias:port[?authcode=key]` — Flex virtual TCP through the relay; `vhostname` is the peer node identity, `linkalias` selects which link (network) to use
- `ws://` / `wss://` — WebSocket transport (for link registration URLs)

### Config Format

Supports JSON (with `//` line comments) and TOML. Top-level sections: `links` (alias → relay registration URL with `?as=` and `?auth=`), `tunnels` (listen/target rules), `api` (control plane), `pprof`. Source of truth: `configv2/model.go`; example: `cmd/config_example/agent.json`.

## Key Dependencies

- `github.com/net-agent/flex/v3` — Virtual network protocol (core transport layer; locally replaced to `../flex`, see above)
- `github.com/net-agent/cipherconn` — Pre-shared key encryption
- `github.com/net-agent/socks` — SOCKS5 implementation

## Language

Code comments and documentation are primarily in Chinese.
