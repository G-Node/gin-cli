package ginclient

import (
	"bufio"
	"crypto/rand"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"testing"

	"github.com/G-Node/gin-cli/ginclient/config"
	"github.com/G-Node/gin-cli/git"
)

func setupClient() {

}

// TestMain sets up a configuration file and the GIN_CONFIG_DIR environment
// variable for tests. Tests calling config.Read() will receive the
// configuration set up by this function.
func TestMain(m *testing.M) {
	// Setup test config
	tmpconfdir, err := ioutil.TempDir("", "gin-cli-test-config-")
	if err != nil {
		os.Exit(-1)
	}
	// set config directory
	os.Setenv("GIN_CONFIG_DIR", tmpconfdir)
	config.SetConfig("annex.minsize", "100kb")
	config.SetConfig("annex.exclude", []string{"*.md", "*.git", "*.txt", "*.py"})
	res := m.Run()

	// Teardown test config
	os.RemoveAll(tmpconfdir)
	os.Exit(res)
}

// setupLocalRepo sets up a repository in a temporary directory with 'dir' type
// remote in another temporary directory.
func setupLocalRepoWithDirRemote(c *Client) (string, error) {
	local, err := ioutil.TempDir("", "gin-cli-test-repo-")
	if err != nil {
		return "", err
	}

	os.Chdir(local)
	err = c.InitDir(false)
	if err != nil {
		return "", err
	}

	remote, err := ioutil.TempDir("", "gin-cli-test-remote-")
	if err != nil {
		return "", err
	}
	gr := git.New(".")
	gr.RemoteAdd("origin", remote)

	return local, nil
}

// Creates a random file with a given name (path) and size.
func createFile(filepath string, size int64) error {
	fp, err := os.OpenFile(filepath, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer fp.Close()
	w := bufio.NewWriter(fp)
	defer w.Flush()
	n, err := io.CopyN(w, rand.Reader, size)
	if err != nil {
		return err
	}
	if n != size {
		return fmt.Errorf("Random file %s created but failed to write all data. %d bytes written, %d needed", filepath, n, size)
	}
	return nil
}

func TestInit(t *testing.T) {
	testclient := New("")

	testdir, err := ioutil.TempDir("", "InitTest")
	if err != nil {
		t.Fatalf("Failed to create temporary directory for test: %s", err.Error())
	}
	defer func() {
		err := os.RemoveAll(testdir)
		if err != nil {
			fmt.Printf("Failed to delete tempdir: %s", err.Error())
		}
	}()

	os.Chdir(testdir)
	err = testclient.InitDir(false)
	if err != nil {
		t.Fatalf("Failed to initialise local repository: %s", err.Error())
	}

	// Check annex info
	gr := git.New(".")
	info, err := gr.AnnexInfo()
	if err != nil {
		t.Fatalf("Failed to get annex info: %s", err.Error())
	}
	if !info.Success {
		t.Fatalf("Annex initialised but info retrieval was unsuccessful")
	}
}

func TestCommit(t *testing.T) {
	testclient := New("")
	_, err := setupLocalRepoWithDirRemote(testclient)

	if err != nil {
		t.Fatalf("Failed to initialise local and remote repositories: %s", err.Error())
	}

	gr := git.New(".")
	err = gr.Commit("")
	if err == nil {
		t.Fatalf("Empty commit should fail")
	}
	err = createFile("afile", 10)
	if err != nil {
		t.Fatalf("smallfile create failed: %s", err.Error())
	}
	conf := config.Read()
	addchan := gr.AnnexAdd([]string{"afile"}, conf.Annex.MinSize, conf.Annex.Exclude)
	for range addchan {
	}

	err = gr.Commit("Test commit")
	if err != nil {
		t.Fatalf("Commit failed: %s", err.Error())
	}
}

// TestCommitMinSize tests a single commit creation with size filtering (annex.minsize)
func TestCommitMinSize(t *testing.T) {
	testclient := New("")
	_, err := setupLocalRepoWithDirRemote(testclient)

	if err != nil {
		t.Fatalf("Failed to initialise local and remote repositories: %s", err.Error())
	}

	var smallsize int64 = 100       // 100 byte file (for git)
	var bigsize int64 = 1024 * 1024 // 1 MiB file (for annex)

	err = createFile("smallfile", smallsize)
	if err != nil {
		t.Fatalf("smallfile create failed: %s", err.Error())
	}
	err = createFile("bigfile", bigsize)
	if err != nil {
		t.Fatalf("bigfile create failed: %s", err.Error())
	}

	conf := config.Read()
	gr := git.New(".")
	addchan := gr.AnnexAdd([]string{"smallfile", "bigfile"}, conf.Annex.MinSize, conf.Annex.Exclude)
	for range addchan {
	}

	err = gr.Commit("Test commit")
	if err != nil {
		t.Fatalf("Commit failed: %s", err.Error())
	}

	gitobjs, err := gr.LsTree("HEAD", nil)
	if err != nil {
		t.Fatalf("git ls-tree failed: %s", err.Error())
	}
	if len(gitobjs) != 2 {
		t.Fatalf("Expected 2 git objects, got %d", len(gitobjs))
	}

	contents, err := gr.CatFileContents("HEAD", "smallfile")
	if err != nil {
		t.Fatalf("Couldn't read git file contents for smallfile")
	}
	if len(contents) != 100 {
		t.Fatalf("Git file content size doesn't match original file size: %d (expected 100)", len(contents))
	}

	contents, err = gr.CatFileContents("HEAD", "bigfile")
	if err != nil {
		t.Fatalf("Couldn't read annex file contents for bigfile")
	}
	if len(contents) == 1024*1024 {
		t.Fatalf("Annex file content was checked into git")
	}
	if len(contents) == 0 {
		t.Fatalf("Annex file not checked into git (content size == 0)")
	}
}

// TestCommitExcludes tests a single commit creation with pattern filtering (annex.excludes)
func TestCommitExcludes(t *testing.T) {
	testclient := New("")
	_, err := setupLocalRepoWithDirRemote(testclient)

	if err != nil {
		t.Fatalf("Failed to initialise local and remote repositories: %s", err.Error())
	}

	var fsize int64 = 1024 * 1024 // 1 MiB files, greater than annex.minsize

	fnames := []string{
		"bigmarkdown.md",
		"bigpython.py",
		"somegitfile.git",
		"plaintextfile.txt",
	}

	for _, fn := range fnames {
		err = createFile(fn, fsize)
		if err != nil {
			t.Fatalf("[%s] file creation failed: %s", fn, err.Error())
		}
	}

	conf := config.Read()
	gr := git.New(".")
	addchan := gr.AnnexAdd([]string{"."}, conf.Annex.MinSize, conf.Annex.Exclude)
	for range addchan {
	}

	err = gr.Commit("Test commit")
	if err != nil {
		t.Fatalf("Commit failed: %s", err.Error())
	}

	gitobjs, err := gr.LsTree("HEAD", nil)
	if err != nil {
		t.Fatalf("git ls-tree failed: %s", err.Error())
	}
	if len(gitobjs) != len(fnames) {
		t.Fatalf("Expected %d git objects, got %d", len(fnames), len(gitobjs))
	}

	// all file sizes in git should be fsize
	for _, fn := range fnames {
		contents, err := gr.CatFileContents("HEAD", fn)
		if err != nil {
			t.Fatalf("Couldn't read git file contents for %s", fn)
		}
		if int64(len(contents)) != fsize {
			t.Fatalf("Git file content size doesn't match original file size: %d (expected %d)", len(contents), fsize)
		}
	}
}
