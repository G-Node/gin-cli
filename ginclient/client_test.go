package ginclient

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/G-Node/gin-cli/ginclient/config"
)

var testsrvcfg = config.ServerCfg{
	Web: config.WebCfg{
		Protocol: "http",
		Host:     "localhost",
		Port:     3000,
	},
	Git: config.GitCfg{
		Host: "localhost",
		Port: 22,
		User: "git",
	},
}

// var testconf = config.GinCliCfg{
// 	Servers:       map[string]config.ServerCfg{"test": testsrvcfg},
// 	DefaultServer: "test",
// 	Bin: config.BinCfg{
// 		Git:      "git",
// 		GitAnnex: "git-annex",
// 		SSH:      "ssh",
// 	},
// 	Annex: config.AnnexCfg{
// 		Exclude: nil,
// 		MinSize: "10M",
// 	},
// }

func TestMain(m *testing.M) {
	tmpconfdir, err := ioutil.TempDir("", "gin-cli-test-config")
	if err != nil {
		os.Exit(-1)
	}
	defer os.RemoveAll(tmpconfdir)
	// set config directory
	os.Setenv("GIN_CONFIG_DIR", tmpconfdir)
	os.Exit(m.Run())
}

func TestInit(t *testing.T) {
	config.AddServerConf("gin", testsrvcfg)
	testclient := New("gin")
	err := testclient.Login("testuser", "a test password 42", "gin-cli-test")
	if err != nil {
		t.Errorf("Failed to login: %s", err)
	}

	// user, err := testclient.RequestAccount("testuser")
	// if err != nil {
	// 	t.Errorf("Failed to get user account: %s", err)
	// }
	// fmt.Printf("%+v\n", user)

	fmt.Println(testclient.Host)
	testdir, err := ioutil.TempDir("", "InitTest")
	if err != nil {
		t.Errorf("Failed to create temporary directory for test: %s", err)
	}
	defer os.RemoveAll(testdir)
	os.Chdir(testdir)

	err = testclient.InitDir(false)
	if err != nil {
		t.Errorf("Failed to initialise local repository: %s", err)
	}

	err = testclient.InitDir(false)
	if err != nil {
		t.Errorf("Failed to initialise local repository: %s", err)
	}
}
