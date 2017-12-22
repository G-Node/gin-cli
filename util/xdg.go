// Simple utility functions for getting configuration paths based on the XDG specification.

package util

import (
	"os"
	"os/user"
	"path/filepath"
)

var suffix = "gin"

// OldConfigPath is the old deprecated config path function. It's used to move the old configuration file to the new location.
func OldConfigPath() (string, error) {
	var path string
	var err error

	xdgconfig := os.Getenv("XDG_CONFIG_HOME")
	if xdgconfig != "" {
		path = filepath.Join(xdgconfig, suffix)
	} else {
		usr, ierr := user.Current()
		if ierr != nil {
			return "", ierr // platform does not implement Current()
		}
		homedir := usr.HomeDir
		path = filepath.Join(homedir, ".config", suffix)
	}

	return path, err
}

// OldDataPath is the old deprecated data path function. It's used to move the old log file to the new location.
func OldDataPath() (string, error) {
	var path string
	var err error

	xdgdata := os.Getenv("XDG_DATA_HOME")
	if xdgdata != "" {
		path = filepath.Join(xdgdata, suffix)
	} else {
		usr, ierr := user.Current()
		if ierr != nil {
			return "", ierr // platform does not implement Current()
		}
		homedir := usr.HomeDir
		path = filepath.Join(homedir, ".local/share", suffix)
	}

	return path, err
}
