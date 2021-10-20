package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/G-Node/gin-cli/ginclient/log"
	"github.com/G-Node/gin-cli/gincmd"
	"github.com/G-Node/gin-cli/git"
)

// Version strings are populated using linker flags //

var (
	gincliversion string
	build         string
	commit        string
	verinfo       gincmd.VersionInfo
)

// ================================================ //

func init() {
	verinfo.Version = gincliversion
	verinfo.Build = build
	verinfo.Commit = commit

	gitVer, err := git.GetGitVersion()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get git version: %s", err.Error())
		gitVer = ""
	}
	verinfo.Git = gitVer

	annexVer, err := git.GetAnnexVersion()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get git-annex version: %s", err.Error())
		annexVer = ""
	}
	verinfo.Annex = annexVer
	err = log.Init()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to initialise log file")
	}
	log.Write("VERSION: %s", verinfo.String())
}

func main() {
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

	rootCmd := gincmd.SetUpCommands(verinfo)
	rootCmd.SetVersionTemplate("{{ .Version }}")

	// Engage
	rootCmd.Execute()

	log.Write("EXIT OK")
}
