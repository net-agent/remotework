#!/bin/bash
# 该脚本用于实现golang程序的跨平台编译，支持windows、linux、mac
# 使用方法：在终端执行 sh build.sh，脚本会自动编译为windows、linux、mac三种不同平台的程序，生成的二进制文件在build目录下

# 设置交叉编译参数
CGO_ENABLED=0
GOOS=("windows" "linux" "darwin")
GOARCH=("amd64")
app="remote"
date="$(date '+%Y%m%d')"
srcPath="./cmd"
distPath="./dist"

# 编译程序
for os in ${GOOS[@]}; do
  for arch in ${GOARCH[@]}; do
    echo "编译${arch}-${os}程序..."
    if [ "$os" = "windows" ]; then
      ext=".exe"
    else
      ext="_bin"
    fi
    bin_name="${app}_${date}_${os}${ext}"
    GOOS="$os" GOARCH="$arch" go build -o "${distPath}/${bin_name}" "${srcPath}"
  done
done

echo "编译完成！"
