package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/G-Node/gin-cli/ginclient/config"
	"github.com/G-Node/gin-cli/ginclient/log"
	"github.com/G-Node/gin-cli/gincmd"
	"github.com/G-Node/gin-cli/git"
	version "github.com/hashicorp/go-version"
)

var gincliversion string
var build string
var commit string
var verstr string
var minAnnexVersion = "6.20160126" // Introduction of git-annex add --json

func checkAnnexVersion() {
	errmsg := fmt.Sprintf("The GIN Client requires git-annex %s or newer", minAnnexVersion)
	verstring, err := git.GetAnnexVersion()
	gincmd.CheckError(err)
	systemver, err := version.NewVersion(verstring)
	if err != nil {
		// Special case for neurodebian git-annex version
		// The versionn string contains a tilde as a separator for the arch suffix
		// Cutting off the suffix and checking again
		verstring = strings.Split(verstring, "~")[0]
		systemver, err = version.NewVersion(verstring)
		if err != nil {
			// Can't figure out the version. Giving up.
			gincmd.Die(fmt.Sprintf("%s\ngit-annex version %s not understood", errmsg, verstring))
		}
	}
	minver, _ := version.NewVersion(minAnnexVersion)
	if systemver.LessThan(minver) {
		gincmd.Die(fmt.Sprintf("%s\nFound version %s", errmsg, verstring))
	}
}

func printversion(args []string) {
	jsonout := true // TODO: make this a new command
	if jsonout {
		verjson := struct {
			Version string `json:"version"`
			Build   string `json:"build"`
			Commit  string `json:"commit"`
		}{
			gincliversion,
			build,
			commit,
		}
		verjsonstr, _ := json.Marshal(verjson)
		fmt.Println(string(verjsonstr))
	} else {
		fmt.Println(verstr)
	}
}

func init() {
	if gincliversion == "" {
		verstr = "GIN command line client [dev build]"
	} else {
		verstr = fmt.Sprintf("GIN command line client %s Build %s (%s)", gincliversion, build, commit)
	}
}

func main() {
	err := log.Init(verstr)
	gincmd.CheckError(err)
	defer log.Close()

	var args = make([]string, len(os.Args))
	for idx, a := range os.Args {
		args[idx] = a
		if strings.Contains(a, " ") {
			args[idx] = fmt.Sprintf("'%s'", a)
		}
	}
	log.Write("COMMAND: %s", strings.Join(args, " "))
	cwd, _ := os.Getwd()
	log.Write("CWD: %s", cwd)

	err = config.LoadConfig()
	gincmd.CheckError(err)
	checkAnnexVersion()

	rootCmd := gincmd.SetUpCommands(verstr)
	rootCmd.SetVersionTemplate("{{ .Version }}")

	// Engage
	rootCmd.Execute()

	log.Write("EXIT OK")
}
