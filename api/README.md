# Remotework API 开发者手册

## 目录

- [概述](#概述)
- [核心架构理解](#核心架构理解)
- [配置与启动](#配置与启动)
- [REST API 参考](#rest-api-参考)
- [WebSocket 实时事件](#websocket-实时事件)
- [数据模型](#数据模型)
- [集成示例](#集成示例)
- [设计决策与注意事项](#设计决策与注意事项)

---

## 概述

`api` 包为 Remotework Agent 提供 HTTP REST + WebSocket 控制面接口。它将 `agent.Hub` 的状态查询和基本操作能力暴露给上层应用（Web UI、CLI 工具、监控系统等）。

核心定位：

- **仅控制面** — 不代理任何数据流量，只提供状态查询和管理操作
- **只读为主** — 大部分接口是状态查询，仅 `stop` 和 `loglevel` 涉及写操作
- **实时感知** — WebSocket 推送网络状态、服务状态、数据流变化等事件
- **零业务逻辑** — `api` 包接收 `*agent.Hub` 作为依赖，所有业务逻辑在 `agent` 包内

```
应用层 (Web UI / CLI / 监控)
    │
    ├─ HTTP REST ──→ 状态查询、ping、日志级别、停止
    │
    └─ WebSocket ──→ 实时事件推送
         │
    api.Server ──→ agent.Hub（只读访问 + Stop）
```

---

## 核心架构理解

在使用 API 之前，理解 Agent 的内部架构有助于正确解读返回数据。

### Hub — 中央编排器

`Hub` 是 Agent 的核心，管理两类资源：

- **Networks（网络）** — 通过中转服务器建立的虚拟网络连接
- **Services（服务）** — 运行在虚拟网络之上的业务（端口转发、SOCKS5 代理等）

```
Hub
 ├── NetworkRegistry
 │    ├── tcp / tcp4 / tcp6    （内置本地网络）
 │    ├── mynet1               （Flex 虚拟网络）
 │    └── mynet2               （Flex 虚拟网络）
 └── ServiceManager
      ├── portproxy-rdp        （端口转发服务）
      ├── portproxy-ssh        （端口转发服务）
      └── socks5-proxy         （SOCKS5 代理服务）
```

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
    "github.com/net-agent/remotework/agent"
    "github.com/net-agent/remotework/api"
)

hub := agent.NewHub(nil)
hub.MountConfig(config)

apiServer := api.New(hub, config.API, nil) // nil 使用默认 logger
apiServer.Start()  // 非阻塞
defer apiServer.Stop()

hub.Start() // 阻塞
```

`api.New()` 接收三个参数：
- `*agent.Hub` — Hub 实例（必须）
- `agent.APIInfo` — 配置信息
- `*slog.Logger` — 日志器，传 `nil` 使用默认

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

获取 Hub 运行状态。

**响应：**

```json
{
  "ErrCode": 0,
  "Data": {
    "running": true
  }
}
```

`running` 为 `true` 表示服务管理器正在运行，`false` 表示所有服务已退出。

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
      "dials": 5
    },
    {
      "name": "tcp",
      "protocol": "",
      "address": "",
      "domain": "",
      "state": "",
      "aliveMs": 0,
      "listens": 0,
      "dials": 0
    }
  ]
}
```

字段说明见 [NetworkStateDTO](#networkstatedto)。

注意：返回列表包含内置的 `tcp`/`tcp4`/`tcp6` 网络，它们的大部分字段为空值。上层应用通常可以过滤掉 `protocol` 为空的条目。

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

### POST /api/v1/stop

优雅停止 Agent。需要确认头防止误操作。

**必须的请求头：**

```
X-Confirm: yes
```

**响应：**

```json
{
  "ErrCode": 0,
  "Data": {
    "status": "stopping"
  }
}
```

调用后 Hub 会异步执行停止流程：关闭所有服务 → 断开所有网络 → 进程退出。

缺少 `X-Confirm: yes` 头时返回错误：

```json
{
  "ErrCode": -1,
  "ErrMsg": "missing header X-Confirm: yes"
}
```

---

## WebSocket 实时事件

### 连接

```
ws://127.0.0.1:8080/api/v1/ws
```

连接建立后，客户端可以发送订阅消息选择感兴趣的事件类型。如果 5 秒内未发送订阅消息，将自动订阅全部事件。

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

网络连接状态发生变化时触发。由 Hub 事件监听器实时推送，无延迟。

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

服务状态发生变化时触发。由 Hub 事件监听器实时推送。

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

Hub 停止运行时触发。由轮询器检测 `IsRunning()` 状态变化后推送。

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
# 检查运行状态
curl http://127.0.0.1:8080/api/v1/status

# 查看网络列表
curl http://127.0.0.1:8080/api/v1/networks

# 查看服务列表
curl http://127.0.0.1:8080/api/v1/services

# 查看数据流（限制 20 条，过滤指定网络）
curl "http://127.0.0.1:8080/api/v1/streams?limit=20&network=mynet"

# 全量 ping 测试
curl -X POST http://127.0.0.1:8080/api/v1/ping

# 单域名 ping
curl -X POST http://127.0.0.1:8080/api/v1/ping/mynet/remote-pc

# 调整日志级别
curl -X PUT http://127.0.0.1:8080/api/v1/loglevel \
  -H "Content-Type: application/json" \
  -d '{"level":"debug"}'

# 停止 Agent
curl -X POST http://127.0.0.1:8080/api/v1/stop \
  -H "X-Confirm: yes"
```

### JavaScript WebSocket 客户端

```javascript
const ws = new WebSocket('ws://127.0.0.1:8080/api/v1/ws');

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
| Hub 事件监听器 | `network.state`、`service.status` | 实时 | agent 包在状态变更时直接回调 |
| StatePoller 轮询 | `stream.update`、`hub.stopped` | ≤ pollInterval | 定期快照对比，检测差异 |

网络和服务状态变化频率低但重要性高，采用实时回调；数据流变化频繁且无内置钩子，采用轮询差异检测。

### 安全性

- 默认监听 `127.0.0.1`，仅本机可访问
- `/stop` 接口需要 `X-Confirm: yes` 头防止误操作
- WebSocket 的 `CheckOrigin` 默认允许所有来源（适合本地开发），生产环境如需暴露到外网应自行添加认证中间件
- API 不暴露密码等敏感配置信息

### 并发安全

- 所有 REST 接口都是并发安全的，Hub 内部使用 `sync.RWMutex` 和 `atomic` 操作
- WebSocket 广播使用快照复制模式，不会阻塞 Hub 的正常运行
- 事件监听器回调通过 `go` 异步执行，不会阻塞 agent 的状态变更流程

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
- Hub 未启动或已停止时查询服务/网络 → `"NO SERVICES"` / `"NO NETWORKS"`
- ping 不存在的网络 → `"network='xxx' not found"`
- 无效的日志级别 → `"invalid level, use: debug/info/warn/error"`
