# GIN CLI configuration

The GIN client allows for certain aspects of its operation to be configured.
This is accomplished by specifying key-value pairs, in YAML format, in a file called `config.yml`.
The location of this file differs per platform ([see below](#config-file-location)).

In addition to this global configuration, the [git-annex filtering criteria](filtering.md) can be configured for individual repositories, by placing a file called `config.yml` at the root of the repository.

## Defaults

The following shows the default values for every configurable option
```yaml
bin:
  git: git
  gitannex: git-annex
  ssh: ssh

servers:
  gin:
    web:
      protocol: https
      host: gin.g-node.org
      port: 443

    git:
      host: gin.g-node.org
      port: 22
      user: git
      hostkey: "gin.g-node.org,141.84.41.216 ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBE5IBgKP3nUryEFaACwY4N3jlqDx8Qw1xAxU2Xpt5V0p9RNefNnedVmnIBV6lA3n+9kT1OSbyqA/+SgsQ57nHo0="

annex:
    minsize: 10M
    exclude: []
```

### Description of the configuration values:
- bin: The bin section is used to specify the names and locations of external programs used by the GIN client. Generally, these are simply specified by the name of the executable, since it is assumed their locations can be found in the user's PATH. If this is not the case, the locations can be specified for each individual program.
    - git: The path to the git executable.
    - gitannex: The path to the git-annex executable.
    - ssh: The path to the ssh executable.
- servers: The servers section is used to define GIN servers that the client can interact with. By default, there is only one server configured called 'gin'.
  - gin: The default GIN server. By default it points to the official G-Node GIN server. This can be changed to work with locally deployed servers. Additional servers can be added with different names (called aliases).
      - protocol: The protocol (scheme) used by the server, typically `http` or `https`.
      - host: The web address of the server.
      - port: The port used by the server, typically `80` for `HTTP` or `443` for `HTTPS`.
  - git: The git section is used to specify the git address, port, username, and host key that the GIN web server is configured to use.
      - address: The git/ssh server address. This is often the same as the web (gin) address, but may be different, or have a different subdomain.
      - port: The ssh server port (typically `22`).
      - user: For most git servers this is simply the user `git`. This is the name of the server-side user that handles all remote git operations.
      - hostkey: The SSH key of the git server. The GIN client uses strict host key checking, so if this is not specified, or is specified incorrectly, git operations will not work. This key is different for each server installation.
- annex: The annex section is used to specify the [git-annex filtering criteria](filtering.md). This is the only configuration section that is read for **local** (per repository) configurations.
    - minsize: The minimum size of a file that should be added to the annex. All files smaller than this size are added to git instead.
    - exclude: Patterns or filenames that should be excluded from the annex. For example, the pattern `*.py` will exclude all Python source code files from the annex, adding them to git instead. Files which match a pattern are always excluded from the annex, even if they are above the minsize. Patterns should be specified as a list of strings, e.g., `["*.py", "*.md", "*.m"]`.


## Config file location

The location of the user global configuration file differs per platform:

- Windows: `%APPDATA%\g-node\gin\config.yml`
- macOS: `${HOME}/Library/Application Support/g-node/gin/config.yml`
- Linux: `${HOME}/.config/g-node/gin/config.yml`
