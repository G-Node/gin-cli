// Package config handles reading of the user configuration for the client.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/G-Node/gin-cli/ginclient/log"
	"github.com/shibukawa/configdir"
	"github.com/spf13/viper"
)

var configDirs = configdir.New("g-node", "gin")

// NOTE: Duplicate function
// pathExists returns true if the path exists
func pathExists(path string) bool {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}
	return true
}

// NOTE: Duplicate function
func FindRepoRoot(path string) (string, error) {
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

	return FindRepoRoot(updir)
}

type GinConfiguration struct {
	GinHost    string
	GitHost    string
	GitUser    string
	GitHostKey string
	Bin        struct {
		Git      string
		GitAnnex string
		SSH      string
	}
	Annex struct {
		Exclude []string
		MinSize string
	}
}

// Read reads in the configuration and makes it available through Config package global
func Read() (Config GinConfiguration) {
	viper.Reset()
	viper.SetTypeByDefaultValue(true)
	// Binaries
	viper.SetDefault("bin.git", "git")
	viper.SetDefault("bin.gitannex", "git-annex")
	viper.SetDefault("bin.ssh", "ssh")

	// Hosts
	viper.SetDefault("gin.address", "https://web.gin.g-node.org")
	viper.SetDefault("gin.port", "443")

	viper.SetDefault("git.address", "gin.g-node.org")
	viper.SetDefault("git.port", "22")
	viper.SetDefault("git.user", "git")
	viper.SetDefault("git.hostkey", "gin.g-node.org,141.84.41.216 ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBE5IBgKP3nUryEFaACwY4N3jlqDx8Qw1xAxU2Xpt5V0p9RNefNnedVmnIBV6lA3n+9kT1OSbyqA/+SgsQ57nHo0=")

	// annex filters
	viper.SetDefault("annex.minsize", "10M")

	// Merge in user config file
	confpath, _ := Path(false)
	configFileName := "config.yml"
	confpath = filepath.Join(confpath, configFileName)

	viper.SetConfigFile(confpath)
	cerr := viper.MergeInConfig()
	if cerr == nil {
		log.Write("Found config file %s", confpath)
	}

	Config.Bin.Git = viper.GetString("bin.git")
	Config.Bin.GitAnnex = viper.GetString("bin.gitannex")
	Config.Bin.SSH = viper.GetString("bin.ssh")

	ginAddress := viper.GetString("gin.address")
	ginPort := viper.GetInt("gin.port")
	Config.GinHost = fmt.Sprintf("%s:%d", ginAddress, ginPort)

	gitAddress := viper.GetString("git.address")
	gitPort := viper.GetInt("git.port")
	Config.GitHost = fmt.Sprintf("%s:%d", gitAddress, gitPort)
	Config.GitUser = viper.GetString("git.user")
	Config.GitHostKey = viper.GetString("git.hostkey")

	// Config file in the repository root (annex excludes and size threshold only)
	reporoot, err := FindRepoRoot(".")
	if err == nil {
		confpath := filepath.Join(reporoot, configFileName)
		viper.SetConfigFile(confpath)
		cerr = viper.MergeInConfig()
		if cerr == nil {
			log.Write("Found config file %s", confpath)
		}
	}
	Config.Annex.Exclude = viper.GetStringSlice("annex.exclude")
	Config.Annex.MinSize = viper.GetString("annex.minsize")

	log.Write("Configuration values")
	log.Write("%+v", Config)

	// TODO: Validate URLs on config read
	return
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
