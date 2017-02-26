# GIN-cli

[![Build Status](https://travis-ci.org/G-Node/gin-cli.svg?branch=master)](https://travis-ci.org/G-Node/gin-cli) [![Coverage Status](https://coveralls.io/repos/github/G-Node/gin-cli/badge.svg?branch=master)](https://coveralls.io/github/G-Node/gin-cli?branch=master) [![GoDoc](https://godoc.org/github.com/G-Node/gin-cli?status.svg)](http://godoc.org/github.com/G-Node/gin-cli)

**G**-Node **In**frastructure command line client

This package is a command line client for interfacing with the [gin-auth](https://github.com/G-Node/gin-auth) and [gin-repo](https://github.com/G-Node/gin-repo) services.
It offers a simplified interface for downloading and uploading files from repositories hosted on Gin.

## Usage

The following is a description of the available commands in the GIN client.
In the command line, you can view a basic list of commands by running 

    gin -h

You can also run

    gin cmdhelp

to get the following full descriptions:

    login        Login to the GIN services

                 If no <username> is specified on the command line, you will be
                 prompted for it. The login command always prompts for a
                 password.


    logout       Logout of the GIN services


    create       Create a new repository on the GIN server

                 If no <name> is provided, you will be prompted for one.
                 A repository <description> can only be specified on the
                 command line after the <name>.
                 Login required.


    get          Download a remote repository to a new directory

                 The repository path <repopath> must be specified on the
                 command line. A repository path is the owner's username,
                 followed by a "/" and the repository name
                 (e.g., peter/eegdata).
                 Login required.


    upload       Upload local repository changes to the remote repository

                 Uploads any changes made on the local data to the GIN server.
                 The upload command should be run from inside the directory of
                 an existing repository.


    download     Download remote repository changes to the local repository

                 Downloads any changes made to the data on the server to the
                 local data directory.
                 The download command should be run from inside the directory
                 of an existing repository.


    repos        List accessible repositories

                 Without any argument, lists all the publicly accessible
                 repositories on the GIN server.
                 If a <username> is specified, this command will list the
                 specified user's publicly accessible repositories.
                 If you are logged in, it will also list any repositories
                 owned by the user that you have access to.


    info         Print user information

                 Without argument, print the information of the currently
                 logged in user or, if you are not logged in, prompt for a
                 username to look up.
                 If a <username> is specified, print the user's information.


    keys         List or add SSH keys

                 By default will list the keys (description and fingerprint)
                 associated with the logged in user. The verbose flag will also
                 print the full public keys.
                 To add a new key, use the --add option and specify a pub key
                 <filename>.
                 Login required.

