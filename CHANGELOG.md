# Changelog

**Beta** releases are not listed. Changes for beta releases are included in the next full release. Current changes are listed in the top **Unreleased** section.

## Version 1.7

### Changes
- Git annex v7 mode: With this version, new repositories and local clones are initialised in git-annex v7 mode. This brings several changes in the behaviour of file locking:
    - Files are added **unlocked** by default.
    - Locking a file is a persistent status change that can be committed to the repository and will affect other clones.
    - **Locked** files are not usable on Windows (not even for reading) and need to be unlocked.
    - If no locking or unlocking is performed, the behaviour is is identical across all platforms.
- New command: `gin sync`
    - Synchronises changes bidirectionally between remote repositories and the local clone.
    - Content can be synchronised at the same time with the `--content` flag.
    - Using the `--content` flag will download **all** data in the repository.
- `gin version --copy-to` behaviour changes:
    - Files can now be checked out from older versions to any location, i.e., the destination directory does not need to be under the repository root.
    - Checked out files (the old versions) are not added to the repository.
    - Annexed content of checked out files is automatically downloaded.
- Custom commit messages: Users can now specify a custom commit message when performing a `gin commit` by specifying the `--message` flag (`-m` for short).
- Better handling of merge conflicts.
- `gin ls` file listings are colour coded. Synced files are green, pointer files without content are cyan, modified files are yellow/orange, removed files are red.
- `gin ls` prints a summary with file counts for each status at the bottom of the output.
- JSON output has been added (`--json` flag) to the `info`, `keys`, `servers`, and `remotes` commands.
- A message is always printed when there is "Nothing to do".
- Output is more consistent across all file operations.

### Bug fixes
- Fixed bugs with colours in Windows terminals.
- Fixed bug where the log would not be properly trimmed on Windows.
- Fixed bug where `gin version --copy-to` would fail on Windows.

### Relevant PRs
- #237: Windows colours
- #241: Fix for adding annexed content to git on Windows
- #242: Count only T (typechange) files
- #244: Log trim fix
- #247: More consistent file operation messages
- #248: Small Changes to Verbose flag
- #250: Windows fixes and annex v7
- #253: Support custom commit messages
- #254: Changes to 'gin version' command behaviour
- #256: LScmd enhancement
- #257: Bugfix for 'git cat-file' on Windows (for 'gin version --copy-to')
- #258: gin ls summary
- #260: Add JSON output to info, keys, servers, and remotes commands

## Version 1.6

### Bug fixes
- Fixed bug where commands `repos` and `repoinfo` would still prompt for login when called with `--json` flag and the user was not logged in. Commands invoked with the `--json` flag should never prompt for any input.
- Fixed colour output in Windows.
- Fixed bug where annexed content would also be added to git on Windows.

### Relevant PRs
- #236: Don't prompt for login with --json
- #237: Windows colours
- #241: Fix for adding annexed content to git on Windows

## Version 1.5

### Bug fixes
- Fixed bug where the annex initialisation on Windows would fail due to the repository working in direct mode.
- Fixed minor bug in the error field for JSON output.

### Relevant PRs
- #232: Use git-annex-sync to create empty init commit

## Version 1.3

### Bug fixes
- Fixed bug when committing or uploading multiple large files. Some files would fail to be added to the annex resulting in them being added to git.

### Internal changes
- Repositories are explicitly set to annex version 5 mode on initialisation.

### Relevant PRs
- #229: Force v5 repos
- #231: Fix filename metadata write

## Version 1.2

### Changes
- Merge conflicts:
    - Downloads will fail with useful error messages if a merge conflict occcurs.
    - If the conflict is between two annexed files, both versions of the file are kept and a warning is presented.
- Push to target: Pushing to a specific target remote does not push git files and metadata to all remotes like before. Only the named remote is updated.
- Commit progress removed: The number of files to be added is no longer counted before doing a `gin commit` so no progress bar is shown. This was removed for performance reasons.


### Bug fixes
- Fixed bug when uploading after annexed file was deleted. The client would try to `annex-copy` the deleted file if it was named and fail with an error.
- Fixed bug where files with unicode characters would cause errors. The client now disabled `quotepath` in the local git configuration to avoid quotes in filenames that interfere with parsing and JSON encoding.


### Internal changes
- Token reuse: Logging in reuses any existing client token to avoid flooding the user's token list.
- Token request function: `NewToken()` function can be called to retrieve a new token without creating a key pair or storing it on disk.

### Relevant PRs
- #214: Split token request code out of Login into new method
- #215: Reuse tokens
- #217: Merge conflict error messages and handling
- #222: Push only to target remote
- #223: Bugfix: Do not try to annex-copy deleted files
- #228: Disable 'quotepath' in local git configuration

## Version 1.1

### Bug fixes
- Fixed a couple of bugs relating to the `add-server` command.
    - The port was not being stored properly when using the input prompts to set up a new server configuration.
    - In some cases, the host key was not written to the `known_hosts` file in the format required (especially for `localhost`).

### Relevant PRs
- #213: Localhost key fix

## Version 1.0

### Changes
- New commands
    - Local workflows:
        - `gin init`: Initialises a directory as a gin repository without creating a repository on a server.
        - `gin commit`: Records changes in the local repository without performing an upload and without requiring a remote or an internet connection.
    - Versioning
        - `gin version`: Rolls back files or directories to older versions. With the `--copy-to` flag, retrieves older files without overwriting the current version and copies them to a specific location.
    - Remotes
        - `gin add-remote`: Adds a remote to the current repository for uploading and downloading. This also brings support for using directory paths on the local filesystem as "remotes" (e.g., external hard drives, network attached storage), without the need to have a GIN server.
        - `gin remove-remote`: Removes a remote from the current repository.
        - `gin remotes`: Lists remotes configured for a repository and shows the default remote used for `gin upload`.
        - `gin use-remote`: Switches the repository's default upload remote.
        - The `gin upload` command now accepts a `--to` argument for uploading annexed content to a specific, non-default remote.
    - Servers
        - `gin add-server`: Adds a new GIN server to the global client configuration.
        - `gin remove-server`: Removes a GIN server from the global client configuration.
        - `gin servers`: Lists the configured servers and shows the default server used for web queries.
        - `gin use-server`: Switches the default server.
        - The `login`, `logout`, `create`, `info`, `keys`, `repos`, `repoinfo`, and `get` commands now accept a `--server` argument for querying or operating on a specific, non-default server.
- Progress bars for file operations: Some operations don't have a per-file progress (add, lock, unlock). There is no partial unlock state for a file, for instance. For these commands, the output shows the overall progress along with the number of total files that are being affected.
- Smaller logfiles: The log file is now limited to 1 MB. No more ever-growing logs.
- The client is now usable even without git-annex installed, but commands that require git and git-annex are disabled.
- Progress is now also printed when uploading git files.
- Minimum required got-annex version: 6.20171109

### Bug fixes
- Fixed a bug where file tracking would register a type change in git when working with direct mode repositories (e.g., on Windows).
- Fixed a bug where the file status (from `gin ls`) was being incorrectly reported when working in direct mode (e.g., on Windows). Direct mode repositories should now show the exact same output as indirect mode ones.

### Relevant PRs
- #191: New command: gin version
- #192: Versioning "copy to"
- #197: Transfer rates
- #199: gin commit: Working without remotes
- #201: Fix file adding in direct mode
- #202: Log trimming
- #203: Progress bars for file operations
- #204: Fix for ls file status in direct mode
- #205: New command: add-remote
- #206: Handle multiple server configurations
- #210: Disable commands that rely on git/annex when either is not available
- #211: Git transfer output

## Version 0.16

### Changes
- Logging changes: More useful logging info and command delimitation.
- Relevant help: When a command is given bad arguments, instead of printing the general help/usage info, it now prints the help/usage for that specific command.
- Fix for stuttering/flashing of text during progress printing on Windows.
- Completely redone command line argument handling and better help formatting.
- New command: `gin repoinfo`
  - Prints the information for a single repository on the server.

### Relevant PRs
- #172: Logging changes
- #174: Use git annex add --update for locking
- #176: Print gin-cli version to log on initialisation (of the log)
- #177: Update help text
- #181: Error message and output improvements
- #182: Print output status only when text has changed
- #184: Better command line argument parsing
- #187: Fix for configuration paths with spaces
- #188: New command: repoinfo


## Version 0.15

### Changes
- No longer commits changes when performing an `AnnexPull` (`git annex sync --no-push`).

### Relevant PRs
- #171: No commit on gin download

## Version 0.14

### Changes
- Host SSH key needs to be added to any non-default host configuration and is strictly checked.
- Various improvements and bug fixes.

### Relevant PRs
- #161: Fixed bug where output message for file removals was 'Adding remove'.
- #162: Strict host key checking for git and ssh commands.
- #163: Allow running git and annex proxy commands without token.
- #168: Ignore old (deprecated) config paths.
- #169: Disable symlinks explicitly on Windows.

## Version 0.13

### New features
- Create repository on the server without cloning: `gin create --no-clone`
    - Cannot be used in combination with `--here`.
- Delete public SSH key from the server: `gin keys --delete <index>`

### Relevant PRs
- #157: Delete key command
- #160: `gin create --no-clone`

## Version 0.12

### New features and feature changes
- Create repository out of current directory
    - `gin create --here` will create a new repository on the remote server and initialise the current working directory with the appropriate remotes and annex configuration.
- JSON output: Most commands now support the `--json` flag for JSON formatted output.
- The location for configuration files has changed on all platforms. Platform specific locations are now used. On first run, the client looks for files in the old configuration directories and transfers them to the new location if necessary.
- If a command requires login and the user is not logged in, instead of simply printing an informative message, the user is prompted for login (unless `--json` is specified).
- Local (in-repo) config file can only be used to specify annex selection. In other words, only the `annex.minsize` and `annex.excludes` is used. All other options are read from the global and default configurations.
- The local (in-repo) configuration file is never checked into annex, regardless of annex minsize rules.
- The local config file can only be used for `annex.minsize` and `annex.excludes` options. All other options are ignored.
- Repository listing function fix:
    - `gin repos` now lists only the logged in user's repositories.
    - `gin repos --all` lists the logged in user's repositories and all other repositories in which they are a member.
    - `gin repos --shared` lists only the repositories in which the user is a member.
- Repository listings now provide more information if available.

### File operation and file transfer progress output
- File operations such as `lock`, `unlock`, and `remove-content` now provide per-file output on whether the operation was successful or not. The output prints per line as the operation finishes for each file.
- File transfers during `upload`, `download`, and `get-content` show transfer rates and percentage complete.
- `gin upload` without an argument no longer warns about the lack of file specification. Previously the warning was meant to inform the user that changes (metadata) are uploaded but no new files. The behaviour of the command (what is updated in the index and what is uploaded) is now clear due to the output progress and success or failure messages printed by the command and the warning message would probably lead to confusion. Note also that the behaviour of the command has changed slightly. New content will be uploaded for files that are already being tracked.

### Bug fixes
- More flexible version checking for git-annex: Handles more types of version string formats.

### Relevant PRs
- #120: `gin create --here`
- #123: File transfer progress output
- #124: Fix for version check
- #126: JSON output for `gin ls`
- #127: Bug fix for file locking when performing download
- #131: Bug fixes for Windows and file query routines
- #135: Platform-specific config and log directories
- #137: Bug fixes and login prompt
- #141: Local config changes: Only annex filtering
- #148: Error handling and messages
- #149: New repository listing command
- #150: Fixes to repos command and error messages
- #153: Minor tweak of `gin repos --json output`

### Known issues
- Some error conditions are not reported properly. In these cases, a command may fail with "unknown error" or in worse cases, appear to succeed without providing information about failure.
    - It is known that the latter will occur when performing an `upload` while there are changes on the server that have not been downloaded. The command will add/lock all local files, will not upload any content, and exit successfully. The occurrence of this can be determined by the lack of upload progress.

## Version 0.11

- The client now supports local, per-repo configuration files. Options specified in a file called `config.yml` in the root of a repository will override options from the global and default configurations.
- Fixed issue where some git implementations would continuously try to use the user's key instead of the one generated by gin (macOS).
- Fixed issue which caused very slow responses on Windows when repositories got too big.

## Version 0.10

- Minor bug fixes and improvements.
- Improved the performance of `gin ls` when querying specific files.

## Version 0.9

- Check files into git: When adding files (via `gin upload`) the client will now check small files (smaller than 10 MiB by default) into git instead of annex. This threshold can be configured in the config file. Additionally, file patterns (globs) can be specified for exclusion from annex. Any files that match a pattern or is below the *small file* threshold, will be checked into git rather than the annex.
    - This behaviour also works in direct mode.
- SSH keys on login: Instead of generating temporary SSH keys for each transaction, a single key pair is created when a user logs in and is deleted when they log out.
- Annex version check: The client will no longer work if it cannot find a git-annex binary, or its version is too old (current minimum version: 6.20160126)

## Version 0.8

- The `download` command now only retrieves changes in metadata and does not retrieve the content of files by default. There are now two ways to download file content:
    - `gin download --content` synchronises all changes that were made remotely to the local repository and downloads the content of **all** files.
    - `gin get-content <filenames>` does not update the local repository to reflect remote changes, but downloads the content of all files specified.
- The `get-content` command is a **new command** introduced in version 0.8.
- The `upload` command does not add any new changes to the repository when no arguments are specified. In order to upload all changes under the current working directory, a period `.` should be specified.

## Version 0.7

- **Content handling**:
    - `gin upload` accepts files or directories as arguments and only commits and uploads the specified files.
    - `gin download` retrieves the content for placeholder files, or recursively downloads files if a directory is specified.
    - `gin rmc` removes the content from local files, leaving only placeholders, if the content can be confirmed to be available on at least one other remote.
- **New commands: lock and unlock**: Starting with version 0.7, files are locked by default and need to be unlocked for editing (Linux and macOS only).

## Version 0.6

- The client secret can now be set in the configuration file. If not set, the secret defaults to the secret for the G-Node GIN server.

## Version 0.5

- New command: `gin ls`
    - Lists files and their status. See `gin help ls` for details and status codes.
- New behaviour for `gin repos`
    - See `gin help repos` for details
- `gin create` now automatically performs a `gin get`
- More informative error messages
- Plenty of bug fixes

## Version 0.4

- Adds better help text.
- Includes complete version string, including commit hash, for better troubleshooting.
- See the wiki downloads page for details on which package to download.
