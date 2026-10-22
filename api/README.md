# Remotework API 开发者手册

## 目录

- [概述](#概述)
- [核心架构理解](#核心架构理解)
- [配置与启动](#配置与启动)
- [认证与来源校验](#认证与来源校验)
- [REST API 参考](#rest-api-参考)
- [WebSocket 实时事件](#websocket-实时事件)
- [数据模型](#数据模型)
- [集成示例](#集成示例)
- [设计决策与注意事项](#设计决策与注意事项)
- [Sidecar 协议](#sidecar-协议)

---

## 概述

`api` 包为 Remotework Agent 提供 HTTP REST + WebSocket 控制面接口。它通过 `Backend` 接口（`api/backend.go`）把运行时状态查询和基本操作能力暴露给上层应用（Web UI、CLI 工具、监控系统等）。

核心定位：

- **仅控制面** — 不代理任何数据流量，只提供状态查询和管理操作
- **只读为主** — 大部分接口是状态查询，写操作仅有 `stop`、`loglevel`、手动关闭数据流和运行时挂载（link/tunnel）
- **实时感知** — WebSocket 推送网络状态、服务状态、数据流变化等事件
- **零业务逻辑** — `api` 包只依赖 `Backend` 接口，不绑定具体运行时；实现由 `plugins/api` 提供，桥接 kernel 的 Catalog/EventBus

```
应用层 (Web UI / CLI / 监控)
    │
    ├─ HTTP REST ──→ 状态查询、ping、日志级别、停止
    │
    └─ WebSocket ──→ 实时事件推送
         │
    api.Server ──→ Backend 接口（plugins/api 实现 ──→ kernel Catalog/EventBus）
```

---

## 核心架构理解

在使用 API 之前，理解 Agent 的内部架构有助于正确解读返回数据。

### Kernel — 插件宿主

Agent 运行时是一个微内核（`kernel/`）：所有功能都是加载在 Host 上的插件，API 本身也是一个观察器插件。与状态相关的两类插件：

- **网络插件**（`network/<alias>`）— 内置的 `tcp`/`tcp4`/`tcp6` 本地网络，以及通过中转服务器建立的 Flex 虚拟网络
- **隧道插件**（`tunnel/<name>`）— 运行在网络之上的业务（端口转发、SOCKS5 代理等）

```
kernel.Host
 ├── network/tcp、network/tcp4、network/tcp6   （内置本地网络）
 ├── network/mynet1、network/mynet2            （Flex 虚拟网络）
 ├── tunnel/rdp-forward                        （端口转发）
 ├── tunnel/socks5-proxy                       （SOCKS5 代理）
 └── api                                       （本控制面，观察器插件）
```

内核按依赖关系排序启停插件（隧道注入网络能力），并通过 Catalog/EventBus 两个预置能力向观察器暴露插件清单与状态事件——API 的 REST 查询和 WebSocket 推送正是由此驱动。

### 网络生命周期

每个虚拟网络（非 tcp/tcp4/tcp6）都有自动重连机制，状态在以下值之间流转：

```
offline → connecting → online → (断线) → offline → connecting → ...
                                                        ↓
                                                      closed （手动关闭）
```

- `offline` — 未连接或连接断开
- `connecting` — 正在尝试连接中转服务器
- `online` — 已连接，可正常使用
- `closed` — 已被手动关闭，不再重连

### 服务生命周期

```
uninit → init → running → stopped
                  ↓
            init failed
```

- `uninit` — 刚注册，尚未初始化
- `init` — 正在初始化（创建 listener 等）
- `running` — 正常运行中
- `stopped` — 已停止
- `init failed` — 初始化失败（通常是 listener 创建失败）

### URL 寻址约定

Remotework 中所有连接都使用 URL 格式寻址，这在 API 返回的 `listenURL` / `targetURL` 中会体现：

| Scheme | 含义 | 示例 |
|--------|------|------|
| `tcp://` | 本地 TCP 连接 | `tcp://0.0.0.0:3389` |
| `vtcp://` | Flex 虚拟网络连接 | `vtcp://remote-pc.mynet:3389?authcode=***` |
| `ws://` | WebSocket 传输 | `ws://server.example.com:8080/ws` |

`?authcode=` 参数表示启用了预共享密钥加密，且只允许出现在 `vtcp://` 端点上；物理 `tcp://` 端点携带会被配置校验拒绝（`ERR_AUTHCODE_NOT_ALLOWED`）。`?authcodeRef=` 是它的环境变量引用形式（`env:NAME` 或裸 `NAME`），未设置时失败关闭。API DTO 返回的 URL 中，该参数已脱敏为 `***`。

### 数据流（Stream）

当服务处理连接时，会在虚拟网络上创建数据流。每个 Stream 代表一条端到端的连接，包含本地/远程地址、读写字节数、存活时间等信息。Stream 是理解实际流量状况的关键。

---

## 配置与启动

### 配置文件

在 JSON 或 TOML 配置文件中添加 `api` 段：

**JSON 格式：**

```json
{
  "api": {
    "enable": true,
    "listen": "127.0.0.1:8080",
    "pollInterval": 5,
    "streamStatsInterval": 1
  }
}
```

**TOML 格式：**

```toml
[api]
enable = true
listen = "127.0.0.1:8080"
pollInterval = 5
streamStatsInterval = 1
```

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `enable` | bool | `false` | 是否启用 API 服务器 |
| `listen` | string | `"127.0.0.1:8080"` | 监听地址。默认仅本地访问；如需远程访问改为 `"0.0.0.0:8080"`，并同时设置 `allowNonLoopback=true` 与非空 token |
| `pollInterval` | int | `5` | 网络/服务状态轮询间隔（秒） |
| `streamStatsInterval` | int | `1` | 数据流采样节拍（秒）：`stream.stats` 增量推送（仅状态变化的流；无变化不发帧，全量锚点 ≤30s）与 `stream.update` 开/关差分 |
| `allowNonLoopback` | bool | `false` | 显式允许 `listen` 绑定非回环地址。开启后进程仍必须设置非空 token，否则 `Start()` 拒绝启动 |

> 默认只允许回环监听；`allowNonLoopback=false` 时配置层会拒绝非回环 `listen`（`ERR_API_NON_LOOPBACK`）。即使开启该选项，空 token + 非回环地址也会在 `Start()` 阶段被拒绝。

### 编程方式启动

如果你在代码中集成，而非通过配置文件：

```go
import (
    "github.com/net-agent/remotework/api"
    "github.com/net-agent/remotework/kernel"
    apiplugin "github.com/net-agent/remotework/plugins/api"
)

// 方式一（推荐）：把 API 作为观察器插件挂到 kernel 宿主
host := kernel.NewHost(nil, nil) // nil 使用默认 logger 与参数
// ... 按需加载网络/隧道插件 ...
host.Load(apiplugin.New(config.API, func() {
    // /stop 被调用时的宿主停止回调，例如触发 Shutdown
}, apiplugin.Options{
    Token: os.Getenv("REMOTEWORK_API_TOKEN"), // 可空，见「认证与来源校验」
    Meta:  api.Meta{Protocol: 3, Mode: "normal", ConfigPath: resolvedPath},
}))

// 方式二：不用 kernel，自行实现 api.Backend 接口后直接起服务
apiServer := api.New(myBackend, api.Config{
    Listen: "127.0.0.1:8080",
    Token:  "...",          // 可空
    Meta:   api.Meta{Protocol: 3, Mode: "normal"},
}, nil) // nil 使用默认 logger
if err := apiServer.Start(); err != nil {
    // 端口绑定失败会同步返回错误
}
defer apiServer.Stop()
// apiServer.Addr() 返回本机可连接的地址：Listen 配 ":0" 时为系统分配端口；
// 通配监听（0.0.0.0/[::]）会收敛为 loopback 地址后再对外报告
```

`api.New()` 接收三个参数：
- `api.Backend` — 控制面后端接口实现（必须）
- `api.Config` — 运行时选项（监听地址、轮询间隔、token、元信息），与文件配置解耦
- `*slog.Logger` — 日志器，传 `nil` 使用默认

`Start()` 是**同步绑定**：端口被占用等错误立即返回，不会等到首次请求才暴露。非回环监听必须同时满足 `AllowNonLoopback=true` 与非空 token，否则 `Start()` 关闭 listener 并返回错误。

---

## 认证与来源校验

自 sidecar 协议 v1 起，API 具备完整的安全层（standalone 无 token 时仅在回环地址开放，其余校验同样生效）：

### Bearer token

- 进程经环境变量 `REMOTEWORK_API_TOKEN` 获取 token（Host 每次 spawn 重新生成），token 不落盘、不出现在命令行参数中。
- token 存在时，所有 `/api/v1/*` HTTP 请求必须携带 `Authorization: Bearer <token>`，缺失或错误返回 **401**（统一错误信封）。token 使用常量时间比较，避免时序侧信道。
- WebSocket 握手经 `?token=<token>` 查询参数传递（webview 无法自定义 WS 请求头），也接受 `Authorization: Bearer` 头；校验失败返回 **401** 且不升级连接。
- token 为空（standalone CLI 场景）时 API 只在回环地址上保持开放，启动日志输出 warn 提示 `api server running without auth token`；若实际绑定到非回环地址，`Start()` 会关闭 listener 并返回错误，不会以开放模式暴露到局域网/公网。

### Host 头白名单（防 DNS rebinding）

默认只允许回环 Host：HTTP 请求的 Host 头去端口后必须在 `127.0.0.1`、`localhost`、`::1`、`[::1]` 内，否则返回 **403**。

当 `api.allowNonLoopback=true`、`listen` 实际绑定到非回环地址且 token 非空时，`Start()` 允许启动；此时 Host 校验放行任意 Host，认证边界由 token 承担（浏览器不知道 token 时无法借 DNS rebinding 发起有效请求）。其余情况（未显式开启、监听仍是回环、或 token 为空）仍按回环白名单返回 403 或拒绝启动。

### CORS / WebSocket Origin 白名单

- 允许的 Origin：`tauri://localhost`（macOS 生产）、`http://tauri.localhost`（Windows 生产）、`http://localhost:1420`（dev）。
- 命中时反射 Origin 并允许 `Content-Type, Authorization` 头；未命中不设置 `Access-Control-Allow-Origin`。
- **写请求（POST/PUT/DELETE 等）携带非白名单 Origin 时直接返回 403**——仅省略 CORS 响应头不足以阻止 CSRF；非浏览器客户端不带 Origin，仍按原有规则放行。
- WebSocket 握手校验 Origin：Origin 为空（非浏览器客户端）直接允许，否则必须在白名单内。

### pprof

pprof 服务与 API 分离，但同样只允许回环地址：未指定 host 或写成 `0.0.0.0`/`::` 时收敛到 `127.0.0.1`，显式指定非回环地址会拒绝启动。

---

## REST API 参考

### 通用响应格式

所有 REST 接口返回统一的 JSON 信封：

```json
{
  "ErrCode": 0,
  "ErrMsg": "",
  "Data": { ... }
}
```

- `ErrCode` — `0` 表示成功，`-1` 表示失败
- `ErrMsg` — 失败时的错误信息
- `Data` — 成功时的业务数据

### GET /api/v1/status

获取宿主运行状态与进程元信息。

**响应：**

```json
{
  "ErrCode": 0,
  "Data": {
    "running": true,
    "mode": "normal",
    "version": "v1.2.0",
    "commit": "abc1234",
    "protocol": 3,
    "startedAtMs": 1708243200000,
    "configPath": "/abs/path/active-config.json",
    "configError": ""
  }
}
```

| 字段 | 说明 |
|------|------|
| `running` | `true` 表示控制面后端运行中；`false` 表示宿主已进入停止流程（插件正在卸载） |
| `mode` | `normal` 或 `degraded`（配置损坏时仅运行 API 的降级模式） |
| `version` / `commit` | 构建期经 `-ldflags -X` 注入的版本信息 |
| `protocol` | sidecar 协议版本，当前为 `3` |
| `startedAtMs` | API 服务器启动时刻（Unix 毫秒）；uptime 由客户端自行换算 |
| `configPath` | 当前配置文件路径 |
| `configError` | 降级模式时的配置错误汇总；正常模式下省略（`omitempty`） |

---

### GET /api/v1/networks

获取所有已注册网络的状态。

**响应：**

```json
{
  "ErrCode": 0,
  "Data": [
    {
      "name": "mynet",
      "protocol": "tcp",
      "address": "server.example.com:9000",
      "domain": "my-pc",
      "state": "online",
      "lastErr": "",
      "startedAtMs": 1708243200000,
      "listens": 2,
      "dials": 5,
      "reconnects": 1,
      "bytesRead": 1048576,
      "bytesWritten": 524288,
      "rttMs": 52
    },
    {
      "name": "tcp",
      "protocol": "",
      "address": "",
      "domain": "",
      "state": "",
      "startedAtMs": 0,
      "listens": 0,
      "dials": 0,
      "reconnects": 0,
      "bytesRead": 0,
      "bytesWritten": 0,
      "rttMs": 0
    }
  ]
}
```

字段说明见 [NetworkStateDTO](#networkstatedto)。

注意：返回列表包含内置的 `tcp` 本地网络（`tcp4`/`tcp6` 已被过滤），其大部分字段为空值。上层应用通常可以过滤掉 `protocol` 为空的条目。

---

### GET /api/v1/services

获取所有已注册服务的状态。

**响应：**

```json
{
  "ErrCode": 0,
  "Data": [
    {
      "id": 1,
      "tunnelId": "b2c3d4e5-f6a7-4b8c-9d0e-1f2a3b4c5d6e",
      "type": "portproxy",
      "name": "rdp-forward",
      "status": "running",
      "listenURL": "vtcp://0:13389?authcode=***",
      "targetURL": "tcp://localhost:3389",
      "actives": 1,
      "dones": 12
    }
  ]
}
```

字段说明见 [ServiceStateDTO](#servicestatedto)。

`actives` 和 `dones` 是实时计数器：
- `actives` — 当前正在处理的活跃连接数
- `dones` — 已完成的连接总数

`id` 由服务构造时按进程内序号分配（同一进程生命周期内稳定）；`tunnelId` 来自配置中的 tunnel `id`；`lastErr` 记录运行时错误（如控制循环永久退出），无错误时省略。`listenURL`/`targetURL` 中的 `authcode`/`authcodeRef`、用户信息口令等敏感内容已由 `utils.RedactURL` 脱敏为 `***`（示例如此，不是明文密钥）。

---

### GET /api/v1/streams

获取数据流状态，包含活跃和已关闭的流。

**查询参数：**

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `limit` | int | `50` | 已关闭流的最大返回数量 |
| `network` | string | 空（全部） | 按网络名称过滤 |

**示例请求：**

```
GET /api/v1/streams?limit=20&network=mynet
```

**响应：**

```json
{
  "ErrCode": 0,
  "Data": [
    {
      "network": "mynet",
      "index": 7,
      "direction": "outbound",
      "localDomain": "my-pc",
      "localAddr": "1:13389",
      "remoteDomain": "remote-pc",
      "remoteAddr": "2:50001",
      "readBytes": 102400,
      "writeBytes": 51200,
      "isClosed": false,
      "readClosed": false,
      "writeClosed": false,
      "sentBufferCount": 12,
      "recvBufferCount": 34,
      "recvDataSize": 102400,
      "recvAckTotal": 102400,
      "sentAckTotal": 51200,
      "readBps": 2048.5,
      "writeBps": 1024.0,
      "serviceName": "rdp-forward",
      "pipeId": "rdp-forward#3"
    }
  ]
}
```

字段说明见 [StreamStateDTO](#streamstatedto)。

返回列表中活跃流在前，已关闭流在后。`limit` 参数仅限制已关闭流的数量，活跃流始终全量返回。

活跃流附带实时流速（`readBps`/`writeBps`，字节/秒）：由控制面按相邻两次采样的字节差计算，首次采样为 0。所有流均尝试标注归属服务（`serviceName`）与 pipe 配对标识（`pipeId`）；无法识别时省略对应字段。

---

### DELETE /api/v1/streams/{network}/{id}

手动关闭指定网络中的一条数据流。`id` 为流的 `index` 字段（见 [StreamStateDTO](#streamstatedto)）。

**示例请求：**

```
DELETE /api/v1/streams/mynet/7
```

**响应：**

```json
{
  "ErrCode": 0,
  "Data": {
    "status": "ok"
  }
}
```

失败时 `ErrCode` 为 `-1`，常见错误：

- `invalid stream id: 'xxx'` — id 不是合法整数
- `network='xxx' not found` — 网络不存在
- `network='xxx' does not support closing streams` — 该网络实现不支持手动关闭
- `stream='xxx/7' not found` — 流不存在或已关闭

---

### POST /api/v1/ping

对所有服务依赖的远程域名执行 ping 测试。后端探测仍按每个域名约 3 秒超时串行执行；API 侧同时最多允许 4 个 ping 后台任务，全量请求最多等待 8 秒，超出后返回 `"ping request timeout"`，并发槽位已满时立即返回 `"too many concurrent ping requests"`。后台任务结束前会持续占用槽位，客户端断开不会产生无界 goroutine。

**响应：**

```json
{
  "ErrCode": 0,
  "Data": [
    {
      "network": "mynet",
      "domain": "remote-pc",
      "result": "52.3ms",
      "usedServices": ["rdp-forward.target", "ssh-forward.target"]
    }
  ]
}
```

`usedServices` 列出了依赖该域名的服务及其依赖方向（`.listen` 或 `.target`）。

`result` 在成功时为延迟值（如 `"52.3ms"`），失败时为错误信息字符串。

---

### POST /api/v1/ping/{network}/{domain}

对指定网络中的指定域名执行单次 ping。

**示例请求：**

```
POST /api/v1/ping/mynet/remote-pc
```

单次请求最多等待 5 秒；与全量 ping 共用 4 个并发槽位，槽位已满时立即返回错误。

**响应：**

```json
{
  "ErrCode": 0,
  "Data": {
    "network": "mynet",
    "domain": "remote-pc",
    "result": "48.7ms"
  }
}
```

---

### PUT /api/v1/loglevel

动态调整全局日志级别。

**请求体：**

```json
{
  "level": "debug"
}
```

可选值：`debug`、`info`、`warn`、`error`

**响应：**

```json
{
  "ErrCode": 0,
  "Data": {
    "level": "debug"
  }
}
```

---

### GET /api/v1/system/listening-ports

枚举本机当前处于 LISTEN 状态的端口（用于隧道配置时辅助选端口）。

结果有 2 秒短缓存：同一实例内 2 秒以内的重复请求复用同一份采集结果，避免高频调用放大系统调用/子进程开销（macOS 上会执行 `netstat`）。

**响应：**

```json
{
  "ErrCode": 0,
  "Data": [
    {
      "port": 3389,
      "protocol": "tcp",
      "address": "0.0.0.0",
      "pid": 1234,
      "processName": "svchost",
      "procStartMs": 1708243200000
    }
  ]
}
```

- `pid` / `processName` 无法获取时省略
- `address` 为监听地址（同一 `(address, port, pid)` 三元组才去重，IPv4/IPv6 双栈与 SO_REUSEPORT 多进程不再合并）；通配监听在 macOS 上按协议族报告为 `0.0.0.0`（IPv4）或 `::`（IPv6 / 双栈）
- 数据源：macOS 解析 `netstat -anv -p tcp`（root / 其他用户进程的监听同样可见），其余平台为 gopsutil 连接表
- `procStartMs` 为**进程启动时刻**（Unix 毫秒）：系统不暴露 socket 创建时间，此为近似值（进程启动 ≠ 开始监听），无法获取时省略

---

### POST /api/v1/links

运行时热挂载一个 link（虚拟网络），免重启生效。持久化由调用方负责（见协议 §8 双写编排），sidecar 不写配置文件。

**请求体：**

```json
{
  "alias": "mynet",
  "url": "tcp://server.example.com:9000?as=mypc&auth=***"
}
```

`alias` 与 `url` 均必填；`url` 中必须包含 `as=` 参数。

**响应：**

```json
{
  "ErrCode": 0,
  "Data": {
    "status": "ok",
    "alias": "mynet"
  }
}
```

---

### POST /api/v1/tunnels

运行时热挂载一个 tunnel（端口转发 / socks5 服务），免重启生效。持久化约定同 `/links`。

**请求体：** 与配置文件 `tunnels` 数组单项相同（原始 JSON 结构，服务端会规范化）。

```json
{
  "name": "rdp-forward",
  "listen": "tcp://127.0.0.1:13389",
  "target": "vtcp://remote-pc.mynet:3389?authcode=key"
}
```

**响应：**

```json
{
  "ErrCode": 0,
  "Data": {
    "status": "ok",
    "name": "rdp-forward",
    "id": "a1b2c3d4-..."
  }
}
```

---

### POST /api/v1/stop

优雅停止 Agent。自 sidecar 协议 v1 起由 token 鉴权保护，**不再需要 `X-Confirm` 头**。

**响应：**

```json
{
  "ErrCode": 0,
  "Data": {
    "status": "stopping"
  }
}
```

调用后宿主会异步执行停止流程：按逆依赖顺序卸载全部插件（隧道 → 网络 → API），随后进程以退出码 0 退出（关停总预算 35 秒；若关停不完整则记录错误并以退出码 1 退出）。关停期间再次收到 SIGINT/SIGTERM 会强制以 130 退出。

---

## WebSocket 实时事件

### 连接

```
ws://127.0.0.1:8080/api/v1/ws?token=<REMOTEWORK_API_TOKEN>
```

token 存在时 `?token=` 查询参数必填（webview 无法自定义 WS 请求头），校验失败返回 401 且不升级连接。Origin 头必须在白名单内（见「认证与来源校验」），非浏览器客户端（无 Origin 头）不受此限。

连接建立后，客户端可以发送订阅消息选择感兴趣的主题或事件类型；未发送订阅消息时默认接收全部事件（兼容旧客户端）。

### 订阅与重同步

**主题订阅（协议 v2，推荐）**：按数据域订阅，订阅后服务端立即回发该主题的 `*.snapshot` 全量快照，随后跟增量事件——客户端不再需要先调 REST 全量接口：

```json
{
  "action": "subscribe",
  "topics": ["hub", "network", "service", "stream"]
}
```

- 可随时重发，全量替换订阅集；重发时已订阅主题不重复快照，新增主题补发快照
- `topics` 为空数组表示退订全部
- 可选主题：`hub`、`network`、`service`、`stream`；未知主题不产生快照

**主题级重同步**：客户端检测到 seq 空洞或本地状态异常时，请求服务端重发快照（无论是否已订阅）：

```json
{
  "action": "resync",
  "topics": ["stream"]
}
```

**事件类型订阅（协议 v1 兼容）**：按事件类型精确过滤，不触发快照：

```json
{
  "action": "subscribe",
  "events": ["network.state", "service.status"]
}
```

`topics` 与 `events` 互斥，同一条消息同时携带时 `topics` 优先；两者都缺省则不改变现有订阅。

### 事件信封格式

所有推送事件使用统一的信封：

```json
{
  "type": "network.state",
  "timestamp": 1708243200000,
  "topic": "network",
  "seq": 42,
  "data": { ... }
}
```

- `type` — 事件类型标识
- `timestamp` — 事件产生时间（Unix 毫秒）
- `topic` — 事件主题（事件类型的 `.` 前缀），协议 v2 新增
- `seq` — 主题内单调递增的序列号，协议 v2 新增。客户端检测 seq 空洞后可发送 `resync` 重同步
- `data` — 事件数据，结构因类型而异

**快照与增量的一致性约定**：`*.snapshot` 事件的 `seq` 表示"本快照已覆盖 ≤ 该 seq 的全部增量事件"。订阅登记先于快照生成，快照采集期间产生的增量事件（seq 大于快照 seq）会正常送达——客户端应用快照后，对 seq 大于快照 seq 的增量做幂等应用即可收敛（建议按实体主键 upsert，如流的 `index`）。

### 可靠性与背压

- 事件分两类投递策略：**可靠事件**（生命周期/状态类：`*.snapshot`、`stream.opened`、`stream.closed`、`network.state`、`service.status`、`hub.stopped`）客户端缓冲区满时服务端会**主动断开该连接**（客户端重连后走快照+增量恢复，不会静默丢事件）；**可丢弃事件**（计数器/速率类：`stream.stats`、`network.stats`、`service.stats`）缓冲区满时丢旧；stats 类事件无变化不发帧，丢失后由 ≤30s 的全量锚点帧兜底收敛
- 客户端发送缓冲区为 256 条消息
- 并发 WebSocket 客户端上限为 64；超限握手返回 **503**（`too many websocket clients`），避免无界连接消耗内存与广播扇出成本
- 慢客户端踢除是幂等的：多个广播并发命中同一满缓冲时只触发一次断连
- 单条客户端消息读取上限 4KB；消息 JSON 无法解析时忽略
- 心跳：服务端每 30s 发送 ping，60s 未收到 pong 判定死连并断开；写操作超时 10s。浏览器 WebSocket 会自动应答 ping

### 事件类型

#### 快照事件（hub/network/service/stream `.snapshot`）

协议 v2。客户端订阅主题或发送 `resync` 时服务端立即回发，作为该主题的状态基线：

| 事件 | 载荷 |
|------|------|
| `hub.snapshot` | Agent 状态全量：`running` / `mode` / `version` / `commit` / `protocol` / `startedAtMs` / `configPath` / `configError`。`mode`、`configError` 在进程生命周期内不变；uptime 不随事件发送，由 `startedAtMs` 客户端自行换算 |
| `network.snapshot` | `NetworkStateDTO[]` 全量（与 REST `/networks` 一致，过滤内置 `tcp4`/`tcp6`） |
| `service.snapshot` | `ServiceStateDTO[]` 全量 |
| `stream.snapshot` | 各网络活跃流的扁平数组（`StreamStateDTO`，含 `network` 字段、速率与归属服务标注）。**已关闭流历史不进推送通道**，排障查询仍用 REST `/streams` |

```json
{
  "type": "hub.snapshot",
  "timestamp": 1708243200000,
  "topic": "hub",
  "seq": 0,
  "data": {
    "running": true, "mode": "normal", "version": "v1.2.0", "commit": "abc1234",
    "protocol": 3, "startedAtMs": 1708243200000, "configPath": "/abs/path/active-config.json"
  }
}
```

#### network.state

网络连接状态发生变化时触发。由 kernel EventBus 实时推送，无延迟。

```json
{
  "type": "network.state",
  "timestamp": 1708243200000,
  "data": {
    "name": "mynet",
    "oldState": "offline",
    "newState": "connecting",
    "report": {
      "name": "mynet",
      "protocol": "tcp",
      "address": "server.example.com:9000",
      "domain": "my-pc",
      "state": "connecting",
      "listens": 0,
      "dials": 0
    }
  }
}
```

`report` 是事件触发时的完整网络状态快照（[NetworkStateDTO](#networkstatedto)），可能为 `null`（查询失败时）。

典型的状态变化序列：
- 首次连接：`offline → connecting → online`
- 断线重连：`online → offline → connecting → online`
- 手动关闭：`* → closed`

#### service.status

服务状态发生变化时触发。由 kernel EventBus 实时推送。

```json
{
  "type": "service.status",
  "timestamp": 1708243200000,
  "data": {
    "name": "rdp-forward",
    "oldStatus": "init",
    "newStatus": "running",
    "service": {
      "id": 1,
      "type": "portproxy",
      "name": "rdp-forward",
      "status": "running",
      "listenURL": "vtcp://0:13389?authcode=***",
      "targetURL": "tcp://localhost:3389",
      "actives": 0,
      "dones": 0
    }
  }
}
```

`service` 是事件触发时的完整服务状态快照（[ServiceStateDTO](#servicestatedto)），可能为 `null`。

典型的状态变化序列：
- 正常启动：`uninit → init → running`
- 初始化失败：`uninit → init → init failed`
- 正常停止：`running → stopped`

注意：除本事件外，poller 在计数器（actives/dones/lastErr）变化时也会广播同类型事件（v1 兼容行为）；v2 客户端的计数器应以 `service.stats` 为准。

#### stream.opened / stream.closed

协议 v2。tcp 与 flex 网络的流打开与完全关闭均由事件驱动**即时推送**（tcp 侧由连接追踪器回调、flex 侧由 Trace 钩子，经 kernel EventBus 汇入），不再依赖 poller 差分；并按约 50ms 窗口合并批量发送（一条事件携带数组，洪流场景不会逐条刷屏）：

```json
{
  "type": "stream.opened",
  "timestamp": 1708243200000,
  "topic": "stream",
  "seq": 123,
  "data": {
    "network": "mynet",
    "streams": [ { "network": "mynet", "index": 7, "createdAtMs": 1708243200000, "...": "..." } ]
  }
}
```

- `stream.opened`：新建流，含 `index` 与 `createdAtMs`，归属服务（`serviceName`）与 pipe 配对标识（`pipeId`）按填充时可识别的标注
- `stream.closed`：完全关闭的流，另含 `closedAtMs`、`closeReason` 与最终字节计数；`serviceName`/`pipeId` 与 poller 差分版一致保持空值（关闭时归属大概率已注销）
- **延迟语义**：opened 为即时（内核事件到广播 <100ms 量级）；tcp 流的 closed 即时，flex 流的 closed 受流滞留期（约 2 秒，等待晚到数据包）影响滞后送达，属已知行为
- poller 差分版 `stream.update` 保留作为过渡（v1 兼容），v2 客户端应以 `stream.opened/closed` 为准

#### stream.update

数据流发生变化时触发。由后台轮询器（StatePoller）的快节拍定期检测差异后推送，延迟取决于 `streamStatsInterval` 配置（默认 1 秒）。

```json
{
  "type": "stream.update",
  "timestamp": 1708243200000,
  "data": {
    "network": "mynet",
    "opened": [
      {
        "network": "mynet",
        "index": 7,
        "direction": "outbound",
        "localDomain": "my-pc",
        "localAddr": "1:13389",
        "remoteDomain": "remote-pc",
        "remoteAddr": "2:50001",
        "readBytes": 0,
        "writeBytes": 0,
        "isClosed": false,
        "readClosed": false,
        "writeClosed": false,
        "sentBufferCount": 0,
        "recvBufferCount": 0,
        "recvDataSize": 0,
        "recvAckTotal": 0,
        "sentAckTotal": 0,
        "readBps": 0,
        "writeBps": 0,
        "serviceName": "rdp-forward",
      "pipeId": "rdp-forward#3"
      }
    ],
    "closed": []
  }
}
```

- `opened` — 自上次轮询以来新建的流（含速率与归属服务标注）
- `closed` — 自上次轮询以来关闭的流（`serviceName` 大概率已注销，保持空值）

两个数组都可能为空，但至少有一个非空时才会推送事件。

#### stream.stats

活跃流状态的周期增量推送（协议 v3 起）。由后台轮询器（StatePoller）的快节拍驱动，采样间隔取决于 `streamStatsInterval` 配置（默认 1 秒），但**不再每 tick 发帧**——每条流的 DTO 与上次已广播值逐字段比对（DTO 全部为时间无关字段，存活时长等派生量由客户端按 `createdAtMs` 自行换算），仅**状态有变化的流**进入当次帧：

```json
{
  "type": "stream.stats",
  "timestamp": 1708243200000,
  "data": {
    "network": "mynet",
    "streams": [
      {
        "network": "mynet",
        "index": 7,
        "direction": "outbound",
        "localDomain": "my-pc",
        "localAddr": "1:13389",
        "remoteDomain": "remote-pc",
        "remoteAddr": "2:50001",
        "readBytes": 102400,
        "writeBytes": 51200,
        "createdAtMs": 1708243200000,
        "isClosed": false,
        "readClosed": false,
        "writeClosed": false,
        "sentBufferCount": 12,
        "recvBufferCount": 34,
        "recvDataSize": 102400,
        "recvAckTotal": 102400,
        "sentAckTotal": 51200,
        "readBps": 2048.5,
        "writeBps": 1024.0,
        "serviceName": "rdp-forward",
      "pipeId": "rdp-forward#3"
      }
    ]
  }
}
```

- `streams` — 自上次广播以来状态有变化的流（[StreamStateDTO](#streamstatedto)），**upsert 语义**：客户端按 `(network, index)` 合并，整条 DTO 替换本地副本
- `full: true`（仅锚点帧携带）— 每 30 秒及网络首帧发送一次全量锚点：`streams` 为该网络当前**全部**活跃流（可为空数组），客户端**整体替换**该网络的活跃流集合；可丢弃帧丢失后由锚点帧兜底收敛
- 该网络无任何变化时不发帧（不消耗主题 seq，不会产生 seq 空洞）；**不得**把本事件的到帧节奏当作心跳或定时器
- 流的移除不走本事件，由可靠事件 `stream.closed`（及兜底的 `stream.update`）送达
- `readBps`/`writeBps` — 按相邻两次轮询的字节差计算的实时流速（字节/秒），流首次出现时为 0；流速随流量归零也是一次状态变化，会触发一帧增量

#### network.stats

协议 v2。每 tick 推送一次全部网络的计数器快照（含 `listens`/`dials`/`reconnects`、`bytesRead`/`bytesWritten`/`rttMs`、`startedAtMs`），计数器不再需要 REST 轮询。

协议 v3 起：快照与上次发送完全一致时不发帧（无变化不消耗 seq），至多 30s 有一帧保底；载荷数组按 `name` 排序。

```json
{
  "type": "network.stats",
  "topic": "network",
  "seq": 45,
  "data": { "networks": [ { "name": "mynet", "state": "online", "bytesRead": 1048576, "...": "..." } ] }
}
```

#### service.stats

协议 v2。每 tick 推送一次全部服务的计数器快照（`actives`/`dones`/`lastErr`），替代轮询 `GET /services`。

协议 v3 起：快照与上次发送完全一致时不发帧（无变化不消耗 seq），至多 30s 有一帧保底；载荷数组按 `name` 排序。

```json
{
  "type": "service.stats",
  "topic": "service",
  "seq": 45,
  "data": { "services": [ { "name": "rdp-forward", "actives": 1, "dones": 12, "...": "..." } ] }
}
```

#### hub.stopped

宿主停止运行时触发。优雅停机（`/stop` 或 OS 信号）时由服务器在关闭连接前**即时推送**；此外轮询器检测 `IsRunning()` 状态变化作为兜底。

```json
{
  "type": "hub.stopped",
  "timestamp": 1708243200000,
  "data": {}
}
```

收到此事件后，WebSocket 连接将很快断开。上层应用应据此触发重连逻辑或显示离线状态。

---

## 数据模型

### NetworkStateDTO

| 字段 | 类型 | 说明 |
|------|------|------|
| `name` | string | 网络名称（如 `"mynet"`、`"tcp"`） |
| `protocol` | string | 传输协议（`"tcp"` 或 `"ws"`） |
| `address` | string | 中转服务器地址 |
| `domain` | string | 本机在虚拟网络中的域名 |
| `state` | string | 当前状态：`offline` / `connecting` / `online` / `closed` |
| `lastErr` | string | 最近一次错误信息（无错误时省略） |
| `startedAtMs` | int64 | 网络对象创建时刻（Unix 毫秒，自创建/热挂载起算；存活时长由客户端按 now-startedAtMs 自行换算） |
| `listens` | int32 | 在此网络上创建的 listener 数量 |
| `dials` | int32 | 通过此网络发起的 dial 数量 |
| `reconnects` | int64 | 累计断线重连次数（虚拟网络有效，本地网络恒为 0） |
| `bytesRead` | int64 | 节点累计读取字节数（虚拟网络有效） |
| `bytesWritten` | int64 | 节点累计写入字节数（虚拟网络有效） |
| `rttMs` | int64 | 最近一次心跳 RTT（毫秒，0 表示未测量或不支持） |

### ServiceStateDTO

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | int32 | 服务注册序号（服务构造时按进程内序号分配） |
| `tunnelId` | string | 配置中该 tunnel 的 UUID（`id` 字段） |
| `type` | string | 服务类型：`portproxy` / `socks5` |
| `name` | string | 服务名称（配置中的 `name` 字段或默认名） |
| `status` | string | 当前状态：`uninit` / `init` / `running` / `pending` / `stopped` / `init failed` |
| `lastErr` | string | 最近一次运行时错误（无错误时省略） |
| `listenURL` | string | 监听地址 URL（敏感参数已脱敏） |
| `targetURL` | string | 目标地址 URL（socks5 类型为空字符串；敏感参数已脱敏） |
| `actives` | int32 | 当前活跃连接数 |
| `dones` | int32 | 已完成连接总数 |

### StreamStateDTO

| 字段 | 类型 | 说明 |
|------|------|------|
| `network` | string | 所属网络名称 |
| `index` | int32 | 流序号（网络内唯一标识，用于手动关闭） |
| `direction` | string | 流方向：`outbound`（本机发起）/ `inbound`（对端发起）/ `unknown` |
| `localDomain` | string | 本地域名 |
| `localAddr` | string | 本地地址（`ip:port` 格式） |
| `remoteDomain` | string | 远程域名 |
| `remoteAddr` | string | 远程地址 |
| `clientPid` | int32 | 客户端进程 PID：仅 `direction=inbound` 且 `remoteAddr` 为 loopback 的流在开流时解析一次并记忆；其余场景或解析失败（权限不足/进程已退出/非 TCP 传输）时省略 |
| `clientProcName` | string | 客户端进程名（解析条件同 `clientPid`） |
| `clientProcStartMs` | int64 | 客户端进程启动时刻（Unix 毫秒），供甄别 PID 复用（解析条件同 `clientPid`） |
| `readBytes` | int64 | 已读取字节数 |
| `writeBytes` | int64 | 已写入字节数 |
| `createdAtMs` | int64 | 流发起时刻（Unix 毫秒）；存活时长由客户端按 now-createdAtMs（已关闭流按 closedAtMs-createdAtMs）自行换算 |
| `closedAtMs` | int64 | 流完全关闭时刻（Unix 毫秒），仅已关闭流携带 |
| `isClosed` | bool | 是否已关闭 |
| `readClosed` | bool | 读方向已关闭（半关闭状态） |
| `writeClosed` | bool | 写方向已关闭（半关闭状态） |
| `closeReason` | string | 关闭原因：`local` / `remote` / `node` / `manual`（未记录时省略） |
| `sentBufferCount` | int32 | 累计发送缓冲数 |
| `recvBufferCount` | int32 | 累计接收缓冲数 |
| `recvDataSize` | int64 | 累计接收数据字节数 |
| `recvAckTotal` | int64 | 累计接收确认字节数 |
| `sentAckTotal` | int64 | 累计发送确认字节数 |
| `readBps` | float64 | 实时读流速（字节/秒），仅活跃流有效，首次采样为 0 |
| `writeBps` | float64 | 实时写流速（字节/秒），仅活跃流有效，首次采样为 0 |
| `serviceName` | string | 归属服务名（无法识别时省略） |
| `pipeId` | string | pipe 配对标识（未配对时省略）：经 tunnel 中转的连接会在两端流上携带相同值（形如 `服务名#序号`，仅进程运行期内唯一），客户端可据此把 listen 侧与 target 侧的流关联为端到端视图；连接关闭后配对注销，已关闭流不携带 |

### PingResultDTO

| 字段 | 类型 | 说明 |
|------|------|------|
| `network` | string | 网络名称 |
| `domain` | string | 目标域名 |
| `result` | string | 延迟值或错误信息 |
| `usedServices` | []string | 依赖此域名的服务列表 |

---

## 集成示例

### curl 快速验证

```bash
# 若进程以 REMOTEWORK_API_TOKEN 启动，所有请求需带：
#   -H "Authorization: Bearer $REMOTEWORK_API_TOKEN"

# 检查运行状态
curl http://127.0.0.1:8080/api/v1/status

# 查看网络列表
curl http://127.0.0.1:8080/api/v1/networks

# 查看服务列表
curl http://127.0.0.1:8080/api/v1/services

# 查看数据流（限制 20 条，过滤指定网络）
curl "http://127.0.0.1:8080/api/v1/streams?limit=20&network=mynet"

# 手动关闭一条数据流（id 为流的 index 字段）
curl -X DELETE http://127.0.0.1:8080/api/v1/streams/mynet/7

# 查看本机监听端口
curl http://127.0.0.1:8080/api/v1/system/listening-ports

# 全量 ping 测试
curl -X POST http://127.0.0.1:8080/api/v1/ping

# 单域名 ping
curl -X POST http://127.0.0.1:8080/api/v1/ping/mynet/remote-pc

# 调整日志级别
curl -X PUT http://127.0.0.1:8080/api/v1/loglevel \
  -H "Content-Type: application/json" \
  -d '{"level":"debug"}'

# 热挂载 link / tunnel
curl -X POST http://127.0.0.1:8080/api/v1/links \
  -H "Content-Type: application/json" \
  -d '{"alias":"mynet","url":"tcp://server.example.com:9000?as=mypc&auth=***"}'
curl -X POST http://127.0.0.1:8080/api/v1/tunnels \
  -H "Content-Type: application/json" \
  -d '{"name":"rdp-forward","listen":"tcp://127.0.0.1:13389","target":"vtcp://remote-pc.mynet:3389?authcode=key"}'

# 停止 Agent（无需 X-Confirm，由 token 鉴权保护）
curl -X POST http://127.0.0.1:8080/api/v1/stop
```

### JavaScript WebSocket 客户端

```javascript
const token = '<REMOTEWORK_API_TOKEN>'; // standalone 无 token 时可省略
const ws = new WebSocket(`ws://127.0.0.1:8080/api/v1/ws?token=${encodeURIComponent(token)}`);

ws.onopen = () => {
  // 订阅感兴趣的事件（可选，不发则接收全部）
  ws.send(JSON.stringify({
    action: 'subscribe',
    events: ['network.state', 'service.status', 'stream.update']
  }));
};

ws.onmessage = (event) => {
  const evt = JSON.parse(event.data);
  console.log(`[${evt.type}] @ ${new Date(evt.timestamp).toISOString()}`);

  switch (evt.type) {
    case 'network.state':
      console.log(`网络 ${evt.data.name}: ${evt.data.oldState} → ${evt.data.newState}`);
      break;
    case 'service.status':
      console.log(`服务 ${evt.data.name}: ${evt.data.oldStatus} → ${evt.data.newStatus}`);
      break;
    case 'stream.update':
      console.log(`网络 ${evt.data.network}: +${evt.data.opened?.length || 0} -${evt.data.closed?.length || 0}`);
      break;
    case 'stream.stats':
      if (evt.data.full) {
        console.log(`网络 ${evt.data.network}: 全量锚点 ${evt.data.streams.length} 条活跃流`);
      } else {
        console.log(`网络 ${evt.data.network}: ${evt.data.streams.length} 条流状态变更（upsert）`);
      }
      break;
    case 'hub.stopped':
      console.log('Agent 已停止');
      break;
  }
};

ws.onclose = () => {
  console.log('连接断开，考虑重连...');
};
```

### Go 客户端轮询示例

```go
package main

import (
    "encoding/json"
    "fmt"
    "net/http"
    "time"
)

type APIResponse struct {
    ErrCode int             `json:"ErrCode"`
    ErrMsg  string          `json:"ErrMsg"`
    Data    json.RawMessage `json:"Data"`
}

type NetworkState struct {
    Name        string `json:"name"`
    State       string `json:"state"`
    Domain      string `json:"domain"`
    StartedAtMs int64  `json:"startedAtMs"`
}

func main() {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()

    for range ticker.C {
        resp, err := http.Get("http://127.0.0.1:8080/api/v1/networks")
        if err != nil {
            fmt.Println("请求失败:", err)
            continue
        }

        var apiResp APIResponse
        json.NewDecoder(resp.Body).Decode(&apiResp)
        resp.Body.Close()

        if apiResp.ErrCode != 0 {
            fmt.Println("API 错误:", apiResp.ErrMsg)
            continue
        }

        var networks []NetworkState
        json.Unmarshal(apiResp.Data, &networks)

        for _, n := range networks {
            if n.State == "online" {
                fmt.Printf("✓ %s (%s) 在线 %dms\n", n.Name, n.Domain, time.Since(time.UnixMilli(n.StartedAtMs)).Milliseconds())
            }
        }
    }
}
```

### 构建监控仪表盘的建议

1. **初始加载** — 并行请求 `/status`、`/networks`、`/services` 获取全量状态
2. **建立 WebSocket** — 连接 `/ws` 并订阅全部事件
3. **增量更新** — 根据 WebSocket 事件更新本地状态
   - `network.state` → 更新网络面板
   - `service.status` → 更新服务面板
   - `stream.update` → 更新数据流列表（开/关差分）
   - `stream.stats` → 按 `(network, index)` upsert 变化的流（增量，含实时流速）；`full=true` 锚点帧整体替换该网络活跃流集合
   - `hub.stopped` → 显示离线状态
4. **按需查询** — 用户点击 "Ping" 按钮时调用 `/ping`
5. **断线重连** — WebSocket 断开后指数退避重连，重连后重新拉取全量状态

---

## 设计决策与注意事项

### 事件推送机制

API 模块使用两种互补的事件推送机制：

| 机制 | 事件类型 | 延迟 | 原理 |
|------|----------|------|------|
| kernel EventBus | `network.state`、`service.status` | 实时 | 插件在状态变更时向事件总线发布事件 |
| 流生命周期回调（tcp 连接追踪器 + flex Trace 钩子） | `stream.opened`、`stream.closed` | 实时（flex 的 closed 含约 2s 滞留期；tcp 的 closed 即时） | tcp 追踪器在连接注册/完全关闭时回调、flex 节点在流打开/完全关闭时回调，均经 EventBus 汇入，api 层按约 50ms 窗口合并批量广播 |
| StatePoller 快节拍 | `stream.update`、`stream.stats` | ≤ streamStatsInterval（默认 1s） | 定期拉取各网络流快照：增量广播变化的流（含实时速率；无变化不发，30s 全量锚点兜底）并做开/关差分兜底 |
| StatePoller 慢节拍 | `network.stats`、`service.stats`、`hub.stopped` | ≤ pollInterval（默认 5s） | 定期快照对比，检测状态差异并刷新计数器 |

网络和服务状态变化频率低但重要性高，采用实时回调；数据流的生命周期事件驱动（tcp 与 flex 均已接入），速率/计数器等高频数据采用快节拍周期快照（stats 类事件可丢弃补新）。

### 安全性

- 默认监听 `127.0.0.1`，仅本机可访问；非回环监听必须 `allowNonLoopback=true` 且 token 非空，否则拒绝启动
- token 鉴权：`Authorization: Bearer <token>`（HTTP）/ `?token=` 或 Bearer 头（WS），token 经 env 注入不落盘；常量时间比较；standalone 无 token 且绑定回环时保持开放并输出 warn 日志
- 写请求（POST/PUT/DELETE）的 Origin 不在白名单时直接 403；无 Origin 的非浏览器客户端允许
- Host 头白名单（`127.0.0.1` / `localhost` / `::1`）防 DNS rebinding；仅在显式开启非回环监听且 token 非空时放行任意 Host（认证由 token 承担）
- CORS 与 WS Origin 收敛为白名单（`tauri://localhost`、`http://tauri.localhost`、`http://localhost:1420`）
- `listenURL`/`targetURL`、`lastErr`、`configError` 中的 `authcode`/`authcodeRef`/`auth`/`token`/`password` 与 URL userinfo 经 `utils.RedactURL` 脱敏后返回
- pprof 服务强制回环监听，非回环地址拒绝启动
- HTTP 读头/读/写/空闲超时为 5s/15s/30s/60s；JSON 请求体上限 1MB

### 并发安全

- 所有 REST 接口都是并发安全的，kernel 宿主内部以 actor 模型串行化插件状态变更
- WebSocket 广播使用快照复制模式，不会阻塞宿主的正常运行
- 事件回调在独立 goroutine 中执行，不会阻塞插件的状态变更流程

### 性能考量

- `GET /streams` 返回的已关闭流数量受 `limit` 参数控制，避免大量历史数据
- `POST /ping` 会阻塞约 3 秒（ping 超时），不适合高频调用
- WebSocket 客户端的发送缓冲区为 256 条消息：可丢弃事件（`*.stats` 类）缓冲区满时丢旧补新；可靠事件（生命周期/快照类）投递失败时服务端主动断开该连接，客户端重连后经快照+增量恢复
- StatePoller 的轮询间隔建议 3-10 秒，过短会增加 CPU 开销
- stats 类事件（stream/network/service）基于时间无关 DTO 做增量比对：无变化不发帧，每 30s 一帧全量锚点兜底可丢弃帧的丢失；活跃流/网络的存活时长等派生量不进协议，由客户端按 `createdAtMs`/`startedAtMs` 自行换算

### 错误处理

上层应用应始终检查 `ErrCode` 字段：

```javascript
const response = await fetch('/api/v1/networks');
const json = await response.json();

if (json.ErrCode !== 0) {
  console.error('API 错误:', json.ErrMsg);
  return;
}

// 正常处理 json.Data
```

常见错误场景：
- 没有任何隧道服务时调用全量 ping → `"NO SERVICES"`
- ping 不存在的网络 → `"network='xxx' not found"`
- 无效的日志级别 → `"invalid level, use: debug/info/warn/error"`

---

## Sidecar 协议

当本进程作为 Tauri 桌面端的 sidecar 运行时，进程间契约（启动握手、stdout 行协议、token 颁发、退出码等）以 [`docs/tauri-sidecar-protocol.md`](../docs/tauri-sidecar-protocol.md) 为准。本文档描述的是 C4 通道（REST/WS）的接口细节，两处关于认证、来源校验与 `/status`、`/stop` 语义的定义保持一致。
