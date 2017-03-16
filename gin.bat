:: GIN client local setup for Windows
:: 
:: This file should be part of the gin client Windows bundle The purpose of the
:: file is to set up the environment and client configuration so that the
:: command line client can be used out of the box, without the need to install
:: or configure any packages.
::
:: The following actions are taken:
::
:: - The gin configuration file (config.yml) is created and the binary paths
:: for git, git-annex, and ssh are set.
:: - The configuration file is copied to its intended location.
:: - The location of gin.exe is appended to the PATH variable to it can be
:: used from the shell.
:: - A shell is started in the user's home directory.

echo off
set gitcmd=%CD%\git\bin\git.exe
set sshcmd=%CD%\git\usr\bin\ssh.exe
set gitannexcmd=%CD%\git\usr\bin\git-annex.exe

set configloc=%USERPROFILE%\.config\gin\
set config=config.yml

echo bin: > %config%
echo     git: %gitcmd% >> %config% 
echo     ssh: %sshcmd% >> %config% 
echo     gitannex: %gitannexcmd% >> %config%

setlocal enableextensions
md %configloc%
move config.yml %configloc%

path %path%;%CD%\bin;%CD%\git\usr\bin;%CD%\git\bin\
cd %USERPROFILE%

cmd /k
