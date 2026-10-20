# net-agent/flex 库改造需求：流关闭事件时机前移（closed event at detach）

- 提出方：remotework 项目组
- 面向库：`github.com/net-agent/flex/v3`（remotework 当前经 `replace` 指向本地工作副本）
- 日期：2026-10-20
- 状态：待提交评审

## 1. 背景与目标

remotework 控制面把 tunnel 的一条端到端连接观测为两条配对流（pipeId 关联）：listen 侧流走本地 tcp 网络（remotework 自研追踪器），target 侧流走 flex 虚拟网络（本库的 `stream.Stream` + `node.StreamHub`）。

实测发现配对观测明显不同步：**tcp 侧流在连接实际关闭的瞬间即从活跃列表消失并发出 closed 事件；flex 侧流在实际关闭后仍滞留约 2 秒**——期间仍出现在 `GetStreamStates()` 活跃列表中，closed 事件（`Trace.StreamClosed`）与 closed 历史（`GetClosedStates`）也滞后约 2 秒才产生。按 pipeId 聚合观测时，用户会看到"一端已关闭、另一端还在"，最大相差约 2 秒。

**目标**：将 flex 流"完全关闭"事件的触发时机从**滞留期届满**前移到 **detach 时刻**（双向关闭齐备的瞬间），使 flex 侧关闭可见性与 tcp 侧对称；滞留期本身保留（继续承担 SID 寻址与晚到包消化职责）。

## 2. 现状分析（已核实源码）

### 2.1 关闭到事件的完整链路

本地主动关闭与对端发起关闭最终都汇聚到 `CloseRead`+`CloseWrite`：

- 本地：`Stream.Close()`（`stream/net_conn_closer.go:15`）→ `CloseWrite` → `SendClose` → 等 closeAck（`DefaultCloseAckTimeout` 2s）→ defer `CloseRead`
- 对端：`Stream.HandleCmdCloseStream`（`stream/handler.go:65`）→ 预设 `CloseReason="remote"` → `CloseRead` → `CloseWrite` → 回 closeAck

双向关闭齐备（`closeMask == closeMaskBoth`）后：

```
tryOnDetach (stream/lifecycle.go:52)
  └─ StreamHub.scheduleStreamCleanup (node/streamhub.go:85)
       └─ time.AfterFunc(detachRetention, doCleanup)   // 默认 2s，DefaultDetachRetention (streamhub.go:16)
            └─ doCleanup: streams.Delete + portm.Release + recordClosedState
                 └─ recordClosedState (streamhub.go:235) → traceStreamClosed(state) + 写入 closedStates 环形历史
```

### 2.2 滞后期内的可观测状态

- 流仍保留在 `hub.streams`（SID 可寻址，晚到包由 stream 层消化——`CloseRead` 后数据被丢弃，仅 `RecvBufferCount` 自增，`handler.go:26-29`）；
- `GetStreamStates()` 不过滤已关闭流，滞留流原样出现在活跃列表（`streamhub.go:146-156`），其 `State.IsClosed=true`、`Closed` 时刻已设置；
- `Trace.StreamClosed` 事件与 `GetClosedStates` 历史均要等滞留期届满（`recordClosedState` 在 `doCleanup` 内执行）。

### 2.3 关键事实：detach 时刻的状态快照已经完整

| 字段 | detach 时刻是否就绪 | 依据 |
|---|---|---|
| `IsClosed` / `Closed` | ✅ | `CloseWrite` 内设置（`net_conn_closer.go:62-88`），detach 必先经 `CloseWrite` |
| `CloseReason` | ✅ | first-wins，local/remote/node/manual 各路径在 detach 前均已落定（`lifecycle.go:20-31`、`handler.go:68`、`streamhub.go:108-119`） |
| `BytesRead` / `BytesWritten` | ✅ 已冻结 | 读已关 → 晚到数据丢弃不计数；写已关 → 无新写 |
| `RecvBufferCount` / `RecvAckTotal` | ⚠️ 可能微增 | 滞留期内晚到的 ack 仍会计数（`handler.go:55-60`） |

即：事件前移的代价仅为 `RecvBufferCount`/`RecvAckTotal` 两个缓冲/确认计数可能与最终历史记录存在微小差异，核心业务字段（地址、方向、字节数、关闭时刻、关闭原因）在 detach 时刻即为终值。

### 2.4 已知替代路径及为何不采纳

`Node.SetDetachRetention(0)`（`node.go:208`）可使清理与事件即时化，但：

1. 失去滞留期的设计价值——晚到包无处消化（`handleCmdPushStreamData` 命中 "stream not found" 告警刷屏），SID 复用防护（`doCleanup` 的 `cur == s` 校验）形同虚设；
2. `detachRetention` 仅允许 Serve 前写入（`streamhub.go:34`），且 `Session` 未暴露该配置——重连新建的 Node 不继承（`SetTrace` 有继承机制，见 `session.go:146-157`，DetachRetention 没有）。

## 3. 改造需求

### R3.1（P0）`Trace.StreamClosed` 事件时机前移至 detach 时刻

`StreamHub.attachStream` 注册的 `OnDetach` 回调改为**立即**触发事件：

```go
// node/streamhub.go attachStream 内
s.SetOnDetach(func() {
    hub.host.traceStreamClosed(s.GetState()) // 新增：detach 时刻立即发事件
    hub.scheduleStreamCleanup(sid, s)        // 保留：滞留期后清理 + 入历史
})
```

`recordClosedState` 相应**移除** `traceStreamClosed` 调用（避免双发），保留 closedStates 历史写入职责（仍在滞留期届满时执行，作为含晚到 ack 的终值记录）。

- 三条关闭路径（本地 Close / 对端 CmdCloseStream / 节点关闭 closeAllStreams）均汇聚于 detach，天然全覆盖；
- 事件回调在流关闭路径上同步执行，现有约束不变（Trace 文档已要求钩子非阻塞，`observe.go:148`）。

### R3.2（P0）重连继承

事件触发点位于 hub 层（每个重连新建的 Node 自带 hub），`Session.SetTrace` 的继承机制（`session.go:146-157`）自动生效，无需额外工作。请在测试中覆盖一次重连场景确认。

### R3.3（P1，可选）`GetStreamStates` 活跃列表语义

现状滞留流仍出现在活跃列表（`IsClosed=true`）。两个方向由库方择一：

- 保持现状（语义："hub 中可寻址的流"），使用方按 `IsClosed` 自行过滤——remotework 已计划按此方式对接；
- 或改为排除 `IsClosed` 流（语义："未完全关闭的流"），与 `closeStreamByIndex` 跳过已关闭流的语义（`streamhub.go:131`）对齐。

remotework 对两者均可适配，倾向后者（多数消费者想要的是"活跃流"），但属语义变更，需评估其他使用方。

## 4. 兼容性评估

- **API 兼容**：无导出符号变更，纯行为（时机）变更；
- **行为变化**：`Trace.StreamClosed` 从"滞留期届满"提前到"detach 时刻"（默认配置下提前约 2 秒）；事件载荷为 detach 时刻快照，`RecvBufferCount`/`RecvAckTotal` 可能与 2 秒后的历史终值微差（见 §2.3）；
- **影响面**：remote_agent 与其他 flex 使用方的流关闭事件同样提前。依赖"事件即终态计数"的消费者需知悉上述微差；依赖事件延迟展开资源的消费者（如有）需自查；
- `GetClosedStates` 历史语义不变（仍在滞留期后写入，游标语义不变）。

## 5. 验收标准

1. 单测覆盖三条关闭路径（本地 `Close`、对端 `CmdCloseStream`、`closeAllStreams`）：detach 时刻事件即时触发，载荷 `IsClosed=true`、`Closed`/`CloseReason` 正确；
2. 同一流的事件只触发一次（`detachDone` 防护 + `recordClosedState` 不再发事件），历史记录不重复；
3. 滞留期行为回归：滞留期内 SID 可寻址、晚到包不告警、期后清理与端口释放正常；
4. Session 重连后新 Node 继承 Trace 钩子，事件时机不变；
5. 语义化版本 tag 发布（建议 minor，如 v3.x.0）。

## 6. remotework 侧对接计划（供评估工作量参考）

库发布新版后，remotework 对接改动很小：

1. 升级 flex 版本（移除 `replace` 或更新指向）；
2. 保留 remotework 侧已规划的活跃列表过滤（collect 剔除 `IsClosed` 流，对齐 stats/快照通道）；
3. 放弃 remotework 侧的"差分提前事件"补丁（R3.1 落地后不再需要）；
4. 更新 `docs/sidecar-v2-response.md` §3.1 与 `api/README.md` 中"flex 流 closed 滞后约 2 秒"的已知行为描述。

如评审结论是维持现状，remotework 将以侧车差分方案（collect 检测 `IsClosed` 跳变自行补发事件 + 去重）兜底，但该方案事件粒度受拉取周期（1s）限制，且去重逻辑分散在 remotework 侧，长期维护成本高，仍建议采纳 R3.1。
