// Package tunnel 隧道插件：portproxy / socks5 服务，内部复用 vservice 控制器。
package tunnel

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/net-agent/cipherconn"
	"github.com/net-agent/remotework/configv2"
	"github.com/net-agent/remotework/kernel"
	"github.com/net-agent/remotework/utils"
	"github.com/net-agent/remotework/vnet"
	"github.com/net-agent/remotework/vservice"
)

// Plugin 隧道插件，每个 tunnel 配置对应一个实例
type Plugin struct {
	spec   configv2.TunnelSpec
	log    *slog.Logger
	svc    *vservice.Service
	closer *onceCloser
}

// New 由 tunnel 配置创建隧道插件
func New(spec configv2.TunnelSpec) *Plugin {
	return &Plugin{spec: spec}
}

func (p *Plugin) Info() kernel.Info {
	return kernel.Info{Name: "tunnel/" + p.spec.Name, Kind: "tunnel"}
}

func (p *Plugin) Init(c *kernel.Context) error {
	p.log = c.Logger()

	// 1. 注入 listen 侧网络并建立监听
	listenEp, err := resolveEndpoint(p.spec.Listen)
	if err != nil {
		return err
	}
	listenNet, err := kernel.Inject[vnet.Network](c, listenEp.network)
	if err != nil {
		return err
	}
	listener, err := listenNet.Listen(listenEp.network, listenEp.addr)
	if err != nil {
		return fmt.Errorf("listen %q failed: %w%s", utils.RedactURL(p.spec.Listen.RawURL), err, portOwnerHint(p.spec.Listen, listenEp.addr))
	}
	if listenEp.authcode != "" {
		listener = utils.NewSecretListener(listener, listenEp.authcode)
	}
	p.closer = &onceCloser{Listener: listener}
	if err := c.DeferCleanup(func(ctx context.Context) error { return p.closer.Close() }); err != nil {
		return err
	}

	// 2. 按 target 类型构建服务
	var svc *vservice.Service
	if p.spec.Target.Kind == configv2.EndpointBuiltin {
		switch p.spec.Target.Scheme {
		case "socks5":
			// socks5 出站拨号经内置 tcp 网络发出，使 target 侧连接纳入
			// 流追踪与归属登记（socks 库默认 net.Dial 不产生可追踪连接）
			dialNet, err := kernel.Inject[vnet.Network](c, "tcp")
			if err != nil {
				return err
			}
			svc = vservice.NewSocks5Service(
				staticListenerFactory{p.closer},
				p.spec.Name, internalURL(p.spec.Listen),
				p.spec.Target.Username, p.spec.Target.Password,
				"tcp", dialNet.Dial,
				p.log,
			)
		default:
			return fmt.Errorf("unsupported builtin service: %s", p.spec.Target.Scheme)
		}
	} else {
		targetEp, err := resolveEndpoint(p.spec.Target)
		if err != nil {
			return err
		}
		targetNet, err := kernel.Inject[vnet.Network](c, targetEp.network)
		if err != nil {
			return err
		}
		dialer := func() (net.Conn, error) {
			conn, err := targetNet.Dial(targetEp.network, targetEp.addr)
			if err != nil {
				return nil, err
			}
			if targetEp.authcode != "" {
				return cipherconn.New(conn, targetEp.authcode)
			}
			return conn, nil
		}
		svc = vservice.NewPortproxyService(
			staticListenerFactory{p.closer}, staticDialerFactory{dialer},
			p.spec.Name, internalURL(p.spec.Listen), internalURL(p.spec.Target),
			p.log,
		)
	}

	svc.TunnelID = p.spec.ID
	svc.SetStatus(vservice.StatusInit)
	if err := svc.Controller.Init(); err != nil {
		return err
	}
	p.svc = svc
	if err := c.DeferCleanup(func(ctx context.Context) error {
		err := svc.Controller.Close()
		svc.SetStatus(vservice.StatusStopped)
		return err
	}); err != nil {
		return err
	}
	return kernel.Provide[*vservice.Service](c, p.spec.Name, svc)
}

func (p *Plugin) Start(c *kernel.Context) error {
	p.svc.SetStatus(vservice.StatusRunning)
	go func() {
		err := p.svc.Controller.Start()
		select {
		case <-c.Done(): // 正常卸载导致的退出
		default:
			if err != nil {
				p.log.Error("service loop exited unexpectedly", "err", err)
			}
		}
	}()
	return nil
}

// Drain 实现 kernel.Drainer：先停止接收新连接，再等待存量连接在时限内结束
func (p *Plugin) Drain(deadline time.Duration) error {
	p.closer.Close()
	if p.svc == nil {
		return nil
	}
	until := time.Now().Add(deadline)
	for time.Now().Before(until) {
		if p.svc.GetActiveCount() <= 0 {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("%d active connection(s) remain", p.svc.GetActiveCount())
}

//
// 端点解析
//

type endpoint struct {
	network  string // 网络能力名（vnet 端点为 link alias，物理端点为协议 scheme）
	addr     string // Listen/Dial 的地址参数（host:port；vnet 端点为 vhostname:port）
	authcode string // 传输层加密密钥（仅 vnet 端点）
}

// resolveEndpoint 将隧道端点解析为网络能力名与地址。
// vtcp 端点必须携带 authcode——这是运行时校验兜底，
// 弥补动态装载路径绕过 configv2.Validate 的缺口。
func resolveEndpoint(ep configv2.EndpointSpec) (endpoint, error) {
	if ep.Kind == configv2.EndpointVNet {
		authcode := utils.FirstString(ep.Authcode, ep.AuthcodeRef)
		if authcode == "" {
			return endpoint{}, fmt.Errorf("vtcp endpoint %q requires authcode", ep.RawURL)
		}
		return endpoint{
			network:  ep.Alias,
			addr:     net.JoinHostPort(ep.VHostname, ep.Port),
			authcode: authcode,
		}, nil
	}
	u, err := url.Parse(ep.RawURL)
	if err != nil {
		return endpoint{}, fmt.Errorf("parse endpoint %q: %w", ep.RawURL, err)
	}
	return endpoint{network: u.Scheme, addr: u.Host}, nil
}

// portOwnerHint 在物理端口监听失败时，尽力找出占用该端口的系统进程，
// 帮助用户区分"tunnel 之间冲突"（校验期报错）与"与系统已有服务冲突"。
// 仅对物理端点（tcp:// 等）有意义；任何一步失败都静默降级为空串。
func portOwnerHint(ep configv2.EndpointSpec, addr string) string {
	if ep.Kind != configv2.EndpointNet {
		return ""
	}
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return ""
	}
	ports, err := utils.ListListeningPorts()
	if err != nil {
		return ""
	}
	for _, item := range ports {
		if strconv.Itoa(int(item.Port)) != port {
			continue
		}
		if item.ProcessName != "" {
			return fmt.Sprintf(" (port %s is occupied by process %q pid=%d)", port, item.ProcessName, item.PID)
		}
		return fmt.Sprintf(" (port %s is occupied by another process)", port)
	}
	return ""
}

// internalURL 生成 ServiceState 中记录的 URL 表示：
// vnet 端点 -> alias://vhost:port[?authcode=xxx]，其余端点保持原始 URL。
// 该 URL 供状态展示与 ping 依赖解析使用。
func internalURL(ep configv2.EndpointSpec) string {
	if ep.Kind == configv2.EndpointVNet {
		u := fmt.Sprintf("%s://%s:%s", ep.Alias, ep.VHostname, ep.Port)
		if authcode := utils.FirstString(ep.Authcode, ep.AuthcodeRef); authcode != "" {
			u += "?authcode=" + url.QueryEscape(authcode)
		}
		return u
	}
	return ep.RawURL
}

//
// 静态工厂：把 Init 阶段已装配好的 listener/dialer 喂给 vservice 控制器
//

type staticListenerFactory struct{ l net.Listener }

func (f staticListenerFactory) ListenURL(string) (net.Listener, error) { return f.l, nil }

type staticDialerFactory struct{ d vservice.QuickDialer }

func (f staticDialerFactory) URLDialer(string) (vservice.QuickDialer, error) { return f.d, nil }

// onceCloser 幂等关闭的 Listener：Drain 提前停止监听与 Controller.Close 互不冲突
type onceCloser struct {
	net.Listener
	once sync.Once
	err  error
}

func (c *onceCloser) Close() error {
	c.once.Do(func() { c.err = c.Listener.Close() })
	return c.err
}
