# GIN-cli

[![Build Status](https://travis-ci.org/G-Node/gin-cli.svg?branch=master)](https://travis-ci.org/G-Node/gin-cli)
[![GoDoc](https://godoc.org/github.com/G-Node/gin-cli?status.svg)](http://godoc.org/github.com/G-Node/gin-cli)

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
- Arguments enclosed in square brackets (e.g., `[filenames]`) are optional.
    - Generally, when a command specifies that it accepts `[filenames]` it means the user can limit the application of the command to specific filenames and directories. Multiple arguments may be specified (separated by space). If none are listed, the command will apply to all files and directories below the _current working directory_.
- Arguments listed together separated by commas (e.g., `--verbose, -v`) denotes that one is equivalent to the other (usually a short form).

command          | arguments                   | description
---------------- | -------------------------   | ----------------------------
`login`          | `[username]`                | Login to the GIN services
`logout`         |                             | Logout from the GIN services
`create`         | `name [description]`        | Create a repository on the remote server and download (clone) it locally
`get`            | `repository`                | Retrieve (clone) a repository from the remote server. Repository should be specified in the form `username/repositoryname`
`ls`             | `[--short, -s] [filenames]` | List the sync status of files in the local repository. Optionally print short listing. See below for description of [status abbreviations](#statusabbrev). Specifying filenames or directories will limit the listing
`unlock`         | `[filenames]`               | Unlock files for editing
`lock`           | `[filenames]`               | Lock files
`upload`         | `[filenames]`               | Upload local changes to remote repository
`download`       | `[--content]`               | Download all new information from a remote repository. Specifying `--content` makes all content in the repository available locally
`get-content`    | `[filenames]`               | Download the content of files from a remote repository
`remove-content` | `[filenames]`               | Remove the content of local files that have already been uploaded
`rmc`            | `[filenames]`               | Synonym for remove-content
`repos`          | `[username]`                | List available remote repositories owned by a specific user. If no username is specified, lists all available remote repositories
`info`           | `[username]`                | Print specific user information. If no username is specified, prints the current logged in user's information
`keys`           | `[--verbose, -v]`           | List the keys associated with the logged in user. Optionally print the full public key
`keys`           | `--add filename`            | Add/upload a new public key to the GIN server
`help`           | `command`                   | Get detailed help for individual commands
