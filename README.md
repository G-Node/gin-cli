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

    gin help <cmd>

to get the full description of any command:

login

    USAGE

      gin login [<username>]

    DESCRIPTION

      Login to the GIN services.

    ARGUMENTS

      <username>
        If no username is specified on the command line, you will be
        prompted for it. The login command always prompts for a
        password.

logout

    USAGE

      gin logout

    DESCRIPTION

      Logout of the GIN services.

      This command takes no arguments.

create

    USAGE

      gin create [<name>] [<description>]

    DESCRIPTION

      Create a new repository on the GIN server.

    ARGUMENTS

      <name>
        The name of the repository. If no <name> is provided, you will be
        prompted for one. If you want to provide a description, you need to
        provide a repository name on the command line. Names should contain
        only alphanumeric characters, '.', '-', and '_'.

      <description>
        A repository description (optional). The description should be
        specified as a single argument. For most shells, this means the
        description should be in quotes.

    EXAMPLES

      Create a repository. Prompt for name

        $ gin create

      Create a repository named 'example' with a description

        $ gin create example "An example repository"

      Create a repository named 'example' with no description

        $ gin create example

get

    USAGE

      gin get <repopath>

    DESCRIPTION

      Download a remote repository to a new directory and initialise the
      directory with the default options. The local directory is referred to as
      the 'clone' of the repository.

    ARGUMENTS

      <repopath>
        The repository path <repopath> must be specified on the command line.
        A repository path is the owner's username, followed by a "/" and the
        repository name.

    EXAMPLES

      Get and intialise the repository named 'example' owned by user 'alice'

        $ gin get alice/example

      Get and initialise the repository named 'eegdata' owned by user 'peter'

        $ gin get peter/eegdata

ls

    USAGE

      gin ls [<directory>]...

    DESCRIPTION

      List one or more files or the contents of directories and the status of the
      files within it. With no arguments, lists the status of the files under the
      current directory. Directory listings are performed recursively.

      The meaning of the status abbreviations is as follows:
        OK: The file is part of the GIN repository and its contents are
        synchronised with the server.
        NC: The local file is a placeholder and its contents have not been
        downloaded.
        MD: The file has been modified locally and the changes have not been
        recorded yet.
        LC: The file has been modified locally, the changes have been recorded
        but they haven't been uploaded.
        ??: The file is not under repository control.

    ARGUMENTS

      <directory>
        One or more directories or files to list.

upload

    USAGE

      gin upload

    DESCRIPTION

      Upload changes made in a local repository clone to the remote repository on
      the GIN server. This command must be called from within the local
      repository clone. All changes made will be sent to the server, including
      addition of new files, modifications and renaming of existing files, and
      file deletions.

      This command takes no arguments.

download

    USAGE

      gin download

    DESCRIPTION

      Download changes made in the remote repository on the GIN server to the
      local repository clone. This command must be called from within the
      local repository clone. All changes made on the remote server will
      be retrieved, including addition of new files, modifications and renaming
      of existing files, and file deletions.

      This command takes no arguments.

repos

    USAGE

      gin repos [<username>]
      gin repos -s, --shared-with-me
      gin repos -p, --public


    DESCRIPTION

      List repositories on the server that provide read access. If no argument is
      provided, it will list the repositories owned by the logged in user. If no
      user is logged in, it will list all public repositories.

    ARGUMENTS

      -s, --shared-with-me
        List all repositories shared with the logged in user.

      -p, --public
        List all public repositories.

      <username>
        The name of the user whose repositories should be listed.  This
        consists of public repositories and repositories shared with the logged
        in user.

info

    USAGE

      gin info [<username>]

    DESCRIPTION

      Print user information. If no argument is provided, it will print the
      information of the currently logged in user.

      Using this command with no argument can also be used to check if a user is
      currently logged in.

    ARGUMENTS

      <username>
        The name of the user whose information should be printed. This can be
        the username of the currently logged in user, in which case the command
        will print all the profile information with indicators for which data
        is publicly visible. If it is the username of a different user, only
        the publicly visible information is printed.

keys

    USAGE

      gin keys [-v | --verbose]
      gin keys --add <filename>

    DESCRIPTION

      List or add SSH keys. If no argument is provided, it will list the
      description and fingerprint for each key associated with the logged in
      account.

      The command can also be used to add a public key to your account from an
      existing filename (see --add argument).

    ARGUMENTS

      -v, --verbose
        Verbose printing. Prints the entire public key when listing.

      --add <filename>
        Specify a filename which contains a public key to be added to the GIN
        server.

    EXAMPLES

      Add a public key to your account, as generated from the default ssh-keygen
      command

        $ gin keys --add ~/.ssh/id_rsa.pub
