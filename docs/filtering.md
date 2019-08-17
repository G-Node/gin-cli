# Filtering files to git or git-annex

During `git-annex add` operations, an exclusion filter is specified, which ensures that files which match certain criteria are not added to the annex.
The purpose of these filters is to separate things which git is well suited to handle (code and other small text files) from things which git-annex is better suited for (large binary files).

By default, the only criterion used is the file size: files below 10 MB are added to git, while files greater or equal to 10 MB are added to the annex. Users can override this threshold in either a global or a local configuration file (see the [configuration](config.md) page for details).
In addition to a file size threshold, users can also specify file patterns to be excluded from the annex, which implies that they will be added directly to git. These patterns are often used to specify file extensions, such as source code or text file extensions (e.g., `*.c`, `*.py`, `*.m`, `*.txt`, `*.md`) but any pattern can be used (e.g., `analysis*.py`). The client also never allows adding a file called `config.yml` to the annex. This can not be changed.

In practice, this filtering is applied using the `--larger-than` and `--exclude` flags of the `git-annex add` command.
The `--larger-than` flag is used to apply the size threshold.
The `--exclude` flag is specified once for each pattern specified.

In code, the implementation of the filtering can be seen in the [`gin-client/git.go/annexExclArgs`](../gin-client/git.go) function.
The string slice returned from this function is always added to the `git-annex add` command.
