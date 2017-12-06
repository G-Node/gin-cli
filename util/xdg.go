// Simple utility functions for getting configuration paths based on the XDG specification.

package util

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
)

var suffix = "gin"

// OldConfigPath is the old deprecated config path function. It's used to move the old configuration file to the new location.
func OldConfigPath(create bool) (string, error) {
	var path string
	var err error

	xdgconfig := os.Getenv("XDG_CONFIG_HOME")
	if xdgconfig != "" {
		path = filepath.Join(xdgconfig, suffix)
	} else {
		usr, err := user.Current()
		if err != nil {
			return "", fmt.Errorf("Error getting user home dir")
		}
		homedir := usr.HomeDir
		path = filepath.Join(homedir, ".config", suffix)
	}

	if create {
		err = os.MkdirAll(path, 0777)
		if err != nil {
			return "", fmt.Errorf("Error creating directory %s", path)
		}
	}
	return path, err
}

// OldDataPath is the old deprecated data path function. It's used to move the old log file to the new location.
func OldDataPath(create bool) (string, error) {
	var path string
	var err error

	xdgdata := os.Getenv("XDG_DATA_HOME")
	if xdgdata != "" {
		path = filepath.Join(xdgdata, suffix)
	} else {
		usr, err := user.Current()
		if err != nil {
			return "", fmt.Errorf("Error getting user home dir")
		}
		homedir := usr.HomeDir
		path = filepath.Join(homedir, ".local/share", suffix)
	}

	if create {
		err = os.MkdirAll(path, 0777)
		if err != nil {
			return "", fmt.Errorf("Error creating directory %s", path)
		}
	}

	return path, err
}
