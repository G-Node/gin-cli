# git error conditions

## Untracked or non-annexed files

This section discusses the behaviour of the gin client when annex content commands are run against untracked files or files tracked only by git, but not git-annex. In the following discussion, *untracked*, unless otherwise stated, will refer to both types of files, since the behaviour is the same.

### git annex behaviour

Git annex does not return errors (or any message, for that matter) when running state and content manipulation commands on untracked files. Specifically, `lock`, `unlock`, `get`, and `drop`, when running on untracked files, are silent no-op commands. The same goes for files tracked by git but not in the annex.

### gin client behaviour

It would be simpler for the gin client to be compatible with the behaviour of git-annex. This isn't necessary, since the purpose of the gin client is to make working with git and git-annex simpler and provide workflows that are easier for non-technical users.

#### Silent no-op

There are two direct advantages to the gin client behaving the same as git-annex.
1. For one, creating an error in the client when an error in the underlying calls does not exist requires extra checks and conditions that complicate the gin-client library.
2. Secondly, the issue is complicated even further when considering how to handle multiple arguments, directories, or entire trees recursively, where some files are annexed and others are not.

I'm especially worried about the complicated checks and conditions required for handling the second case. Consider the following example:

A repository contains two files: `README.md` and `data.h5`. The first is checked into git while the second is in the annex. The user runs `gin unlock .` or `gin unlock *`. If `gin unlock README.md` is a command that *should* throw an error, then whenever a user needs to unlock the entire repository, they would get an error.

One idea would be to only raise errors when the file is mentioned explicitly, but that would make cross-platform behaviour inconsistent in the case of globs. Specifically, `gin unlock *` would expand to both files when passed to gin on Linux and macOS, but not Windows, so on two platforms, it's explicit, while on another it's not. A potential solution to this would be to check if it's mentioned explicitly *after* expanding globs internally, to make behaviour consistent, but I still don't think running `gin unlock .` should be (much) different from running `gin unlock *`. The only difference should be the potential inclusion of dotfiles, and other intricacies of shell glob expansion.


#### Errors or warnings for untracked files

Raising errors for untracked files might be less confusing for the user. If the user attempts to `gin rmc README.md`, a file that is not in the annex, then a message informing the user that the content of the file can't be removed along with the reason for the failure is much more informative than silence.

However, errors are disruptive. If an error is accompanied by a non-zero exit code, it can also disrupt scripting for recursive operations. However, informational text can be added that doesn't quality as an error, or even a warning. For instance, in the scenario described in the previous section, where a repository contains a git tracked file `README.md` and an annexed file `data.h5`:
```
> gin unlock *
Unlocking data.h5 OK
Unlocking README.md N/A (file is not in annex)
```
and exit cleanly (`0` exit status).

Similarly, the message would make other situations informative as well, for instance, if another file is added to the directory, but isn't added to the repository:
```
> gin unlock *
Unlocking data.h5 OK
Unlocking README.md N/A (not in annex)
Unlocking new-file.txt N/A (not under gin control)
```

This still suffers from the issues mentioned in the previous section: Increased code complexity required to determine the state of the file. It also adds noise to the output of commands ran against entire directory trees.
