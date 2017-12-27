package main

const usage = `
GIN command line client

Usage:	gin <command> [<args>...]
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

	create         [<name>] [<description>]
		Create a repository on the remote server and clone it

	get            <repopath>
		Retrieve (clone) a repository from the remote server

	ls             [-s, --short, --json] [<filenames>]
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

	repos          [<username>]
		List available remote repositories

	info           [<username>]
		Print user information

	keys           [-v, --verbose]
		List the keys associated with the logged in user

	keys           --add <filename>
		Add/upload a new public key to the GIN services

	help           <command>
		Get help for individual commands


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

	Create a new repository on the GIN server and clone it locally.

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

const lsHelp = `USAGE

	gin ls [--short, -s, --json] [<filenames>]...

DESCRIPTION

	List one or more files or the contents of directories and the status of the
	files within it. With no arguments, lists the status of the files under the
	current directory. Directory listings are performed recursively.

	In the short form, the meaning of the status abbreviations is as follows:
		OK: The file is part of the GIN repository and its contents are
		synchronised with the server.
		UL: The file has been unlocked for editing.
		NC: The local file is a placeholder and its contents have not been
		downloaded.
		MD: The file has been modified locally and the changes have not been
		recorded yet.
		LC: The file has been modified locally, the changes have been recorded
		but they haven't been uploaded.
		??: The file is not under repository control.

ARGUMENTS

	--short, -s
		Print listing in short form.

	--json
		Print listing in json format (uses short form abbrevations).

	<filenames>
		One or more directories or files to list.
`

const unlockHelp = `USAGE

	gin unlock [<filenames>]...

DESCRIPTION

	Unlock one or more files for editing. Files added to the repository are
	left in a locked state, which allows reading but prevents editing. In order
	to edit or write to a file, it must first be unlocked. When done editing,
	it is recommended that a file be locked again, using the 'lock' command,
	which records changes in the repository.

	After performing an 'upload', 'download', or 'get', affected files are
	reverted to a locked state.

	Unlocking a file takes longer depending on the size of the file.

ARGUMENTS

	<filenames>
		One or more directories or files to unlock.
`

const lockHelp = `USAGE

	gin lock [<filenames>]...

DESCRIPTION

	Lock one or more files after editing. After unlocking files for editing,
	using the 'unlock' command, it is recommended that they be locked again.
	This records any changes made and prepares a file for upload to the GIN
	server.

	Locked files are replaced by symbolic links in the working directory.

	After performing an 'upload', 'download', or 'get', affected files are
	reverted to a locked state.

	Unlocking a file takes longer depending on the size of the file.

ARGUMENTS

	<filenames>
		One or more directories or files to lock.
`

const uploadHelp = `USAGE

	gin upload [<filenames>]...

DESCRIPTION

	Upload changes made in a local repository clone to the remote repository on
	the GIN server. This command must be called from within the local
	repository clone. Specific files or directories may be specified.
	All changes made will be sent to the server, including addition of new
	files, modifications and renaming of existing files, and file deletions.

	If no arguments are specified, only changes to files already being tracked
	are uploaded.

ARGUMENTS

	<filenames>
		One or more directories of files to upload and update.
`

const downloadHelp = `USAGE

	gin download [--content]

DESCRIPTION

	Downloads changes from the remote repository to the local clone. This will
	create new files that were added remotely, delete files that were removed,
	and update files that were changed.

	Optionally downloads the content of all files in the repository. If
	'content' is not specified, new files will be empty placeholders. Content
	of individual files can later be retrieved using the get-content command.

ARGUMENTS

	--content
		Optionally download the content for all files in the repository.
`

const getContentHelp = `USAGE

	gin get-content [<filenames>]...

DESCRIPTION

	Download the content of the listed files. The get-content command is
	intended to be used to retrieve the content of placeholder files in a local
	repository. This command must be called from within the local repository
	clone. With no arguments, downloads the content for all files under the
	working directory, recursively.

ARGUMENTS

	<filenames>
		One or more names of files or directories to retrieve.
`

const rmcHelp = `USAGE

	gin remove-content [<filenames>]...
	gin rmc [<filenames>]...

DESCRIPTION

	Remove the content of local files. This command will not remove the content
	of files that have not been already uploaded to a remote repository, even
	if the user specifies such files exclusively.  Removed content can be
	retrieved from the server by using the 'gin download' command.  With no
	arguments, removes the content of all files under the current working
	directory, as long as they have been safely uploaded to a remote
	repository.

	Note that after removal, placeholder files will remain in the local
	repository. These files appear as 'No Content' when running the 'gin ls'
	command.

ARGUMENTS

	<filenames>
		One or more names of files or directories to remove.
`

const reposHelp = `USAGE

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
`

const infoHelp = `USAGE

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

	-v, --verbose
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
	"login":          loginHelp,
	"logout":         logoutHelp,
	"create":         createHelp,
	"get":            getHelp,
	"ls":             lsHelp,
	"unlock":         unlockHelp,
	"lock":           lockHelp,
	"upload":         uploadHelp,
	"download":       downloadHelp,
	"get-content":    getContentHelp,
	"getc":           getContentHelp,
	"remove-content": rmcHelp,
	"rmc":            rmcHelp,
	"repos":          reposHelp,
	"info":           infoHelp,
	"keys":           keysHelp,
}

// ex: set cc=80:
