# 多链路内网穿透配置协议规范 (v2.3)

> **⚠️ 状态：历史设计文档，部分内容未实现，不能直接作为当前配置手册。**
> 当前实现以 `configv2` 与 [cmd/config_example/agent.json](../cmd/config_example/agent.json) 为准。与本文的主要差异：
>
> - **文件格式**：支持 JSON/JSONC（`//` 与 `/* */` 注释，含行尾注释）与 TOML，**不支持 YAML**；顶层字段为 `links`、`tunnels`、`pprof`、`api`。未知字段会被严格拒绝。
> - **authRef / authcodeRef 语义**：二者都是**环境变量引用**（`env:NAME` 或裸 `NAME`），不是任意密钥管理器键名；变量未设置或为空时失败关闭。link 的 `authRef` 优先于明文 `auth`；tunnel 的明文 `authcode` 优先于 `authcodeRef`。
> - **via / keepalive**：flex v3.1.0 已不支持，校验分别报 `ERR_VIA_NOT_SUPPORTED`、`ERR_KEEPALIVE_NOT_SUPPORTED`；不要再把它们写成可用能力。
> - **authcode/authcodeRef**：只允许用于 `vtcp://` 端点，并且是 **URL query 参数**（`?authcode=` / `?authcodeRef=`），不是 YAML 顶层字段；物理 `tcp://` 端点携带会被 `ERR_AUTHCODE_NOT_ALLOWED` 拒绝。
> - **tunnel name**：当前实现要求**全局唯一**（`ERR_DUPLICATE_TUNNEL_NAME`），不再是“可重复的用户可读名称”；`id` 仍为 UUID 且唯一。
> - **容错**：坏 link/tunnel 逐项丢弃，其余条目保留；跨项冲突只在可用条目之间检测。
> - 文中 YAML 示例均为**历史草案**，仅用于说明设计意图，不能通过当前 `validate`；可运行示例见 §4.2。

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
  - `authRef`：SHOULD。认证密钥引用，**当前实现为环境变量引用**（`env:NAME` 或裸 `NAME`）。未设置/为空时失败关闭；与 `auth` 同时出现时以 `authRef` 为准。不支持任意“密钥管理器键名”。
  - `keepalive`：**已不支持**。flex v3.1.0 的内部心跳不可通过配置覆盖；显式配置会报 `ERR_KEEPALIVE_NOT_SUPPORTED`。
  - `via`：**未实现/已不支持**。flex v3.1.0 没有上游 relay 路由能力；显式配置会报 `ERR_VIA_NOT_SUPPORTED`。

**示例**：

- `tcp://relay.corp.com:7000?as=pc-01&auth=work-secret`
- `wss://tunnel.home.org/v2?as=macbook&authRef=HOME_RELAY_AUTH`

> 历史草案中的 `?keepalive=10`、`?via=gateway.home:7000` 现在都会被校验拒绝。

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
- `via` 已不支持（`ERR_VIA_NOT_SUPPORTED`）；不要把它写进当前配置。
- 当前支持 `as`、`auth`、`authRef` 三个查询参数；`authRef` 为环境变量引用并失败关闭。

**示例**：

- `wss://relay.office.com/ws?as=pc-01&authRef=CORP_RELAY_AUTH`

#### B. Tunnel 端点语义

适用于 `tunnels[].listen` 与 `tunnels[].target`。

- Tunnel URL **MUST** 使用端点语义：`tcp://...`、`vtcp://...` 或内置服务（如 `socks5://`）。
- Tunnel URL **MUST NOT** 携带 `via` 参数。
- Tunnel URL **MUST NOT** 使用 `http/https/ws/wss` 作为端点语义。
- 当 `listen` 或 `target` 使用 `vtcp://` 时，`authcode` 或 `authcodeRef` **MUST** 以 query 参数形式提供（`?authcode=...` / `?authcodeRef=...`）。
- `authcode`/`authcodeRef` **MUST NOT** 出现在物理 `tcp://` 端点：当前实现会报 `ERR_AUTHCODE_NOT_ALLOWED`。
- `authcodeRef` 是环境变量引用，未设置/为空时失败关闭；`authcode` 与 `authcodeRef` 同时存在时以明文 `authcode` 为准。
- `authcode` 的具体校验实现为预共享密钥；当前 `cipherconn` 只提供加密，不提供完整性认证。

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
- `link`：历史草案中的可选字段；**当前实现不支持**（未知字段会被严格拒绝）。`vtcp://` 的 `netalias` 已在 URL 中显式表达，无需额外字段。
- `comment`：MAY。人类可读说明，不参与运行；`name` 必须唯一。

#### C. 示例

> 说明：以下示例省略了 `authcode` 以突出映射方向；当前实现中 `vtcp://` 端点必须携带 `?authcode=` 或 `?authcodeRef=`。

- `net -> vnet`：`listen=tcp://127.0.0.1:13306`，`target=vtcp://db.office:3306`
- `vnet -> net`：`listen=vtcp://pc-01.office:80`，`target=tcp://127.0.0.1:3000`
- `net -> net`：`listen=tcp://127.0.0.1:8080`，`target=tcp://10.0.0.12:80`
- `vnet -> vnet`：`listen=vtcp://a.office:9000`，`target=vtcp://b.home:9000`

---

## 3. 完整参数规范

| 参数 | 适用范围 | 规范要求 | 示例 |
| --- | --- | --- | --- |
| `via` | Registration URL 查询参数 | **已不支持**；flex v3.1.0 无上游路由，报 `ERR_VIA_NOT_SUPPORTED` | ~~`?via=gateway.office:7000`~~ |
| `as` | Registration URL | 注册身份名称，**MUST** | `?as=web-node` |
| `auth` | Registration URL | 明文认证信息，MAY（生产不推荐） | `?auth=***` |
| `authRef` | Registration URL | 环境变量引用（`env:NAME` 或裸 `NAME`）；未设置时失败关闭；优先于 `auth` | `?authRef=RELAY_AUTH` |
| `authcode` | Tunnel URL（仅 `vtcp://`） | `listen` 或 `target` 为 `vtcp://` 时 **MUST** 提供；query 参数 | `?authcode=***` |
| `authcodeRef` | Tunnel URL（仅 `vtcp://`，推荐） | 环境变量引用；与 `authcode` 二选一，明文 `authcode` 优先；未设置时失败关闭 | `?authcodeRef=VTCP_SERVICE_AUTH` |
| `keepalive` | Registration URL | **已不支持**；flex v3.1.0 内部管理心跳，报 `ERR_KEEPALIVE_NOT_SUPPORTED` | ~~`?keepalive=10`~~ |

---

## 4. 配置文件设计（历史 YAML 草案；当前实现为 JSON/JSONC/TOML）

采用 **Link 定义与 Tunnel Rule 定义分离** 的结构。下面的结构图解沿用历史 YAML 写法，仅用于说明层次；**当前实现不支持 YAML**，可运行格式是 JSON/JSONC 或 TOML，字段名与层次相同。

### 4.1 配置结构图解

```yaml
# 结构示意（历史 YAML 写法，不能直接运行）
links:
  <netalias>: <Registration_URL>
  # 当前支持查询参数 as / auth / authRef；via/keepalive 已不支持

tunnels:
  - id: <uuid>        # MUST: UUID、全局唯一
    name: <string>    # MUST: 全局唯一（kernel key tunnel/<name>）
    comment: <string> # MAY: 人类可读说明，不参与运行
    listen: <Endpoint_URL>  # MUST: tcp://... 或 vtcp://...
    target: <Endpoint_or_Builtin_URL>  # MUST: tcp://... / vtcp://... / socks5://...

    # 当 listen 或 target 包含 vtcp:// 时：
    # 必须在 URL 上携带 ?authcode= 或 ?authcodeRef=
```

> 说明：`id` 主要用于配置管理（diff/merge/追踪），不是面向用户记忆的业务字段。`name` 会拼成 kernel 插件名 `tunnel/<name>`，因此当前实现要求唯一。

### 4.2 配置示例（当前 JSONC 形态）

```jsonc
{
  "links": {
    "office": "tcp://relay.corp.com:7000?as=pc-01&authRef=CORP_RELAY_AUTH",
    "home": "wss://tunnel.myhome.org/ws?as=macbook&authRef=HOME_RELAY_AUTH"
  },

  "tunnels": [
    {
      "id": "c45a8ffb-1a5f-4f78-b9ef-a0e7189bd8c1",
      "name": "access-corp-db",
      "comment": "office vnet -> local db",
      "listen": "tcp://127.0.0.1:13306",
      "target": "vtcp://db-server.office:3306?authcodeRef=CORP_DB_VTCP_AUTH"
    },
    {
      "id": "f56d9858-5d57-4a35-a7b7-3e683fe2a8ce",
      "name": "access-home-nas",
      "listen": "tcp://127.0.0.1:8080",
      "target": "vtcp://nas.home:80?authcodeRef=HOME_NAS_VTCP_AUTH"
    },
    {
      "id": "ca6fb919-9e03-4f16-a9ee-f3f194ab17a4",
      "name": "share-web-project",
      "listen": "vtcp://pc-01.office:80?authcodeRef=WEB_SHARE_VTCP_AUTH",
      "target": "tcp://127.0.0.1:3000"
    },
    {
      "id": "e4689cb7-b125-4807-a2c7-d8f52703f31b",
      "name": "provide-socks-proxy",
      "listen": "vtcp://macbook.home:1080?authcodeRef=SOCKS_PROXY_VTCP_AUTH",
      "target": "socks5://"
    },
    {
      "id": "76d0909f-236c-4ef1-bf14-e5d669f27595",
      "name": "local-net-relay",
      "listen": "tcp://127.0.0.1:18080",
      "target": "tcp://10.0.0.12:80"
    }
  ]
}
```

> `authRef`/`authcodeRef` 指向的环境变量必须在启动前设置，否则对应 link/tunnel 会因解析失败被丢弃。示例省略了 `pprof`/`api` 段，需要时按 `cmd/config_example/agent.json` 添加。

---

## 5. 归一化示例（示意，非配置文件格式）

### 5.1 `vtcp` 端点输入

```yaml
target: "vtcp://db.office:3306?authcodeRef=DB_VTCP_AUTH"
```

归一化后（示意）：

```yaml
endpoint:
  kind: vnet
  scheme: vtcp
  address: "db.office:3306"
  authcodeRef: "DB_VTCP_AUTH"
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

### 5.3 Registration URL（authRef）输入

```yaml
link: "wss://relay.office.com/ws?as=pc-01&authRef=CORP_RELAY_AUTH"
```

归一化后（示意）：

```yaml
link:
  scheme: wss
  relayURL: "wss://relay.office.com/ws"
  as: "pc-01"
  authRef: "CORP_RELAY_AUTH"   # 环境变量引用，未设置时 fail closed
```

> 历史草案中的 `via` 已不再解析；带 `via` 的 link 会被 `ERR_VIA_NOT_SUPPORTED` 丢弃。

---

## 6. 解析与校验规则（对应当前实现）

1. `links` 的 Key（`netalias`）**MUST** 唯一，且仅允许 `[a-zA-Z0-9-]`。
2. `tunnels[].id` **MUST** 存在、符合 UUID 格式、在同一文件内唯一。
3. `tunnels[].name` **MUST** 存在且**全局唯一**（kernel key 为 `tunnel/<name>`；重复报 `ERR_DUPLICATE_TUNNEL_NAME`）。
4. `tunnels[].listen` 与 `tunnels[].target` **MUST** 存在。
5. `listen`/`target` 若为端点 URL：
   - `tcp://`/`tcp4://`/`tcp6://` 视为 net endpoint；
   - `vtcp://` 视为 vnet endpoint；
   - `socks5://` 只能作为 target。
6. 任一 `vtcp://host.alias:port` 中的 `alias` **MUST** 在可用的 `links` 中存在。
7. `tunnels[].listen` 与 `tunnels[].target` **MUST NOT** 携带 `via` 查询参数（`ERR_TUNNEL_VIA_NOT_ALLOWED`）。
8. `listen`/`target` **MUST NOT** 使用 `http/https/ws/wss` 作为端点语义；未知 scheme 同样被拒绝。
9. Registration URL 的 `via` **已不支持**，显式配置报 `ERR_VIA_NOT_SUPPORTED`；不存在“`host.alias:port` 格式校验”这一实现。
10. 当 `listen` 或 `target` 含 `vtcp://` 时，`authcode` 或 `authcodeRef` **MUST** 以 query 参数二选一提供（`ERR_VTCP_AUTH_REQUIRED`）。
11. 物理端点（`tcp://` 等）**MUST NOT** 携带 `authcode`/`authcodeRef`（`ERR_AUTHCODE_NOT_ALLOWED`）；`authcodeRef` 未设置/为空时 fail closed。
12. `keepalive` **已不支持**，显式配置报 `ERR_KEEPALIVE_NOT_SUPPORTED`；flex v3.1.0 内部管理心跳。
13. 同一实例内，`listen` 端点 **MUST NOT** 冲突（物理端点按 `scheme + host + port`，vtcp 端点按 `scheme + alias + host + port`）。
14. 非回环物理监听 + `socks5://` target 时 **MUST** 提供用户名和密码（否则是开放代理，`ERR_SOCKS5_AUTH_REQUIRED`）。
15. `pprof.listen` **MUST** 为回环地址；`api.listen` 非回环时 **MUST** 显式设置 `api.allowNonLoopback=true`（`ERR_API_NON_LOOPBACK`）。
16. 未知字段会在文件解析阶段被拒绝（整文件不可解析）；通过解析后，坏 link/tunnel 逐项丢弃，其余条目继续加载。

---

## 7. 非法配置示例与推荐报错

> 以下片段为便于阅读的历史 YAML 写法；错误码与消息对应当前 `configv2` 实现，实际配置请使用 JSON/JSONC 或 TOML。

### 7.1 Tunnel URL 不允许 `via`

```yaml
target: "vtcp://db.office:3306?via=gateway.home:7000"
```

推荐报错：

`ERR_TUNNEL_VIA_NOT_ALLOWED: via is only allowed in registration URL`

### 7.2 Registration URL 不再支持 `via`

```yaml
links:
  office: "wss://relay.office.com/ws?as=pc-01&via=gateway.home"
```

推荐报错：

`ERR_VIA_NOT_SUPPORTED: option 'via' is not supported by flex v3.1.0; remove it (upstream relay routing is not implemented)`

### 7.3 `vtcp` 缺少 `authcode` / `authcodeRef`

```yaml
- id: "c45a8ffb-1a5f-4f78-b9ef-a0e7189bd8c1"
  name: "access-corp-db"
  listen: "tcp://127.0.0.1:13306"
  target: "vtcp://db.office:3306"
```

推荐报错：

`ERR_VTCP_AUTH_REQUIRED: authcode or authcodeRef is required for vtcp endpoint`

### 7.4 `vtcp` 使用不存在的 alias

```yaml
listen: "tcp://127.0.0.1:18080"
target: "vtcp://api.prod:443?authcode=***"
```

推荐报错：

`ERR_ALIAS_NOT_FOUND: alias "prod" not found in usable links`

### 7.5 重复 UUID

```yaml
- id: "c45a8ffb-1a5f-4f78-b9ef-a0e7189bd8c1"
  name: "rule-a"
- id: "c45a8ffb-1a5f-4f78-b9ef-a0e7189bd8c1"
  name: "rule-b"
```

推荐报错：

`ERR_DUPLICATE_TUNNEL_ID: tunnel id "...": must be unique (...)`

### 7.6 重复 tunnel name

```yaml
- id: "c45a8ffb-1a5f-4f78-b9ef-a0e7189bd8c1"
  name: "rule-a"
- id: "f56d9858-5d57-4a35-a7b7-3e683fe2a8ce"
  name: "rule-a"
```

推荐报错：

`ERR_DUPLICATE_TUNNEL_NAME: tunnel name "rule-a" must be unique (kernel key tunnel/<name>; ...)`

### 7.7 物理端点携带 authcode

```yaml
- id: "c45a8ffb-1a5f-4f78-b9ef-a0e7189bd8c1"
  name: "plain-tcp"
  listen: "tcp://127.0.0.1:13306?authcode=***"
  target: "tcp://10.0.0.12:80"
```

推荐报错：

`ERR_AUTHCODE_NOT_ALLOWED: authcode/authcodeRef is only supported on vtcp endpoints; physical endpoints are not encrypted`

### 7.8 显式 keepalive

```yaml
links:
  office: "tcp://relay.corp.com:7000?as=pc-01&keepalive=10"
```

推荐报错：

`ERR_KEEPALIVE_NOT_SUPPORTED: explicit keepalive 10 is not supported by flex v3.1.0; use the runtime default 15`
