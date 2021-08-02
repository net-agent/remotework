@echo off

SET version=%1

if "%version%" == "" (

  echo invalid version info

) else (

  echo version=%version%

  if not exist "dist" mkdir dist
  if not exist "dist\%version%" mkdir "dist\%version%"

  echo start build agent
  cd agent_bin
  go build -o "..\..\dist\%version%\agent_windows_%version%.exe"

  echo start build server
  cd ..\..\server
  go build -o "..\dist\%version%\server_windows_%version%.exe"

  cd ..
  echo finished

)
