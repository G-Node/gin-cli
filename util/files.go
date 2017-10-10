package util

import (
	"fmt"
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

// DataSize returns the simplest representation of bytes as a string (with units)
func DataSize(nbytes int) string {
	units := []string{"B", "KiB", "MiB", "GiB", "TiB", "PiB", "EiB", "ZiB", "YiB"}

	lastPositive := nbytes
	unitIdx := 0
	for b, c := nbytes, 0; b > 0 && c < len(units); b, c = b>>10, c+1 {
		lastPositive, unitIdx = b, c
	}
	return fmt.Sprintf("%d %s", lastPositive, units[unitIdx])
}

// ExpandPaths converts a list of path strings into a list of regular files.
// This includes recursively traversing directories and expanding globs.
func ExpandPaths(paths []string) ([]string, error) {
	var globexppaths, finalpaths []string

	// walker adds files to finalpaths
	walker := func(path string, info os.FileInfo, err error) error {
		if filepath.Base(path) == ".git" {
			LogWrite("Ignoring .git directory")
			return filepath.SkipDir
		}
		if info.IsDir() {
			LogWrite("%s is a directory; descending", path)
			return nil
		}

		LogWrite("Adding %s", path)
		finalpaths = append(finalpaths, path)
		return nil
	}

	// expand potential globs
	for _, p := range paths {
		LogWrite("ExpandPaths: Checking for glob expansion for %s", p)
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

	// traverse directories and list files (don't follow symlinks)
	for _, p := range globexppaths {
		LogWrite("ExpandPaths: Walking path %s", p)
		err := filepath.Walk(p, walker)
		if err != nil {
			return nil, err
		}
	}

	return finalpaths, nil
}
