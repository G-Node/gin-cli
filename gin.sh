#!/bin/sh
#
# GIN bundle script

# run the gin binary using the git annex standalone wrapper
scriptloc=$(readlink -f "$0")
bindir=$(dirname "$scriptloc")
gindir=$(dirname "${bindir}")
annexdir=${gindir}/git-annex.linux

ginbin=${bindir}/gin

annexwrapper=${annexdir}/runshell

# run the command
$annexwrapper $ginbin "$@"
