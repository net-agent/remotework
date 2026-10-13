# CLAUDE.md (desktop)

Remotework 桌面客户端：Tauri 2 + React 18 + TypeScript。架构、环境要求、构建发布、开发指南的完整说明见 [README.md](README.md)，本文件只放常用命令和协作规则。

## 常用命令

```bash
pnpm install              # 安装前端依赖
bash build-sidecar.sh     # 编译当前平台的 Go sidecar（输出到 src-tauri/remotework-<target-triple>）
pnpm dev                  # 仅前端 Vite HMR（浏览器调试）
pnpm test                 # vitest
pnpm tauri dev            # 完整开发模式（beforeDevCommand 自动构建 sidecar）
pnpm tauri build          # 发布安装包
```

## Sidecar 管理（Host ↔ Go sidecar 协议）

协议完整规范见仓库根 [docs/tauri-sidecar-protocol.md](../docs/tauri-sidecar-protocol.md)，Host 侧实现在 `src-tauri/src/sidecar.rs`：

- **启动**：注入 `api.enable=true`、`api.listen="127.0.0.1:0"` 写入 `active-config.json`（端口由 sidecar 自选，Host 不预分配）；先 `-mode validate` 预校验（非零退出即失败），再以 `-mode agent -c <path> -log-format=json` spawn，token（32 字节随机 hex）仅经 env `REMOTEWORK_API_TOKEN` 传入。
- **握手**：以 5s 超时等 stdout 控制行——`ready`（从 `api` 字段解析实际端口）/ `fatal` / 进程先退出，三选一；不轮询 `/api/v1/status`，不猜端口。sidecar `protocol` 高于 Host 支持版本（当前 1）时报错提示升级。
- **日志**：stdout/stderr 逐行解析为结构化 `sidecar-log` 事件 `{source, level, module?, msg?, line}`（`line` 保留原始行文本，前端在 `msg` 缺失时回退展示 `line`）；stderr 维持最近 10 行尾部缓冲，用于启动失败错误信息。
- **退出感知**：进程 Terminated 时清理 child/port/token，emit `sidecar-exited` 事件 `{code, intended}`（`intended` 区分正常停止与崩溃；前端在 `!intended && code!==0` 时 toast 提示异常退出；退出码语义：0 正常、1 配置失败、2 API bind 失败）。
- **停止**：`POST /api/v1/stop` 带 `Authorization: Bearer <token>`，等 Terminated 事件（5s 超时）后强杀，禁止固定 sleep 兜底。
- **前端凭证**：经 `get_api_endpoint() -> { port, token }` 获取（agent 未运行会报错）；`get_agent_port` 已删除。

## 前端协议适配（src/）

- **platform 层**（`src/lib/platform/`）：`PlatformAdapter` 抽象 Host 能力。sidecar 凭证经 `getApiEndpoint(): Promise<{port, token}>` 获取——agent 启动/重启/UI 重连后统一调用，不依赖 `start_agent` 返回值；`readProfileFile(path)` 供 profile 导入读文件（webview 经 `asset://` 读会被 CSP 拦截）；事件订阅 `onSidecarLog` / `onSidecarExited`。MockAdapter（纯浏览器开发）走 localStorage，`getApiEndpoint` 默认 `{port: 8080, token: null}`（localStorage `mock-api-port` 可覆盖端口；token=null 对应 sidecar standalone 开放模式）。
- **API 客户端**（`src/lib/api.ts`、`src/lib/ws.ts`）：endpoint（baseUrl+token）由 agent store 缓存并经 `setEndpoint` 注入；token 非空时所有 fetch 带 `Authorization: Bearer <token>`，WS URL 追加 `?token=`。`/status` 返回 `StatusDTO`（running/mode/version/commit/protocol/uptimeMs/configPath/configError?）。
- **热挂载双写编排**（协议 §8，`src/lib/hot-mount.ts` 的 `persistThenMount`）：新增链路（NetworkForm）/隧道（ServiceForm→`saveTunnelConfig`）时——1) 先 `saveConfig` 落盘活动 profile；2) 再 `POST /links|/tunnels` 热挂载；3) 第 2 步失败回滚 profile（重存旧值）+ toast 报错；第 1 步失败不发起热挂载。
- **日志页**（LogsTab + log-store）：消费结构化 `sidecar-log`，按 level 着色、min-level 过滤（debug/info/warn/error），500 条环形缓冲。
- **状态展示**：`/status` 的 version+commit 显示在设置页"关于"区；`mode==="degraded"` 时 MainPage 顶部告警横幅展示 configError 并可跳转配置页。

## UI Workflow Rules

- In simple mode, after the user selects an existing share link, the primary next step must advance the main user path (for example, moving toward opening/configuring a share service) rather than leaving the user on the same page.
- Avoid redundant actions in the same panel that both effectively navigate to advanced mode; each primary/secondary action in simple mode should have a distinct user-facing purpose.

## 类型同步约定

- `src/lib/types.ts` 镜像 Go `api/dto.go`
- `src/lib/config-types.ts` 镜像 Go `agent/configv2/model.go`

修改 Go 侧结构体后需同步更新对应 TypeScript 类型。
