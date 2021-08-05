@echo off

SET version=%1

if "%version%" == "" (

  echo invalid version info

) else (

  echo version=%version%

  if not exist "dist" mkdir dist
  if not exist "dist\%version%" mkdir "dist\%version%"

  echo start build windows

  echo build agent
  cd agent_bin
  go build -o "..\dist\%version%\agent_windows_%version%.exe"

  echo build server
  cd ..\server
  go build -o "..\dist\%version%\server_windows_%version%.exe"

  cd ..
  echo windows finished


  echo start build linux(amd64) 
  set CGO_ENABLE=0
  set GOOS=linux
  set GOARCH=amd64

  echo build agent
  cd agent_bin
  go build -o "..\dist\%version%\agent_linux_%version%_bin"

  echo build server
  cd ..\server
  go build -o "..\dist\%version%\server_linux_%version%_bin"

  cd ..
  echo linux(amd64) finished



  echo start build linux(darwin)
  set CGO_ENABLE=0
  set GOOS=darwin
  set GOARCH=amd64

  echo build agent
  cd agent_bin
  go build -o "..\dist\%version%\agent_darwin_%version%_bin"

  echo build server
  cd ..\server
  go build -o "..\dist\%version%\server_darwin_%version%_bin"

  cd ..
  echo linux(darwin) finished

)
