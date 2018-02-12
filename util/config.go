package util

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"github.com/shibukawa/configdir"
	"github.com/spf13/viper"
)

var configDirs = configdir.New("g-node", "gin")

type conf struct {
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

// Config makes the configuration options available after LoadConfig is called
var Config conf

func moveOldFiles(newpath string) {
	// move old files and clear old config path
	var movemessages []string
	moveconflicts := false
	oldpath, _ := OldConfigPath()
	if _, operr := os.Stat(oldpath); !os.IsNotExist(operr) {
		files, _ := ioutil.ReadDir(oldpath)
		for _, file := range files {
			oldfilename := file.Name()
			oldfilepath := path.Join(oldpath, oldfilename)
			newfilepath := path.Join(newpath, oldfilename)
			for counter := 0; ; counter++ {
				if _, operr := os.Stat(newfilepath); os.IsNotExist(operr) {
					_, err := ConfigPath(true)
					if err != nil {
						// Config directory could not be created. Can't move files.
						return
					}
					os.Rename(oldfilepath, newfilepath)
					msg := fmt.Sprintf("%s -> %s", oldfilepath, newfilepath)
					movemessages = append(movemessages, msg)
					LogWrite("Moving old config file: %s", msg)
					break
				} else {
					// File already exists - rename to old and place alongside
					newfilepath = path.Join(newpath, fmt.Sprintf("%s.old.%d", oldfilename, counter))
					moveconflicts = true
				}
			}
		}
	}

	if len(movemessages) > 0 {
		fmt.Fprintln(os.Stderr, "NOTICE: Configuration directory changed.")
		fmt.Fprintln(os.Stderr, "The location of the configuration directory has changed.")
		fmt.Fprint(os.Stderr, "Any existing config file, token, and key have been moved to the new location.\n\n")
		for _, msg := range movemessages {
			fmt.Fprintln(os.Stderr, "\t", msg)
		}
		if moveconflicts {
			fmt.Fprint(os.Stderr, "\nSome files were renamed to avoid overwriting new ones.\nYou may want to review the contents of the new configuration directory:\n\n")
			fmt.Fprintln(os.Stderr, "\t", newpath)
		}
		fmt.Fprintln(os.Stderr, "\nThis message should not appear again.")
		fmt.Fprintln(os.Stderr, "END OF NOTICE")

		// Make sure old config directory is empty and remove
		files, _ := ioutil.ReadDir(oldpath)
		if len(files) == 0 {
			os.Remove(oldpath)
		}
		// Pause for the user to notice
		fmt.Print("Press the Enter key to continue")
		fmt.Scanln()
	}
}

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
	viper.SetDefault("git.hostkey", "gin.g-node.org,141.84.41.216 ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBE5IBgKP3nUryEFaACwY4N3jlqDx8Qw1xAxU2Xpt5V0p9RNefNnedVmnIBV6lA3n+9kT1OSbyqA/+SgsQ57nHo0=")

	// annex filters
	viper.SetDefault("annex.minsize", "10M")

	// Merge in user config file
	confpath, _ := ConfigPath(false)
	configFileName := "config.yml"
	confpath = filepath.Join(confpath, configFileName)

	LogWrite("Reading config file %s", confpath)
	viper.SetConfigFile(confpath)
	_ = viper.MergeInConfig()

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
		_ = viper.MergeInConfig()
	}

	Config.Annex.Exclude = viper.GetStringSlice("annex.exclude")
	Config.Annex.MinSize = viper.GetString("annex.minsize")

	LogWrite("Configuration values")
	LogWrite("%+v", Config)

	// TODO: Validate URLs on config read

	return nil
}

// ConfigPath returns the configuration path where configuration files should be stored.
// If the GIN_CONFIG_DIR environment variable is set, its value is returned, otherwise the platform default is used.
// If create is true and the directory does not exist, the full path is created.
func ConfigPath(create bool) (string, error) {
	confpath := os.Getenv("GIN_CONFIG_DIR")
	if confpath == "" {
		confpath = configDirs.QueryFolders(configdir.Global)[0].Path
		moveOldFiles(confpath) // move old files only if default is used
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
