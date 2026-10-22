package kernel

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"sync"
	"time"
)

// 插件回调阶段：Inject/Provide/DeferCleanup 只允许在 Init/Start 回调内调用
type phase int

const (
	phaseIdle phase = iota
	phaseInit
	phaseStart
	phaseClosed
)

// capKey 能力键 = (Go 类型, 名称)
type capKey struct {
	typ  reflect.Type
	name string
}

func keyOf[T any](name string) capKey {
	return capKey{typ: reflect.TypeOf((*T)(nil)).Elem(), name: name}
}

func (k capKey) String() string {
	return fmt.Sprintf("(%s)[%q]", k.typ.String(), k.name)
}

type capEntry struct {
	value any
	owner string
}

// capRegistry 能力注册表。写操作只发生在调度器 goroutine（commit/withdraw），
// 读操作来自插件回调 goroutine（Inject），用 RWMutex 保护。
type capRegistry struct {
	mu   sync.RWMutex
	caps map[capKey]capEntry
}

func newCapRegistry() *capRegistry {
	return &capRegistry{caps: make(map[capKey]capEntry)}
}

func (r *capRegistry) get(k capKey) (capEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.caps[k]
	return e, ok
}

// commit 将一次 attempt 暂存的能力整体提交；任一冲突则全部不提交
func (r *capRegistry) commit(owner string, provides map[capKey]any) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for k := range provides {
		if e, ok := r.caps[k]; ok {
			return &ErrDuplicateCapability{Key: k.String(), Owner: e.owner}
		}
	}
	for k, v := range provides {
		r.caps[k] = capEntry{value: v, owner: owner}
	}
	return nil
}

// withdrawOwner 摘除某插件发布的全部能力
func (r *capRegistry) withdrawOwner(owner string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for k, e := range r.caps {
		if e.owner == owner {
			delete(r.caps, k)
		}
	}
}

// CleanupFunc 副作用消除函数。ctx 携带超时，实现方必须尊重 ctx 否则会被强超时跳过。
type CleanupFunc func(ctx context.Context) error

// Context 宿主提供给插件的能力容器视图。每个插件持有独立 scope：
// 记录自己的依赖、暂存自己的发布、维护自己的 cleanup 栈。
//
// 生命周期事务语义：
//   - Init/Start 期间的 Provide 与 DeferCleanup 均为暂存（staged）
//   - 回调成功：暂存能力一次性提交，cleanup 并入正式栈
//   - 回调失败：暂存能力丢弃，暂存 cleanup 立即 LIFO 回滚执行
type Context struct {
	owner string
	log   *slog.Logger
	caps  *capRegistry

	done      chan struct{}
	closeOnce sync.Once

	mu             sync.Mutex
	phase          phase
	deps           map[capKey]bool // 本次 attempt 已解析的必需依赖
	missing        map[capKey]bool // 本次 attempt 缺失的能力
	stagedProvides map[capKey]any
	stagedCleanups []CleanupFunc
	cleanups       []CleanupFunc // 已提交的 cleanup 栈
}

func newContext(owner string, log *slog.Logger, caps *capRegistry) *Context {
	return &Context{
		owner:   owner,
		log:     log,
		caps:    caps,
		done:    make(chan struct{}),
		phase:   phaseIdle,
		deps:    make(map[capKey]bool),
		missing: make(map[capKey]bool),
	}
}

// Logger 返回插件维度的日志器
func (c *Context) Logger() *slog.Logger { return c.log }

// Done 卸载信号通道。宿主开始卸载该插件时关闭。
// 插件的常驻 goroutine 应监听该通道主动退出；cleanup 栈是其后的兜底。
func (c *Context) Done() <-chan struct{} { return c.done }

// DeferCleanup 登记副作用消除函数，卸载时按 LIFO 执行。
// 仅允许在 Init/Start 回调内调用。
func (c *Context) DeferCleanup(fn CleanupFunc) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.phase != phaseInit && c.phase != phaseStart {
		return errors.New("kernel: DeferCleanup only allowed during Init/Start")
	}
	c.stagedCleanups = append(c.stagedCleanups, fn)
	return nil
}

// Inject 注入必需依赖。该调用同时是依赖声明：宿主据此建依赖图。
// 能力不存在时返回 *ErrCapabilityNotReady，插件通常应直接 return 该错误，
// 使插件进入 Pending 等待能力出现后被自动重试。
// 仅允许在 Init/Start 回调内调用。
func Inject[T any](c *Context, name string) (T, error) {
	var zero T
	if err := c.checkCallable(); err != nil {
		return zero, err
	}
	key := keyOf[T](name)
	entry, ok := c.caps.get(key)
	if !ok {
		c.recordMiss(key)
		return zero, &ErrCapabilityNotReady{Type: key.typ.String(), Name: name}
	}
	if entry.owner == c.owner {
		return zero, fmt.Errorf("kernel: plugin %q cannot inject its own capability %s", c.owner, key)
	}
	val, ok := entry.value.(T)
	if !ok {
		return zero, fmt.Errorf("kernel: capability %s has incompatible type", key)
	}
	c.recordDep(key)
	return val, nil
}

// InjectOptional 注入可选依赖，不构成依赖图边，缺失时返回零值与 false。
func InjectOptional[T any](c *Context, name string) (T, bool) {
	var zero T
	if err := c.checkCallable(); err != nil {
		return zero, false
	}
	key := keyOf[T](name)
	entry, ok := c.caps.get(key)
	if !ok || entry.owner == c.owner {
		return zero, false
	}
	val, ok := entry.value.(T)
	if !ok {
		return zero, false
	}
	return val, true
}

// Provide 发布能力供其他插件注入。回调成功后才对其他插件可见（事务提交）。
// 仅允许在 Init/Start 回调内调用。
func Provide[T any](c *Context, name string, v T) error {
	key := keyOf[T](name)
	// 乐观检查：全局已存在同名能力
	if e, ok := c.caps.get(key); ok {
		return &ErrDuplicateCapability{Key: key.String(), Owner: e.owner}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	// 阶段检查与写入必须在同一临界区：回调超时后宿主会封锁 phase，
	// 迟到的 Provide 必须失败，绝不能写进下一次 attempt 的暂存区。
	if c.phase != phaseInit && c.phase != phaseStart {
		return errors.New("kernel: Inject/Provide/DeferCleanup only allowed during Init/Start")
	}
	if _, ok := c.stagedProvides[key]; ok {
		return &ErrDuplicateCapability{Key: key.String(), Owner: c.owner}
	}
	c.stagedProvides[key] = v
	return nil
}

// Lookup 动态查询能力，任意时刻可调用（不要求处于 Init/Start 回调内）。
// 与 Inject 的区别：不记录依赖边、不提供时序与卸载保护——
// 能力可能随时被其提供者卸载，调用方需自行容忍。
// 适用于观察器类插件的按需查询；需要生命周期保证的场景应使用 Inject。
func Lookup[T any](c *Context, name string) (T, bool) {
	var zero T
	c.mu.Lock()
	closed := c.phase == phaseClosed
	c.mu.Unlock()
	if closed {
		return zero, false
	}
	key := keyOf[T](name)
	entry, ok := c.caps.get(key)
	if !ok {
		return zero, false
	}
	val, ok := entry.value.(T)
	if !ok {
		return zero, false
	}
	return val, true
}

func (c *Context) checkCallable() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.phase != phaseInit && c.phase != phaseStart {
		return errors.New("kernel: Inject/Provide/DeferCleanup only allowed during Init/Start")
	}
	return nil
}

func (c *Context) recordDep(key capKey) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.deps[key] = true
	delete(c.missing, key)
}

func (c *Context) recordMiss(key capKey) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.missing[key] = true
}

//
// 以下方法由宿主调度器驱动，插件不应调用
//

// beginAttempt 开始一次 Init/Start 尝试：重置暂存区
func (c *Context) beginAttempt(ph phase) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.phase = ph
	c.deps = make(map[capKey]bool)
	c.missing = make(map[capKey]bool)
	c.stagedProvides = make(map[capKey]any)
	c.stagedCleanups = nil
}

// attemptSnapshot 本次 attempt 的结果快照（依赖、缺失、暂存能力）
type attemptSnapshot struct {
	deps     map[capKey]bool
	missing  map[capKey]bool
	provides map[capKey]any
}

func (c *Context) snapshotAttempt() attemptSnapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	return attemptSnapshot{
		deps:     copyKeySet(c.deps),
		missing:  copyKeySet(c.missing),
		provides: copyProvides(c.stagedProvides),
	}
}

// commitAttempt 回调成功后提交：cleanup 并入正式栈，暂存区清空。
// 能力提交由调度器经 capRegistry.commit 完成。
func (c *Context) commitAttempt() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cleanups = append(c.cleanups, c.stagedCleanups...)
	c.stagedProvides = nil
	c.stagedCleanups = nil
	c.phase = phaseIdle
}

// sealAttempt 封锁暂存区写入（Init/Start 回调超时后由宿主调用）。
// 已暂存的 provides/cleanups 保留，待回调真正结束后由 rollbackAttempt 统一回滚；
// 迟到的 DeferCleanup/Provide/Inject 因 phase 不再是 Init/Start 而失败，
// 因此不会写进后续 attempt 的暂存区。
func (c *Context) sealAttempt() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.phase == phaseClosed {
		return
	}
	c.phase = phaseIdle
}

// rollbackAttempt 回调失败后回滚：LIFO 执行暂存 cleanup，丢弃暂存能力。
// phaseClosed 不可逆：插件卸载后迟到的回滚不得把 Context 重新变为可写。
func (c *Context) rollbackAttempt(log *slog.Logger, timeout time.Duration) {
	c.mu.Lock()
	staged := c.stagedCleanups
	c.stagedProvides = nil
	c.stagedCleanups = nil
	if c.phase != phaseClosed {
		c.phase = phaseIdle
	}
	c.mu.Unlock()

	runCleanupStack(log, c.owner, staged, timeout)
}

// close 发出卸载信号
func (c *Context) close() {
	c.closeOnce.Do(func() {
		c.mu.Lock()
		c.phase = phaseClosed
		c.mu.Unlock()
		close(c.done)
	})
}

// runStack 卸载时执行正式 cleanup 栈（LIFO），返回各条错误
func (c *Context) runStack(log *slog.Logger, timeout time.Duration) []error {
	c.mu.Lock()
	stack := c.cleanups
	c.cleanups = nil
	c.mu.Unlock()

	return runCleanupStack(log, c.owner, stack, timeout)
}

func copyKeySet(src map[capKey]bool) map[capKey]bool {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[capKey]bool, len(src))
	for k := range src {
		dst[k] = true
	}
	return dst
}

func copyProvides(src map[capKey]any) map[capKey]any {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[capKey]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// runCleanupStack LIFO 执行 cleanup，逐条带超时与 panic 隔离；单条失败不阻断后续
func runCleanupStack(log *slog.Logger, owner string, stack []CleanupFunc, timeout time.Duration) []error {
	var errs []error
	for i := len(stack) - 1; i >= 0; i-- {
		fn := stack[i]
		err := runCleanup(fn, timeout)
		if err != nil {
			log.Warn("cleanup failed", "plugin", owner, "index", i, "err", err)
			errs = append(errs, err)
		}
	}
	return errs
}

func runCleanup(fn CleanupFunc, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- safely(func() error { return fn(ctx) })
	}()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return fmt.Errorf("kernel: cleanup timeout after %v", timeout)
	}
}

// safely 插件回调的 panic 隔离边界
func safely(fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("kernel: panic recovered: %v", r)
		}
	}()
	return fn()
}

// runWithTimeout 带超时执行插件回调；超时不保证回调 goroutine 退出
// （插件契约要求其在超时内返回），仅保证宿主状态机继续推进。
//
// finished 在回调 goroutine 真正结束时关闭；正常完成返回时它保证已关闭。
// timedOut 表示本次调用因超时返回，此时回调可能仍在运行，
// 调用方不得在此期间并发回滚/重置该 attempt 的暂存区。
func runWithTimeout(d time.Duration, fn func() error) (err error, finished <-chan struct{}, timedOut bool) {
	done := make(chan error, 1)
	fin := make(chan struct{})
	finished = fin
	go func() {
		runErr := safely(fn)
		close(fin) // 先发结束信号再投递结果：调用方拿到正常结果时 finished 必已关闭
		done <- runErr
	}()
	if d <= 0 {
		return <-done, finished, false
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case err = <-done:
		return err, finished, false
	case <-timer.C:
		return fmt.Errorf("kernel: callback timeout after %v", d), finished, true
	}
}
