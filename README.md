# Remotework

> Make Remotework easy again

## 介绍

Remotework是一个用来进行辅助远程控制的工具，运行agent的两台机器，可以通过服务端进行流量中转，代理RDP、SSH等端口流量，实现远程控制。

Remotework只是流量的搬运工，实际的远程控制工具依赖RDP client、SSH client等第三方工具。

- 桌面客户端（Tauri 2 + React）：已拆分为独立仓库 remotework-desktop
- 控制面 API（REST + WebSocket）：见 [api/README.md](api/README.md)

## 构建

要求 Go 1.24+。依赖中的 flex 已 pin 到 `github.com/net-agent/flex/v3 v3.1.0`，首次构建时由 Go 模块代理拉取；不需要同级 flex 仓库，也没有 `replace` 指令。

```bash
make build       # 当前平台，输出 dist/remote
make build-all   # 全平台交叉编译，输出 dist/<日期><后缀>/
make test        # 运行全部测试
```

## 运行模式

单一二进制通过 `-mode` 选择运行模式：

| 模式 | 说明 |
|---|---|
| `agent` | 连接 relay，按配置运行 link 与 tunnel（默认模式） |
| `server` | relay 服务端：单端口分流 Flex 协议与 HTTP/WebSocket |
| `validate` | 校验配置文件后退出（成功输出 `OK`） |

公共参数为 `-mode agent|server|validate`、`-c <配置文件>`、`-log-format text|json`；旧的 `-home` 参数已移除。

```bash
go run ./cmd -mode validate -c cmd/config_example/agent.json
```

## 典型配置

配置由 `links`（到中转服务器的连接）和 `tunnels`（转发规则）组成，完整示例见 [cmd/config_example/agent.json](cmd/config_example/agent.json)。

- 支持 JSON/JSONC 与 TOML：JSONC 的 `//`、`/* */` 注释（含行尾注释）可用，字符串内的 `http://` 不会被误删。
- 未知字段会让**整个文件**解析失败（配置整体不可用，可能进入仅 API 的降级模式）；结构合法但校验不通过的 link/tunnel 会被逐项丢弃并记录错误，其余条目仍会加载。
- tunnel 的 `id` 必须是 UUID 且唯一，`name` 必须唯一；`listen` 冲突也会被拒绝。

> 示例中的 `<server_host>`、`<server_port>`、`<shared_key>` 需替换为实际值；替换后可用 `go run ./cmd -mode validate -c <配置文件>` 校验。

### 被控制端的配置（以3389端口为例）
```jsonc
{
    "links": {
        // authRef 是环境变量引用（env:NAME 或裸 NAME）；也可改用明文 auth
        "relay": "tcp://<server_host>:<server_port>?as=agent1&authRef=RELAY_AUTH"
    },

    "tunnels": [
        {
            "id": "b2c3d4e5-f6a7-4b8c-9d0e-1f2a3b4c5d6e",
            "name": "expose-rdp",
            "listen": "vtcp://agent1.relay:1000?authcode=<shared_key>",
            "target": "tcp://localhost:3389"
        }
    ]
}
```
> 说明：被控制端agent按照上述配置进行运行后，将会在虚拟网络relay中把本地端口3389代理出去，访问虚拟网络中agent1的1000端口，相当于访问本机tcp端口3389。

### 控制端的配置
```jsonc
{
    "links": {
        "relay": "tcp://<server_host>:<server_port>?as=controller&authRef=RELAY_AUTH"
    },

    "tunnels": [
        {
            "id": "a1b2c3d4-e5f6-4a7b-8c9d-0e1f2a3b4c5d",
            "name": "rdp",
            "listen": "tcp://localhost:1000",
            "target": "vtcp://agent1.relay:1000?authcode=<shared_key>"
        }
    ]
}
```
> 说明：控制端agent将会监听本机1000端口，并将端口流量转发至虚拟网络中 agent1 的1000端口上。

### 密钥引用

- link 的 `authRef` 与 tunnel 的 `authcodeRef` 都是**环境变量引用**，写法为 `env:NAME` 或裸 `NAME`。进程启动时读取对应环境变量；变量未设置或为空会失败关闭（对应 link/tunnel 被丢弃），不会把引用名字本身当作密钥。
- link 的 `auth` 是明文密钥；`authRef` 与 `auth` 同时出现时以 `authRef` 为准。
- tunnel 的 `authcode`/`authcodeRef` 只能出现在 `vtcp://` 端点上。物理 `tcp://` 端点携带这些参数会被拒绝（`ERR_AUTHCODE_NOT_ALLOWED`），避免误以为已加密而实际仍是明文。
- `keepalive` 与 `via` 已不被 flex v3.1.0 支持，配置校验会分别报告 `ERR_KEEPALIVE_NOT_SUPPORTED`、`ERR_VIA_NOT_SUPPORTED`。

## 服务端

服务端从配置文件的 `server` 段启动：

```json
{
  "server": {
    "listen": "0.0.0.0:2040",
    "password": "change-me-min-8-chars",
    "wsEnable": true,
    "wsPath": "/ws"
  }
}
```

- `server.listen` 必填，且必须是合法的 `host:port`。
- `server.password` 必填，长度至少 8 个字符。
- `wsEnable=true` 时 `server.wsPath` 必填，且必须以 `/` 开头。
- 单端口分流会先探测连接首字节（5s 超时）：Flex 协议进入 relay，HTTP/WebSocket 进入对应 handler；`wsEnable=false` 时 HTTP 连接直接关闭。
- HTTP 服务已设置 5s/30s/30s/60s 的读头/读/写/空闲超时。

服务端运行 `-mode server`；收到 SIGINT/SIGTERM 时优雅关闭，第二次信号强制退出（130）。配置错误或端口绑定失败时以非零码退出。

## 进程关停

- agent 收到 SIGINT/SIGTERM 或 `POST /api/v1/stop` 后按逆依赖顺序卸载插件；关停总预算为 35s。关停不完整（例如 Drain/cleanup 报错或超时）会记录错误并以退出码 1 退出。
- 关停过程中再次收到 SIGINT/SIGTERM 会立即强制退出（130），避免只能 SIGKILL。
- server 的启动/绑定错误与关停超时均返回非零退出码。

## 流量加密

agent默认不会对转发的流量进行加密，网络中的数据包是以明文形式进行流动的。如果数据包需要在被监控或者不信任的网络中进行传递，可以在 tunnel 端点 URL 上添加 `authcode` 参数实现流量的点对点加密。

采用“预共享密钥”方案进行点对点加密，服务端仅仅负责流量转发，无法解密相关内容。收发两端的 `authcode` 必须一致。

> **限制：** 当前 `cipherconn v1.0.2` 只提供加密，不提供完整性认证（不是 AEAD），也不保证半关闭语义；它不能抵御主动篡改。需要抗主动攻击的场景不能只依赖该 `authcode` 通道。

### 流量接收端加密配置
```jsonc
{
    "links": {
        "relay": "tcp://<server_host>:<server_port>?as=agent1&authRef=RELAY_AUTH"
    },
    "tunnels": [{
        "id": "c3d4e5f6-a7b8-4c9d-0e1f-2a3b4c5d6e7f",
        "name": "expose-rdp-encrypted",
        "listen": "vtcp://agent1.relay:1000?authcode=<shared_key>",
        "target": "tcp://localhost:3389"
    }]
}
```

### 流量发起端加密配置
```jsonc
{
    "links": {
        "relay": "tcp://<server_host>:<server_port>?as=controller&authRef=RELAY_AUTH"
    },
    "tunnels": [{
        "id": "d4e5f6a7-b8c9-4d0e-1f2a-3b4c5d6e7f80",
        "name": "rdp-encrypted",
        "listen": "tcp://localhost:1000",
        "target": "vtcp://agent1.relay:1000?authcode=<shared_key>"
    }]
}
```

`authcodeRef` 也可替代 `authcode`，例如 `?authcodeRef=VTCP_SHARED_KEY`；它同样是环境变量引用，未设置时失败关闭。

## 发布（GitHub Release）

发布以 git tag 为唯一事实源。推送形如 `vX.Y.Z` 的 tag 会触发 [.github/workflows/release.yml](.github/workflows/release.yml)，自动完成测试、交叉编译、SHA-256 校验和生成与 Release 归档：

```bash
git tag -a v2.0.0 -m "v2.0.0"
git push origin v2.0.0
```

- 资产命名：`remote_v2.0.0_<os>_<arch>[.exe]`，覆盖 linux/darwin/windows 的 amd64 与 arm64。
- `checksums.txt` 记录全部资产的 SHA-256，下载后可用 `sha256sum -c checksums.txt` 校验。
- 构建期把 tag 名注入 `main.version`、提交短 SHA 注入 `main.commit`；`/api/v1/status` 与托管模式的 `ready` 控制行都会回报该版本。
- 带后缀的 tag（如 `v2.1.0-rc.1`）会以 prerelease 形式发布。
- 工作流失败时在 Actions 页面 Re-run jobs 即可重新构建并更新 Release。
