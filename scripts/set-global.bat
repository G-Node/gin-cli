:: GIN client global setup for Windows
:: 
:: This file should be part of the gin client Windows bundle. The purpose of
:: the file is to permanently change the user path environment to be able to
:: run GIN commands from the command line (cmd and PowerShell).

echo off

set curdir=%~dp0

set ginbinpath=%curdir%\bin
set gitpaths=%curdir%\git\usr\bin;%curdir%\git\bin
echo Appending "%ginbinpath%;%gitpaths%" to path
echo %path%|find /I "%curdir%">nul || setx path "%path%;%ginbinpath%;%gitpaths%"
echo GIN CLI is ready
pause
