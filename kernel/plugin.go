// Package kernel 提供插件化运行时（微内核）：
// 统一管理插件的装载、依赖编排、生命周期状态机，以及卸载时的副作用消除。
//
// 核心概念：
//   - Plugin：一切可装载单元（网络、服务、观察器等），实现 Init/Start 两个生命周期钩子
//   - Context：能力容器，插件间唯一的交互媒介；能力键 = (Go 类型, 名称)
//   - Inject：注入即依赖声明，宿主据此构建依赖图并编排启动顺序
//   - DeferCleanup：副作用消除代码，卸载信号到达时按 LIFO 执行
//
// 时序不变量：
//   - 依赖者的 Start 只在所有必需依赖的提供者 Running 之后执行
//   - 卸载严格逆依赖序：依赖者先卸载完毕，被依赖者才开始清理
//   - 插件回调（Init/Start/cleanup）中的 panic 不会波及其他插件与宿主
package kernel

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// State 插件生命周期状态
type State int

const (
	StateAdded        State = iota // 已装载，等待初始化
	StateInitializing              // Init 执行中
	StatePending                   // 依赖未就绪，等待自动重试
	StateReady                     // Init 完成，等待依赖提供者进入 Running
	StateStarting                  // Start 执行中
	StateRunning                   // 运行中
	StateStopping                  // 卸载中（cleanup 执行中）
	StateFailed                    // 失败（可通过 Host.Retry 重试）
	StateUnloaded                  // 已卸载（终态，仅出现在事件中）
)

func (s State) String() string {
	switch s {
	case StateAdded:
		return "added"
	case StateInitializing:
		return "initializing"
	case StatePending:
		return "pending"
	case StateReady:
		return "ready"
	case StateStarting:
		return "starting"
	case StateRunning:
		return "running"
	case StateStopping:
		return "stopping"
	case StateFailed:
		return "failed"
	case StateUnloaded:
		return "unloaded"
	default:
		return "unknown"
	}
}

// Info 插件元信息
type Info struct {
	Name string // 全局唯一标识，如 "network.office"、"tunnel.rdp-home"
	Kind string // 类别元数据，如 "network"、"service"、"observer"，仅用于展示与过滤
}

// Plugin 可装载单元接口。
// 注意：插件没有 Stop 方法——停止 = 宿主发出卸载信号后执行 Context 的 cleanup 栈。
type Plugin interface {
	// Init 构建期：在此注入依赖（Inject）、构建内部状态、发布能力（Provide）、
	// 登记副作用消除函数（DeferCleanup）。
	// 约定：先 Inject 后产生副作用。Init 失败时本次 attempt 暂存的能力被丢弃、
	// 已登记的 cleanup 自动回滚执行，插件不会泄漏半成品状态。
	// Init 必须在 HostOptions.InitTimeout 内返回。
	Init(c *Context) error

	// Start 激活期：所有必需依赖的提供者均已 Running 后才会被调用。
	// 必须快速返回（HostOptions.StartTimeout）；常驻循环放入插件自有 goroutine，
	// 并通过 ctx.Done() / DeferCleanup 响应卸载。
	Start(c *Context) error

	Info() Info
}

// Drainer 可选接口：卸载时在执行 cleanup 栈之前调用，用于优雅排水
// （停止接收新工作，等待存量完成，不超过 deadline）。
type Drainer interface {
	Drain(deadline time.Duration) error
}

//
// 错误定义
//

var (
	ErrPluginNotFound = errors.New("kernel: plugin not found")
	ErrPluginExists   = errors.New("kernel: plugin already exists")
	ErrInvalidPlugin  = errors.New("kernel: invalid plugin (nil or empty name)")
	ErrHostShutdown   = errors.New("kernel: host has shut down")
)

// ErrCapabilityNotReady 必需的能力尚不可用。
// 插件回调返回该错误（或其包装）时，插件进入 Pending 而非 Failed，
// 待能力出现后由宿主自动重试。
type ErrCapabilityNotReady struct {
	Type string
	Name string
}

func (e *ErrCapabilityNotReady) Error() string {
	return fmt.Sprintf("kernel: capability (%s)[%q] not ready", e.Type, e.Name)
}

// ErrDuplicateCapability 能力键冲突（已被其他插件发布）
type ErrDuplicateCapability struct {
	Key   string
	Owner string
}

func (e *ErrDuplicateCapability) Error() string {
	return fmt.Sprintf("kernel: capability %s already provided by %q", e.Key, e.Owner)
}

// ErrHasDependents 卸载被拒绝：仍有依赖者存活
type ErrHasDependents struct {
	Name       string
	Dependents []string
}

func (e *ErrHasDependents) Error() string {
	return fmt.Sprintf("kernel: cannot unload %q: still required by %s (use Cascade to override)",
		e.Name, strings.Join(e.Dependents, ", "))
}

// ErrDependencyCycle 检测到依赖环，环上所有插件被标记为 Failed
type ErrDependencyCycle struct {
	Path []string
}

func (e *ErrDependencyCycle) Error() string {
	return fmt.Sprintf("kernel: dependency cycle detected: %s", strings.Join(e.Path, " -> "))
}

//
// 事件
//

// EventKind 事件类型
type EventKind int

const (
	EventLoaded       EventKind = iota // 插件已装载
	EventStateChanged                  // 状态迁移
	EventUnloaded                      // 插件已卸载
)

func (k EventKind) String() string {
	switch k {
	case EventLoaded:
		return "loaded"
	case EventStateChanged:
		return "state_changed"
	case EventUnloaded:
		return "unloaded"
	default:
		return "unknown"
	}
}

// Event 宿主发出的插件事件。订阅者按顺序接收（每订阅者独立 channel）。
type Event struct {
	Time   time.Time
	Kind   EventKind
	Plugin string
	From   State
	To     State
	Err    string // Failed 时的错误信息
}
