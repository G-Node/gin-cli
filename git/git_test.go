package git

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func cleanupdir(path string) {
	err := os.RemoveAll(path)
	if err != nil {
		fmt.Printf("Temporary directory cleanup failed: %s\n", err.Error())
	}
}

// TestMain sets up a temporary git configuration directory to avoid effects
// from user or local git configurations.
func TestMain(m *testing.M) {
	// Setup test config
	tmpconfdir, err := ioutil.TempDir("", "git-test-config-")
	if err != nil {
		os.Exit(-1)
	}
	// set temporary git config file path and disable systemwide
	os.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	os.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpconfdir, "gitconfig"))

	// set git user
	SetGitUser("testuser", "")
	res := m.Run()

	// Teardown test config
	os.RemoveAll(tmpconfdir)
	os.Exit(res)
}

func TestInit(t *testing.T) {
	tmpgitdir, _ := ioutil.TempDir("", "git-init-test-")
	os.Chdir(tmpgitdir)

	defer cleanupdir(tmpgitdir)

	err := Init(false)
	if err != nil {
		t.Fatalf("Failed to initialise (non-bare) repository: %s", err.Error())
	}

	bare, _ := ConfigGet("core.bare")
	if bare != "false" {
		t.Fatalf("Expected non-bare repository: %s", bare)
	}

	// Testing bare repository init
	tmpbaredir, _ := ioutil.TempDir("", "git-init-bare-test-")
	os.Chdir(tmpbaredir)

	defer cleanupdir(tmpbaredir)

	err = Init(true)
	if err != nil {
		t.Fatalf("Failed to initialise bare repository: %s", err.Error())
	}

	bare, _ = ConfigGet("core.bare")
	if bare != "true" {
		t.Fatalf("Expected bare repository: %s", bare)
	}
}
