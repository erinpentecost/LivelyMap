@echo off
setlocal

rem --- Move to directory of this script ---
cd /d "%~dp0"
echo %cd%

rem --- Build the tool if it's not already built ---
if not exist ".\cmd\lively\lively.exe" (
    pushd .\cmd\lively
    go build .
    if errorlevel 1 exit /b 1
    popd
)

rem --- Run it ---
rem %* contains all arguments
.\cmd\lively\lively.exe -threads=3 -vanity=F -cfg="%~1"

endlocal
