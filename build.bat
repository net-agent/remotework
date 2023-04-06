@echo off
rem 该脚本用于实现golang程序的跨平台编译，支持windows、linux、mac
rem 使用方法：在终端执行 build.bat，脚本会自动编译为windows、linux、mac三种不同平台的程序，生成的二进制文件在build目录下

rem 设置交叉编译参数
set CGO_ENABLED=0
set GOOS=windows linux darwin
set GOARCH=amd64
set app=remote
set date=%date:~0,4%%date:~5,2%%date:~8,2%
set srcPath=./cmd
set distPath=./dist

rem 编译程序
for %%i in (%GOOS%) do (
  for %%j in (%GOARCH%) do (
    echo 编译%%j-%%i程序...
    if "%%i"=="windows" (
      set ext=.exe
    ) else (
      set ext=_bin
    )
    set bin_name=%app%_%date%_%%i%ext%
    set GOOS=%%i
    set GOARCH=%%j
    go build -o %distPath%\%bin_name% %srcPath%
  )
)

echo 编译完成！