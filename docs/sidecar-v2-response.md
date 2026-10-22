# Sidecar 需求响应文档（协议 v2，已被 v3 取代）

> **⚠️ 历史文档（superseded）：已被 sidecar 协议 v3 取代，不再更新。**
> 本文中的 `protocol=2`、`aliveMs` 保留、每 tick 全量 stats 等描述已经过时。当前协议为 **3**：DTO 移除了 `aliveMs`/`uptimeMs` 等时间派生字段，`stream.stats` 改为增量 upsert（`full=true` 锚点帧），`network.stats`/`service.stats` 无变化不发帧。权威定义见 [tauri-sidecar-protocol.md](tauri-sidecar-protocol.md) 与 [api/README.md](../api/README.md)。

> 对应需求文档：`SIDECAR_REQUESTS.md`（R1–R4）与 `SIDECAR_SUBSCRIPTION.md`（订阅推送改造）。
> 本版 sidecar `protocol` 升至 **2**。所有协议变更为 additive，REST 接口全部保留。
> 事件契约的权威定义已同步更新到 `api/README.md`「WebSocket 实时事件」章节；本文档不重复契约细节，只说明达成情况、偏差与桌面端适配要点。

## 1. 达成情况总览

| 需求 | 状态 | 说明 |
|------|------|------|
| R1 流时间戳 | ✅ 已达 | `createdAtMs` / `closedAtMs` 已加入 `StreamStateDTO` |
| R2 监听时长 | ✅ 已达（近似语义） | `procStartMs`（进程启动时刻）+ `address`，去重键改 `(address, port, pid)` |
| R3 父进程检测 | ✅ 已达 | 托管模式看门狗，Unix/Windows 双实现，实测 kill -9 宿主后 2s 内优雅退出 |
| R4 流生命周期即时推送 | ✅ 已达（flex 侧 closed 有已知滞后） | tcp 由连接追踪器回调、flex 由 `Trace` 钩子，经同一事件管道推送；`stream.opened` 即时，`stream.closed` tcp 即时 / flex 滞后约 2s（见 §3.1） |
| 订阅协议（§2/§3） | ✅ 已达 | snapshot-on-subscribe、resync、seq/topic 信封、心跳、背压全部落地 |
| 分域事件（§4） | ⚠️ 大部分已达 | `hub.started` / `hub.mode` 未实现（冗余，见 §3.2） |

## 2. 已达预期项说明

### 2.1 R1 — 流的发起/关闭时间戳

`StreamStateDTO` 新增字段，REST 与全部 WS 流事件（`stream.snapshot` / `stream.opened` / `stream.closed` / `stream.stats` / `stream.update`）同构生效：

```go
CreatedAtMs int64 `json:"createdAtMs"`          // 流发起时刻（Unix 毫秒）
ClosedAtMs  int64 `json:"closedAtMs,omitempty"` // 仅已关闭流携带
```

`aliveMs` 保留不变，桌面端排序/展示可直接改用 `createdAtMs`。

### 2.2 R2 — 监听端口进程启动时间与地址

`GET /system/listening-ports` 的 DTO 新增：

```jsonc
{
  "port": 3389, "protocol": "tcp",
  "address": "0.0.0.0",            // 新增：监听地址
  "pid": 1234, "processName": "svchost",
  "procStartMs": 1708243200000     // 新增：进程启动时刻（Unix 毫秒）
}
```

- 如文档预判，socket 创建时间无系统数据源，`procStartMs` 是**进程启动时间**（gopsutil `CreateTime`），请按近似语义展示（列名/提示注明"进程运行时长"）。
- 去重键已改为 `(address, port, pid)`：IPv4/IPv6 双栈与 SO_REUSEPORT 多进程监听不再合并，「系统监听」列表可直接展示地址列。
- **范围说明**：仍仅采集 TCP 监听（`Connections("tcp")`），UDP 未纳入；如需 UDP 另提需求。

### 2.3 R3 — 父进程存活检测（孤儿进程防护）

- 实现：托管模式（`-log-format=json`）下启动看门狗，每 2s 检测一次；判定宿主死亡后走与 `POST /api/v1/stop` 完全相同的优雅停机路径（退出码 0）。
- Unix：比对 `getppid()`，被 init/launchd 收养即判定死亡。Windows：`OpenProcess(SYNCHRONIZE)` + `WaitForSingleObject` 监测父进程句柄。
- **不会误杀 standalone**：text 日志模式（用户从终端直接运行）不启用看门狗。
- 实测（macOS）：`kill -9` 宿主进程后，sidecar 2s 内完成优雅退出（8 个插件全部卸载后退出码 0）。
- 桌面端无需任何配合改动。

### 2.4 R4 / §4.4 — 流生命周期即时事件

- `stream.opened` / `stream.closed` 由事件驱动（tcp 侧为连接追踪器回调，flex 侧为 `Trace` 钩子，经同一 EventBus 管道汇入；不再依赖 poller 差分），按约 **50ms 窗口合并批量**发送（一条事件携带数组，满足"同 tick 合并"与洪流场景要求）：

```jsonc
{"type":"stream.opened","topic":"stream","seq":33,"timestamp":...,
 "data":{"network":"net","streams":[{StreamStateDTO...}]}}
```

- 实测延迟：opened 从流建立到 WS 送达 **~10ms** 量级（满足验收 #2 的 <100ms，含批处理窗口）。
- 事件载荷为完整 `StreamStateDTO`（含 `index`、`createdAtMs`；closed 另含 `closedAtMs`、`closeReason`、最终字节计数）。
- poller 差分版 `stream.update` 保留作为过渡，桌面端切换后可忽略它。
- 已关闭流历史不进推送通道，排障仍走 REST `GET /streams`（与 §4.4 一致）。

### 2.5 订阅协议机制（§2/§3/§5）

全部落地：

- **snapshot-on-subscribe**：`subscribe` 携带 `topics` 后立即回发每个主题的 `*.snapshot`（hub/network/service/stream 四个主题）。重发 subscribe 时仅对新增主题补发快照。
- **resync**：`{"action":"resync","topics":[...]}` 重发指定主题快照（无论是否已订阅）。
- **信封**：`{type, timestamp, topic, seq, data}`，`seq` 主题内单调递增。
- **分层推送**：生命周期/状态类即时；`network.stats` / `service.stats` / `stream.stats` 每 tick 合并推送。
- **背压**：发送缓冲区 256 条；stats 类溢出丢旧补新，可靠类溢出**踢掉客户端**（见 §3.3）。
- **心跳**：服务端 30s ping / 60s pong 超时 / 10s 写超时（此前完全没有心跳与读超时，僵尸连接无法发现，本次一并修复）。
- **断线恢复**：重连后重新 subscribe 即重建全部状态，无需 REST 介入（验收 #3）。

## 3. 偏差与未达预期项

### 3.1 `stream.closed` 有约 2 秒滞后（已获豁免）

flex 流完全关闭后有滞留期（`DefaultDetachRetention`，约 2s，用于消化晚到数据包），`StreamClosed` 钩子在滞留期届满时触发。按沟通过程中的决定，closed 暂不按 100ms 标准验收。若后续需要即时化，flex 侧把钩子提前到 detach 时刻即可（改动很小），到时再排期。

### 3.2 `hub.started` / `hub.mode` 未实现（§4.1）

- `hub.started`：进程启动早于任何客户端连接，sidecar 重启后客户端走"重连 → subscribe → `hub.snapshot`"已覆盖全部场景，单独事件冗余。
- `hub.mode`：`mode`/`configError` 在进程生命周期内**不会变化**（normal/degraded 由启动时配置解析结果决定），不存在可推送的变更时机。
- `hub.stopped` 保留并加强：优雅停机时在关闭连接前即时广播（此前仅有 poller 兜底检测）。
- `hub.snapshot` 携带 `startedAtMs`，`uptimeMs` 由客户端换算（与 §4.1 约定一致）。`version`/`commit`/`protocol`/`configPath` 仅在快照携带。

### 3.3 慢客户端策略采用"断连重连"，非分级缓冲

需求 §5 建议分级缓冲。实现选择了更简单的等价方案：可靠事件投递失败即断开该客户端连接，客户端经既有的"重连 → subscribe → 快照+增量"自愈。可靠性语义相同（生命周期事件不静默丢失），且不需要双队列优先级机制。桌面端无需额外适配——正常走断线重连流程即可。

### 3.4 事件信封字段名保持 `timestamp`（未改名 `ts`）

需求 §2.4 草案为 `{type, ts, seq, topic?, data}`。改名会破坏现有消费者且违反 §2.5 自己的 additive 原则，故**保留 `timestamp`**，纯新增 `topic`（必有）与 `seq`（必有）。请桌面端按实际字段名实现。

### 3.5 v1 兼容行为保留（需知悉）

- 未发送 subscribe 的客户端仍接收**全部**事件（包括新的 `*.snapshot` 不会主动发——只有订阅 topics 或 resync 才发；stats 类会收到）。
- v1 `events` 订阅方式继续有效（按事件类型精确过滤，不触发快照）。
- poller 的差分版 `service.status`（计数器变化触发）与 `stream.update` 均保留。v2 客户端计数器请以 `service.stats` / `stream.stats` 为准，避免双通道重复消费。

### 3.6 `stream.opened` 的 `serviceName` 为尽力而为

归属标注在批处理窗口（≤50ms）内反查，入口侧（隧道发起端）流可正常标注；出口侧（vtcp 监听接受端）inbound 流与已关闭流可能为空——这与 REST `/streams` 的现状一致（连接关闭即注销归属），后续流速属性由 `stream.stats` 每 tick 刷新兜底。

### 3.7 `startedAtMs`（network）语义

与需求确认的一致：`aliveMs` 与 `startedAtMs` 同源于**网络对象创建时刻**（约等于 agent 启动/热挂载时刻），不是单次连接在线时长（重连不重置）。若需要"本次在线时长"需另提需求。

### 3.8 `pipeId`（stream）：tunnel 两端流的关联标识

`StreamStateDTO` 追加 additive 字段：

```go
PipeID string `json:"pipeId,omitempty"` // pipe 配对标识，未配对时省略
```

经 tunnel 中转的一条端到端连接在本 agent 上表现为两条流（listen 侧 + target 侧），两者现携带**相同** `pipeId`（形如 `服务名#序号`，仅进程运行期内唯一），桌面端可按键 `pipeId` 把两条流聚合为一个"逻辑连接"视图（字节数/速率求和，或并列展示两端）。语义与 `serviceName` 一致：活跃流在快照与 `stream.opened`/`stream.stats`/`stream.update(opened)` 中按可识别标注；连接关闭即注销，已关闭流与 `stream.closed` 不携带。未经过 tunnel 配对的流（如直连监听）无此字段。

## 4. 桌面端适配指南

### 4.1 types.ts 镜像

```typescript
// 事件信封（v2）
interface SidecarEvent<T = unknown> {
  type: string;
  timestamp: number;  // 注意：保留原字段名，未改名 ts
  topic: string;      // v2 新增
  seq: number;        // v2 新增，主题内单调递增
  data: T;
}

// StreamStateDTO 新增
interface StreamStateDTO {
  // ...既有字段不变...
  createdAtMs: number;   // 流发起时刻（Unix 毫秒）
  closedAtMs?: number;   // 仅已关闭流
  pipeId?: string;       // pipe 配对标识：tunnel 两端流同值（§3.8），未配对省略
}

// NetworkStateDTO 新增
interface NetworkStateDTO {
  // ...既有字段不变...
  startedAtMs: number;   // 网络对象创建时刻
}

// ListeningPortDTO 新增
interface ListeningPortDTO {
  port: number; protocol: string;
  address?: string;      // 监听地址
  pid?: number; processName?: string;
  procStartMs?: number;  // 进程启动时刻（近似：进程运行时长）
}

// hub.snapshot 载荷
interface HubSnapshot {
  running: boolean; mode: string; version: string; commit: string;
  protocol: number; startedAtMs: number; configPath: string; configError?: string;
}
```

### 4.2 订阅 + 快照基线 + seq 空洞检测

```typescript
// 连接后：订阅主题（store 初始化完全走快照，无需先调 REST）
ws.send(JSON.stringify({ action: "subscribe", topics: ["hub", "network", "service", "stream"] }));

// 各主题维护 lastSeq；快照设置基线，增量事件幂等应用
const lastSeq: Record<string, number> = {};

ws.onmessage = (msg) => {
  const evt = JSON.parse(msg.data) as SidecarEvent;

  if (evt.type.endsWith(".snapshot")) {
    lastSeq[evt.topic] = evt.seq;         // 快照覆盖 ≤ seq 的全部增量
    stores[evt.topic].replaceAll(evt.data); // 状态重建
    return;
  }

  const last = lastSeq[evt.topic] ?? 0;
  if (evt.seq <= last) return;             // 快照基线内的迟到事件，幂等跳过
  if (last > 0 && evt.seq > last + 1) {
    // seq 空洞：主题级重同步（快照到达前建议缓冲后续增量）
    ws.send(JSON.stringify({ action: "resync", topics: [evt.topic] }));
    return;
  }
  lastSeq[evt.topic] = evt.seq;
  applyIncremental(evt);                   // 实体按主键 upsert（流用 index）
};
```

要点：
- **幂等应用**：快照采集与增量广播并发，seq > 快照 seq 的增量可能与快照内容重叠，按主键 upsert 即天然幂等。
- **被踢即重连**：可靠事件溢出时服务端会主动断连，走既有指数退避重连 + 重订阅即可，状态自动恢复一致。
- **uptime 展示**：`hub.snapshot.startedAtMs` 本地换算，停止轮询 `/status`。

### 4.3 protocol 版本门控

```typescript
// ready 握手（Rust Host 侧）/ GET /status 的 protocol 字段
if (protocol >= 2) {
  // 走 WS 订阅模式（订阅 topics，消费 snapshot/stats/生命周期事件）
} else {
  // 回退现有 REST 轮询 + events 订阅模式
}
```

**注意**：sidecar 协议 §9 要求 Host 对高于自身支持上限的 protocol 报错——Rust Host 侧的 protocol 支持上限需同步提升到 2，与本版 sidecar 同版本发布。

## 5. 验收标准对照（§6）

| # | 验收项 | 结果 |
|---|--------|------|
| 1 | 纯 WS 渲染四面板 | ✅ 四主题快照 + 增量已覆盖；REST 仅保留兜底 |
| 2 | 流到 UI ≤100ms | ✅ opened 实测 ~10ms 量级；closed 按约定豁免（~2s 滞后，见 §3.1） |
| 3 | 断连 30s 重连恢复 | ✅ 重订阅即快照重建；另有心跳暴露死连 |
| 4 | 1000 流并发不丢 | ✅ 50ms 窗口合并批量 + 可靠事件踢慢客户端（不静默丢）；建议桌面端压测复核 |
| 5 | protocol=1 组合回退 | ✅ v1 events 订阅/全量广播/REST 全部保留；需 Rust Host 同步 protocol 上限（见 §4.3） |

## 6. 交付物索引

- 字段与 DTO：`api/dto.go`（createdAtMs/closedAtMs/startedAtMs/address/procStartMs）
- 订阅协议核心：`api/websocket.go`（信封/订阅/resync/背压/心跳）、`api/snapshot.go`（快照生产器 + 流事件批处理器）
- 周期 stats：`api/poller.go`（network.stats/service.stats 走慢节拍 `pollInterval`；stream.stats/stream.update 走快节拍 `streamStatsInterval`，默认 1s）
- 流事件链路：tcp 侧 `vnet/conn_tracker.go`（StreamTrace 回调）+ flex 侧 `vnet/flex.go`（SetTrace 接线）→ `plugins/nettcp`/`plugins/netflex`（EventBus 主题 `stream.opened/closed`，类型定义在 `vnet/events.go`）→ `plugins/api` → WS
- 父进程看门狗：`cmd/watchdog.go` / `cmd/watchdog_unix.go` / `cmd/watchdog_windows.go`
- 监听端口：`utils/listening_ports.go`
- 协议版本：`cmd/handshake.go`（`protocolVersion = 2`）；`docs/tauri-sidecar-protocol.md` §9 已同步
- 事件契约文档：`api/README.md`「WebSocket 实时事件」
