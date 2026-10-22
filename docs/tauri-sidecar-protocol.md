# Tauri 宿主与 Go Sidecar 通信协议规范 (v3.0)

> 当前 sidecar 协议版本为 **3**。本文档同时记录 v2/v3 的增量与破坏性变更，权威事件契约见 `api/README.md`。

## 0. 规范术语

本文档使用以下规范级别术语：

- **MUST**：必须满足，否则视为不兼容实现。
- **MUST NOT**：绝对禁止。
- **SHOULD**：推荐满足；如不满足，必须有明确理由。
- **MAY**：可选实现。

---

## 1. 角色与职责

1. **Host（Tauri 运行时，Rust）**：进程生命周期编排（拉起/停止/重启/崩溃感知）、配置文件唯一写方、API token 颁发、sidecar stdout/stderr 的第一消费者。
2. **Sidecar（Go agent）**：数据面（隧道/网络）与控制面 API（REST + WebSocket）。sidecar **MUST NOT** 写任何配置文件，**MUST NOT** 自行重启。
3. **Frontend（Webview UI）**：控制面 API 的主要消费者；其凭证（端口与 token）**MUST** 经 Host 的 Tauri command 获取，**MUST NOT** 硬编码。

设计原则：

- Host 与 sidecar 经 `bundle.externalBin` 同版本发布，协议协商失败时 **MUST** 明确报错，**MUST NOT** 静默降级猜测。
- sidecar 脱离 Host 独立运行（standalone CLI）时协议仍 **MUST** 可用，鉴权行为见 §7。

---

## 2. 通道总览

| 通道 | 方向 | 承载内容 |
| --- | --- | --- |
| C1 进程参数与环境变量 | Host → Sidecar | 运行模式、配置路径、日志格式、API token |
| C2 配置文件 | Host → Sidecar | `active-config.json`，Host 是唯一写方 |
| C3 stdout 行协议 | Sidecar → Host | 控制消息（ready/fatal）与结构化日志 |
| C4 HTTP REST / WS | 双向 | 生命周期操作（Host）与控制面（Frontend） |
| C5 进程退出码 | Sidecar → Host | 终止原因分类 |

---

## 3. C1：进程参数与环境变量

Host 拉起 sidecar 时 **MUST** 使用以下形式：

```
remotework -mode agent -c <active-config.json 绝对路径> -log-format=json
```

环境变量：

| 变量 | 规范 | 说明 |
| --- | --- | --- |
| `REMOTEWORK_API_TOKEN` | Host 托管模式下 **MUST** 设置 | 32 字节随机值的 hex 编码，每次 spawn 重新生成 |

约束：

- token **MUST NOT** 写入配置文件或命令行参数（避免落盘与 `ps` 泄漏）。
- `-log-format` 取值 `text|json`，缺省 `text`（standalone 场景面向人类阅读）；Host 托管模式 **MUST** 使用 `json`。

---

## 4. C2：配置文件

1. Host **MUST** 将当前 profile 合并运行时注入项后写入 `<config_dir>/remotework/active-config.json`，sidecar **MUST** 以只读方式消费。
2. Host 注入字段白名单（其余字段原样透传）：
   - `api.enable`：**MUST** 为 `true`。
   - `api.listen`：**MUST** 为 `"127.0.0.1:0"`（端口由 sidecar 自选，见 §5）。
3. 运行时热挂载（`POST /links`、`POST /tunnels`）的产物 **MUST NOT** 由 sidecar 写回配置文件。持久化由 Frontend 双写完成（§8）。

---

## 5. C3：stdout 行协议与启动握手

### 5.1 行格式

sidecar 以 `-log-format=json` 运行时，stdout 每行 **MUST** 是一个独立 JSON 对象，分两类：

**控制消息**（含 `type` 字段）：

```json
{"type":"ready","protocol":3,"version":"0.2.0","commit":"abc1234","api":"127.0.0.1:54321","mode":"normal","pid":1234}
{"type":"ready","protocol":3,"version":"0.2.0","commit":"abc1234","api":"127.0.0.1:54321","mode":"degraded","configError":"...","pid":1234}
{"type":"fatal","stage":"config|bind","error":"..."}
```

**日志记录**（slog JSON，含 `level` 与 `msg` 字段，无 `type`）：

```json
{"time":"2026-10-13T10:00:00+08:00","level":"INFO","msg":"read config","module":"sys","path":"..."}
```

- sidecar **SHOULD NOT** 向 stderr 写入内容；panic 等运行库输出除外。Host **MUST** 将 stderr 行按 `level=error` 原文转发。
- Host 对 stdout 中无法解析为 JSON 的行，**MUST** 按原文日志转发（panic 堆栈可能跨多行）。

### 5.2 握手时序

```
Host                                     sidecar
 |-- spawn(C1 参数 + env) ---------------->|
 |                                         |-- 解析配置 / 加载插件 / 绑定 API 端口
 |<--------------- {"type":"ready",...} ---|
 |-- GET /api/v1/status(Bearer 复核,MAY) ->|
```

规则：

1. sidecar **MUST** 在 API 端口监听就绪后、对外宣告可用前，输出唯一一行 `ready` 消息；`api` 字段 **MUST** 是本机可连接的地址（配置 `:0` 时为系统分配端口；通配监听会收敛为 loopback 地址后再报告）。
2. 配置解析失败进入降级模式时，sidecar **MUST** 输出 `mode:"degraded"` 的 `ready` 并附带 `configError`；进程 **MUST NOT** 直接退出（保留 API 供修复配置）。例外：配置中 `api.enable=false` 导致无法降级时，按 §6.3 退出码 1 退出。
3. 启动阶段不可恢复的错误，sidecar **MUST** 输出 `fatal` 消息后以非零码退出。
4. Host **MUST** 以 5 秒超时等待 `ready`；超时、收到 `fatal`、或进程先行退出，均判定启动失败，并 **SHOULD** 将 stdout/stderr 尾部若干行呈给用户。
5. Host **MUST NOT** 自行猜测端口（废弃"预分配端口"做法）；等不到 `ready` 即视为二进制版本不匹配或启动失败。

---

## 6. C4/C5：生命周期操作

### 6.1 状态查询 `GET /api/v1/status`

响应在原有基础上扩展（纯新增字段，向后兼容）：

```json
{
  "running": true,
  "mode": "normal|degraded",
  "version": "0.2.0",
  "commit": "abc1234",
  "protocol": 3,
  "startedAtMs": 1708243200000,
  "configPath": "/abs/path/active-config.json",
  "configError": ""
}
```

- `mode` 为 `degraded` 时 `configError` **MUST** 非空。
- v3 起 `uptimeMs` 由 `startedAtMs`（Unix 毫秒）取代，uptime 由 Host 自行换算。
- Host 健康检查与 Frontend 关于页/降级引导均 **MUST** 使用本端点。

### 6.2 优雅停止 `POST /api/v1/stop`

1. sidecar 收到请求后 **MUST** 立即应答 `{"status":"stopping"}`，随后异步执行优雅停机。
2. 正常停机后进程 **MUST** 以退出码 0 终止。agent 的关停总预算为 35 秒（Drain + cleanup + API 关闭）；关停不完整时记录错误并以退出码 1 退出。关停过程中再次收到 SIGINT/SIGTERM 会强制以 130 退出。
3. Host **MUST** 等待进程 terminated 事件（建议超时 5 秒），超时后强制 kill；**MUST NOT** 使用固定时长 sleep 作为停止确认。
4. 原 `X-Confirm: yes` 头自本协议版本起废弃，由 token 鉴权替代（§7）。

### 6.3 退出码（C5）

| 退出码 | 含义 |
| --- | --- |
| 0 | 正常退出（`/stop` 请求或 OS 信号） |
| 1 | 启动/关停失败：配置解析失败且 `api.enable=false`、优雅关停不完整等 |
| 2 | API 端口绑定失败 |
| 130 | 关停过程中收到第二次 SIGINT/SIGTERM，强制退出 |
| 其他 | 未捕获异常（panic 等） |

Host **SHOULD** 依据退出码区分"正常停止"与"崩溃"，后者向用户提示并展示日志尾部。

### 6.4 重启与配置变更

- 重启 = Host 编排的 stop + start，sidecar 无 restart 概念。
- 本版本配置变更（含 profile 切换）**MUST** 经全进程重启生效；协议预留 `POST /api/v1/reload` 供未来热载实现，当前版本 **MUST NOT** 调用。

---

## 7. 安全

### 7.1 token 鉴权

1. Host 每次 spawn **MUST** 新生成随机 token，经 `REMOTEWORK_API_TOKEN` 传入。
2. token 存在时，sidecar **MUST** 校验全部 `/api/v1/*` 请求：
   - HTTP：`Authorization: Bearer <token>` 头。
   - WebSocket：`?token=<token>` 查询参数（webview 无法自定义 WS 请求头）。
3. token 缺失或错误 **MUST** 返回 401，WS **MUST** 拒绝握手。
4. standalone 运行（env 未设置 token）时，API 仅在回环地址上保持开放，sidecar **MUST** 在启动日志输出 warn 级提示；若监听地址不是回环，API **MUST** 拒绝启动，不能以无鉴权方式暴露到局域网/公网。

### 7.2 前端凭证获取

Frontend **MUST** 通过 Tauri command `get_api_endpoint() -> { port, token }` 获取连接信息；原 `get_agent_port` 由其实现替代。

### 7.3 来源校验

1. HTTP 请求 Host 头 **MUST** 匹配白名单：`127.0.0.1`、`localhost`、`[::1]`（防 DNS rebinding）。仅当 API 显式开启非回环监听（`api.allowNonLoopback=true`）且 token 非空时，才允许任意 Host，认证由 token 承担。
2. WS 握手 **MUST** 校验 Origin；CORS **MUST** 收敛为 allow-list：
   - `tauri://localhost`（macOS 生产）、`http://tauri.localhost`（Windows 生产）、`http://localhost:1420`（dev）。
3. 写请求（POST/PUT/DELETE 等）携带非白名单 Origin 时 **MUST** 直接返回 403；非浏览器客户端不带 Origin 时仍允许。
4. standalone 开放模式下上述校验仍 **MUST** 生效。

---

## 8. 热挂载持久化（Frontend 双写编排）

配置文件的唯一写方是 Host。Frontend 新增链路/隧道的完整流程：

1. 调 Host `save_profile` 落盘（先持久化）；
2. 调 sidecar `POST /api/v1/links` 或 `POST /api/v1/tunnels` 热挂载（免重启生效）；
3. 第 2 步失败时，Frontend **SHOULD** 回滚第 1 步（重新保存旧值）并向用户报告。

sidecar **MUST NOT** 为热挂载提供任何形式的持久化。

---

## 9. 版本协商

1. 本协议版本 `protocol: 3`，sidecar 在 `ready` 消息与 `/api/v1/status` 中均 **MUST** 携带。
   - v2 新增（additive）：WS 主题订阅/快照/序列号与流生命周期即时事件。
   - v3 变更（**breaking**，Host 必须配套升级）：DTO 移除时间派生字段——`aliveMs`（网络/流）与 `uptimeMs`（/status）删除，存活时长由 `startedAtMs`/`createdAtMs`/`closedAtMs` 换算；`stream.stats` 改为增量 upsert 语义（仅携带状态变化的流，`full=true` 锚点帧整体替换该网络活跃流集合，无变化不发帧）；`network.stats`/`service.stats` 无变化同样不发帧。stats 类锚点间隔 ≤30s。
2. sidecar 的 `version`/`commit` **MUST** 经 `-ldflags -X` 在构建期注入。
3. Host 对 `protocol` 高于自身支持上限的 sidecar **MUST** 报错提示升级，**MUST NOT** 尝试兼容未知协议。
4. Host 不再以 stderr 文本特征（如 `"invalid run-mode"`）探测二进制能力。
5. WS 事件契约（主题、快照、seq/resync、stats 增量与锚点、背压与心跳）以 `api/README.md` 的「WebSocket 实时事件」章节为准。

---

## 10. 决策记录（v1.0 定稿；协议版本已升至 3）

| # | 议题 | 结论 |
| --- | --- | --- |
| D1 | 端口分配 | sidecar 绑定 `127.0.0.1:0`，经 ready-line 回报实际地址；Host 不预分配 |
| D2 | 配置变更方式 | v1 全进程重启；预留 `POST /api/v1/reload` |
| D3 | 停止确认 | Host 等待进程 terminated 事件 + 超时强杀，废弃固定 sleep |
| D4 | 热挂载持久化 | Frontend 双写（先落盘后热挂载，失败回滚）；sidecar 不落盘 |
| D5 | 运行时日志上 WS | 暂缓，协议预留 |
| D6 | token 生成 | 每次 spawn 随机生成，env 传递 |
| D7 | standalone 无 token | 保持开放 + warn 日志 |
