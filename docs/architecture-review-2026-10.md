# Remotework 工程架构与代码质量评估

- **评估对象**：`github.com/net-agent/remotework`，HEAD `f5af01d`（工作区干净）
- **评估日期**：2026-10-22
- **代码规模**：112 个受版本控制文件；生产代码 8,706 行，测试代码 6,783 行（Go）
- **评估方式**：逐包静态阅读（kernel / cmd / configv2 / vnet / vservice / plugins / api / server / utils）、`go vet`、`go test -race`、覆盖率统计、Linux/Windows 交叉编译验证、针对性配置实验
- **说明**：本报告是 **2026-10-22 修复前** 的基线评估；修复状态、验证结果与剩余例外见文末「§8 修复记录」。
- **结论**：**综合 B（3.5/5）**。架构设计明显高于同规模 Go 工具的平均水平，核心抽象（微内核、能力注入、事务化生命周期）是真正的亮点；但**当前 HEAD 的跨平台编译是坏的**，WebSocket 慢客户端路径存在可复现的 `close of closed channel` panic，加上控制面密钥泄露、standalone 模式 CSRF、secret 引用未实现等问题，**尚不建议直接作为生产版本对外发布**。

## 评分总览

| 维度 | 评级 | 分数 | 一句话结论 |
|---|---|---:|---|
| 工程架构 | **B+** | 4.0 | 分层清晰、依赖方向干净；`replace ../flex`、无 CI、构建脚本陈旧拖后腿 |
| 代码架构 | **A-** | 4.3 | 微内核 + 控制面/数据面分离做得好；少量 god interface 与反射 hack |
| 模块设计 | **A-** | 4.2 | 模块边界与命名约定清晰；config 的若干字段是“解析了但没接线” |
| 执行效率 | **B** | 3.5 | 桌面规模足够；热路径无 buffer 池、API 逐流全量枚举、缺少连接上限 |
| 可维护性 | **B** | 3.6 | 文档与核心测试优秀；无 lint/CI、文档漂移、死代码、CRLF |
| 可扩展性 | **A-** | 4.3 | 插件机制是真扩展点；API Backend 偏宽、配置无版本字段 |
| 安全性 | **C** | 2.6 | 基础防护齐全，但存在密钥泄露、CSRF、弱认证加密与未硬化监听 |
| 可靠性/并发 | **C+** | 3.0 | 回调超时/panic 隔离好；WS 踢除可 panic、关停预算不足、accept 循环不重试、无拨号超时、卸载通道有泄漏 |
| 可测试性 | **B** | 3.5 | 核心包 67%–94%；cmd/plugins 0%，无 E2E，无 CI |
| 可观测性 | **B** | 3.6 | 结构化日志 + WS 主题/序列号设计好；无 metrics/trace，失败插件不可见 |
| 文档 | **B+** | 4.0 | `kernel/README`、`api/README` 质量高；与实现存在多处漂移 |

> 评级含义：A（优秀，可直接作为同类项目范本）/ B（良好，生产可用但需硬化）/ C（及格，存在明显短板）/ D（不合格）。

---

## 1. 项目定位与总体架构

### 1.1 定位

Remotework 是 Go 编写的流量中继工具：两台运行 agent 的机器通过中央 relay（server）建立虚拟网络，按 `listen -> target` 规则转发 TCP 流量（RDP/SSH/SOCKS5 等）。桌面端（Tauri）已拆分到独立仓库，本仓库保留 agent/server 内核、控制面 API 与 sidecar 进程契约。

### 1.2 运行形态

| 模式 | 入口 | 说明 |
|---|---|---|
| `agent` | `cmd/main.go` → `runAgent` | 微内核 + 插件：内置 TCP 网络、flex 虚拟网络、tunnel 服务、api 观察器 |
| `server` | `cmd/main.go` → `server.RunServer` | 独立实现：flex switcher + 单端口协议分流（TCP/WS），**不走 kernel** |
| `validate` | `cmd/main.go` → `validateConfig` | 配置校验后退出 |

### 1.3 依赖方向（实际代码）

```
cmd
 ├─ kernel.Host ──────────────► utils
 │    ├─ plugins/nettcp ─┐
 │    ├─ plugins/netflex ─┼──► vnet ───► flex, utils
 │    ├─ plugins/tunnel ──┴──► vservice ─► utils, flex/stream
 │    └─ plugins/api ────────► api ──────► vnet, vservice, configv2
 ├─ configv2 ─────────────────► utils
 └─ server ───────────────────► flex/switcher, utils
```

没有循环依赖；`vnet` 与 `vservice` 之间通过结构化接口（`ConnID()`/`GetState()`）解耦，`api` 通过 `Backend` 接口与运行时解耦，`plugins/api` 是二者唯一的桥。整体依赖方向是健康的。

### 1.4 核心数据流

1. **链路**：`links.alias -> tcp/ws/wss relay`，`netflex` 插件创建 flex Session，断线由 flex 内部退避重连；注册身份 `as` 与 `auth` 走 AES-GCM 握手。
2. **隧道**：`tunnel` 插件按 `listen` 的类型注入 `vnet.Network` 能力，先 `Listen`，再为 `target` 构造 `Dialer`，最后把 `*vservice.Service` 发布为能力。
3. **数据面**：`vservice` 的 Portproxy/Socks5 控制器为每条连接起 goroutine，经 `utils.RelayConns` 双向拷贝；`vnet.ReportingNetwork` + `connTracker`/`streamStateTracker` 负责字节数、流状态与生命周期事件。
4. **控制面**：`api` 插件把 `Catalog`/`EventBus` 适配成 `api.Backend`；REST 拉取快照，WS 推送 `topic + seq` 增量事件；`StatePoller` 兜底轮询。
5. **进程契约**：托管模式（`-log-format=json`）下 stdout 输出 `ready`/`fatal` 控制行与 JSON 日志，父进程看门狗在宿主消失时走优雅退出。

---

## 2. 做得好的地方

1. **微内核是“真”设计，不是包装**。`kernel` 有完整状态机（Added/Initializing/Pending/Ready/Starting/Running/Stopping/Failed/Unloaded）、依赖图、能力注册表、事务化 Init/Provide/cleanup、LIFO 回滚、panic 隔离、超时控制、依赖环检测。`kernel/README.md` 把契约写清楚了，这在小项目里非常少见。
2. **控制面与数据面分离干净**。`api.Backend` 只暴露查询/挂载/订阅/停止，`plugins/api` 实现它；`api` 包不反向依赖 plugins，测试可用 fake backend。
3. **配置容错的设计意图正确**。`configv2` 的 Raw → Normalize → Validate 管线清晰，`normalizeAll` 支持“坏项跳过、好项照跑”，降级模式保留 API 供桌面端修配置——这是产品化思维。
4. **安全基础件齐全**。默认 `127.0.0.1` 监听、Bearer token、Host 白名单（防 DNS rebinding）、CORS/WS Origin 白名单、URL 日志脱敏、relay 握手 AES-GCM、kernel 回调 panic 隔离。
5. **WS 协议设计成熟**。主题订阅、`snapshot-on-subscribe`、`seq` 单调、`resync`、可丢弃/可靠事件分流、慢客户端踢除、50ms 批量合并——不是“能推就行”的水平。
6. **核心包测试质量不错**。`configv2` 94%、`vservice` 92%、`kernel` 87%、`api` 70%，覆盖了依赖顺序、回滚、超时、鉴权、Host/CORS、WS v2 等关键路径；`go test -race ./...` 在当前平台通过。
7. **sidecar 契约考虑周全**。`ready` 回报实际端口、`fatal` 分阶段、退出码 0/1/2、父进程看门狗、token 经 env 注入、stdout 行协议——对桌面端集成很友好。

---

## 3. 分维度评估

### 3.1 工程架构：B+（4.0/5）

**优点**

- 单模块、单二进制、三种模式，边界清晰；`cmd` 只做装配，不掺业务。
- `plugins` 是薄适配层（nettcp 43 行、netflex 84 行），领域逻辑在 `vnet`/`vservice`，可独立测试。
- 命名与能力约定集中写在 `plugins/doc.go`、`kernel/README.md`，观察器可据此枚举实例。
- 发布产物通过 `-ldflags -X` 注入 version/commit，配合 sidecar `ready` 可追溯。

**问题**

1. **依赖不可复现**：`go.mod` 的 `replace github.com/net-agent/flex/v3 => ../flex` 要求同级 checkout，README/CLAUDE.md 虽已说明，但对 CI、Docker、新人是硬门槛；没有 vendoring，也没有版本 pin 的替代路径。
2. **交付链路缺失**：无 `.github/workflows`、无 lint 门禁、无 release 自动化；`build.sh` 手工按日期建目录；`DockerfileAgent` 已完全过时（见 P1）。
3. **两种生命周期架构并存**：agent 有 kernel 的优雅卸载，server 没有信号处理与 graceful shutdown；同一个仓库里两套标准，长期维护成本高。
4. **没有 `internal/`**：所有包都是公共 API，重构时缺少“可自由改”的边界；对工具类项目问题不大，但 `api`/`kernel` 一旦被桌面端或第三方引用，演化会受限。
5. **配置 schema 没有版本字段**：目录叫 `configv2`，但 `RawConfig` 无 `version`，也没有迁移机制；`docs/config-url-design.md` 描述的是 v2.3 规范，与实现已分叉。

### 3.2 代码架构：A-（4.3/5）

**优点**

- `kernel` 的 actor 调度 + 能力容器 + 事务化 attempt 是本项目最好的代码；`host.go`/`context.go`/`graph.go` 职责切分合理。
- `api` 包按关注点拆成 `server/handlers/websocket/poller/snapshot/meter/dto/clientproc`，可读性好。
- `vnet`/`vservice` 不互相 import，靠 `ConnID()`、`GetState()` 这类窄接口协作，避免了大包耦合。
- 错误大多带上下文；日志统一走 `slog` 与 `module` 标签。

**问题**

1. `api.Backend` 是 14 个方法的宽接口（`api/backend.go:14-44`），新增端点要同时改接口、实现、DTO、文档；测试 fake 也必须全实现。建议按读/写/订阅拆成 2–3 个接口。
2. `api` 直接依赖 `configv2`（`Backend.MountLink` 参数），控制面与配置模型耦合；更干净的做法是 API 层 DTO → 内部 spec 转换。
3. `vservice/connattr.go:150-170` 用反射读取 cipherconn 的未导出结构内嵌字段 `Conn`，属于“知道库实现细节”的 hack；库升级即可能静默失效。
4. `ServiceState` 把状态、计数器、连接登记表、锁混在一个结构里，且字段导出；`api` 直接读 `&svc.ServiceState`，边界偏松。
5. 错误处理风格不统一：`portproxy.go:40,46` 用 `%v` 丢链，部分错误全大写（`"NO SERVICES"`），`WriteJSON` 把所有业务错误压成 HTTP 200；全仓库 `errors.Is` 出现 0 次，调用方很难按类型分支。
6. **接口缺少 context**：`vnet.Network.Dial/Listen`、`vservice.ServiceController`、`kernel.Plugin.Init/Start` 都没有 `context.Context` 参数（`vnet/network.go:13-19`、`vservice/service.go:68-72`、`kernel/plugin.go:71-82`），取消/超时只能靠 `Done()` 通道与宿主约定；这直接导致拨号/握手超时无法沿调用链下传。
7. `vnet.NewTcpNetwork` 是导出构造函数却返回未导出类型 `*tcpNetwork`（`vnet/tcp.go:17`）；外部包无法在签名中引用该类型，只能靠接口断言，公共 API 设计不够干净。

### 3.3 模块设计速评

| 包 | 生产 LOC | 评级 | 评价 |
|---|---:|---|---|
| `kernel` | 1,681 | **A** | 状态机、依赖图、事务回滚、事件总线都成立；唯一硬伤是超时后 attempt 复用（P1） |
| `cmd` | 500 | **B** | 装配清晰；API 启动靠 20ms 轮询、`-home` 未使用、`managedMode` 与日志格式耦合 |
| `configv2` | 657 | **B+** | 管线设计好、测试充分；校验错误不落地、`via`/`keepalive` 未接线、无版本字段 |
| `vnet` | 787 | **B+** | Network 接口小而稳，Reporting/tracker 设计好；无拨号超时、反射解包、tcp 插件无 Stop |
| `vservice` | 592 | **B+** | 控制器与服务状态分离，连接归属/pipe 关联实用；失败状态不更新、Close 不清理活跃连接 |
| `plugins` | 886 | **B** | 薄适配正确；`authRef` 当密钥、`waitSettled` 把 Pending 当成功、0 测试 |
| `api` | 2,105 | **B** | Backend 边界、WS 协议、DTO 设计好；鉴权/超时/CSRF/密钥泄露需修 |
| `server` | 235 | **C+** | 能跑，但无 TLS、无超时、无 graceful stop、配置无校验，是明显的“另一套标准” |
| `utils` | 1,263 | **B-** | 日志/relay/脱敏实用；但职责过杂、死代码、平台编译 bug、CRLF |

### 3.4 执行效率：B（3.5/5）

**数据面（每条连接）**

- 2 个 goroutine（`RelayConns` 双向）+ 每方向 `io.Copy` 默认 32KB buffer，即每连接约 64KB 临时内存，没有 `sync.Pool`/`io.CopyBuffer`；高并发连接下 GC 压力明显。
- `utils/linkconn.go:114-118` 在 Debug 被禁用时仍会执行 `fmt.Sprintf("%T", a/b)`，每条连接多 2 次分配；`vservice/portproxy.go:99,105` 每条 pipe 两条 Info 日志，连接率高时日志 I/O 成为噪声与成本。
- `trackedConn.Read/Write` 每次 syscall 做两次 atomic，量级可接受；`connTracker` 的全局锁在 10k conn/s 级会有竞争，桌面场景无感。
- **没有拨号超时**：`vnet/tcp.go:24` 的 `net.Dial` 与 `flex` 会话拨号都依赖 OS 默认（可能 75s+）；目标侧黑洞时连接 goroutine 与 fd 会被长期占用，且没有每条隧道/每个服务的最大连接数。
- `SecretListener` 对每个 Accept 起一个握手 goroutine（`utils/secret_listener.go:33`），无并发上限；握手慢速攻击可放大 goroutine 数。

**控制面**

- `StatePoller` 快节拍每秒执行一次 `GetAllNetworkState` + `GetDataStreamState`；对每条活跃流调用 `GetStreamService`/`GetStreamPipe`，而这两个方法**每次都会 `catalog.List()`**（`plugins/api/api.go:187-220`），即每条流每秒 2 次调度器往返 + O(流×隧道) 的 `sync.Map` 查询。100 条流×20 条隧道时是可接受的；1,000 条流时是每秒数千次命令与数万次查询。
- `stream.stats` 的增量判断把 `ReadBps/WriteBps` 算进 DTO 比较（`api/poller.go:189-198,257-267`），有流量的流每秒都会“变化”，与文档“仅状态变化的流”不符，实际会持续推送全量活跃流。
- 关闭流历史每个网络固定 1024 条，`GetStreamStates` 每次全量复制后再由 poller 截断到 100（`vnet/stream_tracker.go:43-50`、`vnet/conn_tracker.go:259-272`），有可观的无效分配。
- `clientProcResolver.entries` 保留 10 分钟且无上限（`api/clientproc.go:21-24`），高 churn 场景内存随流速率线性增长；macOS 上 `netstat` 每秒最多一次、`/system/listening-ports` 每次请求都 exec 一次，均可被认证客户端放大。
- `WSHub.Broadcast` 在没有客户端时仍然 `json.Marshal`（`api/websocket.go:141`），可以零成本 early-return。

**结论**：桌面端（几十条流、几个隧道）完全够用；若定位为公网 relay 或高并发 agent，需要补 buffer 池、拨号超时、连接上限、批量查询。

### 3.5 可维护性：B（3.6/5）

**优点**

- 文档密度高：`kernel/README.md`（268 行）、`api/README.md`（1,249 行）、`docs/tauri-sidecar-protocol.md`、配置 URL 设计文档；注释中文、术语一致。
- 核心包测试结构清晰，fake 与表驱动测试多；`go vet` 干净。
- 文件规模控制得好，`api` 最大文件约 400 行；没有“巨石文件”。

**问题**

1. **当前 HEAD 跨平台编译失败**（P0）：`utils/conn_owner.go:25-34` 在共享文件里引用只在 `conn_owner_darwin.go` 定义的 `netstatTCP`/`parseNetstatTCP`；`GOOS=linux`/`windows go build` 直接报 `undefined`。这不是“潜在风险”，是已发生的事实（最新提交 `441c272` 引入），且没有 CI 拦截。
2. **无 lint/format 门禁**：`gofmt -l` 列出 7 个文件；其中 `utils/cooldown.go`、`linkconn.go`、`log.go`、`macaddr.go`、`tablewrite.go` 是 CRLF 行尾，`configv2/normalize_test.go`、`validate_test.go` 有对齐格式问题。`make fmt` 会产生大面积无意义 diff。
3. **文档漂移**：
   - `api/README.md:1208` 声称“API 不暴露密码等敏感配置信息”，但 `authcode` 确实通过 `ServiceState.ListenURL/TargetURL` 暴露（P0）。
   - `cmd/config_example/agent.json` 含未定义的 `comment`/`trace` 字段，且 README 示例缺少 v2.3 必需的 `id` 与 `authcode`。
   - `docs/config-url-design.md` 描述的 `authcodeRef` 字段与 YAML 示例，与实现的“query 参数 + JSON/TOML”不一致。
   - `docs/tauri-sidecar-protocol.md:79` 示例仍写 `protocol: 1`，正文要求 3。
4. **死代码/死依赖**：`utils/rdp_*.go`（`GetRDPPort` 无调用）、`utils/tablewrite.go`（`WriteTable` 无调用，却引入 `tablewriter` 依赖）、`utils.ParseRespJSON`、`cmd/flags.go` 的 `-home`、`ServiceState.ID` 等。
5. **`utils` 是杂物间**：日志、HTTP、netstat、进程归属、RDP、表格、MAC、pprof、relay 共 25 个文件；低内聚导致平台 bug 容易藏在这里（本次即如此）。
6. **重复代码**：`Normalize` 与 `normalizeAll` 两套几乎相同的循环；`loadPlugin` 与 `startAPIPlugin` 的启动/等待逻辑分散。

### 3.6 可扩展性：A-（4.3/5）

**做得好**

- 新增一种网络：实现 `vnet.Network` + 一个 ~40 行插件 + 在 `cmd` 注册；新增隧道协议：实现 `vservice.ServiceController` + 在 `tunnel.Init` 增加分支。
- 能力键 `(Go 类型, name)` + `Inject/Provide` 让插件依赖显式、可检查；`Catalog` 支持运行时装载/卸载。
- 观察器模式（`plugins/api`）可以复制出 metrics、tracing、审计等新观察器而不改数据面。

**限制**

- `Backend` 宽接口、`tunnel.Init` 的 `switch builtin`、`configv2` 的无版本 schema，都会让“新增一种能力”变成多点修改。
- API 只提供 `POST /links`、`POST /tunnels`，没有 DELETE/UPDATE；动态挂载是单向的，卸载只能重启进程。
- 插件是编译期注册，不是外部 `.so`/子进程插件；这是合理取舍，但“可扩展”不等于“可插拔第三方”。

### 3.7 安全性：C（2.6/5）

**已有防护（值得肯定）**

- API 默认 `127.0.0.1`；Host 白名单防 DNS rebinding；CORS/WS Origin 白名单；Bearer token 可空但会 warn；WS token 支持 query（webview 限制）。
- relay 注册握手用 AES-GCM + HMAC-SHA256（`flex/internal/admit`），密码不明文传输；flex switcher 自带连接数/每 IP 握手限流。
- 日志有 `RedactURL`；kernel 对回调 panic 隔离，避免单插件打崩进程。

**问题（按严重度）**

1. **控制面泄露传输密钥**（P0，High）：`plugins/tunnel/tunnel.go:219-227` 把 `authcode` 拼进 `ServiceState.ListenURL/TargetURL`，`api/dto.go:109-121` 原样返回；REST `/services`、WS `service.snapshot/stats` 都会把预共享密钥交给任何持有 API token 的客户端。`api/README.md:1208` 的“不过暴露敏感配置”被直接违反。
2. **standalone 模式 CSRF**（P0，High）：`api/server.go:141-153` 对未命中白名单的 Origin 仍继续处理请求；token 为空时 `authMiddleware` 直接放行。任意网页可用 `Content-Type: text/plain` 的简单请求 POST `http://127.0.0.1:8080/api/v1/stop`（Host 是 127.0.0.1，能过白名单），或挂载恶意 link/tunnel。sidecar 模式有 token 时被 CORS 预检挡住，但开放模式是文档支持的场景。
3. **secret 引用未实现却当密钥用**（P0，High）：`plugins/netflex/netflex.go:78-84` 的 `authRef` 直接返回引用名；`plugins/tunnel/tunnel.go:170-175` 的 `authcodeRef` 同样。设计文档说引用环境变量/密钥管理器；实际会用 `"CORP_RELAY_AUTH"` 这个字面量做 PSK——要么两端“碰巧”都用字面量而安全性归零，要么握手失败。正确行为应是解析失败即 fail closed。
4. **认证加密强度不足**（P1，Medium）：`cipherconn v1.0.2` 的隧道 `authcode` 使用 AES-CTR（`cipher.Stream`）+ MD5 校验 IV，**没有对密文做完整性认证**；主动 MITM 可比特翻转。README 只承诺“加解密”，未承诺认证，但对“不信任网络”的威胁模型仍不够。
5. **监听面未硬化**（P1）：`server` 配置无校验——`password` 为空会接受空密钥握手，`listen` 为空会在所有网卡上随机端口监听（`server/config.go`、`server/server.go:26,38`）；`server/mux.go:48-56` 在分流前 `Peek(1)` 没有读超时，慢速连接可在进入 flex 限流前无限占用 goroutine/fd。`api` HTTP server 同样没有 `ReadHeaderTimeout/ReadTimeout/WriteTimeout/IdleTimeout`（`api/server.go:94-97`）。
6. **pprof 无认证**：`utils/pprof.go:26-49` 直接 `ListenAndServe` 用户给的地址，`runAgent` 忽略启动错误；配置成 `0.0.0.0:6060` 即向网络暴露 heap/profile（可能含 token、authcode、配置内容）。
7. **token 细节**：`api/server.go:164` 用 `!=` 比较（非常量时间）；WS token 放 query string，可能进代理日志/浏览器历史；token 为空时完全开放且只 warn。
8. **信息暴露**：`clientproc` 把本机进程名/PID/启动时间通过 API 暴露给控制面；`GetMacAddressStr` 把所有网卡 MAC 拼起来上报 relay，既是指纹也是设备身份不稳定源；`ConfigError` 可能包含带 `authcode` 的畸形 URL。
9. **依赖陈旧**：`gorilla/websocket v1.4.2`（2020，存在弱 PRNG mask key 的安全公告 GHSA-w67g-5rqw-f597，未用 govulncheck 复核）、`BurntSushi/toml v0.4.1`（2020）、`gorilla/mux v1.8.1`；无依赖扫描/SBOM。
10. **物理 `tcp://` 端点上的 `authcode` 被静默忽略**（Medium）：`configv2/normalize.go:189-192` 对所有 scheme 提取 authcode，但 `plugins/tunnel/tunnel.go:182-186` 只在 `EndpointVNet` 分支使用；tcp 端点既没有加密，`internalURL` 又把 `?authcode=` 原样回显（`:227`）。用户会以为“写了 authcode 就加密了”。
11. **SOCKS5 默认可无认证**（Medium）：`vservice/socks5.go:41` 在 username/password 都为空时走 `socks` 库的 `NoAuth`；`configv2` 不要求凭据，示例又用 `socks5://`。一旦 listen 是 `tcp://0.0.0.0:1080`，就是开放代理/SSRF 入口。
12. **URL userinfo 泄露**（Medium）：`configv2/normalize.go:91-94` 只清 query/fragment，保留 `user:pass@`；`RelayURL` 进入 `NetworkReport.Address`，并经 `/networks` 返回（`api/dto.go:129`），`wss://user:pass@relay` 的密码会出现在 API/WS。
13. **relay 共享口令 + 域名可被抢占**（Medium，读码结论）：server 只配置一个 `Password`（`server/server.go:26`），flex registry 在目标域名失联时允许后来者替换（`../flex/switcher/registry.go:57-75`）。多租户/公网 relay 下，任一知道口令的 agent 都能抢占离线节点身份、劫持流量。
14. **无 TLS**（Medium）：relay 仅 `net.Listen` + `http.Serve`（`server/server.go:37-53`），API 也是明文 HTTP；`wss` 只能靠外部 TLS 代理。token/PSK 在链路上可被嗅探（relay 握手本身有 AES-GCM，但 API token 与 WS 事件没有传输层保护）。

### 3.8 可靠性与并发：C+（3.0/5）

**优点**

- kernel 对 Init/Start/cleanup 都有超时与 panic 边界；Drain 可选；卸载严格逆依赖序；`go test -race` 通过。
- flex 会话自带重连；`Cooldown` 提供 3s→1min 退避；WS 有心跳与写超时；事件总线非阻塞。

**问题**

1. **WS 慢客户端踢除可 panic 整个进程**（P0）：`api/websocket.go:169-179` 的 `kickLocked` 在只持有 `h.mu.RLock()` 的情况下做“检查再 close” `c.kicked`；多个广播 goroutine（poller、eventLoop、sendSnapshot）可同时进入 `default` 分支并二次 `close`，触发 `panic: close of closed channel`。panic 发生在没有 recover 的 poller/eventLoop goroutine 中，会直接终止 agent、断开全部隧道。`-race` 不会报告 close-of-closed-channel，现有测试只覆盖单广播者。
2. **kernel 的 attempt 复用是逻辑竞态**（P1）：`context.go:259-307` 的 `beginAttempt/rollbackAttempt` 共用一个 `Context`，`gen` 只用于丢弃过期的 `evtInitDone`；但超时后旧回调仍在运行，它可能在新 attempt 已开始后调用 `DeferCleanup/Provide`，把旧 attempt 的副作用写进新 attempt 的暂存区，或把新 attempt 的暂存区清空。`runWithTimeout` 文档承认“不保证回调 goroutine 退出”，但没隔离它对 Context 的写入。现有 `TestInitTimeout` 只验证状态推进，没覆盖“超时后重试”的组合。
3. **accept 循环不重试、状态不更新**（P1）：`vservice/portproxy.go:51-62` 对任何 `Accept` 错误直接 return；`plugins/tunnel/tunnel.go:126-139` 在启动前就把状态置 `Running`，循环退出只打一条 Error 日志，插件仍是 Running，API 仍显示 running。EMFILE 等瞬时错误会让隧道永久失效而 UI 无感。
4. **关停预算不收敛、退出码仍为 0**（P1）：`cmd/main.go:151-156` 只给 15s，而 kernel 最坏路径是 Drain 10s + 每条 cleanup 5s（tunnel 有 2 条）= 20s；`Drain` 只等待、不强制关闭残留连接，`PortproxyController.Close`/`Socks5Controller.Close` 也只关 listener。结果是活跃 RDP/SSH 会话被进程退出直接重置，`main` 仍返回 0，桌面端无法区分“正常停”与“超时停”。
5. **卸载通道泄漏**（P1/P2）：`kernel/host.go:344-347` 的 `cmdUnsubscribe` 只 `delete`，不 `close` 通道；`plugins/api` 的 `SubscribeServiceStatus` 用 `for range src`（`plugins/api/api.go:332-358`），`Server.Stop` 退订后该 goroutine 永远阻塞，`out` 不关闭，`eventLoop` 也退不出。进程随后退出，影响有限，但违背“优雅卸载”的承诺；动态卸载 API 插件时会真泄漏。
6. **无拨号超时/连接上限**：目标黑洞时连接 goroutine 长占；agent 侧没有 per-service/per-tunnel 的最大连接数；`SecretListener` 握手 goroutine 无上限。
7. **SOCKS5 握手无超时、Close 阻塞**（P1）：`vservice/socks5.go` 的活跃计数只在 linker 内累加，握手阶段不可见；`socks@v1.0.7` 的 `Serve` 不设 deadline，`Close()` 会 `connWait.Wait()` 等所有连接。半开连接/慢握手可无上限占用 goroutine+fd，关停时 Drain 看不到它，Close 超时后 goroutine 继续泄漏。
8. **Pending 永不超时**：提供者失败/被卸载后，等待该能力的插件会永久 Pending（`kernel/host.go:682-689` 只在能力出现时推进），没有超时或“依赖消失”事件；对配置错误表现为“静默不工作”。
9. **事件丢弃语义**：`Host.emit` 与 `EventBus.Publish` 在缓冲满时静默丢弃；API 的 `service.status` 若丢事件，`pollServices` 只遍历现存服务，不会补发“已移除”状态。

> 说明：`go test -race` 全绿只证明现有测试路径上没有数据竞争，不能覆盖 F1 这类“close of closed channel” panic，也不能覆盖 F2 这类加锁但语义错误的逻辑竞态。

### 3.9 可测试性：B（3.5/5）

`go test -cover`（当前 macOS 主机）：

| 包 | 覆盖率 | 包 | 覆盖率 |
|---|---:|---|---:|
| `configv2` | 94.2% | `kernel` | 87.1% |
| `vservice` | 92.3% | `api` | 70.0% |
| `vnet` | 67.6% | `utils` | 52.8% |
| `server` | 40.8% | `cmd` / `plugins/*` | **0%（无测试）** |

- 按生产 LOC 加权估算整体约 **62%**。
- 强项：kernel 的依赖顺序/回滚/超时、api 的鉴权/Host/CORS/WS、configv2 的 normalize/validate 都有针对性测试。
- 弱项：`cmd` 的启动/降级/握手/看门狗、`plugins` 的装配与真实生命周期、跨包集成（network → tunnel → api）完全没有自动化覆盖；没有 E2E（起真实 server+agent 跑一条 TCP 转发）。
- 并发路径：没有“多广播者 + 满缓冲 WS 客户端”“kernel 超时后重试”“关停中活跃连接”的回归测试；本次补充审计用 `go test -overlay`（探针文件在仓库外）复现了 WS 二次 close panic、Context 跨 attempt 污染、Catalog 退订不关 channel、socks 未 Start 时 Close panic 四类问题。
- Linux/Windows 测试当前**无法编译**，等于跨平台行为零验证。
- 平台相关测试（darwin netstat、Windows watchdog）无法在 CI 矩阵中跑，除非补 CI。

### 3.10 可观测性与文档：B+（4.0/5）

- 日志：`slog` + `module` 标签 + 文本/JSON 双格式；`kernel` 规定了“日志器必须来自 kernel”的约定；URL 脱敏。
- 指标：API 暴露网络/服务/流计数与字节数，WS 有增量与锚点；没有 Prometheus/OTLP/trace，也没有进程级 goroutine/fd/队列深度指标。
- 失败可见性：初始化失败的 link/tunnel 不会出现在 `/networks`、`/services`（它们没有发布能力），API 也没有暴露 kernel 的 plugin 状态；用户只能看日志。这是产品化的重要缺口。
- 文档：数量与质量都在平均线以上，但“规范文档 vs 实现”的漂移已经影响可信度（`authcodeRef`、`via`、`id`、sidecar protocol 示例）。

---

## 4. 问题清单（按优先级）

> 标记：**P0** = 发布前必须修；**P1** = 一迭代内修；**P2** = 有机会再修。每条给出证据、影响、建议。

### P0-0 WS 慢客户端踢除竞态可打崩整个 agent

- **证据**：`api/websocket.go:169-179` 的 `kickLocked` 用 `select` + `default` 做 check-then-close，调用方只持有 `h.mu.RLock()`（`api/websocket.go:149-163`、`220-230`）；`h.mu` 是读写锁，多个广播者可以同时进入。两个广播 goroutine 同时遇到同一慢客户端 `send` 满时，都会 close `c.kicked`，第二个 close 触发 `panic: close of closed channel`。
- **影响**：panic 发生在 `StatePoller`/`Server.eventLoop` 这类没有 recover 的 goroutine 中，会直接终止整个 agent 进程，所有隧道同时断开；触发条件是“慢 WS 客户端 + 两条可靠事件并发”，在桌面端 UI 卡顿/断网重连时并不罕见。现有测试只覆盖单广播者，`-race` 也不会报告这类 panic。
- **建议**：给 `WSClient.kicked` 配 `sync.Once`（或 `atomic.Bool` CAS）做幂等关闭；`kickLocked` 改成 `c.kickOnce.Do(func(){ close(c.kicked) })`；补一个多 goroutine 并发 `Broadcast` 到慢客户端的回归测试。

### P0-1 跨平台编译失败（Linux/Windows）

- **证据**：`utils/conn_owner.go:25-34` 在共享文件中调用 `netstatTCP()`/`parseNetstatTCP()`；二者只定义在 `utils/conn_owner_darwin.go:13-15,32`。实测：
  ```
  GOOS=linux   GOARCH=amd64 go build ./cmd  -> utils/conn_owner.go:27:15: undefined: netstatTCP
  GOOS=windows GOARCH=amd64 go build ./cmd  -> 同上
  ```
- **影响**：`make build-all` / `build.sh` 的 windows、linux 产物全部失败（脚本只打印“编译失败”并继续）；Linux CI 连 `go test` 都跑不起来；这是当前 HEAD 的发布阻断项。引入提交为 `441c272`。
- **建议**：把平台选择下沉到文件级：`conn_owner_darwin.go` 提供 `listTCPConnections`，新增 `conn_owner_other.go`（`//go:build !darwin`）提供 gopsutil 版本；共享文件只调用函数。加 GitHub Actions 矩阵（linux/windows/darwin）跑 `go build ./... && go test ./...`。

### P0-2 控制面泄露 `authcode` 预共享密钥

- **证据**：`plugins/tunnel/tunnel.go:219-227` 的 `internalURL` 把 `?authcode=...` 拼进 `ServiceState.ListenURL/TargetURL`；`vservice/service.go:135-136` 保存；`api/dto.go:109-121` 原样返回；`api/README.md:1208` 却声称不暴露敏感配置。
- **影响**：任何拿到 API token 的客户端（桌面 UI、日志采集、截图、浏览器 devtools）都能看到隧道两端 PSK；PSK 同时是数据面加密密钥，泄露即失去加密意义。WS 快照/统计也会反复推送。
- **建议**：`ServiceState` 增加“展示用 URL（脱敏）”与“密钥”分离；DTO 只返回 `utils.RedactURL` 后的 URL，或把 authcode 移到独立的 secret 字段且永不序列化；加单测断言 `/services` 与 `service.snapshot` 不含 `authcode`。

### P0-3 standalone 模式 CSRF（跨站可停止/篡改 agent）

- **证据**：`api/server.go:141-153` 对未命中 Origin 白名单的请求不拒绝，仅不加 CORS 头；`api/server.go:158-169` 在 `token == ""` 时放行所有 `/api/v1/*`；`POST /stop`、`POST /links`、`POST /tunnels` 都是状态变更接口。
- **影响**：在未实施/未覆盖 Private Network Access 预检的浏览器（以及本机任意进程）中，任意网页可发 `text/plain` 简单请求到 `http://127.0.0.1:8080/api/v1/stop`（Host 为 127.0.0.1，通过 Host 白名单），无需预检；也可挂载攻击者控制的 link/tunnel。Chrome 较新版本的 PNA 预检会挡住一部分场景，但 Firefox/Safari、旧版浏览器与本地脚本不受保护；sidecar 模式有 token 时被预检挡住，但开放模式是文档支持且默认 warn 后继续运行的。
- **建议**：对 `POST/PUT/DELETE` 强制校验 `Origin`（空 Origin 仅允许非浏览器客户端或要求自定义头）；要求 `Content-Type: application/json`；给 standalone 也生成临时 token 或改用 Unix socket；至少对 `/stop` 增加二次确认头。

### P0-4 `authRef` / `authcodeRef` 未解析却当密钥使用

- **证据**：`plugins/netflex/netflex.go:78-84` 注释“待 secret provider 落地”，实际 `return authRef`；`plugins/tunnel/tunnel.go:170-175` 用 `utils.FirstString(ep.Authcode, ep.AuthcodeRef)` 直接当密钥；`docs/config-url-design.md:45,152` 明确说引用环境变量/密钥管理器。
- **影响**：用户按文档写 `authRef=CORP_RELAY_AUTH`，实际 PSK 就是字符串 `CORP_RELAY_AUTH`；两端都这么写会“能用但零安全”，只一端写则握手失败；`authcodeRef` 同理会把引用名当加密密钥。属于“静默的安全降级”。
- **建议**：实现 `SecretResolver`（env/KMS/文件），解析失败返回明确错误并让插件 Failed；在配置规范中明确支持范围；补“引用不存在”“引用解析成功”两类测试；若短期不实现，应在 Validate 阶段直接拒绝 `authRef/authcodeRef`，不要静默当字面量。

### P1-1 配置校验错误不落地，坏项仍被加载

- **证据**：`configv2/parse.go:30-36` 只把 `Validate` 错误追加到 `errs`，不修改 `canonical`；`cmd/main.go:115-118` 打印“config item invalid, skipped”后继续加载。实测：缺 `id` 的 tunnel 与缺 `authcode` 的 vtcp tunnel 都留在 `canonical.Tunnels` 中。另有两类校验缺口：`Validate` 不校验 link scheme（`vnet/flex.go:118-147` 的 `buildConnector` 到运行时才拒绝）；tunnel name 没有唯一性校验，而 kernel 以 `tunnel/<name>` 为唯一键，重复 name 在 `validate` 中为 OK、运行时第二个插件被静默跳过。
- **影响**：缺 `authcode` 的 tunnel 在 `tunnel.Init` 里失败 → 插件 Failed → 不会发布能力 → `/services` 看不到它，用户只看到一条启动日志；缺 `id` 的 tunnel 照常运行但违反 v2.3 规范。日志说“skipped”但实际没 skip，误导排障。
- **建议**：按项校验、按项丢弃（或整份配置 fail-fast，由 `-strict` 控制）；`LoadAndCheck` 返回“已接受项 + 被拒项”结构；`cmd` 日志区分 `skipped` 与 `loaded-with-warning`；API 暴露被拒项。

### P1-2 kernel 超时回调污染后续 attempt

- **证据**：`kernel/context.go:259-307` 单一 Context 复用暂存区；`kernel/host.go:718-750` 只给事件带 `gen`；超时后旧 goroutine 仍可调用 `DeferCleanup/Provide`。
- **影响**：插件 Init 超时→Failed→Retry 后，旧 Init 可能把 cleanup/provide 写进新 attempt，或清空新 attempt 的暂存区，造成能力误发布、cleanup 丢失/重复、状态不可预测。属于低频但难排查的逻辑竞态。
- **建议**：每次 attempt 使用独立 `Context`（或 attempt token，所有写方法校验 token）；超时后旧 Context 直接 `closed`，其 `DeferCleanup/Provide` 返回错误；补“超时后 Retry”测试并 `-race`。

### P1-3 `server` 模式配置无校验、无超时、无 graceful stop

- **证据**：`server/config.go` 只做反序列化；`server/server.go:26` 把 `Password` 直接交给 switcher；`server/server.go:38` `net.Listen("tcp", s.config.Server.Listen)`；`server/mux.go:48-56` 无读超时；`RunServer` 不处理信号；bind 失败时 `RunServer` 只 `syslog.Error` 后返回，进程退出码仍是 0。
- **影响**：漏写 `password` 即空密钥 relay；漏写 `listen` 即在所有网卡随机端口监听；未分类连接可在 mux 层慢速耗尽 goroutine/fd；`WsEnable=false` 时 `HTTPListener` 无人消费，64 个 HTTP 连接后 `Offer` 永久阻塞；relay 本身不做 TLS（`wss` 只能靠外部代理），SIGTERM 直接杀进程；bind 失败被服务管理器误判为成功。
- **建议**：启动时校验 `listen` 非空、`password` 非空（或显式 `allowEmptyPassword`）、`wsPath` 合法；mux 对 `Peek` 设 5s deadline，`WsEnable=false` 时直接关闭 HTTP 通道；HTTP server 设 `ReadHeaderTimeout/IdleTimeout`；`RunServer` 接 `signal.NotifyContext` 做 `srv.Close()`，bind 失败返回非 0。

### P1-4 API HTTP 面缺少超时与请求体限制

- **证据**：`api/server.go:94-97` 只设置 `Addr`/`Handler`；`utils/http.go:10-17` `io.ReadAll` 无 `MaxBytesReader`。
- **影响**：本机恶意进程（或 token 泄露后）可慢速发 header/body 占用 goroutine；大 body 造成内存峰值。
- **建议**：`ReadHeaderTimeout: 5s`、`ReadTimeout: 15s`、`WriteTimeout: 30s`、`IdleTimeout: 60s`；`ReadJSON` 用 `http.MaxBytesReader(w, r.Body, 1<<20)`（需要把 `w` 传进去或改为 `LimitReader`）。

### P1-5 隧道 accept 循环退出后状态仍为 Running

- **证据**：`vservice/portproxy.go:51-62`、`vservice/socks5.go:95-100` 对任何错误 return；`plugins/tunnel/tunnel.go:126-139` 启动前 `SetStatus(StatusRunning)`，退出只打日志。
- **影响**：EMFILE/临时网络错误会让 listener 永久失效，但 UI/API 仍显示 running，用户以为服务正常。
- **建议**：区分临时错误（`net.Error.Temporary()`/退避重试）与永久错误；永久退出时 `SetStatus(StatusFailed)` 并记录 `LastErr`；把 plugin 状态通过 API 暴露，或让 kernel 提供失败通知。

### P1-6 无拨号超时、无连接上限

- **证据**：`vnet/tcp.go:23-29` `net.Dial`；`plugins/tunnel/tunnel.go:93-102` 直接调用，target 侧 `cipherconn.New` 的 IV 交换没有 deadline；`vservice/portproxy.go:60` 每 Accept 一个 goroutine；无 semaphore/limit。
- **影响**：目标黑洞时 goroutine/fd 长占；对端 accept 后不发 IV 会让 serve goroutine 永久卡住；公网 listener 可被连接洪泛打满。
- **建议**：`net.DialTimeout`（如 10s，可配置）；每服务/每隧道 `MaxConns` 信号量；超限快速拒绝并计数；对 `SecretListener` 握手加并发上限。

### P1-7 卸载时 Catalog 订阅通道不关闭，导致 goroutine 泄漏

- **证据**：`kernel/host.go:344-347` 只 delete 不 close；`plugins/api/api.go:332-358` 用 `for range src`；`api/server.go:233-241` 退订后不等待 eventLoop。
- **影响**：`Server.Stop` 后 service 订阅 adapter 永久阻塞、`eventLoop` 无法退出；进程退出掩盖了问题，动态卸载 API 插件时会真泄漏。
- **建议**：`onCmdUnsubscribe` 在调度器内 `delete` 后 `close(ch)`（并保证不双关）；`Host.Subscribe` 文档明确通道会被关闭；`Server.Stop` 增加 `eventLoop` 的 done 等待。

### P1-8 动态挂载 API 缺少校验，坏请求返回成功

- **证据**：`api/handlers.go:259-291` 只检查 `len(canonical.Tunnels)>0`，不校验 `id/name/listen/target`；`plugins/api/api.go:266-286` 把 `Pending` 当成功。
- **影响**：`POST /tunnels` 传 `{}` 会创建名为 `tunnel/` 的插件，因缺网络能力永久 Pending，但 HTTP 返回 `{"status":"ok"}`；同名第二次请求才报错。UI 会认为挂载成功。
- **建议**：动态路径复用 `Validate`（单条）；拒绝空 name/listen/target；`waitSettled` 对 Pending 设置短超时并返回“依赖未就绪”而非成功；提供 DELETE 以便清理。

### P1-9 交付链路：Dockerfile、build.sh、CI、LICENSE

- **证据**：`DockerfileAgent:4` 用 `golang:1.18`（go.mod 要求 1.24）；`DockerfileAgent:11` `cd /app/agent_bin`（目录已不存在）；`COPY . /app` 不含 `../flex`，`replace` 必然失败；无 `USER`，root 运行；无 `.dockerignore`（当前构建上下文约 245MB，`dist/` 占 232MB）；`:12` 没有 ldflags。`build.sh:7` 的 `CGO_ENABLED=0` 未 export，实际 native 构建仍可能启用 CGO；`Makefile:14` 用 `sh build.sh`，而脚本用了 bash 数组，Ubuntu 的 `/bin/sh`（dash）会语法错误；脚本没有 `set -e`，单个目标失败后继续，最终仍 `exit 0`（用 fake go 模拟 5 个目标全失败，脚本仍输出“全部编译任务完成！”）。`build.bat:18,22` 仍是旧名 `abaddon_*` 且不注入版本。
- **影响**：Docker 镜像无法构建；`make build-all` 在 Linux 上可能直接失败或被误判成功；发布产物非静态/不可复现、sidecar `version` 可能为 `dev`；无 CI 导致 P0-1 这类问题直接进主干。
- **建议**：多阶段 Docker（`golang:1.24` → `gcr.io/distroless/static`）、`CGO_ENABLED=0`、非 root；`build.sh` 改 `#!/usr/bin/env bash` 并由 `bash build.sh` 调用，或改 POSIX；加 CI（build/test/vet/gofmt + 交叉编译）；补 LICENSE（当前仓库无 LICENSE 文件）。

### P1-10 传输加密缺少完整性保护；cipherconn 不支持半关闭

- **证据**：`cipherconn v1.0.2` 使用 AES-CTR（`cipher.Stream`）+ MD5(IV||password) 校验，密文无 MAC；`cipherconn` 类型未实现 `CloseWrite`，`utils/linkconn.go:88-99` 会走“不支持半关闭 → 全关闭”分支。
- **影响**：主动 MITM 可篡改加密流量；半关闭协议（客户端发完请求 shutdown write 等响应）在加密隧道上可能被提前全关，导致响应截断。
- **建议**：升级/替换为 AEAD（AES-GCM/ChaCha20-Poly1305）并做密钥确认；给 `cipherconn` 补 `CloseWrite/CloseRead` 或在其外层实现；若短期不改，README 明确“仅加密、不防篡改，半关闭不保证”。

### P1-11 轮询/归属解析的规模化成本

- **证据**：`plugins/api/api.go:187-220` 每流 `catalog.List()`；`api/poller.go:186-198,257-267` 每 tick 逐流调用并把 Bps 纳入 diff；`api/clientproc.go:21-24` 10 分钟保留无上限；`vnet/stream_tracker.go:43-50` 每次复制全部历史。
- **影响**：流数/隧道数增长时 CPU、分配、调度器往返线性/乘积增长；高 churn 时 resolver 内存无界。桌面规模可接受，公网/大客户会暴露。
- **建议**：一次快照内缓存 `catalog.List()` 与 service 列表；按 `limit` 向下传递，避免复制 1024 条；把速率与状态分开做 diff；resolver 加 LRU/上限；`/system/listening-ports` 加短 TTL 缓存。

### P1-12 安全细节：token 比较、WS query token、pprof

- **证据**：`api/server.go:164` 字符串 `!=`；`api/websocket.go:298` `?token=`；`utils/pprof.go:26-49` 无鉴权且启动错误被忽略。
- **影响**：本机时序侧信道（低）；token 进代理/历史日志（中）；pprof 暴露堆/配置（高，若绑定非 loopback）。
- **建议**：`subtle.ConstantTimeCompare`；WS 用一次性 ticket 或 `Sec-WebSocket-Protocol`；pprof 强制 loopback 或加 token，并把 `Start` 改为同步返回 bind 错误。

### P1-13 关停预算不足，活跃会话被重置且退出码为 0

- **证据**：`cmd/main.go:151-156` 给 `host.Shutdown` 15s；`kernel/host.go:16-20` 默认 Drain 10s + 单条 cleanup 5s，tunnel 插件有 listener 与 controller 两条 cleanup，最坏 20s；`plugins/tunnel/tunnel.go:142-155` 的 Drain 只轮询等待，`vservice/portproxy.go:64-69` 与 `socks5.go:102-109` 的 Close 只关 listener；`main` 对 `Shutdown` 错误只 `Warn`，最终仍以 0 退出。
- **影响**：活跃 RDP/SSH 连接在进程退出时被 OS 直接重置（用户看到会话中断），桌面端无法区分“正常停止”与“超时/不完整停止”；`Shutdown` 返回 `context.DeadlineExceeded` 时也未触发非零退出。
- **建议**：把全局关停预算下发到 kernel（按插件剩余时间计算 Drain/Cleanup），Drain 到期后强制关闭该服务的活跃连接（vservice 需要活跃连接表）；`main` 在 `Shutdown` 失败时返回非零；支持第二次 SIGINT 立即退出。

### P1-14 SOCKS5 握手无超时，Close 等待所有连接

- **证据**：`vservice/socks5.go:46-55` 只在 `SetConnLinker` 回调里 `AddActiveCount`，握手阶段不计入；`socks@v1.0.7/serve.go` 的 `serve` 对 `checker.Start` 没有 deadline，`Close()` 先关 listener 再 `connWait.Wait()`，而 `connWait.Done()` 在 `serve` 返回后才执行（panic 时甚至不执行）。
- **影响**：半开 TCP 连到 SOCKS 端口或慢握手可无上限占用 goroutine/fd；关停时 Drain 看不到这些连接，`Close` 阻塞到 kernel 5s 超时，之后 goroutine 继续泄漏。
- **建议**：accept 后立即计数并设置 3–5s 握手 deadline；cleanup 不调用阻塞式 `Server.Close`，改为关 listener + 主动关闭自己维护的活跃连接；给 `connWait.Done` 加 `defer` 的库补丁或用 fork。

### P1-15 物理 `tcp://` 端点的 `authcode` 被静默忽略却回显

- **证据**：`configv2/normalize.go:189-192` 不区分 scheme 地提取 `authcode`；`plugins/tunnel/tunnel.go:182-186` 只对 `EndpointVNet` 使用，`EndpointNet` 直接丢弃；`internalURL` 对非 vnet 端点返回 `ep.RawURL`（`:227`），于是 `/services` 会显示 `tcp://...?authcode=SECRET`，但流量并未加密。
- **影响**：用户以为在物理端点上配置了加密，实际明文传输且密钥仍经 P0-2 泄露；属于“安全假象”，比不配置更危险。
- **建议**：Validate 拒绝非 `vtcp://` 端点的 `authcode`/`authcodeRef`，或实现物理端点加密；无论如何在 DTO 中脱敏。

### P1-16 SOCKS5 默认可无认证，公网监听即开放代理

- **证据**：`vservice/socks5.go:41` `socks.NewPswdServer(username, password)`；`socks@v1.0.7/serve.go:47-56` 在两者都为空时使用 `NoAuth`；`configv2` 不要求凭据；`cmd/config_example/agent.json:21-22` 使用 `socks5://`。
- **影响**：若 tunnel 的 listen 是 `tcp://0.0.0.0:1080`（或任何可达地址），任何客户端都能无认证使用该代理访问内网，形成 SSRF/开放代理。
- **建议**：非回环 listen 时强制要求 username/password（Validate 检查并报错）；示例显式加凭据或改成 `127.0.0.1` 监听；文档标注“无认证 socks5 仅限虚拟网络/回环”。

### P2（择要）

- `utils.WriteJSON` 不设 `Content-Type`，所有业务错误走 HTTP 200（`utils/http.go:19-42`）；与 REST 习惯不符，客户端易误判。
- `api` 默认监听 `:0` 时 `Addr()` 返回 `[::]:port`，`ready` 行与桌面端连接语义不明确；`0.0.0.0` 监听会被 Host 白名单 403，配置层应校验/警告。
- `configv2` 的 `via`（`model.go:57`、`normalize.go:113-120`）与 `keepalive`（`model.go:56`）只解析/校验，**从未传给 flex Session**；用户按文档配置 `via` 上游路由/心跳间隔不会生效。属于“静默 no-op”，应实现或删除字段并在 Validate 报错。
- JSON 注释只支持整行 `//`（`utils/utils.go:23-24`），行尾注释（如 `"a": 1 // x`）会解析失败；未知字段静默忽略（`comment`/`trace` 示例即如此）。建议用 jsonc 解析器或 `DisallowUnknownFields` + 位置化错误。
- `gofmt` 7 个文件不通过（5 个 CRLF）；`go vet` 干净但无 staticcheck/golangci-lint。
- `utils/linkconn.go` 的 `fmt.Sprintf("%T")` 在 Debug 关闭时也执行；`RelayConns` 无 `io.CopyBuffer`/pool。
- `vservice` 每条 pipe 两条 Info 日志（`portproxy.go:99,105`）；高连接率下日志量过大。
- `Server.Stop` 固定 `Sleep(100ms)`（`api/server.go:231`）；`startAPIPlugin` 20ms 轮询（`cmd/main.go:96`）；`DockerfileAgent` 的 `go mod tidy` 会改 go.mod。
- `utils.FileExist` 在权限错误时 `log.Fatal`（`utils/utils.go:55`），库函数不应退出进程；`ResolveConfigFile` 用 `path` 而非 `filepath`，Windows 路径 fallback 可能错。
- 无 LICENSE；`.vscode/launch.json` 被跟踪，`.claude/settings.local.json` 在本地存在但未跟踪；`docs` 示例版本号与协议版本漂移。
- 文档具体漂移：`api/README.md:99,102,328,767` 仍写 `?secret=`（实现是 `authcode`），`:969` 写已删除的 `rdpserver` 类型；`docs/tauri-sidecar-protocol.md:79-80` 示例 `protocol:1` 但正文要求 3；`CLAUDE.md:44` 的 `LinkReadWriteCloser` 不存在（实际是 `utils.RelayConns`）；`README.md:29-38` 的典型配置直接跑 `validate` 会报 `id`/`authcode` 缺失。
- `cmd/flags.go:11,18` 的 `-home` 解析后未使用；`utils/rdp_*.go`、`tablewrite.go`、`ParseRespJSON` 为死代码；`tablewriter` 可移除。
- `vnet` 的 `nettcp` 无 cleanup/Stop；`PortproxyController.Close` 不关闭活跃连接；`ServiceState.LastErr` 永远为空、`ServiceState.ID` 永远为 0（`api/dto.go:110-120` 暴露空值）。
- `wsHub.Broadcast` 无客户端时仍 Marshal；`sendSnapshot` 可被认证客户端反复 `resync` 放大（无速率限制）。
- `GetMacAddressStr` 汇总所有 MAC（含虚拟网卡）作为设备标识，隐私与稳定性都欠佳。
- 初始化期 logger 快照问题：`server` 包级 `syslog` 在 `utils.SetFormat` 前创建，`FanoutHandler.WithAttrs` 复制 handler（`utils/handlers.go:233-242`），所以 `-log-format=json` 时 server 日志仍走 stderr 文本；`cmd` 已规避，但这是隐式陷阱。
- `vservice/socks5.go:102-109`：若 `Start` 未执行（Init 失败后卸载），`socks.Server.Close()` 会解引用 nil `listener` 触发 panic（被 kernel `safely` recover，但 cleanup 报错）；`Close` 应先判空或记录是否已启动。
- `api/poller.go:87-89` 与 `api/server.go:228-251` 的 `Stop` 非幂等：第二次调用会 `close` 已关闭的 channel；建议 `sync.Once`。
- `api/poller.go:319-348` 把 `actives/dones` 计数纳入 `svcSnapshot` 差分，每完成一条连接就发一次**可靠** `service.status`（old==new），高连接率下可靠事件洪泛，还可能踢掉慢客户端；计数应走 stats 通道。
- `api/poller.go:203-214` 用 `ds.Actives[i]` 与 `currentDTOs` 下标耦合；`Actives` 含 nil 时会在 `streamKeyFromState` 解引用 panic（`meter.sample` 已防御 nil，这里没有），下标耦合本身也让后续改动容易错配。
- `utils/pprof.go:26-63` 的 `p.server` 字段无锁读写（Start/Stop/IsRunning 并发），且 `Start` 忽略 bind 错误。
- `server/mux.go:91-97,108-115` 的 `Offer` 与 `Close` 存在竞态：`done` 关闭后仍可能把 conn 投进无人接收的 `ch`，造成 fd 泄漏；`connChan.Close` 的 check-then-close 也非原子。
- `utils/handlers.go:214-230` 每条日志复制整个 handler map 并对每个 handler `r.Clone()`；日志热路径可优化。
- `utils/cooldown.go` 的 `Wait()` 用 `time.After`、不可取消、字段无锁，且生产代码零调用（死代码）；CLAUDE.md 所述 3s→1min 退避实际在 flex Session 内部。
- URL userinfo 泄露：`configv2/normalize.go:91-94` 保留 `wss://user:pass@relay` 的 userinfo，`RelayURL` 进入 `NetworkReport.Address` 并由 `/networks` 返回（`api/dto.go:129`）；`RedactURL` 只覆盖日志，不覆盖 DTO。
- 凭证明文落盘且无权限校验：`configv2` 的 `auth`/`authcode`、`server` 的 `password` 都是明文；`utils.LoadJSONFile`/`server.NewConfig` 不检查文件是否为 0600，示例配置还带真实感的口令。
- 日志注入：`ConsoleHandler` 原样拼接字符串（`utils/handlers.go:145-163`），恶意 tunnel name 或错误文本带换行即可伪造日志行；结构化日志应做引号/转义。
- 配置路径回退：`utils.ResolveConfigFile` 在指定文件不存在时回退同目录/cwd 的 `config.json`/`config.toml`（`utils/utils.go:69-83`），可能加载非预期/可被其他用户写入的配置。
- `plugins/api/api.go:18-19,248-262` 直接 import 并 `new` 具体插件（`netflex`/`tunnel`），`api/README.md:27` 的“零业务逻辑/只依赖 Backend”不成立；`api/handlers.go:220-291` 还自己做 `Normalize`。应改为工厂/能力注册。
- 测试稳定性：`api/websocket_v2_test.go:274`、`server/mux_test.go:93` 等 9 处 `time.Sleep`/轮询等待，存在 flake 风险；`api/server_test.go:53` 仍用 `Protocol: 1` 作为 fixture，未对齐协议 3；无 benchmark/fuzz。
- 依赖告警：`gorilla/websocket v1.4.2` 的 GHSA-w67g-5rqw-f597（弱 PRNG mask key，CVSS 6.9，修复 1.5.3）来自检索、未本地 govulncheck 复核；升级前应按公告确认影响。

---

## 5. 改进路线图

### 立即（0–3 天，发布阻断）

0. 修 P0-0 WS 踢除竞态：`WSClient.kicked` 用 `sync.Once`/CAS，补多广播者回归测试。
1. 修 P0-1 跨平台编译：平台文件拆分 + 本地跑 `GOOS=linux/windows go build ./...`。
2. 修 P0-2 authcode 泄露：DTO 脱敏 + 单测断言。
3. 修 P0-3 CSRF：Origin 强校验 + JSON Content-Type；standalone 也要求 token。
4. 修 P0-4 secret 引用：解析 env 或 fail closed；未实现前拒绝 `authRef/authcodeRef`。
5. 加最小 CI：`go build ./...`（三平台）、`go test -race ./...`、`go vet`、`gofmt -l`。

### 一迭代内（1–2 周）

1. 配置校验落地（P1-1）：坏项丢弃/`-strict`、API 暴露被拒项。
2. kernel attempt 隔离（P1-2）+ “超时后重试”测试。
3. `server` 配置校验、mux/HTTP 超时、graceful stop（P1-3/P1-4）。
4. accept 循环重试与失败状态（P1-5）、拨号超时/连接上限（P1-6）。
5. 修 Catalog 退订通道（P1-7）、动态挂载校验（P1-8）。
6. Dockerfile/build.sh/CI/ LICENSE（P1-9）。
7. pprof 与 token 细节（P1-12）。
8. 关停预算/强杀（P1-13）、SOCKS5 握手超时（P1-14）、物理端点 authcode 拒绝（P1-15）、SOCKS5 凭据校验（P1-16）。

### 一个月内

1. cipherconn AEAD / 半关闭（P1-10），或明确威胁模型与文档免责。
2. 轮询与 resolver 的规模化改造（P1-11）：快照内缓存、limit 下传、LRU。
3. 依赖升级（websocket 1.5.x、toml 1.x）、依赖扫描、`-trimpath`/可复现构建。
4. 文档与实现对齐：`docs/config-url-design.md`、README 示例、sidecar protocol、API 安全声明。
5. 增加 E2E：起 server + 两个 agent，验证 portproxy/socks5/加密隧道/停止流程。
6. 死代码/依赖清理、gofmt/CRLF 归一、staticcheck 门禁。

### 一个季度

1. 配置 schema 版本化与迁移；`via`/`keepalive` 要么实现要么移除。
2. 失败插件/被拒配置的可视化（API `plugins` 端点或 `/status` 扩展）。
3. metrics/tracing（连接数、字节、重连、goroutine、队列深度）；结构化错误码。
4. 插件契约测试框架（kernel 提供 test harness）；fuzz（配置解析、WS 订阅、URL 解析）。
5. 供应链：SBOM、签名发布、依赖更新策略。

---

## 6. 附录：验证记录

```text
$ go vet ./...
（无输出，退出码 0）

$ go test -race -count=1 ./...
ok  github.com/net-agent/remotework/api        4.874s
ok  github.com/net-agent/remotework/configv2   2.619s
ok  github.com/net-agent/remotework/kernel     3.418s
ok  github.com/net-agent/remotework/server     3.599s
ok  github.com/net-agent/remotework/utils      1.721s
ok  github.com/net-agent/remotework/vnet       3.669s
ok  github.com/net-agent/remotework/vservice   4.122s
（cmd/plugins/* 无测试）

$ go build -o /tmp/rw-remote ./cmd
（成功，约 11 MB）

$ CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./cmd
# github.com/net-agent/remotework/utils
utils/conn_owner.go:27:15: undefined: netstatTCP
utils/conn_owner.go:31:10: undefined: parseNetstatTCP
（Windows 同样失败）

$ gofmt -l .
configv2/normalize_test.go
configv2/validate_test.go
utils/cooldown.go
utils/linkconn.go
utils/log.go
utils/macaddr.go
utils/tablewrite.go
```

> 说明：跨平台编译失败是评估过程中实际复现的，不是推测；`go test -race` 只在 darwin/arm64 上通过。并发专项的 `go test -overlay` 探针（文件在 `/tmp`，未写入仓库）额外复现了：WS 多广播者二次 close panic、kernel 超时 attempt 的 cleanup 串入下一次 attempt、Catalog 退订 200ms 后 channel 仍未关闭、socks 未 Start 时 `Close` 的 nil pointer panic。

---

## 7. 一句话总结

**这是一个“架构品味明显高于平均、工程化尚未跟上”的项目**：kernel 的状态机与事务化插件模型、控制面/数据面分离、sidecar 契约和文档质量，说明作者对长期演进有清晰设计；但当前 HEAD 的跨平台编译断裂、WS 踢除竞态 panic、控制面密钥泄露、standalone CSRF、secret 引用未实现、无 CI/交付链路，说明它还没有跨过“可发布”的门槛。先修 P0（尤其是跨平台编译与 WS panic）和构建/CI，再谈性能与扩展，投入产出比最高。


---

## 8. 修复记录（2026-10-22）

本轮按评估清单完成除 cipherconn 相关之外的修复。以下为落地情况与验证结果。

### 8.1 已完成

| 项 | 状态 | 关键实现 |
|---|---|---|
| P0-0 WS 踢除竞态 | 已修 | `WSClient.kickOnce` 幂等关闭；64 广播者并发回归测试 |
| P0-1 跨平台编译 | 已修 | `conn_owner` 按 `darwin`/`!darwin` 拆分；linux/windows/darwin × amd64/arm64 全部通过 |
| P0-2 authcode 泄露 | 已修 | `toServiceDTO`/`toNetworkDTO`/ConfigError 统一 `RedactURL`；WS 各路径断言无密钥 |
| P0-3 standalone CSRF | 已修 | 非白名单 Origin 的写请求 403；无 Origin 的 CLI 兼容；token 常量时间 |
| P0-4 secret 引用 | 已修 | `utils.ResolveSecret`（`env:NAME`/裸名）fail closed；authcodeRef/authRef 不再当字面量 |
| P1-1 配置校验不落地 | 已修 | `ValidationError.Scope/Item/ItemIndex` + 逐项 prune；未知字段/尾随数据严格拒绝 |
| P1-2 kernel attempt 污染 | 已修 | `attemptRunning` 门禁 + `sealAttempt` + `ErrAttemptRunning`；超时回调结束后才允许 Retry |
| P1-3 server 硬化 | 已修 | listen/password/wsPath 校验；5s peek；HTTP 超时；优雅关闭；bind/关停超时非零 |
| P1-4 API HTTP 超时/body | 已修 | 5/15/30/60s + 1MB body limit |
| P1-5 accept 循环失效 | 已修 | 可恢复错误指数退避；永久退出 `StatusFailed` + `LastErr` |
| P1-6 拨号超时/连接上限 | 已修 | tcp/flex 10s；单服务 4096 连接上限；relay mux 4096 pending 上限 |
| P1-7 退订通道 | 已修 | `cmdUnsubscribe` 关闭 channel；Shutdown 不双关 |
| P1-8 动态挂载假成功 | 已修 | 四字段校验；`waitSettled` 仅 Running 成功；Pending 超时报错 |
| P1-9 交付链路 | 部分 | 多阶段非 root Docker、`build.sh` bash/`set -e`/CGO export、CI 三平台矩阵、gofmt 阻塞；LICENSE 未添加（需 owner 决策） |
| P1-11 轮询规模 | 已修 | `StreamAnnotator` 一次 `catalog.List()`；service.status 去计数；基线清理；resolver 上限/锁外 netstat；端口缓存 |
| P1-12 token/pprof | 已修 | 常量时间；WS 接受 Bearer；pprof 强制 loopback、同步 bind、超时、幂等 |
| P1-13 关停预算 | 已修 | agent 35s；Drain 超时强关；Shutdown 聚合错误；不完整退 1；二次信号 130 |
| P1-14 SOCKS5 握手 | 已修 | 5s deadline；accept 计数；Close 关 listener+连接且不阻塞 |
| P1-15 物理 authcode | 已修 | Validate 拒绝非 vtcp；动态路径兜底；DTO 脱敏 |
| P1-16 SOCKS5 无认证 | 已修 | 非回环物理监听强制凭据 |
| P2（多数） | 已修 | StartAPIPlugin/pprof/gofmt/CRLF/死代码（tablewriter/RDP/Cooldown）/JSONC/路径/日志转义/`:0` 地址等 |

### 8.2 剩余例外（有意保留）

1. **cipherconn 相关**：AES-CTR 无完整性认证、缺少半关闭、无 AEAD；按用户要求本轮不修，文档已明确限制。
2. **relay 的 TLS 终止**：仍建议由反向代理提供 `wss`，不在本仓库内做单端口 TLS。
3. **WS 一次性 ticket**：保留 `?token=` 兼容，同时支持 `Authorization: Bearer`；未改桌面端协议。
4. **HTTP 业务错误仍返回 200 + ErrCode**：保持既有 API 信封契约，未改为 4xx/5xx。
5. **配置 schema 版本/迁移**：未知字段已严格拒绝，但没有 `schemaVersion` 字段与迁移器。
6. **LICENSE**：仓库仍无 LICENSE，需 owner 决定授权方式。
7. **兼容层死代码**：`utils.NamedLogger` 等旧兼容符号保留，避免破坏外部引用。

### 8.3 验证

```text
$ gofmt -l .                 # 无输出
$ go vet ./...               # exit 0
$ go test -race -count=1 ./...  # 全部包 ok（api/configv2/kernel/plugins/.../vservice）
$ GOOS=linux/windows/darwin GOARCH=amd64 go build ./...  # 全部 exit 0
$ GOOS=... GOARCH=arm64 go build ./...                   # 全部 exit 0
$ bash build.sh              # exit 0，生成 5 个平台产物
```

手工冒烟：
- sidecar `-log-format json` 输出 `protocol:3` ready 行；无 token 401、有 token 200；`POST /stop` 正常退出。
- 非白名单 Origin 的 `POST /stop` 返回 403，白名单 Origin 放行。
- `api.listen=0.0.0.0:0` + token + `allowNonLoopback=true` 启动后，ready 行回报 `127.0.0.1:<port>` 且可连接。
- `server` 空 listen 配置退出码 1。

> 本轮未执行 `docker build`（本机 Docker daemon 不可用）与 Windows 实机 smoke；Dockerfile/CI 已静态校验，建议在 CI 首次运行时确认。
