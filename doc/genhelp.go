package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/G-Node/gin-cli/gincmd"
	"github.com/alecthomas/template"
	"github.com/bbrks/wrap"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var helpfilename = "GinClientHelp.md"

var order = []string{
	"login",
	"logout",
	"create",
	"get",
	"download",
	"upload",
	"ls",
	"get-content",
	"remove-content",
	"lock",
	"unlock",
}

var skip = []string{
	"delete",
	"git",
	"annex",
	"version",
}

var mdTemplate = `## {{ .Short }}

    {{ .UseLine }}

{{ . | fmtargs }}

{{ . | fmtflags }}

{{ . | fmtexamples }}

`

var wrapper = wrap.NewWrapper()

func fmtargs(cmd *cobra.Command) (fdescription string) {
	fdescription = cmd.Long
	fdescription = strings.Replace(fdescription, "<", "- `<", -1)
	fdescription = strings.Replace(fdescription, ">", ">`\n", -1)
	fdescription = strings.Replace(fdescription, "Description:", "**Description**", -1)
	fdescription = strings.Replace(fdescription, "Arguments:", "**Arguments**", -1)
	return
}

func fmtflags(cmd *cobra.Command) (fflags string) {
	flags := cmd.Flags()

	buf := new(bytes.Buffer)

	ff := func(flg *pflag.Flag) {
		_, usage := pflag.UnquoteUsage(flg)
		buf.WriteString(fmt.Sprintf("- `--%s`: %s\n", flg.Name, usage))
	}

	flags.VisitAll(ff)

	if buf.Len() > 0 {
		fflags = fmt.Sprintf("**Flags**\n\n%s", buf.String())
	}

	return
}

func fmtexamples(cmd *cobra.Command) (fexamples string) {
	fexamples = cmd.Example
	if fexamples == "" {
		return
	}
	fexamples = strings.Replace(fexamples, "$", "\n    $", -1)
	fexamples = fmt.Sprintf("**Examples**\n\n%s", fexamples)
	return
}

func checkError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func gendoc(cmd *cobra.Command) string {
	fmt.Printf("Generating help text for command %s\n", cmd.Name())

	funcMap := template.FuncMap{
		"fmtargs":     fmtargs,
		"fmtflags":    fmtflags,
		"fmtexamples": fmtexamples,
	}
	t, err := template.New("command").Funcs(funcMap).Parse(mdTemplate)
	checkError(err)

	rendered := new(bytes.Buffer)
	err = t.Execute(rendered, cmd)
	checkError(err)

	return rendered.String()
}

func main() {
	rootcmd := gincmd.SetUpCommands("")
	fmt.Println("Generating help file")

	buf := new(bytes.Buffer)

	header := `
# GIN detailed command overview

This page describes the purpose and usage of each command of the GIN command line client by elaborating on its *Usage*, *Description*, and *Arguments*.

The same information can be obtained from the client with the commands

`
	buf.WriteString(header)
	buf.WriteString("```\ngin help\ngin help <command>\n```\n\n")

	cmdmap := make(map[string]*cobra.Command)
	for _, subcmd := range rootcmd.Commands() {
		cmdmap[subcmd.Name()] = subcmd
	}

	// Remove entries found in skip
	for _, name := range skip {
		delete(cmdmap, name)
	}

	// Generate ordered entries first
	for _, name := range order {
		subcmd := cmdmap[name]
		buf.WriteString(gendoc(subcmd))
		delete(cmdmap, name)
	}

	// Generate any remaining subcommands
	for name, subcmd := range cmdmap {
		buf.WriteString(gendoc(subcmd))
		delete(cmdmap, name)
	}

	fmt.Printf("Writing to file %s\n", helpfilename)
	fb, err := os.Create(helpfilename)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer fb.Close()

	buf.WriteTo(fb)
	fmt.Println("Done")
}
