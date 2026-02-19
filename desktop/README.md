# Remotework Desktop

Remotework 的桌面客户端，基于 Tauri 2 + React 构建。作为 Go agent 的图形化宿主，提供可视化的配置管理、网络状态监控和系统托盘后台运行能力。

## 架构概览

```
┌─────────────────────────────────────────────┐
│           Tauri 窗口 (React + TypeScript)     │
│                                              │
│  ┌─ fetch / WebSocket ──→ Go Agent API ────┐ │
│  │                       (127.0.0.1:port)  │ │
│  └─ Tauri IPC ──→ Rust 后端 ──────────────┘ │
│                    ├─ sidecar.rs  进程管理    │
│                    ├─ config.rs   配置读写    │
│                    └─ tray.rs     系统托盘    │
└─────────────────────────────────────────────┘
```

核心设计原则：

- **Go agent 作为 sidecar**：应用启动时 spawn Go 二进制，通过 HTTP/WebSocket 通信
- **Rust 只做三件事**：进程管理、文件 I/O、系统托盘 — 不中转 API 请求
- **React 直连 agent API**：前端通过 `127.0.0.1` 直接与 agent 通信，实时获取状态

## 环境要求

| 工具 | 最低版本 | 说明 |
|------|---------|------|
| Node.js | 18+ | 前端构建 |
| pnpm | 9+ | 包管理 |
| Rust | 1.77+ | Tauri 后端编译 |
| Go | 1.21+ | 编译 agent sidecar |

macOS 额外需要 Xcode Command Line Tools；Linux 需要 `libwebkit2gtk-4.1-dev`、`libappindicator3-dev` 等系统库，详见 [Tauri 官方文档](https://v2.tauri.app/start/prerequisites/)。

## 快速开始

```bash
# 1. 安装前端依赖
cd desktop
pnpm install

# 2. 编译 Go sidecar（当前平台）
bash build-sidecar.sh

# 3. 启动开发模式（Vite HMR + Rust 热编译）
pnpm tauri dev
```

## 构建发布包

```bash
# 编译全平台 sidecar
bash build-sidecar.sh all

# 构建安装包（.dmg / .msi / .AppImage）
pnpm tauri build
```

产物位于 `src-tauri/target/release/bundle/`。

## 技术栈

| 层 | 选型 | 用途 |
|---|---|---|
| 桌面框架 | Tauri 2.0 | 原生窗口、托盘、进程管理 |
| 前端 | React 18 + TypeScript | UI 渲染 |
| UI 组件 | shadcn/ui + Tailwind CSS 4 | 组件库 |
| 状态管理 | Zustand | 轻量响应式状态 |
| 图标 | Lucide React | 图标集 |
| Tauri 插件 | shell（sidecar）、dialog（文件选择）、process（退出） | 原生能力 |

## 项目结构

```
desktop/
├── build-sidecar.sh              # Go sidecar 交叉编译脚本
├── index.html                    # 入口 HTML
├── package.json
├── components.json               # shadcn/ui 配置
├── vite.config.ts
├── tsconfig.json
│
├── src/                          # React 前端
│   ├── main.tsx                  # 入口
│   ├── App.tsx                   # 根组件（路由 + 全局 Provider）
│   │
│   ├── lib/                      # 核心库
│   │   ├── types.ts              # API DTO 类型（镜像 Go api/dto.go）
│   │   ├── config-types.ts       # 配置类型（镜像 Go agent/config.go）
│   │   ├── api.ts                # REST 客户端
│   │   ├── ws.ts                 # WebSocket 客户端（自动重连）
│   │   └── utils.ts              # shadcn 工具函数
│   │
│   ├── stores/                   # Zustand 状态管理
│   │   ├── agent-store.ts        # 运行时状态：networks, services, streams
│   │   ├── profile-store.ts      # Profile CRUD + 配置编辑
│   │   └── ui-store.ts           # UI 状态：页面、展开、弹窗
│   │
│   ├── hooks/                    # React Hooks
│   │   ├── use-sidecar.ts        # Tauri sidecar 生命周期
│   │   ├── use-agent-api.ts      # REST API 调用封装
│   │   └── use-websocket.ts      # WebSocket 连接管理
│   │
│   ├── pages/
│   │   ├── MainPage.tsx          # 主页：网络卡片列表
│   │   └── SettingsPage.tsx      # 设置：日志级别、Profile 管理
│   │
│   └── components/
│       ├── layout/
│       │   ├── AppShell.tsx      # 整体布局（Header + Content + StatusBar）
│       │   └── StatusBar.tsx     # 底部状态栏
│       ├── network/
│       │   ├── NetworkCard.tsx   # 网络卡片（折叠态）
│       │   ├── NetworkCardExpanded.tsx  # 网络卡片（展开态 + 服务列表）
│       │   ├── NetworkForm.tsx   # 添加/编辑网络对话框
│       │   └── EmptyState.tsx    # 空白引导
│       ├── service/
│       │   ├── ServiceRow.tsx    # 服务行
│       │   └── ServiceForm.tsx   # 添加/编辑服务（端口转发/SOCKS5/RDP）
│       ├── profile/
│       │   ├── ProfileSwitcher.tsx  # 顶部 Profile 下拉切换
│       │   └── ProfileManager.tsx   # Profile 列表管理
│       ├── shared/
│       │   ├── StatusDot.tsx     # 状态指示点
│       │   ├── ConfirmDialog.tsx # 确认对话框
│       │   └── UrlField.tsx      # URL 编辑器（结构化 ↔ 高级模式）
│       └── ui/                   # shadcn/ui 组件（自动生成）
│
└── src-tauri/                    # Rust 后端
    ├── Cargo.toml
    ├── tauri.conf.json           # Tauri 配置（窗口、CSP、sidecar）
    ├── capabilities/
    │   └── default.json          # 权限声明
    ├── binaries/                 # Sidecar 二进制（构建脚本填充，gitignore）
    ├── icons/                    # 应用图标（各平台格式）
    └── src/
        ├── main.rs               # 入口
        ├── lib.rs                # 插件注册、命令导出
        ├── sidecar.rs            # Go 进程生命周期管理
        ├── config.rs             # Profile 文件 CRUD
        └── tray.rs               # 系统托盘
```

## 核心概念

### Profile

Profile 是一份完整的 agent 配置，对应一个 JSON 文件。用户可以创建多个 Profile（如"办公室"、"家里"），在不同场景间快速切换。

存储位置：`$APPDATA/remotework/profiles/`（macOS 为 `~/Library/Application Support/remotework/profiles/`）

Profile 的 JSON 格式与 Go agent 的 Config 结构完全一致：

```json
{
  "agents": [
    {
      "name": "office",
      "protocol": "vtcp",
      "address": "relay.example.com:8080",
      "domain": "my-laptop",
      "password": "secret123"
    }
  ],
  "portproxy": [
    {
      "listen": "tcp://127.0.0.1:13389",
      "target": "vtcp://remote-pc:secret@relay.example.com:8080",
      "log": "rdp-tunnel"
    }
  ],
  "socks5": [],
  "rdp": [],
  "api": { "enable": true, "listen": "127.0.0.1:0", "pollInterval": 5 }
}
```

### Sidecar 生命周期

1. 用户选择 Profile → Rust 读取配置 JSON
2. 自动分配可用端口写入 `api.listen`，确保 `api.enable = true`
3. 写入 `$APPDATA/remotework/active-config.json`
4. Spawn Go 二进制：`remotework -mode agent -c <config_path>`
5. 轮询 `GET /api/v1/status` 最多 3 秒确认启动
6. 返回 API 端口 → 前端建立 HTTP + WebSocket 连接

停止时先发送 `POST /api/v1/stop`（优雅关闭），2 秒后仍存活则 kill。

### 实时状态更新

前端通过 WebSocket 订阅 4 种事件，增量更新本地状态：

| 事件 | 说明 |
|------|------|
| `network.state` | 网络连接状态变化（online/offline/connecting） |
| `service.status` | 服务状态变化（running/stopped） |
| `stream.update` | 活跃连接的打开/关闭 |
| `hub.stopped` | Agent 停止 |

WebSocket 断线后自动指数退避重连（1s → 30s）。

### 系统托盘

- 关闭窗口 → 隐藏到托盘（不退出）
- 点击托盘图标 → 恢复窗口
- 右键菜单：显示窗口 / 退出

## 开发指南

### 前端开发

前端使用 Vite 开发服务器，支持 HMR 热更新。

```bash
# 仅启动前端（不启动 Tauri 窗口，用浏览器调试）
pnpm dev
# 浏览器访问 http://localhost:1420
```

路径别名 `@/` 映射到 `src/`，在 `tsconfig.json` 和 `vite.config.ts` 中同步配置。

#### 添加 shadcn/ui 组件

```bash
pnpm dlx shadcn@latest add <component-name>
```

组件会生成到 `src/components/ui/`，可直接修改。

#### 状态管理约定

三个 Zustand store 职责分明：

- **agent-store**：只存运行时数据（来自 API/WebSocket），不持久化
- **profile-store**：配置数据，通过 Tauri IPC 调用 Rust 命令读写文件
- **ui-store**：纯 UI 状态（当前页面、展开的卡片、弹窗开关），不涉及业务逻辑

#### 类型同步

`lib/types.ts` 和 `lib/config-types.ts` 手动镜像 Go 的结构体定义。修改 Go 侧 DTO 后需同步更新 TypeScript 类型。

关键映射：
- `types.ts` ← `api/dto.go`（NetworkStateDTO, ServiceStateDTO, StreamStateDTO）
- `config-types.ts` ← `agent/config.go`（AgentInfo, PortproxyInfo, Socks5Info, RDPInfo）

### Rust 后端开发

```bash
# 类型检查（快速）
cd src-tauri && cargo check

# 完整编译
cd src-tauri && cargo build
```

#### 添加 Tauri 命令

1. 在对应模块（`config.rs` / `sidecar.rs`）中添加 `#[tauri::command]` 函数
2. 在 `lib.rs` 的 `invoke_handler` 中注册
3. 如需新权限，更新 `capabilities/default.json`
4. 前端通过 `invoke<ReturnType>("command_name", { args })` 调用

#### Sidecar 二进制命名

Tauri 要求 sidecar 按目标三元组命名：

```
remotework-aarch64-apple-darwin        # macOS ARM
remotework-x86_64-apple-darwin         # macOS Intel
remotework-x86_64-pc-windows-msvc.exe  # Windows
remotework-x86_64-unknown-linux-gnu    # Linux
```

`build-sidecar.sh` 会自动处理命名，开发时只需运行 `bash build-sidecar.sh` 编译当前平台。

### CSP 配置

`tauri.conf.json` 中的 CSP 必须允许前端访问本地 agent API：

```
connect-src 'self' http://127.0.0.1:* ws://127.0.0.1:*
```

如果遇到网络请求被拦截，检查此配置。

## 使用说明

### 首次使用

1. 启动应用，点击右上角 Profile 下拉 → "新建 Profile"
2. 输入名称（如"办公室"），创建后自动激活
3. 点击"添加网络"，填写中继服务器信息：
   - **名称**：自定义标识（如 `office-relay`）
   - **协议**：`vtcp`（Flex 虚拟网络）或 `ws`（WebSocket）
   - **服务器地址**：中继服务器的 `host:port`
   - **域名**：本机在虚拟网络中的唯一标识
   - **密码**：连接密码（可选）
4. 网络卡片变绿表示连接成功
5. 展开网络卡片 → "添加服务"，配置端口转发等

### 配置端口转发

最常见的场景：通过中继服务器访问远程机器的端口。

1. 展开已连接的网络卡片 → 添加服务 → 选择"端口转发"
2. 填写：
   - **监听地址**：本地监听，如 `tcp://127.0.0.1:13389`
   - **目标地址**：远程端口，如 `vtcp://remote-pc:secret@relay:8080`
3. 保存后点击"重启 Agent"使配置生效
4. 连接 `127.0.0.1:13389` 即可访问远程机器的对应端口

### 配置 SOCKS5 代理

1. 添加服务 → 选择"SOCKS5"
2. 填写监听地址（如 `tcp://127.0.0.1:1080`）和可选的用户名/密码
3. 重启后，将浏览器或应用的代理设置指向 `127.0.0.1:1080`

### 配置 RDP 快捷方式

1. 添加服务 → 选择"RDP"
2. 只需填写本地监听端口，目标自动指向 `tcp://localhost:3389`
3. 适用于远程桌面场景的快速配置

### Profile 管理

- **切换**：右上角下拉菜单选择，会自动停止当前 agent 并用新配置重启
- **导入**：设置页 → Profile 管理 → 导入，支持 JSON（含 `//` 注释）格式
- **导出**：点击 Profile 旁的下载图标，导出为标准 JSON

### 配置变更

当前 agent 不支持热重载。编辑配置后，设置页会显示"重启以应用更改"按钮，点击即可重启 agent 使新配置生效。

## 常见问题

**Q: 关闭窗口后应用还在运行吗？**

是的，关闭窗口只是隐藏到系统托盘，agent 继续运行。点击托盘图标可恢复窗口，右键托盘选择"退出"才会真正关闭。

**Q: 端口被占用怎么办？**

应用会自动为 agent API 分配可用端口，不会冲突。如果服务配置的监听端口被占用，agent 日志会报错，需要修改配置中的监听端口。

**Q: 如何查看 agent 日志？**

设置页可以调整日志级别（debug/info/warn/error）。agent 的 stdout/stderr 输出会打印到 Tauri 的开发者控制台（开发模式下可见）。

**Q: 支持 TOML 配置吗？**

导入时支持 JSON（含 `//` 注释）。应用内编辑和保存始终使用标准 JSON 格式。
