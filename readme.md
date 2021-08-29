# Remotework

> Make Remotework easy again

## 介绍

Remotework是一个用来进行辅助远程控制的工具，运行agent的两台机器，可以通过服务端进行流量中转，代理RDP、SSH等端口流量，实现远程控制。

Remotework只是流量的搬运工，实际的远程控制工具依赖RDP client、SSH client等第三方工具。

## 典型配置
### 被控制端的配置（以3389端口为例）
```jsonc
{
    "agents": [{
        "enable": true,
        "address": "<server ip>:<server port>",
        "password": "<server password>",

        "network": "vtcp",
        "domain": "test_agent",
    }],

    "portproxy": [
        { "listen": "vtcp://0:1000", "target": "tcp://localhot:3389" }
    ]
}
```
> 说明：被控制端agent按照上述配置进行运行后，将会在虚拟网络vtcp中把本地端口3389代理出去，访问虚拟网络vtcp的1000端口，相当于访问本机tcp端口3389。

### 控制端的配置
```jsonc
{
    "agents": [{
        "enable": true,
        "address": "<server ip>:<server port>",
        "password": "<server password>",

        "network": "vtcp",
        "domain": "controller",
    }],

    "portproxy": [
        { "listen": "tcp://localhost:1000", "target": "vtcp://test_agent:1000" }
    ]
}
```
> 说明：控制端agent将会监听本机1000端口，并将端口流量转发至虚拟网络vtcp中的100端口上。

## 流量加密

agent默认不会对转发的流量进行加密，网络中的数据包是以明文形式进行流动的。如果数据包需要在被监控或者不信任的网络中进行传递，可以通过添加参数实现流量的加解密。

采用“预共享密钥”方案进行点对点加解密，服务端仅仅负责流量转发，无法解密相关内容。

### 流量发起端加解密配置
```jsonc
{
    "portproxy": [{
        "listen": "tcp://localhost:1000",
        "target": "vtcp://test_agent:1000?secret=password123456"
    }]
}
```

### 流量接收端加解密配置
```jsonc
{
    "portproxy": [{
        "listen": "vtcp://test_agent:1000?secret=password123456",
        "target": "tcp://localhost:3389"
    }]
}
```