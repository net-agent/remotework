# Remotework

> Make Remotework easy again

## 介绍

Remotework是一个用来进行辅助远程控制的工具，运行agent的两台机器，可以通过服务端进行流量中转，代理RDP、SSH等端口流量，实现远程控制。

Remotework只是流量的搬运工，实际的远程控制工具依赖RDP client、SSH client等第三方工具。

- 桌面客户端（Tauri 2 + React）：见 [desktop/README.md](desktop/README.md)
- 控制面 API（REST + WebSocket）：见 [api/README.md](api/README.md)

## 构建

要求 Go 1.24+。当前依赖尚未发布的 flex 特性，需要在仓库同级目录 checkout flex 仓库（`go.mod` 通过 `replace` 指向 `../flex`）。

```bash
make build       # 当前平台，输出 dist/remote
make build-all   # 全平台交叉编译，输出 dist/<日期><后缀>/
make test        # 运行全部测试
```

## 典型配置

配置由 `links`（到中转服务器的连接）和 `tunnels`（转发规则）组成，完整示例见 [cmd/config_example/agent.json](cmd/config_example/agent.json)。

### 被控制端的配置（以3389端口为例）
```jsonc
{
    "links": {
        "relay": "tcp://<server_host>:<server_port>?as=agent1&auth=<password>"
    },

    "tunnels": [
        { "name": "expose-rdp", "listen": "vtcp://agent1.relay:1000", "target": "tcp://localhost:3389" }
    ]
}
```
> 说明：被控制端agent按照上述配置进行运行后，将会在虚拟网络relay中把本地端口3389代理出去，访问虚拟网络中agent1的1000端口，相当于访问本机tcp端口3389。

### 控制端的配置
```jsonc
{
    "links": {
        "relay": "tcp://<server_host>:<server_port>?as=controller&auth=<password>"
    },

    "tunnels": [
        { "name": "rdp", "listen": "tcp://localhost:1000", "target": "vtcp://agent1.relay:1000" }
    ]
}
```
> 说明：控制端agent将会监听本机1000端口，并将端口流量转发至虚拟网络中 agent1 的1000端口上。

## 流量加密

agent默认不会对转发的流量进行加密，网络中的数据包是以明文形式进行流动的。如果数据包需要在被监控或者不信任的网络中进行传递，可以在 tunnel 端点 URL 上添加 `authcode` 参数实现流量的点对点加解密。

采用“预共享密钥”方案进行点对点加解密，服务端仅仅负责流量转发，无法解密相关内容。收发两端的 `authcode` 必须一致。

### 流量接收端加解密配置
```jsonc
{
    "tunnels": [{
        "listen": "vtcp://agent1.relay:1000?authcode=password123456",
        "target": "tcp://localhost:3389"
    }]
}
```

### 流量发起端加解密配置
```jsonc
{
    "tunnels": [{
        "listen": "tcp://localhost:1000",
        "target": "vtcp://agent1.relay:1000?authcode=password123456"
    }]
}
```
