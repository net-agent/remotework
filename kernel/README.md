# kernel — 插件运行时开发者手册

`kernel` 是 remotework 的插件化运行时（微内核）：统一管理插件的装载、依赖编排、生命周期状态机，以及卸载时的副作用消除。内核本身不含任何网络/服务领域逻辑，一切功能以插件形式存在。

## 目录

- [核心概念](#核心概念)
- [生命周期状态机](#生命周期状态机)
- [Init 契约](#init-契约)
- [Start 契约](#start-契约)
- [Running 与动态查询](#running-与动态查询)
- [卸载协议](#卸载协议)
- [失败与重试语义](#失败与重试语义)
- [依赖环检测](#依赖环检测)
- [内核预置能力](#内核预置能力)
- [插件开发指南](#插件开发指南)
- [日志约定](#日志约定)
- [命名与能力约定](#命名与能力约定)
- [Host API 速查](#host-api-速查)
- [内置插件清单](#内置插件清单)

## 核心概念

| 概念 | 说明 |
|---|---|
| `Plugin` | 一切可装载单元（网络、服务、观察器），实现 `Init`/`Start` 两个生命周期钩子。没有 Stop 方法——停止 = 卸载信号触发 cleanup 栈 |
| `Context` | 能力容器，插件间唯一的交互媒介。每个插件持有独立 scope：记录自己的依赖、暂存自己的发布、维护自己的 cleanup 栈 |
| 能力（Capability） | 键 = `(Go 类型, 名称)`，经泛型 `Inject[T]` / `Provide[T]` 存取，类型安全 |
| Inject | 注入即依赖声明。宿主据此构建依赖图并编排启动/停止顺序 |
| `DeferCleanup` | 副作用消除函数，卸载信号到达时按 LIFO 执行 |

两个驱动原则：

1. **单 goroutine actor 调度**：所有状态判定在调度器内串行发生；插件回调（Init/Start/cleanup）在独立 worker goroutine 执行，结果以事件回投。同一插件的回调绝不会并发执行。
2. **不动点收敛（reconcile）**：任何事件（Load、能力提交、某插件进入 Running、Unload、Retry）都会触发一轮收敛扫描。依赖后到也能收敛，插件无需关心装载顺序。

## 生命周期状态机

```
                    Load(p)
                      │
                      ▼
                   Added
                      │
                      ▼
               Initializing ──ok────► Ready ──依赖全部Running──► Starting ──ok────► Running
                      │                                                              │
        缺依赖 ──────► Pending ──缺失能力出现──► 回到 Initializing 重试                │
        其他错误 ────► Failed ◄── Host.Retry 重试                                    │
                      ▲                                                              │
                      │                                                    Unload / Shutdown
                      │                                                              ▼
              Unloaded ◄── Stopping（Done 信号 → Drain → cleanup 栈 LIFO） ◄── (任意状态)
```

| 状态 | 含义 |
|---|---|
| `Added` | 已装载登记，等待 Init |
| `Initializing` | Init 执行中 |
| `Pending` | 依赖未就绪，等待自动重试 |
| `Ready` | Init 完成，等待依赖提供者进入 Running |
| `Starting` | Start 执行中 |
| `Running` | 运行中 |
| `Stopping` | 卸载中（cleanup 执行中） |
| `Failed` | 失败（可通过 `Host.Retry` 重试） |
| `Unloaded` | 已卸载（终态，仅出现在事件中） |

## Init 契约

**触发时机：**

| 场景 | 说明 |
|---|---|
| `Host.Load` 之后 | 插件登记为 `Added`，reconcile 立刻发起 Init |
| `Pending` 且缺失能力已出现 | 自动重试（重新执行完整 Init） |
| `Failed`（Init 阶段失败）后被 `Retry` | 重新执行完整 Init |

**调用环境：** 调度器先把 Context 置为 Init 阶段，然后在带 `InitTimeout`（默认 30s）超时与 panic 隔离的 goroutine 中调用 `Init(c)`。仅在此回调期间，`Inject` / `InjectOptional` / `Provide` / `DeferCleanup` 合法。

**事务语义：**

- Init 期间的 `Provide` 与 `DeferCleanup` 先进入暂存区；
- Init 返回 nil → 暂存能力一次性提交（对其他插件可见）、cleanup 并入正式栈、状态 → `Ready`；
- Init 返回 error 或 panic → 暂存能力丢弃、暂存 cleanup 立即 LIFO 回滚执行，插件不泄漏半成品状态。

**返回值语义分支（必须分清）：**

| Init 返回 | 结果状态 | 后续 |
|---|---|---|
| `nil` | `Ready` | 等待依赖 Running 后进 Start |
| `*ErrCapabilityNotReady`（Inject 的原样返回或其 `%w` 包装） | `Pending` | 缺失能力出现时自动重试 |
| 其他 error / panic / 超时 | `Failed` | 等待 `Retry` |

依赖注入失败时应**直接 return 该 err**，不要消化或误包装为非 Cause 错误。

## Start 契约

**唯一触发条件：** 插件已 `Ready`，且其所有已记录的必需依赖的提供者均处于 `Running`（内核预置能力视为永远就绪）。

Start 门槛按插件独立判定，全局自然形成拓扑启动序列——这就是"宿主根据依赖关系设置启动序列"的兑现方式。

**契约要点：**

- **必须快速返回**（默认 30s `StartTimeout`）。常驻循环（Accept 循环等）放入插件自有 goroutine，通过 `c.Done()` 感知卸载；
- Start 内同样允许 Inject/Provide/DeferCleanup（事务语义同 Init）；Start 阶段新注入的依赖并入依赖边，卸载排序对其同样生效；
- 结果分支：成功 → `Running`；返回 NotReady → `Pending`（重试时直接回 `Ready` 再 Start，不重跑 Init）；其他失败 → `Failed`（`Retry` 只重跑 Start）。

## Running 与动态查询

回调返回后 Context 锁定：再调 Inject/Provide/DeferCleanup 会报错。

运行期需要按需查询能力时使用 `kernel.Lookup[T](c, name)`：

- 任意时刻可调用（不要求处于回调内）；
- **不记录依赖边、不提供时序与卸载保护**——能力可能随时被其提供者卸载，调用方需自行容忍；
- 适用于观察器类插件的枚举查询；需要生命周期保证的场合必须用 `Inject`。

## 卸载协议

入口 `Host.Unload(name, opts)`，或被级联标记 / `Shutdown` 触发：

1. **依赖守卫**：仍有存活依赖者时默认拒绝（返回 `ErrHasDependents`，列出依赖者）；`UnloadOptions{Cascade: true}` 时级联标记整个传递依赖闭包；
2. **顺序保证**：依赖者彻底移除后，被依赖者才开始清理（严格逆依赖序）；
3. 状态 → `Stopping`，**先关闭 `Done()`**（插件 goroutine 应在此刻开始退出）；
4. 若插件实现 `Drainer` → 调用 `Drain(deadline)`（排水：停止接收新工作，等待存量完成）；
5. cleanup 栈 **LIFO** 逐条执行，每条带超时（默认 5s `CleanupTimeout`）、panic 隔离、单条失败不阻断后续，错误汇聚后作为 `Unload` 返回值；
6. 摘除该插件发布的全部能力，移除插件，广播 `EventUnloaded`。

`Pending`/`Added`/`Failed` 状态的插件直接卸载：仅执行已提交的 cleanup（未提交的暂存已在失败时回滚）。

## 失败与重试语义

`Host.Retry(name)` 区分失败阶段：

| 失败阶段 | Retry 行为 |
|---|---|
| Init 失败 | 状态回 `Added`，重新完整 Init |
| Start 失败 | 状态回 `Ready`，仅重跑 Start |
| `Pending` | 强制立即重新尝试（正常场景由依赖出现自动触发） |

所有插件回调 panic 都会被隔离：该插件判 `Failed`，宿主与其他插件不受影响。

Init/Start 超时（默认各 30s）按失败处理；超时不保证回调 goroutine 退出——**插件契约要求回调在超时内返回**，cleanup 必须尊重传入的 `ctx`。

## 依赖环检测

- 已提交的依赖图**按构造天然无环**：能力只在 Init/Start 成功后提交，Inject 只能命中已存在的能力，边总是从后装载者指向先提交者；
- 环只可能出现在"互相等待对方发布能力"的 Pending 插件之间。宿主在等待图（missing → 曾尝试提供者）上做 DFS，发现环则将环上所有插件判 `Failed` 并报出环路径（`ErrDependencyCycle`）。

## 内核预置能力

宿主在 `NewHost` 时以属主 `kernel` 发布两个单例能力（名称为 `""`），任何插件可立即注入，且永远视为就绪：

```go
bus, err := kernel.Inject[kernel.EventBus](c, "")  // 领域事件总线
catalog, err := kernel.Inject[kernel.Catalog](c, "") // 插件目录与生命周期管理
```

- **`EventBus`**：领域事件（如网络会话状态变化）的发布/订阅。`Publish` 可从任意 goroutine 调用，非阻塞（订阅者缓冲满则丢弃）。与插件生命周期 `Event` 不同，`DomainEvent` 不经过调度器。
- **`Catalog`**：`List`/`State`/`Subscribe`（生命周期事件）+ `Load`/`Unload`/`Retry`（动态装载管理）。观察器类插件（如 api）经此驱动管理界面。

## 插件开发指南

### 标准骨架

```go
func (p *MyPlugin) Init(c *kernel.Context) error {
    // 1. 先注入依赖（失败原样返回 → Pending 自动等待）
    mnet, err := kernel.Inject[vnet.Network](c, p.needNet)
    if err != nil {
        return err
    }

    // 2. 再产生副作用，且每个副作用紧跟一个 cleanup
    l, err := mnet.Listen("tcp", p.addr)
    if err != nil {
        return err
    }
    c.DeferCleanup(func(ctx context.Context) error { return l.Close() })

    // 3. 最后发布能力（Init 成功才对其他插件可见）
    return kernel.Provide[SomeCap](c, p.name, p.impl)
}

func (p *MyPlugin) Start(c *kernel.Context) error {
    go p.serveLoop(c.Done()) // 常驻循环放自有 goroutine，响应 Done
    return nil               // 快速返回
}

// 可选：优雅排水
func (p *MyPlugin) Drain(deadline time.Duration) error {
    p.stopAccepting()
    return p.waitIdle(deadline)
}
```

### 正确姿势

- Init 内**先 Inject 后产生副作用**（依赖缺失时避免白做功）；
- 每个副作用（listener、goroutine、注册项）都紧跟一个 `DeferCleanup`；
- cleanup 尊重传入 `ctx` 的超时；
- 常驻 goroutine 监听 `c.Done()` 主动退出，cleanup 是兜底。

### 禁忌

- ❌ 在 `Start` 里阻塞（会被判超时 Failed）；
- ❌ 在回调之外调 Inject/Provide/DeferCleanup（报错；改用 `Lookup`）；
- ❌ 插件间私下传递能力对象绕过 Context（失去依赖排序与卸载保护）；
- ❌ 依赖失败时消化错误（应原样返回 `*ErrCapabilityNotReady` 以进入 Pending）。

## 日志约定

**所有日志器都必须来自 kernel，禁止插件或其组件自行创建。**

- 插件通过 `c.Logger()` 获取日志器。它由 Host 派生，根源是 `NewHost(log)` 传入的全局日志器——改这一处即可全局调整日志行为；
- **`[module]` 标签 = 日志的归属者**：插件日志器的标签就是插件名（如 `[tunnel/rdp]`、`[network/office]`），`[kernel]` 只用于 Host 自身的编排日志（plugin loaded/unloaded、init failed 等）。实现上依赖 ConsoleHandler 对 `module` 属性"后出现覆盖先出现"的语义；
- Logger 是宿主基础设施，**不走 Inject**（它不是能力，插件无需声明依赖）；
- 插件把组件交给底层库（vnet/vservice 等）时，必须把 `c.Logger()` 透传下去：库构造函数的 logger 参数传 nil 会回退到自建日志器，只有显式透传才能让日志链路收敛到 kernel 命名空间。

```go
// 正确：透传 context 日志器
svc := vservice.NewPortproxyService(lf, df, name, listenURL, targetURL, c.Logger())

// 错误：nil 会让库自建日志器，脱离 kernel 管理
svc := vservice.NewPortproxyService(lf, df, name, listenURL, targetURL, nil)
```

**敏感信息脱敏**：日志中禁止明文打印含凭证的 URL（`authcode`/`authcodeRef`/`auth`/`secret`/userinfo 密码）。输出 URL 前必须经过 `utils.RedactURL()` 脱敏；API DTO 是否脱敏由控制面另行决定，不影响日志侧强制要求。

## 命名与能力约定

观察器类插件据此枚举实例（见 `plugins/doc.go`）：

| 插件类别 | Name | Kind | 提供能力 |
|---|---|---|---|
| 网络 | `network/`+alias | `network` | `vnet.Network[alias]` |
| 隧道 | `tunnel/`+隧道名 | `tunnel` | `*vservice.Service[隧道名]` |
| 观察器 | `api` 等 | `observer` | 按需 |

## Host API 速查

```go
host := kernel.NewHost(logger, &kernel.HostOptions{
    InitTimeout:    30 * time.Second, // 默认值
    StartTimeout:   30 * time.Second,
    DrainTimeout:   10 * time.Second, // Drain 默认时限
    CleanupTimeout: 5 * time.Second,  // 单条 cleanup 时限
})

err  := host.Load(plugin)                              // 登记并异步驱动 Init
err  := host.Unload(name, kernel.UnloadOptions{Cascade: true}) // 等待卸载完成
err  := host.Retry(name)                               // 重试 Failed/Pending
st   := host.State(name)                               // 状态快照
list := host.List()                                    // 按装载顺序列出
ch, unsub := host.Subscribe(64)                        // 生命周期事件流
err  := host.Shutdown(ctx)                             // 逆依赖序卸载全部并终止
```

状态快照 `PluginState` 含 `Info`/`State`/`Since`/`LastErr`/`Provides`/`Deps`/`Missing`，全部为只读拷贝。

## 内置插件清单

| 包 | 作用 |
|---|---|
| `plugins/nettcp` | 内置本地 TCP 网络（tcp/tcp4/tcp6） |
| `plugins/netflex` | flex 虚拟网络（每个 link 一个实例；会话状态经 EventBus 发布 `network.state`） |
| `plugins/tunnel` | portproxy / socks5 隧道（复用 vservice 控制器；实现 Drainer 优雅排水） |
| `plugins/api` | HTTP/WebSocket 控制面（观察器插件，经 Catalog/EventBus 驱动 `api.Backend`） |
