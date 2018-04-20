package git

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/G-Node/gin-cli/ginclient/log"
	humanize "github.com/dustin/go-humanize"
)

// General utility functions for the git and git-annex shell commands and their output.

var annexmodecache = make(map[string]bool)

func makeFileList(header string, fnames []string) string {
	if len(fnames) == 0 {
		return ""
	}
	var filelist bytes.Buffer
	_, _ = filelist.WriteString(fmt.Sprintf("%s (%d)\n", header, len(fnames)))
	for idx, name := range fnames {
		_, _ = filelist.WriteString(fmt.Sprintf("  %d: %s\n", idx+1, name))
	}
	_, _ = filelist.WriteString("\n")
	return filelist.String()
}

func calcRate(dbytes int, dt time.Duration) string {
	dtns := dt.Nanoseconds()
	if dtns <= 0 || dbytes <= 0 {
		return ""
	}
	rate := int64(dbytes) * 1000000000 / dtns
	return fmt.Sprintf("%s/s", humanize.IBytes(uint64(rate)))
}

func logstd(out, err []byte) {
	log.LogWrite("[stdout]\n%s\n[stderr]\n%s", string(out), string(err))
}

func cutline(b []byte) (string, bool) {
	idx := -1
	cridx := bytes.IndexByte(b, '\r')
	nlidx := bytes.IndexByte(b, '\n')
	if cridx > 0 {
		idx = cridx
	} else {
		cridx = len(b) + 1
	}
	if nlidx > 0 && nlidx < cridx {
		idx = nlidx
	}
	if idx <= 0 {
		return string(b), true
	}
	return string(b[:idx]), false
}

// FindRepoRoot starts from a given directory and searches upwards through a directory structure looking for the root of a repository, indicated by the existence of a .git directory.
// A path to the repository root is returned, or an error if the root of the filesystem is reached first.
// The returned path is absolute.
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

// pathExists returns true if the path exists
func pathExists(path string) bool {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}
	return true
}

func stringInSlice(element string, strlist []string) bool {
	for _, str := range strlist {
		if element == str {
			return true
		}
	}
	return false
}

// filterpaths takes path descriptions (full paths, relative paths, files, or directories) and returns a slice of filepaths (as strings) excluding the files listed in excludes.
func filterpaths(paths, excludes []string) (filtered []string) {

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
			log.LogWrite("Error occured during path filtering: %s", err.Error())
		}
	}
	return
}
