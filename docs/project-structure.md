# GIN CLI Project Structure

## Package documentation

Automatically generated documentation for subpackages and public functions can be found on [GoDoc](https://godoc.org/github.com/G-Node/gin-cli).


## Packages

The code for the GIN CLI is structured into subpackages.  Some of the packages define a `struct` that handles all instance methods for the package (we will refer to this as the *representative struct* for the package).  An instance of this type can always be obtained with the respective `New()` function.  The following illustrates how the packages are structured conceptually (but not structurally) and what they add to each package below them.

```
gincmd: Defines command line interface subcommands and common top-level functionality.
   |
   |
   |---- gincmd/errors: Common error messages.
   |
   |
   |---- ginclient: Combines and augments the web and git packages with GIN specific functionality.
           |        The ginclient.Client struct is the representative type for this package.
           |
           |
           |---- web: Augments the Go standard library net/http.Client with convenience functions
           |          for making authenticated calls.
           |
           |
           |---- git: Augments the git/shell subpackages with git and git-annex specific functions.
           |       |  The git.Runner struct is the representative type for this package.
           |       |
           |       |
           |       |---- git/shell: Augments the standard library os/exec.Cmd with convenience
           |                        functions for reading piped output.
           |
           |
           |---- config: Handles client configuration (reading and writing).
           |
           |
           |---- log: Handles logging for the client.

```


The following list describes the purpose of each subpackage and some of its key functions.  This list is ordered in a *top-down*, *depth first* fashion; it explains higher level functionality first, starting from the package that the user interfaces with directly.


## gincmd

[Source](../gincmd) | [GoDoc](https://godoc.org/github.com/G-Node/gin-cli/gincmd)

*Package gincmd defines the subcommands of the gin command line client. It handles setting up the commands, their usage and help text (including examples), the top-level command logic, and output formatting.*

The `gincmd` package contains one file for each subcommand available on the command line (e.g., `create`, `upload`, `download`).  These files are named after the command with the suffix `cmd`.

Each file contains a public function that initialises the respective command using [Cobra](https://github.com/spf13/cobra).  The function that handles the top-level logic of each command is also contained in each command's respective file.

**Example** The `gin upload` command is specified in the file `uploadcmd.go`.  It's initialised in the function `UploadCmd()`.

The file [`common.go`](../gincmd/common.go) contains (mostly private) functions for functionality that is shared between multiple commands.  This includes output formatting and printing and error checking.

All output printing, including errors, that is displayed to the user should be handled in this package.  None of the other packages should ever print stdout or stederr.


## gincmd/errors

[Source](../gincmd/ginerrors) | [GoDoc](https://godoc.org/github.com/G-Node/gin-cli/gincmd/ginerrors)

A collection of error messages for printing to the user.  This is rather small currently and should be expanded to contain all potential error messages (even ones that only occur in one location in the code).  This may be expanded in the future to support localised messages.


## ginclient

[Source](../ginclient) | [GoDoc](https://godoc.org/github.com/G-Node/gin-cli/ginclient)

*Package ginclient augments the web package with functions to interact with GIN Gogs (https://github.com/G-Node/gogs) specifically.*

Top-level commands from the `gincmd` package are usually composed of one or more functions from this package.

**Example** The [`create`](https://godoc.org/github.com/G-Node/gin-cli/gincmd/createcmd.go) command calls `CreateRepo()` (creates a repository on the server) followed by `CloneRepo()`.

The `Client` struct handles method calls that require specific instances of a configuration for this package.  It's initialised with a *server alias*, which represents a GIN server configuration.  It holds references to `web.Client` and `git.Runner` instances, which are used to make web API calls and run git commands, respectively.

Functions in this package are separated into three files:
- [`gin.go`](../ginclient/gin.go) handles user-related functions (authentication and user info queries).
- [`repos.go`](../ginclient/repos.go) handles repository-related functions (both through the `web.Client` and the `git.Runner`).
- [`ssh.go`](../ginclient/ssh.go) handles SSH user and host key management.


## ginclient/config

[Source](../ginclient/config) | [GoDoc](https://godoc.org/github.com/G-Node/gin-cli/ginclient/config)

*Package config handles reading of the user configuration for the client.*

Holds the default configuration values, loads user configurations form the file and writes new configuration values.

The `GinCliCfg` represents the entire GIN CLI configuration.


## ginclient/log

[Source](../ginclient/log) | [GoDoc](https://godoc.org/github.com/G-Node/gin-cli/ginclient/log)

*Package log handles logging for the client.*

Augments the Go standard library `log.Logger` with convenience functions.


## web

[Source](../web) | [GoDoc](https://godoc.org/github.com/G-Node/gin-cli/web)

*Package web provides functions for interacting with a REST API. It was designed to work with GIN Gogs (https://github.com/G-Node/gogs), a fork of the Gogs git service (https://github.com/gogits/gogs), and therefore only implements requests and assumes responses for working with that particular API. Beyond that, the implementation is relatively general and service agnostic.*

Augments the Go standard library `net/http.Client` with convenience functions for making authenticated calls.  It was written to work with the GIN web server, though no GIN-specific functionality is explicitly included.

The `web.Client` struct is the representative type for this package.  It holds a web address and user token and implements standard HTTP methods (GET, POST, DELETE) with user authentication.


## git

[Source](../git) | [GoDoc](https://godoc.org/github.com/G-Node/gin-cli/git)

*Package git provides functions for running git and git-annex shell commands.*

The `git.Runner` struct is the representative type for this package.

Functions in this package are separated into three files:
- [`git.go`](../git/git.go) handles git commands.
- [`annex.go`](../git/annex.go) handles git-annex commands.
- [`util.go`](../git/util.go) contains utility functions.

Git and git-annex functions all use the `git.Command()` function which sets up and returns a `shell.Cmd` instance for *shelling out* to run the appropriate commands.  Most git-annex commands support printing output in JSON format and this is used where available.

The doc string for git and git-annex functions mention the respective shell commands that they run.


## git/shell

[Source](../git/shell) | [GoDoc](https://godoc.org/github.com/G-Node/gin-cli/git/shell)

*Package shell augments the standard library os/exec Cmd struct and functions with convenience functions for reading piped output.*

The `shell.Cmd` struct is the representative type for this package.

The most important addition to the standard `exec.Cmd` functionality is the addition of two buffered readers, `OutReader` and `ErrReader` (type `bufio.Reader`).  These can be used to read the output and error of a command as streams while it runs asynchronously and allows printing progress output to the user, through the use of channels.
