package util

import (
	"fmt"
	"os"
	"path"
	"strings"

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

	viper.SetConfigName("config")

	// Highest priority config file is in the repository root
	reporoot, err := FindRepoRoot(".")
	if err == nil {
		viper.AddConfigPath(reporoot)
		LogWrite("Config path added %s", reporoot)
	}

	// Second prio config files in the directory of the executable
	// this is useful for portable packaging
	execloc, err := os.Executable()
	execpath, _ := path.Split(execloc)
	if err == nil {
		viper.AddConfigPath(execpath)
		LogWrite("Config path added %s", execpath)
	}

	xdgconfpath, _ := ConfigPath(false)
	viper.AddConfigPath(xdgconfpath)
	LogWrite("Config path added %s", xdgconfpath)

	err = viper.ReadInConfig()
	if err != nil {
		if !strings.Contains(err.Error(), "Not Found") {
			LogError(err)
		}
	}
	fileused := viper.ConfigFileUsed()
	if fileused != "" {
		LogWrite("Loading config file %s", viper.ConfigFileUsed())
	} else {
		LogWrite("No config file found. Using defaults.")
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
