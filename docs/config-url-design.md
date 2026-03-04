# 多链路内网穿透配置协议规范 (v2.3)

## 0. 规范术语

本文档使用以下规范级别术语：

- **MUST**：必须满足，否则视为不兼容实现。
- **MUST NOT**：绝对禁止。
- **SHOULD**：推荐满足；如不满足，必须有明确理由。
- **MAY**：可选实现。

---

## 1. 核心概念与设计原则

### 1.1 两个关键定义

1. **Link（链路）**：节点与中继服务（Relay）之间的物理连接。
   - 每个链路在本地配置文件中拥有唯一 `alias`（别名）。
   - `alias` 仅在本地有效，用于隔离不同的虚拟网络空间。

2. **Addressing（寻址）**：在虚拟网络中定位某个节点或节点端口。
   - **节点标识**：`vhostname.netalias`
   - **节点端点**：`vhostname.netalias:vport`
   - 所有虚拟寻址 **MUST** 显式包含 `netalias`，**MUST NOT** 省略。

### 1.2 设计原则（端点显式、内部归一）

- 用户配置层通过显式 scheme 区分端点类型：`tcp://`（net）与 `vtcp://`（vnet）。
- 解析器在加载配置后，**MUST** 归一化为统一的内部表示（Canonical Form）。
- 执行层与路由层 **MUST** 仅依赖归一化结构，不直接依赖用户原始写法。

---

## 2. URL 方案详解

### 2.1 链路注册 URL（Registration URL）

用于描述如何连接中继服务。此 URL 不包含 `alias`，`alias` 由配置文件 `links` 的 Key 决定。

- **格式**：`[scheme]://[relay_host]:[port]?[params]`
- **参数**：
  - `as`：**MUST**。节点在该链路虚拟网络中的唯一名称（`vhostname`）。
  - `auth`：MAY。明文认证密钥（生产环境不推荐）。
  - `authRef`：SHOULD。认证密钥引用（环境变量名、密钥管理器键名等）。
  - `keepalive`：MAY。心跳间隔，单位秒（整数），默认 `15`。
  - `via`：MAY。上游中转端点，格式 `host.alias:port`。仅用于 Registration URL 的链路建立语义。

**示例**：

- `tcp://relay.corp.com:7000?as=pc-01&auth=work-secret`
- `wss://tunnel.home.org/v2?as=macbook&authRef=HOME_RELAY_AUTH&keepalive=10`
- `wss://relay.office.com/ws?as=pc-01&authRef=CORP_RELAY_AUTH&via=gateway.home:7000`

---

### 2.2 虚拟网络寻址（Virtual Addressing）

用于在 `listen` 或 `target` 中引用虚拟节点。

- **节点端点语法**：`[vhostname].[netalias]:[vport]`
- `vhostname`：目标节点注册名（`as` 的值）。
- `netalias`：本地配置文件中定义的链路别名。
- `vport`：目标节点监听的虚拟端口。

**`vtcp` 定义（新增）**：

- `vtcp://` 表示虚拟网络端点（vnet endpoint）。
- `tcp://` 表示物理网络端点（net endpoint）。
- 虚拟端点 **SHOULD** 使用 `vtcp://[vhostname].[netalias]:[vport]` 明确表达，避免歧义。

**URL 表示约定**：

- 在 `listen`/`target` 中引用虚拟节点时，使用：`vtcp://[vhostname].[netalias]:[vport]`

---

### 2.3 端点与远程 URL 语义约束

#### A. Registration URL 远程语义

适用于 `links.<alias>` 的 Registration URL（例如 `wss://...`）。

- Registration URL **MUST** 是完整 URL（包含协议、主机、可选路径与查询）。
- `via` 若存在，**MUST** 满足 `host.alias:port`。
- `via` 仅用于链路建立阶段（registration），不参与 tunnel endpoint 语义。

**示例**：

- `wss://relay.office.com/ws?as=pc-01&authRef=CORP_RELAY_AUTH&via=gateway.home:7000`

#### B. Tunnel 端点语义

适用于 `tunnels[].listen` 与 `tunnels[].target`。

- Tunnel URL **MUST** 使用端点语义：`tcp://...`、`vtcp://...` 或内置服务（如 `socks5://`）。
- Tunnel URL **MUST NOT** 携带 `via` 参数。
- Tunnel URL **MUST NOT** 使用 `http/https/ws/wss` 作为目标端点语义。
- 当 `listen` 或 `target` 使用 `vtcp://` 时，`authcode` **MUST** 提供（推荐通过 `authcodeRef` 引用）。
- `authcode` 的具体校验实现可为明文比对或 challenge-response；实现层 **SHOULD** 优先 challenge-response。

#### C. 归一化要求

解析器加载配置后：

- **MUST** 将不同输入写法统一映射为同一内部结构。
- **MUST** 在执行路由时仅使用归一化结果。
- **SHOULD** 提供“导出规范化配置”能力（便于审计与排障）。

---

### 2.4 隧道映射配置（Tunnel Mapping）

隧道配置描述端点之间的数据流。`tunnel.type` 字段已移除，路由方向由 `listen` 与 `target` 的 URL 类型自动推导。

#### A. 端点类型推导规则

- `tcp://...` => 物理网络端点（net）
- `vtcp://...` => 虚拟网络端点（vnet）

由此可推导四种合法映射：

- `net -> vnet`
- `vnet -> net`
- `net -> net`
- `vnet -> vnet`

#### B. 配置字段

- `listen`：**MUST**。监听端点 URL（`tcp://` 或 `vtcp://`）。
- `target`：**MUST**。目标端点 URL（`tcp://`、`vtcp://` 或内置服务）。
- `link`：MAY。当 `listen` 为虚拟监听（`vtcp://`）时，`netalias` 已在 URL 中显式表达，通常无需额外 `link` 字段。

#### C. 示例

- `net -> vnet`：`listen=tcp://127.0.0.1:13306`，`target=vtcp://db.office:3306`
- `vnet -> net`：`listen=vtcp://pc-01.office:80`，`target=tcp://127.0.0.1:3000`
- `net -> net`：`listen=tcp://127.0.0.1:8080`，`target=tcp://10.0.0.12:80`
- `vnet -> vnet`：`listen=vtcp://a.office:9000`，`target=vtcp://b.home:9000`

---

## 3. 完整参数规范

| 参数 | 适用范围 | 规范要求 | 示例 |
| --- | --- | --- | --- |
| `via` | Registration URL 查询参数 | `host.alias:port`；仅用于链路建立阶段（registration） | `?via=gateway.office:7000` |
| `as` | Registration URL | 注册身份名称，**MUST** | `?as=web-node` |
| `auth` | Registration URL | 明文认证信息，MAY（生产不推荐） | `?auth=123456` |
| `authRef` | Registration URL | 认证引用，SHOULD | `?authRef=RELAY_AUTH` |
| `authcode` | Tunnel Rule（含 `vtcp://` 时） | `listen` 或 `target` 为 `vtcp://` 时 **MUST** 提供；用于 vtcp 服务接入校验 | `authcode: "my-vtcp-code"` |
| `authcodeRef` | Tunnel Rule（推荐） | `authcode` 的引用形式，SHOULD；与 `authcode` 二选一 | `authcodeRef: "VTCP_SERVICE_AUTH"` |
| `keepalive` | Registration URL | 整数秒，默认 `15` | `?keepalive=10` |

---

## 4. 配置文件设计（YAML）

采用 **Link 定义与 Tunnel Rule 定义分离** 的结构。

### 4.1 配置结构图解

```yaml
# 1) 定义所有上游链路
links:
  <netalias>: <Registration_URL>
  # Registration URL 支持 via 查询参数，用于链路建立阶段

# 2) 定义隧道规则
tunnels:
  - id: <uuid>        # MUST: 全局唯一、不可变（机器标识）
    name: <string>    # MUST: 可重复（用户可读名称）
    listen: <Endpoint_URL>  # MUST: tcp://... 或 vtcp://...
    target: <Endpoint_or_Builtin_URL>  # MUST: tcp://... / vtcp://... / socks5://...

    # 当 listen 或 target 包含 vtcp:// 时：
    # authcode 或 authcodeRef MUST 二选一提供
```

> 说明：`id` 主要用于配置管理（diff/merge/追踪），不是面向用户记忆的业务字段。

### 4.2 完整配置示例

```yaml
links:
  office: "tcp://relay.corp.com:7000?as=pc-01&authRef=CORP_RELAY_AUTH&keepalive=15"
  home: "wss://tunnel.myhome.org/ws?as=macbook&authRef=HOME_RELAY_AUTH&keepalive=10&via=gateway.office:7000"

tunnels:
  - id: "c45a8ffb-1a5f-4f78-b9ef-a0e7189bd8c1"
    name: "access-corp-db"
    listen: "tcp://127.0.0.1:13306"
    target: "vtcp://db-server.office:3306"
    authcodeRef: "CORP_DB_VTCP_AUTH"

  - id: "f56d9858-5d57-4a35-a7b7-3e683fe2a8ce"
    name: "access-home-nas"
    listen: "tcp://127.0.0.1:8080"
    target: "vtcp://nas.home:80"
    authcodeRef: "HOME_NAS_VTCP_AUTH"

  - id: "58f7f892-c962-4505-95e1-c643f1814e63"
    name: "bridge-to-gateway-service"
    listen: "tcp://127.0.0.1:2222"
    target: "vtcp://gateway.office:7000"
    authcodeRef: "GATEWAY_VTCP_AUTH"

  - id: "ca6fb919-9e03-4f16-a9ee-f3f194ab17a4"
    name: "share-web-project"
    listen: "vtcp://pc-01.office:80"
    target: "tcp://127.0.0.1:3000"
    authcodeRef: "WEB_SHARE_VTCP_AUTH"

  - id: "e4689cb7-b125-4807-a2c7-d8f52703f31b"
    name: "provide-socks-proxy"
    listen: "vtcp://macbook.home:1080"
    target: "socks5://"
    authcodeRef: "SOCKS_PROXY_VTCP_AUTH"

  - id: "76d0909f-236c-4ef1-bf14-e5d669f27595"
    name: "local-net-relay"
    listen: "tcp://127.0.0.1:18080"
    target: "tcp://10.0.0.12:80"

  - id: "b4fbb117-82d3-4fd4-b8c1-f95d9ed4f584"
    name: "cross-vnet-bridge"
    listen: "vtcp://a.office:9000"
    target: "vtcp://b.home:9000"
    authcodeRef: "CROSS_VNET_BRIDGE_AUTH"
```

---

## 5. 归一化示例（推荐）

### 5.1 `vtcp` 端点输入

```yaml
target: "vtcp://db.office:3306"
```

归一化后（示意）：

```yaml
endpoint:
  kind: vnet
  scheme: vtcp
  address: "db.office:3306"
```

### 5.2 `tcp` 端点输入

```yaml
target: "tcp://127.0.0.1:5432"
```

归一化后（示意）：

```yaml
endpoint:
  kind: net
  scheme: tcp
  address: "127.0.0.1:5432"
```

### 5.3 Registration URL（含 via）输入

```yaml
link: "wss://relay.office.com/ws?as=pc-01&authRef=CORP_RELAY_AUTH&via=gateway.home:7000"
```

归一化后（示意）：

```yaml
link:
  scheme: wss
  relayURL: "wss://relay.office.com/ws"
  as: "pc-01"
  authRef: "CORP_RELAY_AUTH"
  via: "gateway.home:7000"
```

---

## 6. 解析与校验规则（最小必选集）

1. `links` 的 Key（`netalias`）**MUST** 唯一，且仅允许 `[a-zA-Z0-9-]`。
2. `tunnels[].id` **MUST** 存在、符合 UUID 格式、在同一文件内唯一。
3. `tunnels[].name` **MUST** 存在，允许重复。
4. `tunnels[].listen` 与 `tunnels[].target` **MUST** 存在。
5. `listen`/`target` 若为端点 URL：
   - `tcp://` 视为 net endpoint；
   - `vtcp://` 视为 vnet endpoint。
6. 任一 `vtcp://host.alias:port` 中的 `alias` **MUST** 在 `links` 中存在。
7. `tunnels[].listen` 与 `tunnels[].target` **MUST NOT** 携带 `via` 查询参数。
8. `tunnels[].target` **MUST NOT** 使用 `http/https/ws/wss` 作为端点语义。
9. Registration URL 的 `via`（若存在）**MUST** 满足 `host.alias:port`（端口必填）。
10. 当 `listen` 或 `target` 含 `vtcp://` 时，`authcode` 或 `authcodeRef` **MUST** 二选一提供。
11. `authcode` 与 `authcodeRef` **MUST NOT** 同时缺失；`authcodeRef` 为推荐形式（SHOULD）。
12. `keepalive` 为整数秒，缺省值 `15`，建议最小值 `5`。
13. 同一实例内，`listen` 端点 **MUST NOT** 冲突（同 `scheme + host + port` 视为冲突）。

---

## 7. 非法配置示例与推荐报错

### 7.1 Tunnel URL 不允许 `via`

```yaml
target: "vtcp://db.office:3306?via=gateway.home:7000"
```

推荐报错：

`ERR_TUNNEL_VIA_NOT_ALLOWED: via is only allowed in registration URL`

### 7.2 Registration URL 的 `via` 缺少端口

```yaml
links:
  office: "wss://relay.office.com/ws?as=pc-01&via=gateway.home"
```

推荐报错：

`ERR_INVALID_VIA: registration via must match host.alias:port`

### 7.3 `vtcp` 缺少 `authcode` / `authcodeRef`

```yaml
- id: "rule-1"
  name: "access-corp-db"
  listen: "tcp://127.0.0.1:13306"
  target: "vtcp://db.office:3306"
```

推荐报错：

`ERR_VTCP_AUTH_REQUIRED: authcode or authcodeRef is required when vtcp endpoint is used`

### 7.4 `vtcp` 使用不存在的 alias

```yaml
listen: "tcp://127.0.0.1:18080"
target: "vtcp://api.prod:443"
```

推荐报错：

`ERR_ALIAS_NOT_FOUND: alias 'prod' not found in links`

### 7.5 重复 UUID

```yaml
- id: "c45a8ffb-1a5f-4f78-b9ef-a0e7189bd8c1"
  name: "rule-a"
- id: "c45a8ffb-1a5f-4f78-b9ef-a0e7189bd8c1"
  name: "rule-b"
```

推荐报错：

`ERR_DUPLICATE_TUNNEL_ID: tunnel id must be unique`
