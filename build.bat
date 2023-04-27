@echo off

echo ø™ º±‡“Î
if not exist dist mkdir dist

set CGO_ENABLED=0
setlocal enabledelayedexpansion
set arch=amd64

for %%i in (windows linux darwin) do (

  set GOOS=%%i
  set GOARCH=%arch%

  if "%%i"=="windows" (
    set ext=.exe
  ) else (
    set ext=_bin
  )

  echo compile arch=%arch% os=%%i ext=!ext!
  go build -o dist/abaddon_%%i_%arch%!ext! ./cmd
)

echo ±‡“ÎΩ· ¯
