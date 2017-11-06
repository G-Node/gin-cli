package util

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/spf13/viper"
)

type conf struct {
	GinHost string
	GitHost string
	GitUser string
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

// Config makes the configuration options available after LoadConfig is called
var Config conf

// LoadConfig reads in the configuration and makes it available through Config package global
func LoadConfig() error {
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

	// annex filters
	viper.SetDefault("annex.minsize", "10M")

	// configpaths is a prioritised list of locations for finding configuration files (priority is lowest to highest)
	var configpaths []string

	// Global xdg config path
	xdgconfpath, err := ConfigPath(false)
	if err == nil {
		configpaths = append(configpaths, xdgconfpath)
	}
	// Second prio config files in the directory of the executable
	// this is useful for portable packaging
	execloc, err := os.Executable()
	if err == nil {
		execpath, _ := path.Split(execloc)
		configpaths = append(configpaths, execpath)
	}
	// Highest priority config file is in the repository root
	reporoot, err := FindRepoRoot(".")
	if err == nil {
		configpaths = append(configpaths, reporoot)
	}

	configFileName := "config.yml"
	for _, path := range configpaths {
		confPath := filepath.Join(path, configFileName)
		LogWrite("Reading config file %s", confPath)
		viper.SetConfigFile(confPath)
		_ = viper.MergeInConfig()
	}

	Config.Bin.Git = viper.GetString("bin.git")
	Config.Bin.GitAnnex = viper.GetString("bin.gitannex")
	Config.Bin.SSH = viper.GetString("bin.ssh")
	Config.Annex.Exclude = viper.GetStringSlice("annex.exclude")
	Config.Annex.MinSize = viper.GetString("annex.minsize")

	ginAddress := viper.GetString("gin.address")
	ginPort := viper.GetInt("gin.port")
	Config.GinHost = fmt.Sprintf("%s:%d", ginAddress, ginPort)

	gitAddress := viper.GetString("git.address")
	gitPort := viper.GetInt("git.port")
	Config.GitHost = fmt.Sprintf("%s:%d", gitAddress, gitPort)

	Config.GitUser = viper.GetString("git.user")

	LogWrite("Configuration values")
	LogWrite("%+v", Config)

	return nil
}
