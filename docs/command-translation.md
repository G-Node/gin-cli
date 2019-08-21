# GIN CLI command translation

How GIN CLI commands translate to git and git-annex operations.

*Note: This document does not mention any system or GIN web API calls unless they are necessary to explain a git or git-annex operation.*

## gin create

Brief:
1. Creates repository on the server (`/api/v1/user/repos`)
2. Clones repository: See [gin get](#gin-get).

## gin get

Brief:
1. `git clone ssh://git@<server>:<port>/<user>/<repository>`
2. Initialises default options: See [gin init](#gin-init).

Details:
1. The `gin get` command expects the unique repository name in the form `<user>/<repository>`.  The full clone URL is created by appending this to the Git URL of the configured server: `ssh://git@gin.g-node.org:22/<user>/<repository>`

## gin init

Brief
1. `git config core.quotepath false`
2. `git config core.symlinks false` (Windows only)
3. `git commit --allow-empty -m "Initial commit: Repository initialised on <hostname>"` (if the repository is new).
4. `git config annex.backends MD5`
5. `git config annex.addunlocked true`
6. `git annex init --version=7`

Details:
1. The `quotepath` option is disabled since it can break JSON formatting by including escape sequences in filenames with special characters.  Keeping it enabled instead quotes filenames with special characters, which is easier to work with.
2. Git can detect whether symlinks are supported during a repository initialisation.  On some Windows configurations this is possible, but git-annex can misbehave when this happens (see [this relevant issue on the git-annex wiki](https://git-annex.branchable.com/bugs/Symlink_support_on_Windows_10_Creators_Update_with_Developer_Mode/) for more info).  This might have been solved in newer versions of git-annex.
3. If the repository is new, creating an empty commit makes commands like `gin ls` work more predictably.  Working in a repository with no HEAD makes certain commands not work.
4. Limiting the available git-annex backends to MD5 alleviates an issue that can occur on Windows with very long path names.  See [this Wiki page](https://gin.g-node.org/G-Node/Info/wiki/SomeNotesOnGitAnnex) on GIN for more info.
5. The `addunlocked` option has been added recently to make files get added in unlocked mode by default.  This means that files added to the annex are not converted to symlinks and makes repositories easier to work with.
6. The annex is initialised in **version 7** mode.  Although this is still not the default for git-annex (by default, `git annex init` initialises repositories to version 5), it has many advantages that make working with repositories more straightforward, such as the `addunlocked` option described above.

## gin commit

Brief
1. `git add <deleted files>`
2. `git annex add <filenames>` (with filters)
3. `git commit`

Details:
1. Running `git add` on deleted files is necessary since the subsequent `git annex add` will ignore this change on some platforms.
2. `git annex add` is run with configurable filters that automatically add files to either git or the annex.  See [filtering](filtering.md) for details.

## gin upload

Brief:
1. If files are specified that have modifications, [gin commit](#gin-commit) is performed first.
2. `git push`
3. `git annex copy`

Details:
1. If the user specifies paths on the command line as arguments to the upload command, the client first runs `gin commit` on those paths.
2. Changes are pushed to the default remote if none are specified, otherwise the specified remote is used.
3. As with 2., if no remote is specified, the default is used.  If no files are specified, `git annex copy` is run with the `--all` flag.

### Code

The `gin upload` command starts in the [`cmd/uploadcmd.go/upload`][uploadcmd.go] function, but all the important steps are broken down in the [`gin-client/repos.go/Upload`][repos.go] function.
The [`gin-client/git.go/AnnexAdd`][git.go] and [`gin-client/git.go/GitAdd`][git.go] commands handle the `git-annex add` and `git add` operations respectively.
Finally, [`gin-client/git.go/AnnexPush`][git.go] performs the `git-annex sync` and `git-annex copy` operations.

[uploadcmd.go]: ../cmd/uploadcmd.go
[repos.go]: ../gin-client/repos.go
[git.go]: ../gin-client/git.go
