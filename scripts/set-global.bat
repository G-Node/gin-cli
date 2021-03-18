:: GIN client global setup for Windows
::
:: This file should be part of the gin client Windows bundle. The purpose of
:: the file is to permanently change the user path environment to be able to
:: run GIN commands from the command line (cmd and PowerShell).

@echo off

set ginpath=%~dp0

:: Get user path *only* (No system path)
for /F "skip=2 tokens=1,2*" %%N in ('%SystemRoot%\System32\reg.exe query "HKCU\Environment" /v "Path" 2^>nul') do if /I "%%N" == "Path" call set "userpath=%%P"

echo "Checking"
echo %userpath%|find /I "gin">nul && (
    echo Please edit the "Path" variable and remove all previous GIN CLI paths.
    echo The environment variables settings window will open automatically.
    pause
    rundll32 sysdm.cpl,EditEnvironmentVariables
)

:: Read again
for /F "skip=2 tokens=1,2*" %%N in ('%SystemRoot%\System32\reg.exe query "HKCU\Environment" /v "Path" 2^>nul') do if /I "%%N" == "Path" call set "userpath=%%P"

:: Prepend GIN path
echo Prepending "%ginpath%;" to path
setx path "%ginpath%;%userpath%"

echo GIN CLI is ready
pause
