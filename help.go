package main

const usage = `
GIN command line client

Usage: gin <command> [<args>...]
       gin --help
       gin --version

Options:
    -h --help    This help screen
    --version    Client version

Commands:
    login    [<username>]
    logout
    create   [<name>] [<description>]
    get      <repopath>
    upload
    download
    repos    [<username>]
    info     [<username>]
    keys     [-v | --verbose]
    keys     --add <filename>
    help     <command>

Use 'help' followed by a command to see full description of the command.
`

const loginHelp = `USAGE

	gin login [<username>]

DESCRIPTION

	Login to the GIN services.

ARGUMENTS

	<username>
		If no username is specified on the command line, you will be
		prompted for it. The login command always prompts for a
		password.
`

const logoutHelp = `USAGE

	gin logout

DESCRIPTION

	Logout of the GIN services.

	This command takes no arguments.
`

const createHelp = `USAGE

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
`

const getHelp = `USAGE

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
`

const uploadHelp = `USAGE

	gin upload

DESCRIPTION

	Upload changes made in a local repository clone to the remote repository on
	the GIN server. This command must be called from within the (cloned)
	repository directory. All changes made will be sent to the server,
	including addition of new files, modifications and renaming of existing
	files, and file deletions.

	This command takes no arguments.
`

const downloadHelp = `USAGE

	gin download

DESCRIPTION

	Download changes made in the remote repository on the GIN server to the
	local repository clone. This command must be called from within the
	(cloned) repository directory. All changes made on the remote server will
	be retrieved, including addition of new files, modifications and renaming
	of existing files, and file deletions.

	This command takes no arguments.
`

const reposHelp = `USAGE

	gin repos [<username>]


DESCRIPTION

	List repositories on the server that provide read access. If no argument is
	provided, it will list all publicly accessible repositories on the GIN
	server.

ARGUMENTS

	<username>
		The name of the user whose repositories should be listed. This can be
		the username of the currently logged in user (YOU), in which case the
		command will list all repositories owned by YOU. If it is the username
		of a different user, it will list all the repositories owned by the
		specified user that YOU have access to. This consists of public
		repositories and repositories shared with YOU.
`

const infoHelp = `USAGE

	gin info [<username>]

DESCRIPTION

	Print user information. If no argument is provided, it will print the
	information of the currently logged in user.

ARGUMENTS

	<username>
		The name of the user whose information should be printed. This can be
		the username of the currently logged in user, in which case the command
		will print all the profile information with indicators for which data
		is publicly visible. If it is the username of a different user, only
		the publicly visible information is printed.
`
const keysHelp = `USAGE

	gin keys [-v | --verbose]
	gin keys --add <filename>

DESCRIPTION

	List or add SSH keys. If no argument is provided, it will list the
	description and fingerprint for each key associated with the logged in
	account.

	The command can also be used to add a public key to your account from an
	existing filename (see --add argument).

ARGUMENTS

	--verbose, -v
		Verbose printing. Prints the entire public key when listing.

	--add <filename>
		Specify a filename which contains a public key to be added to the GIN
		server.

EXAMPLES 

	Add a public key to your account, as generated from the default ssh-keygen 
	command

		$ gin keys --add ~/.ssh/id_rsa.pub
`

var cmdHelp = map[string]string{
	"login":    loginHelp,
	"logout":   logoutHelp,
	"create":   createHelp,
	"get":      getHelp,
	"upload":   uploadHelp,
	"download": downloadHelp,
	"repos":    reposHelp,
	"info":     infoHelp,
	"keys":     keysHelp,
}
