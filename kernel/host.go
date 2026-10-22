package kernel

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/net-agent/remotework/utils"
)

const (
	defaultInitTimeout       = 30 * time.Second
	defaultStartTimeout      = 30 * time.Second
	defaultDrainTimeout      = 10 * time.Second
	defaultCleanupTimeout    = 5 * time.Second
	defaultDependencyTimeout = 5 * time.Minute
)

// HostOptions 宿主可调参数，零值字段取默认值
type HostOptions struct {
	InitTimeout       time.Duration // Init 回调超时（超时按失败处理，默认 30s）
	StartTimeout      time.Duration // Start 回调超时（默认 30s）
	DrainTimeout      time.Duration // 卸载时 Drain 的默认时限（默认 10s）
	CleanupTimeout    time.Duration // 单条 cleanup 的执行时限（默认 5s）
	DependencyTimeout time.Duration // Pending 等待依赖的最长时限，超时转 Failed（默认 5m）
}

func (o HostOptions) withDefaults() HostOptions {
	if o.InitTimeout <= 0 {
		o.InitTimeout = defaultInitTimeout
	}
	if o.StartTimeout <= 0 {
		o.StartTimeout = defaultStartTimeout
	}
	if o.DrainTimeout <= 0 {
		o.DrainTimeout = defaultDrainTimeout
	}
	if o.CleanupTimeout <= 0 {
		o.CleanupTimeout = defaultCleanupTimeout
	}
	if o.DependencyTimeout <= 0 {
		o.DependencyTimeout = defaultDependencyTimeout
	}
	return o
}

// UnloadOptions 单次卸载参数
type UnloadOptions struct {
	Cascade      bool          // true 时级联卸载全部传递依赖者；false 时有依赖者则拒绝
	DrainTimeout time.Duration // 覆盖默认 Drain 时限
}

// PluginState 插件状态快照（只读视图）
type PluginState struct {
	Info     Info
	State    State
	Since    time.Time // 进入当前状态的时间
	LastErr  string
	Provides []string // 已发布的能力
	Deps     []string // 已记录的必需依赖
	Missing  []string // 当前缺失的能力（Pending 时非空）
}

// node 调度器内部的插件账本
type node struct {
	plugin  Plugin
	info    Info
	state   State
	since   time.Time
	lastErr string

	ctx *Context

	deps              map[capKey]bool // 已记录的必需依赖（跨 attempt 累积）
	missing           map[capKey]bool // 最近一次失败 attempt 缺失的能力
	depProviders      map[string]bool // deps 解析出的提供者插件名
	attemptedProvides []capKey        // 曾尝试发布的能力（含已回滚），用于环检测
	provides          []capKey        // 已提交的能力

	gen            uint64    // 过渡代次，用于丢弃过期的异步回调结果
	attemptRunning bool      // Init/Start 回调（或失败回滚）尚未真正结束，期间禁止新 attempt
	pendingStart   bool      // Pending 的起因是 Start 阶段（重试时回 Ready 而非重新 Init）
	failStart      bool      // Failed 的起因是 Start 阶段（Retry 时回 Ready）
	pendingSince   time.Time // 进入 Pending 的时间，用于依赖等待超时
	unloadMarked   bool
	drainTimeout   time.Duration
	unloadWaiters  []chan error
}

// Host 插件宿主：插件仓库 + 生命周期编排 + 事件分发。
// 内部为单 goroutine actor 调度循环，所有状态迁移串行发生。
type Host struct {
	log  *slog.Logger
	opts HostOptions
	caps *capRegistry
	bus  *eventBus

	cmdCh chan any
	evtCh chan any
	done  chan struct{}

	// 以下字段仅调度器 goroutine 访问
	nodes        map[string]*node
	order        []string // 装载顺序
	subs         map[chan Event]struct{}
	shutdown     bool
	shutdownRes  []chan error
	shutdownErrs []error // shutdown 期间各插件 unload（Drain/cleanup）的错误
}

// NewHost 创建宿主并启动调度循环
func NewHost(log *slog.Logger, opts *HostOptions) *Host {
	if log == nil {
		log = utils.NewModuleLogger("kernel")
	}
	o := HostOptions{}
	if opts != nil {
		o = *opts
	}
	h := &Host{
		log:   log,
		opts:  o.withDefaults(),
		caps:  newCapRegistry(),
		bus:   newEventBus(),
		cmdCh: make(chan any),
		evtCh: make(chan any, 128),
		done:  make(chan struct{}),
		nodes: make(map[string]*node),
		subs:  make(map[chan Event]struct{}),
	}
	// 预置内核能力：事件总线与插件目录，任何插件可立即注入
	_ = h.caps.commit(builtinOwner, map[capKey]any{
		keyOf[EventBus](""): h.bus,
		keyOf[Catalog](""):  h,
	})
	go h.loop()
	return h
}

//
// 公开 API
//

// Load 装载插件。返回时插件已登记（StateAdded），Init 由调度器异步驱动；
// 后续状态通过 State()/List()/Subscribe 观察。
func (h *Host) Load(p Plugin) error {
	res := make(chan error, 1)
	if err := h.send(cmdLoad{plugin: p, res: res}); err != nil {
		return err
	}
	return <-res
}

// Unload 卸载插件并等待完成。
// 默认拒绝卸载仍有依赖者的插件（ErrHasDependents）；Cascade 时按逆依赖序级联卸载。
// 返回值为 cleanup 过程汇聚的错误（nil 表示完全成功）。
func (h *Host) Unload(name string, opts UnloadOptions) error {
	res := make(chan error, 1)
	if err := h.send(cmdUnload{name: name, opts: opts, res: res}); err != nil {
		return err
	}
	return <-res
}

// Retry 重试 Failed / Pending 插件：Init 失败的重新 Init，Start 失败的重新 Start。
func (h *Host) Retry(name string) error {
	res := make(chan error, 1)
	if err := h.send(cmdRetry{name: name, res: res}); err != nil {
		return err
	}
	return <-res
}

// State 查询单个插件状态
func (h *Host) State(name string) (PluginState, error) {
	res := make(chan stateResult, 1)
	if err := h.send(cmdStateQuery{name: name, res: res}); err != nil {
		return PluginState{}, err
	}
	r := <-res
	return r.state, r.err
}

// List 按装载顺序列出全部插件状态
func (h *Host) List() []PluginState {
	res := make(chan []PluginState, 1)
	if err := h.send(cmdListQuery{res: res}); err != nil {
		return nil
	}
	return <-res
}

// Subscribe 订阅插件事件。每订阅者独立 channel，事件按序投递；
// channel 满时丢弃（调用方应给足缓冲）。
// 返回的函数用于退订（幂等）：退订后该 channel 被关闭，调用方的 range 可正常退出；
// 宿主 Shutdown 时也会关闭所有仍存活的订阅 channel。
func (h *Host) Subscribe(buf int) (<-chan Event, func()) {
	if buf <= 0 {
		buf = 64
	}
	ch := make(chan Event, buf)
	if err := h.send(cmdSubscribe{ch: ch}); err != nil {
		close(ch)
		return ch, func() {}
	}
	var once sync.Once
	return ch, func() {
		once.Do(func() { h.send(cmdUnsubscribe{ch: ch}) })
	}
}

// Shutdown 级联卸载全部插件并终止调度循环。幂等。
// 返回值为 shutdown 期间全部插件 Drain/cleanup 错误的聚合（nil 表示完全成功）；
// 调用方可用非 nil 结果判断关停不完整（如决定非零退出码）。
func (h *Host) Shutdown(ctx context.Context) error {
	res := make(chan error, 1)
	if err := h.send(cmdShutdown{res: res}); err != nil {
		return nil // 循环已退出：此前已完成关闭
	}
	select {
	case err := <-res:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (h *Host) send(cmd any) error {
	select {
	case h.cmdCh <- cmd:
		return nil
	case <-h.done:
		return ErrHostShutdown
	}
}

func (h *Host) sendEvt(evt any) {
	select {
	case h.evtCh <- evt:
	case <-h.done:
	}
}

//
// 命令与事件
//

type cmdLoad struct {
	plugin Plugin
	res    chan error
}

type cmdUnload struct {
	name string
	opts UnloadOptions
	res  chan error
}

type cmdRetry struct {
	name string
	res  chan error
}

type cmdShutdown struct {
	res chan error
}

type stateResult struct {
	state PluginState
	err   error
}

type cmdStateQuery struct {
	name string
	res  chan stateResult
}

type cmdListQuery struct {
	res chan []PluginState
}

type cmdSubscribe struct {
	ch chan Event
}

type cmdUnsubscribe struct {
	ch chan Event
}

type evtInitDone struct {
	name     string
	gen      uint64
	err      error
	snap     attemptSnapshot
	finished <-chan struct{} // 回调 goroutine 真正结束的信号
	timedOut bool            // true 时回调可能仍在运行
}

type evtStartDone struct {
	name     string
	gen      uint64
	err      error
	snap     attemptSnapshot
	finished <-chan struct{}
	timedOut bool
}

// evtAttemptFinished 超时回调/提交失败回滚真正结束，宿主可解除 attemptRunning。
type evtAttemptFinished struct {
	name string
	gen  uint64
}

type evtUnloadDone struct {
	name string
	gen  uint64
	errs []error
}

//
// 调度循环
//

func (h *Host) loop() {
	defer close(h.done)
	// 单一调度器定时器承载全部 Pending 依赖超时，不为每个插件起 goroutine。
	timer := time.NewTimer(time.Hour)
	if !timer.Stop() {
		<-timer.C
	}
	defer timer.Stop()
	var pendingC <-chan time.Time
	for {
		pendingC = h.armDependencyTimer(timer, pendingC)
		select {
		case cmd := <-h.cmdCh:
			h.handleCmd(cmd)
		case evt := <-h.evtCh:
			h.handleEvt(evt)
		case <-pendingC:
			pendingC = nil
			h.checkDependencyTimeouts()
		}
		h.reconcile()
		if h.shutdown && len(h.nodes) == 0 {
			// 聚合 shutdown 期间所有节点的 Drain/cleanup 错误：
			// 调用方（cmd）据此判断关停是否完整并决定退出码。
			err := errors.Join(h.shutdownErrs...)
			for _, res := range h.shutdownRes {
				res <- err
			}
			for ch := range h.subs {
				close(ch)
			}
			h.subs = nil
			h.bus.close()
			return
		}
	}
}

func (h *Host) handleCmd(cmd any) {
	switch c := cmd.(type) {
	case cmdLoad:
		h.onCmdLoad(c)
	case cmdUnload:
		h.onCmdUnload(c)
	case cmdRetry:
		h.onCmdRetry(c)
	case cmdShutdown:
		h.onCmdShutdown(c)
	case cmdStateQuery:
		h.onCmdStateQuery(c)
	case cmdListQuery:
		h.onCmdListQuery(c)
	case cmdSubscribe:
		h.subs[c.ch] = struct{}{}
	case cmdUnsubscribe:
		// 调度器内 delete 后再 close：Shutdown 只关闭仍在册的 channel，
		// 因此同一 channel 不会被关闭两次。退订后接收方 range 能正常退出。
		if _, ok := h.subs[c.ch]; ok {
			delete(h.subs, c.ch)
			close(c.ch)
		}
	}
}

func (h *Host) onCmdLoad(c cmdLoad) {
	if h.shutdown {
		c.res <- ErrHostShutdown
		return
	}
	if c.plugin == nil || c.plugin.Info().Name == "" {
		c.res <- ErrInvalidPlugin
		return
	}
	info := c.plugin.Info()
	if _, exists := h.nodes[info.Name]; exists {
		c.res <- fmt.Errorf("%w: %q", ErrPluginExists, info.Name)
		return
	}
	n := &node{
		plugin:       c.plugin,
		info:         info,
		state:        StateAdded,
		since:        time.Now(),
		deps:         make(map[capKey]bool),
		missing:      make(map[capKey]bool),
		depProviders: make(map[string]bool),
	}
	// module 属性后出现者覆盖先前者（见 utils.ConsoleHandler），
	// 插件日志器的 [module] 标签即插件名，如 [tunnel/xxx]
	n.ctx = newContext(info.Name, h.log.With("module", info.Name), h.caps)
	h.nodes[info.Name] = n
	h.order = append(h.order, info.Name)
	h.log.Info("plugin loaded", "name", info.Name, "kind", info.Kind)
	h.emit(Event{Kind: EventLoaded, Plugin: info.Name, From: StateAdded, To: StateAdded})
	c.res <- nil
}

func (h *Host) onCmdUnload(c cmdUnload) {
	n := h.nodes[c.name]
	if n == nil {
		c.res <- ErrPluginNotFound
		return
	}
	if n.unloadMarked {
		// 已在卸载中：挂到同一等待队列，与本次调用一同完成
		n.unloadWaiters = append(n.unloadWaiters, c.res)
		return
	}
	dependents := h.transitiveDependents(c.name)
	if len(dependents) > 0 && !c.opts.Cascade {
		c.res <- &ErrHasDependents{Name: c.name, Dependents: dependents}
		return
	}
	dt := c.opts.DrainTimeout
	if dt <= 0 {
		dt = h.opts.DrainTimeout
	}
	for _, name := range dependents {
		h.markUnload(h.nodes[name], dt)
	}
	h.markUnload(n, dt)
	n.unloadWaiters = append(n.unloadWaiters, c.res)
}

func (h *Host) markUnload(n *node, drainTimeout time.Duration) {
	n.unloadMarked = true
	n.drainTimeout = drainTimeout
}

func (h *Host) onCmdRetry(c cmdRetry) {
	if h.shutdown {
		c.res <- ErrHostShutdown
		return
	}
	n := h.nodes[c.name]
	if n == nil {
		c.res <- ErrPluginNotFound
		return
	}
	if n.unloadMarked {
		c.res <- fmt.Errorf("kernel: plugin %q is unloading", c.name)
		return
	}
	if n.attemptRunning {
		// 上一次 Init/Start 超时后回调可能仍未返回：此时开始新 attempt
		// 会与旧回调争用同一 Context 暂存区，必须等其真正结束。
		c.res <- fmt.Errorf("%w: plugin %q callback is still running; retry after it returns",
			ErrAttemptRunning, c.name)
		return
	}
	switch n.state {
	case StateFailed:
		if n.failStart {
			n.failStart = false
			h.setState(n, StateReady)
		} else {
			h.setState(n, StateAdded)
		}
		c.res <- nil
	case StatePending:
		n.pendingStart = false
		h.setState(n, StateAdded)
		c.res <- nil
	default:
		c.res <- fmt.Errorf("kernel: retry only applies to failed/pending plugins, current state: %s", n.state)
	}
}

func (h *Host) onCmdShutdown(c cmdShutdown) {
	h.shutdownRes = append(h.shutdownRes, c.res)
	if h.shutdown {
		return
	}
	h.shutdown = true
	h.log.Info("host shutting down", "plugins", len(h.nodes))
	for _, n := range h.nodes {
		h.markUnload(n, h.opts.DrainTimeout)
	}
}

func (h *Host) onCmdStateQuery(c cmdStateQuery) {
	n := h.nodes[c.name]
	if n == nil {
		c.res <- stateResult{err: ErrPluginNotFound}
		return
	}
	c.res <- stateResult{state: h.snapshotNode(n)}
}

func (h *Host) onCmdListQuery(c cmdListQuery) {
	out := make([]PluginState, 0, len(h.order))
	for _, name := range h.order {
		if n := h.nodes[name]; n != nil {
			out = append(out, h.snapshotNode(n))
		}
	}
	c.res <- out
}

func (h *Host) snapshotNode(n *node) PluginState {
	return PluginState{
		Info:     n.info,
		State:    n.state,
		Since:    n.since,
		LastErr:  n.lastErr,
		Provides: keyStrings(n.provides),
		Deps:     keySetStrings(n.deps),
		Missing:  keySetStrings(n.missing),
	}
}

func keyStrings(keys []capKey) []string {
	if len(keys) == 0 {
		return nil
	}
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		out = append(out, k.String())
	}
	sort.Strings(out)
	return out
}

func keySetStrings(set map[capKey]bool) []string {
	if len(set) == 0 {
		return nil
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k.String())
	}
	sort.Strings(out)
	return out
}

func (h *Host) handleEvt(evt any) {
	switch e := evt.(type) {
	case evtInitDone:
		h.onInitDone(e)
	case evtStartDone:
		h.onStartDone(e)
	case evtUnloadDone:
		h.onUnloadDone(e)
	case evtAttemptFinished:
		h.onAttemptFinished(e)
	}
}

func (h *Host) onInitDone(e evtInitDone) {
	n := h.nodes[e.name]
	if n == nil || n.gen != e.gen {
		return
	}
	n.attemptedProvides = keysOf(e.snap.provides)
	n.deps = e.snap.deps
	n.depProviders = h.resolveOwners(e.snap.deps)
	if e.timedOut {
		// 回调仍在运行：保持 attemptRunning，Retry 会被拒绝；
		// 暂存区留给 watcher 在回调真正结束后回滚，避免与新 attempt 交错。
		n.missing = e.snap.missing
		n.failStart = false
		n.lastErr = e.err.Error()
		h.log.Error("plugin init timed out", "plugin", n.info.Name, "err", e.err)
		h.setState(n, StateFailed)
		h.watchTimedOutAttempt(n, e.finished, e.gen)
		return
	}
	n.attemptRunning = false
	if e.err == nil {
		if err := h.caps.commit(n.info.Name, e.snap.provides); err != nil {
			// 提交冲突（并发重复发布）：暂存区回滚期间继续持有 attemptRunning，
			// 防止 Retry 启动的新 attempt 被回滚清空；回滚完成后才解锁。
			n.attemptRunning = true
			n.failStart = false
			n.lastErr = err.Error()
			h.log.Error("plugin provide commit failed", "plugin", n.info.Name, "err", err)
			h.setState(n, StateFailed)
			h.rollbackAttemptAsync(n, e.gen)
			return
		}
		n.ctx.commitAttempt()
		n.provides = keysOf(e.snap.provides)
		n.missing = nil
		h.setState(n, StateReady)
		return
	}
	n.missing = e.snap.missing
	var notReady *ErrCapabilityNotReady
	if errors.As(e.err, &notReady) {
		h.setState(n, StatePending)
		h.cycleCheck(n)
		return
	}
	n.failStart = false
	n.lastErr = e.err.Error()
	h.log.Error("plugin init failed", "plugin", n.info.Name, "err", e.err)
	h.setState(n, StateFailed)
}

func (h *Host) onStartDone(e evtStartDone) {
	n := h.nodes[e.name]
	if n == nil || n.gen != e.gen {
		return
	}
	// Start 阶段可能声明了新依赖，增量合并
	for k := range e.snap.deps {
		n.deps[k] = true
	}
	n.depProviders = h.resolveOwners(n.deps)
	if e.timedOut {
		n.missing = e.snap.missing
		n.failStart = true
		n.lastErr = e.err.Error()
		h.log.Error("plugin start timed out", "plugin", n.info.Name, "err", e.err)
		h.setState(n, StateFailed)
		h.watchTimedOutAttempt(n, e.finished, e.gen)
		return
	}
	n.attemptRunning = false
	if e.err == nil {
		if err := h.caps.commit(n.info.Name, e.snap.provides); err != nil {
			n.attemptRunning = true
			n.failStart = true
			n.lastErr = err.Error()
			h.log.Error("plugin provide commit failed", "plugin", n.info.Name, "err", err)
			h.setState(n, StateFailed)
			h.rollbackAttemptAsync(n, e.gen)
			return
		}
		n.ctx.commitAttempt()
		n.provides = append(n.provides, keysOf(e.snap.provides)...)
		n.missing = nil
		h.setState(n, StateRunning)
		return
	}
	n.missing = e.snap.missing
	var notReady *ErrCapabilityNotReady
	if errors.As(e.err, &notReady) {
		n.pendingStart = true
		h.setState(n, StatePending)
		h.cycleCheck(n)
		return
	}
	n.failStart = true
	n.lastErr = e.err.Error()
	h.log.Error("plugin start failed", "plugin", n.info.Name, "err", e.err)
	h.setState(n, StateFailed)
}

// watchTimedOutAttempt 等待超时回调真正结束，回滚该 attempt 的暂存副作用，
// 然后通知调度器解除 attemptRunning。回滚完成前 Retry 始终被拒绝。
func (h *Host) watchTimedOutAttempt(n *node, finished <-chan struct{}, gen uint64) {
	go func() {
		<-finished
		n.ctx.rollbackAttempt(h.log, h.opts.CleanupTimeout)
		h.sendEvt(evtAttemptFinished{name: n.info.Name, gen: gen})
	}()
}

// rollbackAttemptAsync 回调已结束（提交冲突）时的异步回滚；
// 完成前维持 attemptRunning，避免新 attempt 复用暂存区。
func (h *Host) rollbackAttemptAsync(n *node, gen uint64) {
	go func() {
		n.ctx.rollbackAttempt(h.log, h.opts.CleanupTimeout)
		h.sendEvt(evtAttemptFinished{name: n.info.Name, gen: gen})
	}()
}

func (h *Host) onAttemptFinished(e evtAttemptFinished) {
	n := h.nodes[e.name]
	if n == nil || n.gen != e.gen {
		return // 插件已进入新 attempt 或已卸载，无需解除
	}
	n.attemptRunning = false
}

func (h *Host) onUnloadDone(e evtUnloadDone) {
	n := h.nodes[e.name]
	if n == nil || n.gen != e.gen {
		return
	}
	h.caps.withdrawOwner(n.info.Name)
	delete(h.nodes, n.info.Name)
	for i, name := range h.order {
		if name == n.info.Name {
			h.order = append(h.order[:i], h.order[i+1:]...)
			break
		}
	}
	err := errors.Join(e.errs...)
	if h.shutdown && err != nil {
		// 只在 shutdown 期间累计；普通 Unload 仍只通过 waiter 返回错误。
		h.shutdownErrs = append(h.shutdownErrs, err)
	}
	for _, w := range n.unloadWaiters {
		w <- err
	}
	h.log.Info("plugin unloaded", "name", n.info.Name)
	h.emit(Event{Kind: EventUnloaded, Plugin: n.info.Name, From: StateStopping, To: StateUnloaded, Err: errString(err)})
}

//
// reconcile：事件/命令处理后驱动状态机收敛，直至不动点
//

func (h *Host) reconcile() {
	for i := 0; i <= len(h.order)+2; i++ {
		changed := false

		if !h.shutdown {
			for _, name := range h.order {
				n := h.nodes[name]
				if n == nil || n.unloadMarked {
					continue
				}
				switch n.state {
				case StateAdded:
					h.beginInit(n)
					changed = true
				case StatePending:
					if h.missingResolved(n) {
						if n.pendingStart {
							n.pendingStart = false
							h.setState(n, StateReady)
						} else {
							h.beginInit(n)
						}
						changed = true
					}
				case StateReady:
					if h.depsRunning(n) {
						h.beginStart(n)
						changed = true
					}
				}
			}
		}

		for _, name := range h.order {
			n := h.nodes[name]
			if n == nil || !n.unloadMarked {
				continue
			}
			switch n.state {
			case StateInitializing, StateStarting, StateStopping:
				continue // 过渡中的插件等待其异步回调完成
			}
			if h.hasDependentsInRegistry(n) {
				continue // 依赖者未清退完毕，保持等待
			}
			h.beginUnload(n)
			changed = true
		}

		if !changed {
			break
		}
	}
}

func (h *Host) missingResolved(n *node) bool {
	for k := range n.missing {
		if _, ok := h.caps.get(k); !ok {
			return false
		}
	}
	return true
}

//
// Pending 依赖等待超时：全部插件共用调度循环里的一个定时器，无每插件 goroutine。
//

// armDependencyTimer 把定时器对准最早的 Pending 截止时刻；无 Pending 插件时停表。
// 仅在调度循环内调用（timer 与 armed 均由单个 goroutine 访问）。
func (h *Host) armDependencyTimer(timer *time.Timer, armed <-chan time.Time) <-chan time.Time {
	deadline, ok := h.earliestPendingDeadline()
	if !ok {
		if armed != nil {
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
		}
		return nil
	}
	if armed != nil && !timer.Stop() {
		// 定时器可能已触发但事件尚在 channel 中，先排空再 Reset。
		select {
		case <-timer.C:
		default:
		}
	}
	d := time.Until(deadline)
	if d < 0 {
		d = 0
	}
	timer.Reset(d)
	return timer.C
}

// earliestPendingDeadline 返回仍在等待依赖的插件中最早的超时时刻
func (h *Host) earliestPendingDeadline() (time.Time, bool) {
	var earliest time.Time
	for _, name := range h.order {
		n := h.nodes[name]
		if n == nil || n.state != StatePending || n.unloadMarked || n.pendingSince.IsZero() {
			continue
		}
		deadline := n.pendingSince.Add(h.opts.DependencyTimeout)
		if earliest.IsZero() || deadline.Before(earliest) {
			earliest = deadline
		}
	}
	return earliest, !earliest.IsZero()
}

// checkDependencyTimeouts 将等待依赖超时的 Pending 插件转为 Failed。
// 若依赖在截止前出现，插件已离开 Pending，因此不影响正常的自动重试。
func (h *Host) checkDependencyTimeouts() {
	timeout := h.opts.DependencyTimeout
	now := time.Now()
	for _, name := range h.order {
		n := h.nodes[name]
		if n == nil || n.state != StatePending || n.unloadMarked || n.pendingSince.IsZero() {
			continue
		}
		if now.Sub(n.pendingSince) < timeout {
			continue
		}
		if h.missingResolved(n) {
			// 依赖在定时器触发前已出现：交给随后的 reconcile 自动重试，
			// 不要因为边界竞态把本可恢复的插件判失败。
			continue
		}
		missing := keySetStrings(n.missing)
		n.failStart = n.pendingStart
		n.pendingStart = false
		n.lastErr = (&ErrDependencyTimeout{
			Plugin:  name,
			Missing: missing,
			Timeout: timeout,
		}).Error()
		h.log.Error("plugin dependency wait timed out",
			"plugin", name, "missing", missing, "timeout", timeout)
		h.setState(n, StateFailed)
	}
}

// builtinOwner 内核预置能力的属主标识
const builtinOwner = "kernel"

// depsRunning 全部必需依赖的提供者均已 Running。
// 已提交依赖图无环，因此该检查不会死锁。
// 内核预置能力（属主 builtinOwner）没有对应插件节点，视为始终就绪。
func (h *Host) depsRunning(n *node) bool {
	for k := range n.deps {
		e, ok := h.caps.get(k)
		if !ok {
			return false
		}
		if e.owner == builtinOwner {
			continue
		}
		owner := h.nodes[e.owner]
		if owner == nil || owner.state != StateRunning {
			return false
		}
	}
	return true
}

//
// 状态过渡（调度器内触发，插件回调在 worker goroutine 中执行）
//

func (h *Host) beginInit(n *node) {
	h.setState(n, StateInitializing)
	n.gen++
	gen := n.gen
	n.attemptRunning = true
	n.ctx.beginAttempt(phaseInit)
	go func() {
		err, finished, timedOut := runWithTimeout(h.opts.InitTimeout, func() error {
			return n.plugin.Init(n.ctx)
		})
		if timedOut {
			// 回调可能仍在写 Context：先封锁暂存区，使迟到的 Provide/DeferCleanup
			// 失败；这里绝不回滚（会与旧回调并发清空暂存区），交由宿主在回调
			// 真正结束后回滚并解除 attemptRunning。
			n.ctx.sealAttempt()
		}
		snap := n.ctx.snapshotAttempt()
		if err != nil && !timedOut {
			n.ctx.rollbackAttempt(h.log, h.opts.CleanupTimeout)
		}
		h.sendEvt(evtInitDone{
			name: n.info.Name, gen: gen, err: err, snap: snap,
			finished: finished, timedOut: timedOut,
		})
	}()
}

func (h *Host) beginStart(n *node) {
	h.setState(n, StateStarting)
	n.gen++
	gen := n.gen
	n.attemptRunning = true
	n.ctx.beginAttempt(phaseStart)
	go func() {
		err, finished, timedOut := runWithTimeout(h.opts.StartTimeout, func() error {
			return n.plugin.Start(n.ctx)
		})
		if timedOut {
			n.ctx.sealAttempt()
		}
		snap := n.ctx.snapshotAttempt()
		if err != nil && !timedOut {
			n.ctx.rollbackAttempt(h.log, h.opts.CleanupTimeout)
		}
		h.sendEvt(evtStartDone{
			name: n.info.Name, gen: gen, err: err, snap: snap,
			finished: finished, timedOut: timedOut,
		})
	}()
}

func (h *Host) beginUnload(n *node) {
	h.setState(n, StateStopping)
	n.ctx.close() // 发出卸载信号
	n.gen++
	gen := n.gen
	drainTimeout := n.drainTimeout
	go func() {
		var errs []error
		if d, ok := n.plugin.(Drainer); ok {
			if err, _, _ := runWithTimeout(drainTimeout, func() error { return d.Drain(drainTimeout) }); err != nil {
				errs = append(errs, fmt.Errorf("drain: %w", err))
			}
		}
		errs = append(errs, n.ctx.runStack(h.log, h.opts.CleanupTimeout)...)
		h.sendEvt(evtUnloadDone{name: n.info.Name, gen: gen, errs: errs})
	}()
}

// cycleCheck 插件进入 Pending 后做等待图环检测；环上插件判 Failed
func (h *Host) cycleCheck(start *node) {
	cycle := h.findCycle(start.info.Name)
	if len(cycle) == 0 {
		return
	}
	errText := (&ErrDependencyCycle{Path: cycle}).Error()
	h.log.Error("dependency cycle detected", "path", cycle)
	for _, name := range cycle {
		n := h.nodes[name]
		if n == nil || n.state != StatePending {
			continue
		}
		n.pendingStart = false
		n.lastErr = errText
		h.setState(n, StateFailed)
	}
}

//
// 事件分发（非阻塞；订阅者需自备缓冲）
//

func (h *Host) setState(n *node, to State) {
	from := n.state
	if from == to {
		return
	}
	n.state = to
	n.since = time.Now()
	if to == StatePending {
		n.pendingSince = n.since
	} else {
		n.pendingSince = time.Time{}
	}
	h.log.Debug("plugin state changed", "name", n.info.Name, "from", from.String(), "to", to.String())
	h.emit(Event{Kind: EventStateChanged, Plugin: n.info.Name, From: from, To: to, Err: n.lastErr})
}

func (h *Host) emit(evt Event) {
	if len(h.subs) == 0 {
		return
	}
	evt.Time = time.Now()
	for ch := range h.subs {
		select {
		case ch <- evt:
		default:
			h.log.Debug("event dropped: subscriber channel full", "plugin", evt.Plugin, "kind", evt.Kind.String())
		}
	}
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
