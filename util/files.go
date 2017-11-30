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

// ExpandGlobs expands a list of globs into paths (files and directories).
func ExpandGlobs(paths []string) (globexppaths []string, err error) {
	if len(paths) == 0 {
		// Nothing to do
		globexppaths = paths
		return
	}
	// expand potential globs
	for _, p := range paths {
		LogWrite("ExpandGlobs: Checking for glob expansion for %s", p)
		exp, globerr := filepath.Glob(p)
		if globerr != nil {
			LogWrite(globerr.Error())
			LogWrite("Bad file pattern %s", p)
			return nil, globerr
		}
		if exp != nil {
			globexppaths = append(globexppaths, exp...)
		}
	}
	if len(globexppaths) == 0 {
		// Invalid paths
		LogWrite("ExpandGlobs: No files matched")
		err = fmt.Errorf("No files matched %v", paths)
	}
	return
}

func stringInSlice(element string, strlist []string) bool {
	for _, str := range strlist {
		if element == str {
			return true
		}
	}
	return false
}

// FilterPaths takes path descriptions (full paths, relative paths, files, or directories) and returns a slice of filepaths (as strings) excluding the files listed in excludes.
func FilterPaths(paths, excludes []string) (filtered []string) {

	// walker adds files to filtered if they don't match excludes
	walker := func(path string, info os.FileInfo, err error) error {
		if filepath.Base(path) == ".git" {
			return filepath.SkipDir
		}
		if info.IsDir() {
			return nil
		}
		if !stringInSlice(path, excludes) {
			filtered = append(filtered, path)
		}
		return nil
	}

	for _, p := range paths {
		err := filepath.Walk(p, walker)
		if err != nil {
			LogWrite("Error occured during path filtering: %s", err.Error())
		}
	}
	return
}

// FindRepoRoot starts from a given directory and searches upwards through a directory structure looking for the root of a repository, indicated by the existence of a .git directory.
// A path to the repository root is returned, or an error if the root of the filesystem is reached first.
// The returned path is absolute.
func FindRepoRoot(path string) (string, error) {
	path, _ = filepath.Abs(path)
	gitdir := filepath.Join(path, ".git")
	if PathExists(gitdir) {
		return path, nil
	}
	updir := filepath.Dir(path)
	if updir == path {
		// root reached
		return "", fmt.Errorf("Not a repository")
	}

	return FindRepoRoot(updir)
}
