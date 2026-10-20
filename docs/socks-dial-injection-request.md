# net-agent/socks 库改造需求：出站拨号注入能力（Dialer Injection）

- 提出方：remotework 项目组
- 面向库：`github.com/net-agent/socks`（当前使用版本 v1.0.7，go 1.15）
- 日期：2026-10-20
- 状态：暂缓提交 —— remotework 已按 §2.1 临时方案（`SetRequster`）落地出站拨号注入；本文保留备后续评审使用

## 1. 背景与目标

remotework 的 socks5 隧道服务（`vservice/socks5.go`）基于本库构建。remotework 控制面要求对**每一条连接**做到可观测：流记录（地址、方向、字节计数、开/关事件）、归属标注（连接属于哪个隧道服务）、隧道两端配对（pipeId 关联）。

目前 socks5 服务的 **listen 侧**连接已纳入观测（经 `ServiceState.TrackConn` 登记），但 **target 侧**（库内部 `DefaultRequester` 发起的出站连接）是观测盲区：

- 不经过 remotework 的 network plugin（`vnet.Network` 接口），无流记录、无字节计数、无开/关事件；
- 无法做服务归属标注与隧道两端配对；
- 出站固定 `net.Dial("tcp4", ...)`，不支持 tcp6，也无法复用 remotework 的连接追踪/加密/限速等能力。

**目标**：让库的使用方能够将出站拨号行为替换为自定义实现（经 remotework network plugin 发出），同时保持库自身的 SOCKS5 协议语义（命令分发、应答码、认证）不变。

## 2. 现状分析

### 2.1 库的现有扩展点（已核实 v1.0.7 源码）

库已具备完整的请求处理注入点：

```go
// serve.go
type Requester func(req Request, ctx Context) (net.Conn, error)
type Server interface {
    SetRequster(Requester)   // 注意：方法名拼写错误（Requster）
    ...
}

// requester.go
func DefaultRequester(req Request, ctx Context) (net.Conn, error) {
    if req.GetCommand() != ConnectCommand {
        return nil, ErrReplyCmdNotSupported
    }
    return net.Dial("tcp4", req.GetAddrPortStr())
}
```

`Request` 接口提供结构化目标地址（`GetAddress()` 返回 atype+buf，支持 IPv4/IPv6/Domain；`GetPort()`），错误到 RFC1928 应答码的映射已存在（`ErrReplyConnectionRefused` 等哨兵错误 → 对应应答码，未识别错误一律 `repFailure=0x01`）。

**因此 remotework 当前即可用如下临时方案满足基本诉求**（绕过 `net.Dial`，改走 network plugin）：

```go
server.SetRequster(func(req socks.Request, ctx socks.Context) (net.Conn, error) {
    if req.GetCommand() != socks.ConnectCommand {
        return nil, socks.ErrReplyCmdNotSupported
    }
    return trackedNet.Dial("tcp", req.GetAddrPortStr()) // vnet.Network 实现
})
```

即：**本项目未被阻塞**，本需求属于"能力正式化与补齐"，而非"解锁阻塞"。

### 2.2 现有扩展点的不足

1. **注入粒度过粗**：只想改变拨号行为，却必须整体替换 `Requester`，自行重新实现命令分发（CONNECT/BIND/UDP 判断），库未来升级命令语义时替换实现会悄悄过时。
2. **方法名拼写错误**：`SetRequster` 应为 `SetRequester`，已固化在公开接口上。
3. **出站网络硬编码**：`DefaultRequester` 固定 `tcp4`，tcp6-only 环境不可用。
4. **错误语义断裂**：自定义拨号器返回的标准网络错误（`*net.OpError`：connection refused / no route to host / timeout / DNS 失败）全部坍缩为 `repFailure(0x01)`，客户端无法区分失败原因；只有返回库的 `ErrReply*` 哨兵错误才能映射到精确应答码，但该契约无文档。
5. **无超时与取消**：`Requester` 签名无 `context.Context`，拨号不可设超时、不可随服务关闭取消（`server.Close()` 等待 `connWait`，但拨号中的 goroutine 不可中断）。
6. **UDP ASSOCIATE 未支持**（`UDPCommand` 直接拒绝）——非本需求范围，列为远期项。

## 3. 改造需求列表

按优先级排列，R1–R3 为 P0（正式化），R4–R5 为 P1（体验与健壮性），R6 为 P2（远期）。

### R1（P0）新增细粒度拨号注入点 `SetDialer`

```go
// Dialer 出站拨号抽象，与 net.Dialer / remotework vnet.Network 形状一致
type Dialer interface {
    Dial(network, addr string) (net.Conn, error)
}

// SetDialer 仅替换出站拨号行为，命令分发与应答语义仍由库负责。
// 与 SetRequester 并存；同时设置时 Requester 优先（或显式互斥报错，由库方定）。
SetDialer(d Dialer)
```

- 库内部 `DefaultRequester` 改为基于当前 Dialer 实现（默认 `&net.Dialer{}`），消除 `tcp4` 硬编码：Dialer 返回的 conn 决定实际网络（建议默认实现按地址族选择 tcp4/tcp6，或直接 `tcp`）。
- 函数形态 `SetDialerFunc(func(network, addr string) (net.Conn, error))` 可二选一或都提供，依库方风格定。

### R2（P0）修正方法名拼写

新增 `SetRequester`，保留 `SetRequster` 作为别名并标注 `Deprecated:`，不删除（向后兼容）。

### R3（P0）明确并文档化错误→应答码契约

- 文档写明：`Requester`/`Dialer` 返回 `ErrReply*` 哨兵错误时映射为对应 RFC1928 应答码。
- 建议 additionally 内置常见标准错误映射（可用 `errors.As`/`syscall` 判定）：
  `ECONNREFUSED → repConnectionRefused(0x05)`、`EHOSTUNREACH → repHostUnreachable(0x04)`、
  `ENETUNREACH → repNetworkUnreachable(0x03)`、超时 → `repTTLExpired(0x06)`、其余 → `repFailure(0x01)`。

### R4（P1）拨号超时与取消

二选一：

- `Requester`/`Dialer` 签名引入 `context.Context`（破坏性变更，需大版本）；或
- `Server` 增加配置项 `SetDialTimeout(d time.Duration)`，库在调用拨号前包裹 `net.Dialer{Timeout: ...}` 语义。

### R5（P1）出站连接元数据回调（可选，增值项）

```go
// OnTargetConnect 出站连接建立/失败时回调，便于使用方统一记日志与指标
SetOnTargetConnect(func(req Request, target net.Conn, err error))
```

remotework 可借此完成 target 侧的归属登记与配对（目前计划在 ConnLinker 包装层实现，若库提供回调则更干净）。

### R6（P2）UDP ASSOCIATE 支持

远期项，本需求不展开。

## 4. 兼容性要求

- 不删除/不改变现有导出符号语义（`SetRequster`、`Requester`、`Request`、`Context`、`ErrReply*`、`NewServer`/`NewPswdServer`），新增能力一律 additive；
- `DefaultRequester` 默认行为保持 CONNECT-only + 相同的应答码语义；
- Go 版本约束保持 ≤ 当前使用方要求（remotework 使用 go 1.2x 构建，但库自身 go directive 建议维持现状或仅小幅提升）；
- 发布后打 tag（建议 v1.1.0），便于使用方以语义化版本 pin 定，避免 `replace` 指令。

## 5. 验收标准

1. 使用方可仅通过 `SetDialer` 将出站连接切换到自定义网络栈，且：CONNECT 成功/失败路径的应答码与默认实现一致；非 CONNECT 命令仍返回 `repCmdNotSupported`。
2. 拨号器返回 `ErrReplyConnectionRefused` 时客户端收到 `0x05`；返回 `*net.OpError`(ECONNREFUSED) 时同样收到 `0x05`（若采纳 R3 的内置映射）。
3. `SetRequester` 与 `SetRequster` 均可设置请求处理器，行为一致；`go doc` 中后者标注废弃。
4. 库提供针对 R1/R3 的单元测试（httptest 级即可），CI 通过。
5. 语义化版本 tag 发布。

## 6. remotework 侧对接计划（供评估工作量参考）

库发布新版后，remotework 在 `vservice/socks5.go` 做如下对接（已在私有分支验证设计可行）：

1. `Socks5Controller` 注入目标网络能力（默认 `tcp` 插件，可配置）；
2. `SetDialer`（或临时方案 `SetRequster`）将出站拨号路由到该网络 → 出站连接自动获得流记录与字节计数；
3. target 侧 conn 经 `ServiceState.TrackConn` 登记 → 归属标注与 tunnel 两端 pipeId 配对对 socks5 全量生效；
4. 控制面 DTO 无需协议变更，纯数据完备度提升。

如评审结论是"维持 `SetRequster` 现状、不新增 `SetDialer`"，remotework 可按 §2.1 临时方案先行落地，但命令分发语义将由使用方各自实现，长期有分叉风险，仍建议采纳 R1/R2。
