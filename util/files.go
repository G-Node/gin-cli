package util

import (
	"os"
	"path/filepath"
)

// PathExists returns true if the path exists
func PathExists(path string) bool {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}
	return true
}

// PathSplit returns the directory path separated from the filename. If the
// argument is a directory, the filename "." is returned. Errors are ignored.
func PathSplit(path string) (string, string) {
	info, err := os.Stat(path)
	if err != nil {
		return path, "."
	}
	var dir, filename string
	switch mode := info.Mode(); {
	case mode.IsRegular():
		dir, filename = filepath.Split(path)
	case mode.IsDir():
		dir = path
		filename = "."
	}
	return dir, filename
}

// ExpandGlobs expands a list of globs into paths (files and directories).
func ExpandGlobs(paths []string) (globexppaths []string, err error) {
	// expand potential globs
	for _, p := range paths {
		LogWrite("ExpandGlobs: Checking for glob expansion for %s", p)
		exp, err := filepath.Glob(p)
		if err != nil {
			LogWrite(err.Error())
			LogWrite("Bad file pattern %s", p)
			return nil, err
		}
		if exp != nil {
			globexppaths = append(globexppaths, exp...)
		}
	}
	return
}
