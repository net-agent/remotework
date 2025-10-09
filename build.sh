#!/bin/bash
# 该脚本用于实现golang程序的跨平台编译，支持windows、linux、mac
# 使用方法：在终端执行 sh build.sh
# 脚本会自动创建一个带日期和版本后缀的目录（如 20251008a），并将所有编译产物放入其中

# 设置通用编译参数
CGO_ENABLED=0
GOOS_LIST=("windows" "linux" "darwin")
app="remote"
srcPath="./cmd"
baseDistPath="./dist" # 基础输出目录

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
  echo "错误：今日构建次数已达上限 (a-z)，请清理旧目录或明天再试。"
  exit 1
fi

# 创建目标输出目录
mkdir -p "$target_dir"
echo "编译产物将输出到: ${target_dir}"
echo "----------------------------------------"


# --- 2. 循环编译程序 ---
for os in ${GOOS_LIST[@]}; do
  # 根据不同的操作系统，设置需要编译的架构列表
  target_archs=()
  case "$os" in
    "windows" | "linux")
      # windows和linux只编译amd64
      target_archs=("amd64")
      ;;
    "darwin")
      # darwin (macOS) 需要同时编译amd64和arm64
      target_archs=("amd64" "arm64")
      ;;
  esac

  for arch in ${target_archs[@]}; do
    echo "=> 开始编译 ${os}-${arch} 平台..."
    
    # 根据操作系统设置文件扩展名
    ext=""
    if [ "$os" = "windows" ]; then
      ext=".exe"
    fi
    
    # 构造不含日期的二进制文件名
    bin_name="${app}_${os}_${arch}${ext}"
    
    # 执行编译，输出到新创建的目标目录
    GOOS="$os" GOARCH="$arch" go build -o "${target_dir}/${bin_name}" "${srcPath}"
    
    # 检查编译是否成功
    if [ $? -eq 0 ]; then
      echo "   编译成功: ${target_dir}/${bin_name}"
    else
      echo "   编译失败: ${os}-${arch}"
    fi
  done
done

echo "----------------------------------------"
echo "全部编译任务完成！"