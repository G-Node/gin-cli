package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/G-Node/gin-cli/ginclient"
	"github.com/G-Node/gin-cli/gincmd"
	util "github.com/G-Node/gin-cli/util"
	version "github.com/hashicorp/go-version"
)

var gincliversion string
var build string
var commit string
var verstr string
var minAnnexVersion = "6.20160126" // Introduction of git-annex add --json

func checkAnnexVersion() {
	errmsg := fmt.Sprintf("The GIN Client requires git-annex %s or newer", minAnnexVersion)
	verstring, err := ginclient.GetAnnexVersion()
	util.CheckError(err)
	systemver, err := version.NewVersion(verstring)
	if err != nil {
		// Special case for neurodebian git-annex version
		// The versionn string contains a tilde as a separator for the arch suffix
		// Cutting off the suffix and checking again
		verstring = strings.Split(verstring, "~")[0]
		systemver, err = version.NewVersion(verstring)
		if err != nil {
			// Can't figure out the version. Giving up.
			util.Die(fmt.Sprintf("%s\ngit-annex version %s not understood", errmsg, verstring))
		}
	}
	minver, _ := version.NewVersion(minAnnexVersion)
	if systemver.LessThan(minver) {
		util.Die(fmt.Sprintf("%s\nFound version %s", errmsg, verstring))
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
	err := util.LogInit(verstr)
	util.CheckError(err)
	defer util.LogClose()

	var args = make([]string, len(os.Args))
	for idx, a := range os.Args {
		args[idx] = a
		if strings.Contains(a, " ") {
			args[idx] = fmt.Sprintf("'%s'", a)
		}
	}
	util.LogWrite("COMMAND: %s", strings.Join(args, " "))
	cwd, _ := os.Getwd()
	util.LogWrite("CWD: %s", cwd)

	err = util.LoadConfig()
	util.CheckError(err)
	checkAnnexVersion()

	rootCmd := gincmd.SetUpCommands(verstr)
	rootCmd.SetVersionTemplate("{{ .Version }}")

	// Engage
	rootCmd.Execute()

	util.LogWrite("EXIT OK")
}
