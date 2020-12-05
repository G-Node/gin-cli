:: GIN client entry point for Windows
::
:: This file should be part of the gin client Windows bundle.
::
:: The purpose of the file is to act as a wrapper for the GIN CLI binary. It
:: sets the path to use the bundled dependencies without polluting the user's
:: permanent global path.


@echo off

setlocal
set gindir=%~dp0
set ginpaths=%gindir%bin;%gindir%git\usr\bin;%gindir%git\bin
set path=%ginpaths%;%path%

gin.exe %*
endlocal
