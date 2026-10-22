#!/usr/bin/env bash
# 该脚本用于实现 golang 程序的跨平台编译，支持 windows、linux、macOS。
# 使用方法：在项目根目录执行 bash build.sh（或 make build-all）
# 脚本会自动创建一个带日期和版本后缀的目录（如 20251008a），并将所有编译产物放入其中。
# 任一目标编译失败时脚本立即以退出码 1 结束。
set -euo pipefail

# 静态编译，避免依赖目标平台 libc
export CGO_ENABLED=0

# 允许从任意目录调用：后续相对路径均以仓库根目录为基准
cd "$(dirname "${BASH_SOURCE[0]}")"

# 设置通用编译参数
GOOS_LIST=("windows" "linux" "darwin")
app="remote"
srcPath="./cmd"
baseDistPath="./dist" # 基础输出目录

# 版本信息注入（sidecar 协议：version/commit 经 -ldflags -X 写入二进制）
VERSION="$(git describe --tags --always 2>/dev/null || echo dev)"
COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo unknown)"
LDFLAGS="-X main.version=${VERSION} -X main.commit=${COMMIT}"
echo "版本: ${VERSION} (${COMMIT})"

# --- 1. 确定本次编译的输出目录 ---
date_prefix="$(date '+%Y%m%d')"
target_dir=""

# 循环 a 到 z，找到第一个可用的目录后缀
for suffix in {a..z}; do
  check_dir="${baseDistPath}/${date_prefix}${suffix}"
  if [ ! -d "$check_dir" ]; then
    target_dir="$check_dir"
    break
  fi
done

# 如果从 a 到 z 的目录都存在，则报错退出
if [ -z "$target_dir" ]; then
  echo "错误：今日构建次数已达上限 (a-z)，请清理旧目录或明天再试。" >&2
  exit 1
fi

# 创建目标输出目录
mkdir -p "$target_dir"
echo "编译产物将输出到: ${target_dir}"
echo "----------------------------------------"

# --- 2. 循环编译程序 ---
for os in "${GOOS_LIST[@]}"; do
  # 根据不同的操作系统，设置需要编译的架构列表
  target_archs=()
  case "$os" in
    "linux")
      # linux 编译 amd64 和 arm64
      target_archs=("amd64" "arm64")
      ;;
    "windows")
      # windows 只编译 amd64
      target_archs=("amd64")
      ;;
    "darwin")
      # darwin (macOS) 需要同时编译 amd64 和 arm64
      target_archs=("amd64" "arm64")
      ;;
    *)
      echo "错误：未知目标平台 ${os}" >&2
      exit 1
      ;;
  esac

  for arch in "${target_archs[@]}"; do
    echo "=> 开始编译 ${os}-${arch} 平台..."

    # 根据操作系统设置文件扩展名
    ext=""
    if [ "$os" = "windows" ]; then
      ext=".exe"
    fi

    # 构造不含日期的二进制文件名
    bin_name="${app}_${os}_${arch}${ext}"

    # 执行编译，输出到新创建的目标目录；失败必须中止整个脚本
    if GOOS="$os" GOARCH="$arch" go build -trimpath -ldflags "${LDFLAGS}" \
      -o "${target_dir}/${bin_name}" "${srcPath}"; then
      echo "   编译成功: ${target_dir}/${bin_name}"
    else
      echo "   编译失败: ${os}-${arch}" >&2
      exit 1
    fi
  done
done

echo "----------------------------------------"
echo "全部编译任务完成！"
