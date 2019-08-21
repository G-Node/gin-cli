# GIN CLI command translation

How GIN CLI commands translate to git and git-annex operations.

*Nb: This document does not mention any system or GIN web API calls unless they are necessary to explain a git or git-annex operation.*

## gin create

Brief:
1. Creates repository on the server (`/api/v1/user/repos`)
2. Clones repository: See [gin get](#gin-get).

## gin get

Brief:
1. `git clone`
2. `git config core.quotepath false`
3. `git config core.symlinks false` (Windows only)
4. `git commit --allow-empty -m "Initial commit: Repository initialised on <hostname>"` (if the repository is new).
5. `git config annex.backends MD5`
6. `git config annex.addunlocked true`
7. `git annex init --version=7`

Details:
1. The `gin get` command expects the unique repository name in the form `<user>/<repository>`.  The full clone URL is created by appending this to the Git URL of the configured server: `ssh://git@gin.g-node.org:22/<user>/<repository>`
2. The `quotepath` option is disabled since it can break JSON formatting by including escape sequences in filenames with special characters.  Keeping it enabled instead quotes filenames with special characters, which is easier to work with.
3. Git can detect whether symlinks are supported during a repository initialisation.  On some Windows configurations this is possible, but git-annex can misbehave when this happens (see [this relevant issue on the git-annex wiki](https://git-annex.branchable.com/bugs/Symlink_support_on_Windows_10_Creators_Update_with_Developer_Mode/) for more info).  This might have been solved in newer versions of git-annex.
4. If the repository is new, creating an empty commit makes commands like `gin ls` work more predictably.  Working in a repository with no HEAD makes certain commands not work.
5. Limiting the available git-annex backends to MD5 alleviates an issue that can occur on Windows with very long path names.  See [this Wiki page](https://gin.g-node.org/G-Node/Info/wiki/SomeNotesOnGitAnnex) on GIN for more info.
6. The `addunlocked` option has been added recently to make files get added in unlocked mode by default.  This means that files added to the annex are not converted to symlinks and makes repositories easier to work with.
7. The annex is initialised in **version 7** mode.  Although this is still not the default for git-annex (by default, `git annex init` initialises repositories to version 5), it has many advantages that make working with repositories more straightforward, such as the `addunlocked` option described above.

## gin upload

Brief: `gin upload <filenames>` → `git-annex-add <filenames>` (with filtering) > `git add <filenames>` (leftover from filter) > `git-annex sync --no-pull --commit` > `git-annex copy --to=origin`

The `gin upload` command takes care of every step from the adding of a file to the git index up until it arrives on the server.
In the steps outlined below, `<filenames>` refers to a list of files or directories supplied by the user on the command line when invoking the command.


1. `git-annex add`: The client first adds files to the annex. This doesn't add all files matched by `<filenames>` to the annex; instead the files are filtered based on certain rules about what should go into git and what should go into the annex. See [filtering](filtering.md) for details on this.
2. `git add`: Any files that were filtered out in step 1 and therefore not added to the annex are added to git.
3. `git-annex sync --no-pull --commit`: This command performs a number of operations in and of itself. Since git-annex, much like our GIN client, wraps git operations in many cases, this is not uncommon, but it can make describing exactly what is happening a bit harder.
4. `git-annex copy --to=origin`: This copies any new annexed content to the remote named origin (the default remote). In the background, git-annex used rsync to transfer the files.

Step 3 is itself a multistep operation, so let's break it down further. First, it performs a `git commit` (specified by the `--commit` flag). The client specifies a commit message with subject `GIN upload from <HOSTNAME>` and the body describes the changes associated with the commit (file addition, deletion, and modification counts).
Then a `git push` is performed, along with a few git-annex bookkeeping tasks that are handled in the annex branches.
The `--no-pull` flag ensures that this sync operation does not do a bidirectional sync. Since this is part of the `gin upload` command, the client does not attempt to pull and merge any changes that may have occurred on the server.
If there are changes on the server that have not been pulled, the push will fail.

### Code

The `gin upload` command starts in the [`cmd/uploadcmd.go/upload`][uploadcmd.go] function, but all the important steps are broken down in the [`gin-client/repos.go/Upload`][repos.go] function.
The [`gin-client/git.go/AnnexAdd`][git.go] and [`gin-client/git.go/GitAdd`][git.go] commands handle the `git-annex add` and `git add` operations respectively.
Finally, [`gin-client/git.go/AnnexPush`][git.go] performs the `git-annex sync` and `git-annex copy` operations.

[uploadcmd.go]: ../cmd/uploadcmd.go
[repos.go]: ../gin-client/repos.go
[git.go]: ../gin-client/git.go
