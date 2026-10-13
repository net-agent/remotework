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
- **只读为主** — 大部分接口是状态查询，写操作仅有 `stop`、`loglevel` 和运行时挂载（link/tunnel）
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
| `vtcp://` | Flex 虚拟网络连接 | `vtcp://remote-pc:3389?secret=mykey` |
| `ws://` | WebSocket 传输 | `ws://server.example.com:8080/ws` |

`?secret=` 参数表示启用了预共享密钥加密。

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
    "pollInterval": 5
  }
}
```

**TOML 格式：**

```toml
[api]
enable = true
listen = "127.0.0.1:8080"
pollInterval = 5
```

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `enable` | bool | `false` | 是否启用 API 服务器 |
| `listen` | string | `"127.0.0.1:8080"` | 监听地址。默认仅本地访问，如需远程访问改为 `"0.0.0.0:8080"` |
| `pollInterval` | int | `5` | 数据流变化轮询间隔（秒） |

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
    Meta:  api.Meta{Protocol: 1, Mode: "normal", ConfigPath: resolvedPath},
}))

// 方式二：不用 kernel，自行实现 api.Backend 接口后直接起服务
apiServer := api.New(myBackend, api.Config{
    Listen: "127.0.0.1:8080",
    Token:  "...",          // 可空
    Meta:   api.Meta{Protocol: 1, Mode: "normal"},
}, nil) // nil 使用默认 logger
if err := apiServer.Start(); err != nil {
    // 端口绑定失败会同步返回错误
}
defer apiServer.Stop()
// apiServer.Addr() 返回实际绑定地址（Listen 配 ":0" 时为系统分配端口）
```

`api.New()` 接收三个参数：
- `api.Backend` — 控制面后端接口实现（必须）
- `api.Config` — 运行时选项（监听地址、轮询间隔、token、元信息），与文件配置解耦
- `*slog.Logger` — 日志器，传 `nil` 使用默认

`Start()` 是**同步绑定**：端口被占用等错误立即返回，不会等到首次请求才暴露。

---

## 认证与来源校验

自 sidecar 协议 v1 起，API 具备完整的安全层（standalone 开放模式下同样生效）：

### Bearer token

- 进程经环境变量 `REMOTEWORK_API_TOKEN` 获取 token（Host 每次 spawn 重新生成），token 不落盘、不出现在命令行参数中。
- token 存在时，所有 `/api/v1/*` HTTP 请求必须携带 `Authorization: Bearer <token>`，缺失或错误返回 **401**（统一错误信封）。
- WebSocket 握手经 `?token=<token>` 查询参数传递（webview 无法自定义 WS 请求头），校验失败返回 **401** 且不升级连接。
- token 为空（standalone CLI 场景）时 API 保持开放，启动日志输出 warn 提示 `api server running without auth token`。

### Host 头白名单（防 DNS rebinding）

HTTP 请求的 Host 头去端口后必须在白名单内：`127.0.0.1`、`localhost`、`::1`、`[::1]`，否则返回 **403**。

### CORS / WebSocket Origin 白名单

- 允许的 Origin：`tauri://localhost`（macOS 生产）、`http://tauri.localhost`（Windows 生产）、`http://localhost:1420`（dev）。
- 命中时反射 Origin 并允许 `Content-Type, Authorization` 头；未命中不设置 `Access-Control-Allow-Origin`。
- WebSocket 握手校验 Origin：Origin 为空（非浏览器客户端）直接允许，否则必须在白名单内。

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
    "protocol": 1,
    "uptimeMs": 12345,
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
| `protocol` | sidecar 协议版本，当前为 `1` |
| `uptimeMs` | API 服务器启动至今的毫秒数 |
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
      "aliveMs": 360000,
      "listens": 2,
      "dials": 5,
      "reconnects": 1
    },
    {
      "name": "tcp",
      "protocol": "",
      "address": "",
      "domain": "",
      "state": "",
      "aliveMs": 0,
      "listens": 0,
      "dials": 0,
      "reconnects": 0
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
      "type": "portproxy",
      "name": "rdp-forward",
      "status": "running",
      "listenURL": "vtcp://0:13389?secret=mykey",
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
      "localDomain": "my-pc",
      "localAddr": "1:13389",
      "remoteDomain": "remote-pc",
      "remoteAddr": "2:50001",
      "readBytes": 102400,
      "writeBytes": 51200,
      "aliveMs": 15000,
      "isClosed": false
    }
  ]
}
```

字段说明见 [StreamStateDTO](#streamstatedto)。

返回列表中活跃流在前，已关闭流在后。`limit` 参数仅限制已关闭流的数量，活跃流始终全量返回。

---

### POST /api/v1/ping

对所有服务依赖的远程域名执行 ping 测试。此接口会阻塞约 3 秒（每个域名的超时时间）。

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

**响应：**

```json
{
  "ErrCode": 0,
  "Data": [
    {
      "port": 3389,
      "protocol": "tcp",
      "pid": 1234,
      "processName": "svchost"
    }
  ]
}
```

`pid` / `processName` 无法获取时省略。

---

### POST /api/v1/links

运行时热挂载一个 link（虚拟网络），免重启生效。持久化由调用方负责（见协议 §8 双写编排），sidecar 不写配置文件。

**请求体：**

```json
{
  "alias": "mynet",
  "url": "tcp://server.example.com:9000?as=mypc&auth=secret"
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

调用后宿主会异步执行停止流程：按逆依赖顺序卸载全部插件（隧道 → 网络 → API），随后进程以退出码 0 退出（停机宽限 15 秒）。

---

## WebSocket 实时事件

### 连接

```
ws://127.0.0.1:8080/api/v1/ws?token=<REMOTEWORK_API_TOKEN>
```

token 存在时 `?token=` 查询参数必填（webview 无法自定义 WS 请求头），校验失败返回 401 且不升级连接。Origin 头必须在白名单内（见「认证与来源校验」），非浏览器客户端（无 Origin 头）不受此限。

连接建立后，客户端可以发送订阅消息选择感兴趣的事件类型；未发送订阅消息时默认接收全部事件。

### 订阅

发送 JSON 消息：

```json
{
  "action": "subscribe",
  "events": ["network.state", "service.status"]
}
```

可随时重新发送订阅消息更改订阅列表。

### 事件信封格式

所有推送事件使用统一的信封：

```json
{
  "type": "network.state",
  "timestamp": 1708243200000,
  "data": { ... }
}
```

- `type` — 事件类型标识
- `timestamp` — 事件产生时间（Unix 毫秒）
- `data` — 事件数据，结构因类型而异

### 事件类型

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
      "aliveMs": 0,
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
      "listenURL": "vtcp://0:13389?secret=mykey",
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

#### stream.update

数据流发生变化时触发。由后台轮询器（StatePoller）定期检测差异后推送，延迟取决于 `pollInterval` 配置。

```json
{
  "type": "stream.update",
  "timestamp": 1708243200000,
  "data": {
    "network": "mynet",
    "opened": [
      {
        "network": "mynet",
        "localDomain": "my-pc",
        "localAddr": "1:13389",
        "remoteDomain": "remote-pc",
        "remoteAddr": "2:50001",
        "readBytes": 0,
        "writeBytes": 0,
        "aliveMs": 100,
        "isClosed": false
      }
    ],
    "closed": []
  }
}
```

- `opened` — 自上次轮询以来新建的流
- `closed` — 自上次轮询以来关闭的流

两个数组都可能为空，但至少有一个非空时才会推送事件。

#### hub.stopped

宿主停止运行时触发。由轮询器检测 `IsRunning()` 状态变化后推送。

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
| `aliveMs` | int64 | 当前连接存活时间（毫秒），离线时为 0 |
| `listens` | int32 | 在此网络上创建的 listener 数量 |
| `dials` | int32 | 通过此网络发起的 dial 数量 |
| `reconnects` | int64 | 累计断线重连次数（虚拟网络有效，本地网络恒为 0） |

### ServiceStateDTO

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | int32 | 服务注册序号 |
| `type` | string | 服务类型：`portproxy` / `socks5` / `rdpserver` |
| `name` | string | 服务名称（配置中的 `log` 字段或默认名） |
| `status` | string | 当前状态：`uninit` / `init` / `running` / `stopped` / `init failed` |
| `listenURL` | string | 监听地址 URL |
| `targetURL` | string | 目标地址 URL（socks5 类型无此字段） |
| `actives` | int32 | 当前活跃连接数 |
| `dones` | int32 | 已完成连接总数 |

### StreamStateDTO

| 字段 | 类型 | 说明 |
|------|------|------|
| `network` | string | 所属网络名称 |
| `localDomain` | string | 本地域名 |
| `localAddr` | string | 本地地址（`ip:port` 格式） |
| `remoteDomain` | string | 远程域名 |
| `remoteAddr` | string | 远程地址 |
| `readBytes` | int64 | 已读取字节数 |
| `writeBytes` | int64 | 已写入字节数 |
| `aliveMs` | int64 | 存活时间（毫秒） |
| `isClosed` | bool | 是否已关闭 |

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
  -d '{"alias":"mynet","url":"tcp://server.example.com:9000?as=mypc&auth=secret"}'
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
    Name    string `json:"name"`
    State   string `json:"state"`
    Domain  string `json:"domain"`
    AliveMs int64  `json:"aliveMs"`
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
                fmt.Printf("✓ %s (%s) 在线 %dms\n", n.Name, n.Domain, n.AliveMs)
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
   - `stream.update` → 更新数据流列表
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
| StatePoller 轮询 | `stream.update`、`hub.stopped` | ≤ pollInterval | 定期快照对比，检测差异 |

网络和服务状态变化频率低但重要性高，采用实时回调；数据流变化频繁且无内置钩子，采用轮询差异检测。

### 安全性

- 默认监听 `127.0.0.1`，仅本机可访问
- token 鉴权：`Authorization: Bearer <token>`（HTTP）/ `?token=`（WS），token 经 env 注入不落盘；standalone 无 token 时保持开放并输出 warn 日志
- Host 头白名单（`127.0.0.1` / `localhost` / `::1`）防 DNS rebinding
- CORS 与 WS Origin 收敛为白名单（`tauri://localhost`、`http://tauri.localhost`、`http://localhost:1420`）
- API 不暴露密码等敏感配置信息

### 并发安全

- 所有 REST 接口都是并发安全的，kernel 宿主内部以 actor 模型串行化插件状态变更
- WebSocket 广播使用快照复制模式，不会阻塞宿主的正常运行
- 事件回调在独立 goroutine 中执行，不会阻塞插件的状态变更流程

### 性能考量

- `GET /streams` 返回的已关闭流数量受 `limit` 参数控制，避免大量历史数据
- `POST /ping` 会阻塞约 3 秒（ping 超时），不适合高频调用
- WebSocket 客户端的发送缓冲区为 64 条消息，缓冲区满时新消息会被丢弃（而非阻塞）
- StatePoller 的轮询间隔建议 3-10 秒，过短会增加 CPU 开销

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
