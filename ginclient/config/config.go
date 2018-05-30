// Package config handles reading of the user configuration for the client.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/G-Node/gin-cli/ginclient/log"
	"github.com/fatih/color"
	"github.com/shibukawa/configdir"
	"github.com/spf13/viper"
)

const (
	defaultFileName = "config.yml"
)

var (
	configDirs = configdir.New("g-node", "gin")
	yellow     = color.New(color.FgYellow).SprintFunc()

	ginDefaultServer = ServerCfg{
		WebCfg{
			Protocol: "https",
			Host:     "web.gin.g-node.org",
			Port:     443,
		},
		GitCfg{
			Host:    "gin.g-node.org",
			Port:    22,
			User:    "git",
			HostKey: "gin.g-node.org,141.84.41.216 ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBE5IBgKP3nUryEFaACwY4N3jlqDx8Qw1xAxU2Xpt5V0p9RNefNnedVmnIBV6lA3n+9kT1OSbyqA/+SgsQ57nHo0=",
		},
	}

	defaultConf = map[string]interface{}{
		// Binaries
		"bin.git":      "git",
		"bin.gitannex": "git-annex",
		"bin.ssh":      "ssh",
		// Annex filters
		"annex.minsize": "10M",
		"servers.gin":   ginDefaultServer,
	}

	// configuration cache: used to avoid rereading during a single command invocation
	configuration GinCliCfg
	set           = false
)

// Types

// WebCfg is the configuration for the web server.
type WebCfg struct {
	Protocol string
	Host     string
	Port     uint16
}

// AddressStr constructs a full address string from the configuration.
// The string has the format Scheme://Host:Port (e.g., https://web.gin.g-node.org:443)
func (c WebCfg) AddressStr() string {
	return fmt.Sprintf("%s://%s:%d", c.Protocol, c.Host, c.Port)
}

// GitCfg is the configuration for the git server.
type GitCfg struct {
	User    string
	Host    string
	Port    uint16
	HostKey string
}

// AddressStr constructs a full address string from the configuration.
// The string has the format ssh://User@Host:Port (e.g., ssh://git@gin.g-node.org:22)
func (c GitCfg) AddressStr() string {
	return fmt.Sprintf("ssh://%s@%s:%d", c.User, c.Host, c.Port)
}

// ServerCfg holds the information required for GIN servers (web and git).
type ServerCfg struct {
	Web WebCfg
	Git GitCfg
}

// GinCliCfg holds the client configuration values.
type GinCliCfg struct {
	Servers map[string]ServerCfg
	Bin     struct {
		Git      string
		GitAnnex string
		SSH      string
	}
	Annex struct {
		Exclude []string
		MinSize string
	}
}

// Read loads in the configuration from the config file(s), merges any defined values into the default configuration, and returns a populated GinConfiguration struct.
// The configuration is cached. Subsequent reads reuse the already loaded configuration.
func Read() GinCliCfg {
	if set {
		return configuration
	}
	viper.Reset()
	viper.SetTypeByDefaultValue(true)

	for k, v := range defaultConf {
		viper.SetDefault(k, v)
	}

	// Merge in user config file
	confpath, _ := Path(false)
	confpath = filepath.Join(confpath, defaultFileName)
	viper.SetConfigFile(confpath)
	cerr := viper.MergeInConfig()
	if cerr == nil {
		log.Write("Found config file %s", confpath)
	}

	viper.Unmarshal(&configuration)

	removeInvalidServerConfs()

	// configuration file in the repository root (annex excludes and size threshold only)
	reporoot, err := findreporoot(".")
	if err == nil {
		confpath := filepath.Join(reporoot, defaultFileName)
		viper.SetConfigFile(confpath)
		cerr = viper.MergeInConfig()
		if cerr == nil {
			log.Write("Found config file %s", confpath)
		}
	}
	configuration.Annex.Exclude = viper.GetStringSlice("annex.exclude")
	configuration.Annex.MinSize = viper.GetString("annex.minsize")

	set = true
	return configuration
}

func removeInvalidServerConfs() {
	// Check server configurations for invalid names and port numbers
	for alias := range viper.GetStringMap("servers") {
		if alias == "dir" || alias == "all" {
			fmt.Fprintf(color.Error, "%s server alias '%s' is not allowed (reserved word): server configuration ignored\n", yellow("[warning]"), alias)
			delete(configuration.Servers, alias)
			continue
		}
		webport := viper.GetInt(fmt.Sprintf("servers.%s.web.port", alias))
		gitport := viper.GetInt(fmt.Sprintf("servers.%s.git.port", alias))
		if webport < 0 || webport > 65535 || gitport < 0 || gitport > 65535 {
			if alias == "gin" {
				fmt.Fprintf(color.Error, "%s invalid value found in configuration for '%s': using default\n", yellow("[warning]"), alias)
				configuration.Servers["gin"] = ginDefaultServer
			} else {
				fmt.Fprintf(color.Error, "%s invalid value found in configuration for '%s': server configuration ignored\n", yellow("[warning]"), alias)
				delete(configuration.Servers, alias)
			}
		}
	}
}

// WriteServerConf writes a new server configuration into the user config file.
func WriteServerConf(alias string, newcfg ServerCfg) error {
	// Read in the file configuration ONLY
	confpath, _ := Path(true) // create config path if necessary
	confpath = filepath.Join(confpath, defaultFileName)
	v := viper.New()
	v.SetConfigFile(confpath)

	v.ReadInConfig()

	v.Set(fmt.Sprintf("servers.%s.web.protocol", alias), newcfg.Web.Protocol)
	v.Set(fmt.Sprintf("servers.%s.web.host", alias), newcfg.Web.Host)
	v.Set(fmt.Sprintf("servers.%s.web.port", alias), newcfg.Web.Port)

	v.Set(fmt.Sprintf("servers.%s.git.user", alias), newcfg.Git.User)
	v.Set(fmt.Sprintf("servers.%s.git.host", alias), newcfg.Git.Host)
	v.Set(fmt.Sprintf("servers.%s.git.port", alias), newcfg.Git.Port)

	v.WriteConfig()

	return nil
}

// Path returns the configuration path where configuration files should be stored.
// If the GIN_CONFIG_DIR environment variable is set, its value is returned, otherwise the platform default is used.
// If create is true and the directory does not exist, the full path is created.
func Path(create bool) (string, error) {
	confpath := os.Getenv("GIN_CONFIG_DIR")
	if confpath == "" {
		confpath = configDirs.QueryFolders(configdir.Global)[0].Path
	}
	var err error
	if create {
		err = os.MkdirAll(confpath, 0755)
		if err != nil {
			return "", fmt.Errorf("could not create config directory %s", confpath)
		}
	}
	return confpath, err
}

// Util functions

// pathExists returns true if the path exists
func pathExists(path string) bool {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}
	return true
}

func findreporoot(path string) (string, error) {
	var err error
	path, err = filepath.Abs(path)
	if err != nil {
		return "", err
	}
	gitdir := filepath.Join(path, ".git")
	if pathExists(gitdir) {
		return path, nil
	}
	updir := filepath.Dir(path)
	if updir == path {
		// root reached
		return "", fmt.Errorf("Not a repository")
	}

	return findreporoot(updir)
}
