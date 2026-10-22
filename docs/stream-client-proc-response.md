# Sidecar 需求响应：流 DTO 携带客户端进程信息

> 对应需求文档：`SIDECAR_REQUEST_STREAM_CLIENT_PROC.md`（流 DTO 携带客户端进程信息）。
> 结论：**需求合理，已实现**。字段为 additive 可选字段（`omitempty`），`protocol` 保持 2 不变。
> 字段契约已同步至 `api/README.md`「StreamStateDTO」章节，本文档说明达成情况、实现语义与桌面端适配要点。

## 1. 达成情况总览

| 需求项 | 状态 | 说明 |
|--------|------|------|
| `clientPid` / `clientProcName` 字段 | ✅ 已达 | 按需求命名与类型（`int32` / `string`），`omitempty` |
| 仅 inbound + loopback 解析 | ✅ 已达 | outbound、非 loopback 远端、非 `ip:port` 地址（如 flex 虚拟地址）一律跳过 |
| 解析失败降级留空 | ✅ 已达 | 权限不足 / 进程已退出 / 端口不可查均字段省略，不视为错误 |
| 性能约束（热路径不无缓存全量枚举） | ✅ 已达 | 采用需求建议的方案 1（TTL 缓存映射表），见 §2.2 |
| 开流时解析一次、后续透传 | ✅ 已达 | 按流记忆，REST `/streams`、`stream.snapshot`、`stream.opened` / `stream.closed`、`stream.stats`、`stream.update` 全部同构携带 |
| `procStartMs`（非必需，欢迎携带） | ✅ 已达（顺带实现） | 字段名 `clientProcStartMs`，见 §3.1 |

## 2. 实现语义

### 2.1 解析规则（与需求一致）

- 仅当 `direction=inbound` 且 `remoteAddr` 的 IP 为 loopback（`127.0.0.0/8` 或 `::1`，含 IPv4-in-IPv6 映射地址归一化）时尝试解析。
- 反查方式：以 `remoteAddr` 的 `(ip, port)` 匹配本机 TCP 连接表中**本机侧地址**（Laddr）相同的行，取该行 PID——客户端连接的临时端口登记的属主即客户端进程；不会误匹配 agent 自身持有该连接的服务端行（其 Laddr 为监听端口，键不同）。
- 进程名 / 进程启动时刻复用 `utils/listening_ports.go` 同款 gopsutil v4 机制（`process.NewProcess(pid)` → `Name()` / `CreateTime()`），跨 macOS / Linux / Windows。
- PID≤0（权限不足等）视为合法「未知」终态：字段留空且不再重试。

### 2.2 性能设计（方案 1：TTL 缓存 + 每流记忆）

两级缓存，热路径零系统调用：

1. **端点表 TTL 缓存（1s）**：`本机端点→PID` 映射全量枚举后缓存 1 秒；仅当存在未解析的流且缓存过期时才重建。无任何待解析流时**完全不产生**枚举调用。数据源按平台分派：macOS 解析 `netstat -anv -p tcp`（gopsutil 在 darwin 上 shell 到 lsof，非 root 只能看到本用户进程，存在权限盲区；netstat 无此限制，root/其他用户进程的 socket 均携带属主，且 TIME_WAIT 残留在窗口期内仍保留 PID），其余平台维持 gopsutil `Connections("tcp")`。
2. **每流解析记忆**：单条流的解析结果（含「留空」终态）在首次解析后记忆，后续所有快照 / 事件 / REST 直接透传，不再查表。缓存键为 `(network, index, createdNs, 四元组)`，端口复用或会话重连不会命中旧流结果。

降级与兜底：

- 枚举失败沿用旧表，且 TTL 照常推进（系统调用失败期间不会每条流触发一次枚举）。
- 首次查表可能因缓存表早于连接建立而 miss：进入最长 **10s** 的重试窗口，由后续 `stream.stats` tick（默认 1s）顺带重试，命中即终态；窗口届满放弃，字段留空。重试由解析器按待解条目驱动（`retryPending`），**已关闭的短连接在窗口期内同样补查**（macOS 的 TIME_WAIT 残留保留属主），避免「开流 miss + 秒关」永久缺失；补解析结果经 REST `/streams` 历史透传。典型情况下 `stream.opened`（≤50ms 批处理窗口）即已携带进程信息；小概率首帧缺失时，1–2s 内由 `stream.stats` 补齐。
- 流关闭后解析记忆保留 10 分钟（覆盖 flex `stream.closed` 约 2s 滞后与 REST 历史查询），闲置超时自动淘汰；活跃流每次被引用刷新，不会被误淘汰。

### 2.3 生效范围

以下通道的 `StreamStateDTO` 同构携带新字段（含已关闭流——其解析结果来自记忆，连接已断开不影响展示）：

- REST `GET /api/v1/streams`
- WS `stream.snapshot` / `stream.opened` / `stream.closed` / `stream.stats` / `stream.update(opened|closed)`

## 3. 偏差与说明

### 3.1 顺带携带 `clientProcStartMs`（需求标注「非必需，欢迎一并携带」）

```go
ClientProcStartMs int64 `json:"clientProcStartMs,omitempty"` // 客户端进程启动时刻（Unix 毫秒）
```

取自 gopsutil `CreateTime`，与 `ListeningPortDTO.procStartMs` 同源同语义，供桌面端甄别 PID 复用。成本为每条已解析流一次系统调用（已含在每流记忆内，无周期开销）。若桌面端不需要可忽略该字段。

### 3.2 UDP / 非 TCP 传输不解析

反查基于 TCP 连接表（darwin 为 `netstat -anv -p tcp`，其余平台为 gopsutil `Connections("tcp")`），与现有「系统监听」能力范围一致。非 TCP 传输承载的 inbound loopback 流字段留空（按降级处理）。如需 UDP 支持请另提需求。

### 3.3 字段仅在 agent 所在机器语义正确

解析在 sidecar 本机进行：loopback `remoteAddr` 表示对端就在本机，查得的是**本机**进程。桌面端场景（sidecar 与客户端同机）完全覆盖；若未来 sidecar 部署在与客户端不同的机器上，非 loopback 的 `remoteAddr` 按需求规则本就不解析。

## 4. 桌面端适配指南

### 4.1 types.ts 镜像

```typescript
interface StreamStateDTO {
  // ...既有字段不变...
  clientPid?: number;         // 客户端进程 PID（inbound + loopback 解析成功时携带）
  clientProcName?: string;    // 客户端进程名
  clientProcStartMs?: number; // 客户端进程启动时刻（Unix 毫秒）
}
```

### 4.2 展示建议

- 「客户端」列：`clientProcName` 存在时内联展示（如 `Chrome·127.0.0.1:64443`），PID 放 tooltip；字段缺席时维持现状（仅 `ip:port`）。
- 字段在流存续期内不变（开流时确定一次），无需处理中途变更；首帧缺席、后续 `stream.stats` 补齐属于预期内小概率事件（§2.2），按字段到达自然刷新即可。

### 4.3 版本 pin

当前实现位于分支 `cc-improve`（基线 commit `c47664a4`）。字段合并到 main 后，以**合并 commit 的 40 位 SHA** 作为 `SIDECAR_VERSION` pin 值即可，合并后由 sidecar 侧提供该 ref。

## 5. 验证情况

- `go test ./...` 全绿（含 `-race`）；新增单测覆盖：loopback 解析命中与记忆、outbound/非 loopback/虚拟地址跳过、miss 重试与 TTL 节流、重试窗口放弃、枚举失败降级、PID=0 终态、闲置淘汰、并发安全。
- macOS 实测冒烟：真实 loopback TCP 连接的临时端口反查到本进程 PID 与进程名，机制端到端成立。

## 6. 交付物索引

- 字段定义：`api/dto.go`（`StreamStateDTO` 新增 `clientPid` / `clientProcName` / `clientProcStartMs`）
- 解析器：`api/clientproc.go`（TTL 端点表 + 每流记忆 + 重试窗口 + 闲置淘汰）
- OS 层反查：`utils/conn_owner.go`（`ListTCPLocalEndpoints` / `GetProcessName` / `GetProcessStartMs`，平台分派）；darwin 端点表实现：`utils/conn_owner_darwin.go`（解析 `netstat -anv -p tcp`），其余平台沿用 gopsutil `Connections("tcp")`（同 `listening_ports.go` 机制）
- 接线：`api/server.go`（解析器共享）、`api/snapshot.go`（快照与流事件批处理）、`api/poller.go`（`stream.stats` / `stream.update` 与周期淘汰）、`api/handlers.go`（REST `/streams`）
- 契约文档：`api/README.md`「StreamStateDTO」章节
- 测试：`api/clientproc_test.go`、`api/clientproc_retry_test.go`（closed 流窗口期补查）、`utils/conn_owner_test.go`、`utils/conn_owner_darwin_test.go`（netstat 输出解析）

## 7. 修订记录

- **2026-10-21**（依据桌面端排查反馈修复两处 sidecar 缺陷）：
  1. **darwin 端点表换源**：gopsutil 在 macOS 上 `Connections("tcp")` 经由 lsof 实现，非 root 运行时看不到 root / 其他用户进程的 socket（典型受害场景：客户端是以 root 运行的系统代理核心），导致 `clientPid` / `clientProcName` 永远缺席。darwin 改为解析 `netstat -anv -p tcp` 输出（无权限盲区，root 进程 socket 携带属主，TIME_WAIT 残留在窗口期内保留 PID）；进程名 / 启动时间仍走 proc_pidinfo（跨用户可用）。其他平台行为不变。已知残留盲区：`utils/listening_ports.go` 的 LISTEN 枚举在 darwin 仍走 lsof 路径，对 root 监听进程有同样限制（本次未改）。
  2. **closed 流窗口期补查**：此前重试由 `stream.stats` tick 顺带驱动，短连接「开流首查 miss → 秒关」后不再出现于 stats 通道，重试永不发生。现每次 stats tick 由解析器主动遍历仍在重试窗口内的未解条目（含已关闭流）补查（`retryPending`），命中或窗口届满才终止；补解析结果经 REST `/streams` 历史透传。
