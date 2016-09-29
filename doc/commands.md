# Command usage instructions and workflow

## Login

To login via the command line client:

`gin login <username>`

If *username* is omitted, the user will be prompted for input.
The password is also provided via prompt.

### Internals

On successful login, a bearer token is stored in the configuration directory.

## Create repository

To create a new repository:

`gin create <name> -d <description>`

If *name* is omitted, the user will be prompted for input.
The *description* is optional and can be set later.

### Internals

This command creates a new repository on the server and locally creates a local clone under the working directory with the name of the repository.
The repository is also initialised with an *annex* (`git annex init`).

## Upload data

To upload data to the server, the files must be placed in a directory that is a GIN repository.

`gin upload <file> ...`

The user can specify any number of files, or omit the file argument altogether.
If no file is specified, everything under the working directory is uploaded.

### Internals

This command adds all specified files (or all changes under the working directory) to the repository, commits, and pushes the changes.
The commit message describes the changes made to the repository: file additions, deletions, and modifications.

```shell
git annex add <file> ...
git commit -m <description>
git push
git annex sync --no-pull --content
```

## Download data

The download command can be used to create a local copy of a remote repository, or to download newer versions of files in an existing local copy.
