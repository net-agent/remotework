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

## UI Workflow Rules

- In simple mode, after the user selects an existing share link, the primary next step must advance the main user path (for example, moving toward opening/configuring a share service) rather than leaving the user on the same page.
- Avoid redundant actions in the same panel that both effectively navigate to advanced mode; each primary/secondary action in simple mode should have a distinct user-facing purpose.

## 类型同步约定

- `src/lib/types.ts` 镜像 Go `api/dto.go`
- `src/lib/config-types.ts` 镜像 Go `agent/configv2/model.go`

修改 Go 侧结构体后需同步更新对应 TypeScript 类型。
