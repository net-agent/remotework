@echo off
rem Cross-platform build for remote (Windows, Linux, macOS amd64).
rem Usage: build.bat   (run from the repository root)
setlocal enabledelayedexpansion

set "CGO_ENABLED=0"
set "APP=remote"
set "SRC=./cmd"
set "DIST=dist"

if not exist "%DIST%" mkdir "%DIST%"
if errorlevel 1 (
  echo failed to create output directory: %DIST% 1>&2
  exit /b 1
)

rem Inject version/commit (sidecar protocol: -X main.version / main.commit).
set "VERSION=dev"
for /f "delims=" %%v in ('git describe --tags --always 2^>nul') do set "VERSION=%%v"
set "COMMIT=unknown"
for /f "delims=" %%c in ('git rev-parse --short HEAD 2^>nul') do set "COMMIT=%%c"
set "LDFLAGS=-X main.version=%VERSION% -X main.commit=%COMMIT%"
echo version: %VERSION% (%COMMIT%)
echo ----------------------------------------

for %%o in (windows linux darwin) do (
  set "GOOS=%%o"
  set "GOARCH=amd64"
  set "EXT="
  if "%%o"=="windows" set "EXT=.exe"
  set "OUT=%DIST%\%APP%_%%o_amd64!EXT!"

  echo =^> building %%o-amd64 ...
  go build -trimpath -ldflags "!LDFLAGS!" -o "!OUT!" "%SRC%"
  if errorlevel 1 (
    echo build failed: %%o-amd64 1>&2
    exit /b 1
  )
  echo    built: !OUT!
)

echo ----------------------------------------
echo All builds completed.
exit /b 0
