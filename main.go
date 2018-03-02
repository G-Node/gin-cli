package main

import (
	"encoding/json"
	"fmt"
	"strings"

	gincmd "github.com/G-Node/gin-cli/cmd"
	ginclient "github.com/G-Node/gin-cli/gin-client"
	util "github.com/G-Node/gin-cli/util"
	version "github.com/hashicorp/go-version"
	cobra "github.com/spf13/cobra"
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

	err = util.LoadConfig()
	util.CheckError(err)
	checkAnnexVersion()

	var rootCmd = &cobra.Command{
		Use:                   "gin",
		Long:                  "GIN Command Line Interface and client for the GIN services", // TODO: Add license and web info
		Version:               fmt.Sprintln(verstr),
		DisableFlagsInUseLine: true,
	}
	rootCmd.SetVersionTemplate("{{ .Version }}")

	cobra.AddTemplateFunc("wrappedFlagUsages", wrappedFlagUsages)
	rootCmd.SetHelpTemplate(helpTemplate)
	rootCmd.SetUsageTemplate(usageTemplate)

	// Login
	rootCmd.AddCommand(gincmd.LoginCmd())

	// Logout
	rootCmd.AddCommand(gincmd.LogoutCmd())

	// Create repo
	rootCmd.AddCommand(gincmd.CreateCmd())

	// Delete repo (unlisted)
	rootCmd.AddCommand(gincmd.DeleteCmd())

	// Get repo
	rootCmd.AddCommand(gincmd.GetCmd())

	// List files
	rootCmd.AddCommand(gincmd.LsRepoCmd())

	// Unlock content
	rootCmd.AddCommand(gincmd.UnlockCmd())

	// Lock content
	rootCmd.AddCommand(gincmd.LockCmd())

	// Upload
	rootCmd.AddCommand(gincmd.UploadCmd())

	// Download
	rootCmd.AddCommand(gincmd.DownloadCmd())

	// Get content
	rootCmd.AddCommand(gincmd.GetContentCmd())

	// Remove content
	rootCmd.AddCommand(gincmd.RemoveContentCmd())

	// Account info
	rootCmd.AddCommand(gincmd.InfoCmd())

	// List repos
	rootCmd.AddCommand(gincmd.ReposCmd())

	// Keys
	rootCmd.AddCommand(gincmd.KeysCmd())

	// git and annex passthrough (unlisted)
	rootCmd.AddCommand(gincmd.GitCmd())
	rootCmd.AddCommand(gincmd.AnnexCmd())

	// Engage
	rootCmd.Execute()

	util.LogWrite("EXIT OK")
}
