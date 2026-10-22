# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Remotework is a Go traffic relay tool for remote control assistance. Two machines running agents connect through a central server to proxy port traffic (RDP, SSH, etc.). It is not a remote control tool itself — it relays traffic between endpoints.

The desktop client (Tauri 2 + React, hosting the agent as a sidecar via the `api/` control plane) lives in the separate `remotework-desktop` repository; it was split out of this repo's former `desktop/` directory.

## Build & Test Commands

```bash
make build        # Build for current platform (CGO_ENABLED=0 go build -o dist/remote ./cmd)
make build-all    # Cross-platform build (bash build.sh, outputs to dist/<date><suffix>/)
make test         # go test ./...
make vet          # go vet ./...
make fmt          # go fmt ./...
```

Entry point is `./cmd` (not root). The build scripts disable CGO for static binaries.

Release is tag-driven: pushing `vX.Y.Z` runs `.github/workflows/release.yml` (test → linux/darwin/windows amd64+arm64 builds → `checksums.txt` → GitHub Release). The tag name and short commit SHA are injected as `main.version`/`main.commit` via `-ldflags -X`, so the API `/status` and the sidecar `ready` line report the release tag.

## Dependencies

`go.mod` pins `github.com/net-agent/flex/v3 v3.1.0`; the former local `replace` directive is gone. Building requires only Go 1.24+ and network access to the module proxy — no sibling flex checkout. Do not reintroduce the `replace` directive; upgrade the pinned version instead.

## Architecture

Single binary, three run modes selected via `-mode` flag (default: `agent`):

- **agent** — Connects to relay server, runs tunnels (port forwarding / socks5). Entry: `cmd/main.go`
- **server** — Relay server multiplexing Flex protocol and HTTP/WebSocket on a single port. Implementation: `server/` package (`server/server.go`), uses `flex/v3/switcher`
- **validate** — Validate a config file and exit

### Key Packages

- **`kernel/`** — Plugin runtime (microkernel): plugin lifecycle state machine, dependency-injection capability container (`Context` + `Inject`/`Provide`), ordered startup/shutdown orchestration, side-effect cleanup on unload. Developer manual: `kernel/README.md`. Timed-out callbacks keep the attempt busy: `Retry` returns `ErrAttemptRunning` until the old callback really returns; Pending waits are bounded by `DependencyTimeout` (default 5m) and end in `Failed` (`ErrDependencyTimeout`); `Shutdown` aggregates Drain/cleanup errors; subscription cancellation closes the channel.
- **`plugins/`** — Loadable plugins on kernel: `nettcp` (builtin TCP networks; unload calls `Stop`), `netflex` (Flex virtual networks), `tunnel` (portproxy/socks5 services), `api` (control plane observer). Naming/capability conventions: `plugins/doc.go`
- **`vnet/`** — `Network` interface implementations (Flex virtual network, local TCP) plus state reporting (`ReportingNetwork`); consumed by network plugins. Local TCP dial and flex dial use a 10s timeout.
- **`vservice/`** — Service controllers (portproxy, socks5) and `ServiceState`; consumed by the tunnel plugin. SOCKS5 handshake has a 5s deadline and `Close` does not block on it; the tunnel plugin's `Drain` force-closes remaining connections after its deadline.
- **`configv2/`** — Config parsing/normalization/validation (JSON/JSONC with `//` and `/* */` comments, or TOML); consumed by `cmd` and plugins. Unknown fields are rejected. Bad links/tunnels are pruned item by item (`configv2.LoadAndCheck`) while valid ones survive. `authRef`/`authcodeRef` are environment-variable references (`env:NAME` or bare `NAME`) and fail closed when unset/empty; `keepalive` and `via` are rejected (`ERR_KEEPALIVE_NOT_SUPPORTED` / `ERR_VIA_NOT_SUPPORTED`); `authcode`/`authcodeRef` are only valid on `vtcp://` endpoints (`ERR_AUTHCODE_NOT_ALLOWED` on physical endpoints); tunnel names must be unique.
- **`api/`** — Control plane only: HTTP REST + WebSocket, driven by the `Backend` interface (`api/backend.go`), implemented by `plugins/api`. No data traffic. Runtime options come from its own `api.Config` (not `configv2.APIInfo`); binds synchronously in `Start()`, refuses non-loopback listeners unless `AllowNonLoopback=true` and a token is set, and reports a locally usable address via `Addr()` (wildcard listeners advertise loopback). Security: Bearer token (HTTP) / `?token=` or Bearer header (WS), constant-time token comparison, Host-header whitelist that defaults to loopback, CORS/Origin allow-list (non-whitelisted Origin is 403 for writes; requests without Origin are allowed). When a non-loopback listener is explicitly enabled with a token, Host checking accepts any Host and token auth is the boundary. HTTP timeouts are 5s/15s/30s/60s and JSON bodies are capped at 1MB. Full reference: `api/README.md`
- **`server/`** — Relay server internals: config, mux, `Server` lifecycle. `server.listen` must be present and valid, `server.password` must be at least 8 characters, and `wsEnable=true` requires a `/`-prefixed `wsPath`. The mux probes the first byte with a 5s deadline; when `wsEnable=false` HTTP connections are closed immediately. Startup/shutdown handles SIGINT/SIGTERM; HTTP timeouts are set.
- **`utils/`** — Shared utilities: logging (`NewModuleLogger`, `NamedLogger`, `SetFormat` — `json` writes slog JSON to stdout for sidecar managed mode, `text` writes console format to stderr), encrypted connections (`SecretListener` via cipherconn; encryption only, no AEAD/integrity authentication), bidirectional relay (`RelayConns`), loopback-only pprof server (start failures are logged and do not abort the agent), listening-port probe, secret references (`ResolveSecret`) and URL redaction (`RedactURL`).
- **`cmd/`** — CLI flags (`agent/server/validate`, `-c`, `-log-format text|json`; the old `-home` flag is removed), config loading, kernel host assembly, signal handling, and the sidecar process contract (C3 stdout control lines: `ready`/`fatal`, protocol 3, exit codes 0/1/2/130, `version`/`commit` injected via `-ldflags -X` by the build scripts). Protocol spec: `docs/tauri-sidecar-protocol.md`. Sample configs: `cmd/config_example/`

### Core Flow (Agent Mode)

1. Parse flags → load config (JSON/JSONC or TOML)
2. Create `kernel.Host`, load plugins: builtin networks → link networks (`netflex`) → tunnels → api
3. The kernel scheduler initializes plugins in dependency order (tunnel plugins inject network capabilities; unmet deps park in `Pending` until provided or until `DependencyTimeout`)
4. Networks auto-reconnect through the flex session; local/flex dials time out after 10s
5. Config loading is fault-tolerant per item: invalid links/tunnels are skipped with logged errors (`configv2.LoadAndCheck`) while valid ones load normally — one bad tunnel never blocks others. Degraded mode (API only) only triggers when the config file itself cannot be parsed, so the desktop UI can fix it
6. Transport encryption is optional and only available on `vtcp` endpoints via `?authcode=` or `?authcodeRef=`; physical endpoints reject it. `authcodeRef` is resolved from the environment and fails closed
7. Shutdown (OS signal or `POST /api/v1/stop`) unloads all plugins in reverse dependency order with a 35s budget. Incomplete shutdown returns a non-zero exit (1); a second SIGINT/SIGTERM forces exit 130

### URL Scheme Convention

All connections use URL-based addressing:
- `tcp://host:port` — standard TCP (local network)
- `vtcp://vhostname.linkalias:port[?authcode=key|?authcodeRef=ENV_NAME]` — Flex virtual TCP through the relay; `vhostname` is the peer node identity, `linkalias` selects which link (network) to use
- `ws://` / `wss://` — WebSocket transport (for link registration URLs)

Registration URLs accept `?as=` (required), `?auth=` (literal secret) and `?authRef=` (environment-variable reference). `keepalive` and `via` are not supported by flex v3.1.0 and are rejected by validation.

### Config Format

Supports JSON/JSONC (`//` and `/* */` comments, including trailing comments) and TOML. Top-level sections: `links` (alias → relay registration URL with `?as=` and `?auth=`/`?authRef=`), `tunnels` (id/name/comment/listen/target rules), `api` (control plane), `pprof`. Unknown fields are rejected; duplicate tunnel `name`/`id` and conflicting `listen` endpoints are rejected. Source of truth: `configv2/model.go`; example: `cmd/config_example/agent.json`.

### Security Notes

- API: token comparison is constant-time; writes with a non-whitelisted Origin get 403; non-browser requests without Origin are allowed. Non-loopback listeners require `AllowNonLoopback=true` plus a non-empty token; without a token only loopback is allowed. pprof is forced to loopback.
- `api` DTOs redact `authcode`/`authcodeRef`/`auth`/`token`/`password` and URL userinfo via `utils.RedactURL`; logs do the same.
- cipherconn provides encryption only (AES-CTR), not integrity authentication or half-close support; do not describe it as AEAD.

## Key Dependencies

- `github.com/net-agent/flex/v3 v3.1.0` — Virtual network protocol (core transport layer; pinned in `go.mod`)
- `github.com/net-agent/cipherconn v1.0.2` — Pre-shared key encryption (encryption only, no integrity authentication)
- `github.com/net-agent/socks v1.0.7` — SOCKS5 implementation

## Language

Code comments and documentation are primarily in Chinese.
