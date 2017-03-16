:: GIN client local setup for Windows
:: 
:: This file should be part of the gin client Windows bundle The purpose of the
:: file is to set up the environment and client configuration so that the
:: command line client can be used out of the box, without the need to install
:: or configure any packages.
::
:: The following actions are taken:
::
:: - The location of gin.exe is appended to the PATH variable so it can be
:: used from the shell.
:: - The locations of the git, annex, and utility binaries (SSH, rsync, etc)
:: are appended to the PATH variable.
:: - A shell is started in the user's home directory.

echo off

set ginbinpath=%CD%\bin
set gitpaths=%CD%\git\usr\bin;%CD%\git\bin

path %path%;%ginbinpath%;%gitpaths%

cd %USERPROFILE%

cmd /k
