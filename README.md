# GIN-cli

[![GoDoc](https://godoc.org/github.com/G-Node/gin-cli?status.svg)](http://godoc.org/github.com/G-Node/gin-cli)

[![Build Status](https://travis-ci.org/G-Node/gin-cli.svg?branch=master)](https://travis-ci.org/G-Node/gin-cli)
[![Build status](https://ci.appveyor.com/api/projects/status/gu9peb10f9k8ed3d/branch/master?svg=true)](https://ci.appveyor.com/project/G-Node/gin-cli/branch/master)

---

**G**-Node **In**frastructure command line client

This package is a command line client for interfacing with repositories hosted on [GIN](https://gin.g-node.org).
It offers a simplified interface for downloading and uploading files from repositories hosted on Gin.
For a guide on how to use the GIN service and this client, see the [GIN quick start](https://web.gin.g-node.org/G-Node/Info/wiki/Quick+start) page.

## Usage

The following is a description of the available commands in the GIN client.
In the command line, you can view a basic list of commands by running

    gin -h

You can also run

    gin help <cmd>

to get the full description of any command.

The table below describes the commands and their arguments.
Please note:
- Arguments enclosed in square brackets (e.g., `[<filenames>]`) are optional.
    - Generally, when a command specifies that it accepts `[<filenames>]` it means the user can limit the application of the command to specific filenames and directories. Multiple arguments may be specified (separated by space). If none are listed, the command will apply to all files and directories below the _current working directory_.
- Arguments listed together separated by a pipe (e.g., `--here | --no-clone`) denotes that the two options cannot be specified at the same time.

GIN command line client

Usage:

	gin <command> [<args>...]
	gin --help
	gin --version

Options:

	-h --help    This help screen
	--version    Client version

Commands:

	login          [<username>]
		Login to the GIN services

	logout
		Logout from the GIN services

	create         [--here | --no-clone] [<name>] [<description>]
		Create a repository on the remote server and clone it

	get            <repopath>
		Retrieve (clone) a repository from the remote server

	ls             [-s | --short | --json] [<filenames>]
		List the sync status of files in a local repository

	unlock         [<filenames>]
		Unlock files for editing

	lock           [<filenames>]
		Lock files

	upload         [<filenames>]
		Upload local changes to a remote repository

	download       [--content]
		Download all new information from a remote repository

	get-content    [<filenames>]
		Download the content of files from a remote repository

	getc           [<filenames>]
		Synonym for get-content

	remove-content [<filenames>]
		Remove the content of local files that have already been uploaded

	rmc            [<filenames>]
		Synonym for remove-content

	repos          [--shared | --all]
		List remote repositories

	repos          [<username>]
		List available remote repositories for specific user

	info           [<username>]
		Print user information

	keys           [-v | --verbose]
		List the keys associated with the logged in user

	keys           --add <filename>
		Add/upload a new public key to the GIN services

	help           <command>
		Get help for individual commands


Use 'help' followed by a command to see full description of the command.

