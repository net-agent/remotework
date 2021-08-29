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

## agent完整配置示例与说明
```jsonc
{
  "agents": [{
    "enable": true,

    // 通过url简化配置，url = <network>://<domain>:<password>@<address>
    "url": "txy://test:pswd-gogo@localhost:2000",
    "wsEnable": true,
    "wss": false,
    "wsPath": "/wsconn",
    
    // trust信任列表，在信任列表中的domain，可以直接进行任意端口转发
    // 配合对端的visit服务发挥作用
    "trust": {
      "enable": true,
      "whiteList": {
        "test": "1234", // domain: password
        "cmsoffice_sgz": "abcde"
      }
    }
  }, {
    "enable": true,
    "url": "local://test2:pswd-gogo@localhost:2000"
  }],

  // portproxy 端口转发示例
  "portproxy": [
    { "log": "portp-1",
      "listen": "tcp://localhost:1070",      "target": "txy://test:1070?secret=12345" },
    
    { "log": "portp-2",
      "listen": "txy://0:1070?secret=12345", "target": "local://test2:1070" }
  ],

  // visit 配合agent.trust开放指定任意端口
  "visit": [
    { "log": "visit-1", 
      "listen": "tcp://localhost:1000", "target": "txy://office_pc:pswd@localhost:3389" },

    { "log": "visit-2", 
      "listen": "tcp://localhost:1001", "target": "txy://office_pc:pswd@localhost:1001" },

    { "log": "visit-3", 
      "listen": "tcp://localhost:1002", "target": "txy://office_pc:pswd@localhost:1002" }
  ],

  // 将本机的rdp端口代理出去。windows上会读取注册表，获取rdp端口。
  "rdp": [
    { "log": "rdp-1", "listen": "txy://0:3389" },
    { "log": "rdp-2", "listen": "local://0:3389" }
  ],

  // 将本机作为socks5服务器进行网络开放
  "socks5": [
    { "log": "socks-1", "listen": "local://0:1070", "username": "", "password": "" }
  ]
}
```