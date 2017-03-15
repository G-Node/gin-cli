#!/bin/sh
#
# GIN bundle script

# set the path for the bundled git annex standalone binaries
scriptloc=$(readlink -f "$0")
bindir=$(dirname "$scriptloc")
gindir=$(dirname "${bindir}")
annexbindir=${gindir}/git-annex.linux/bin
PATH=${annexbindir}:${PATH}

ginbin=${bindir}/gin

# run the command
$ginbin "$@"
