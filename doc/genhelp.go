package main

import (
	"fmt"
	"os"

	gincmd "github.com/G-Node/gin-cli/cmd"
	"github.com/spf13/cobra/doc"
)

func main() {
	ginCmd := gincmd.SetUpCommands("")
	fmt.Println("Generating help files")

	var docdir = "cmdhelp"
	os.MkdirAll(docdir, 0755)
	err := doc.GenMarkdownTree(ginCmd, docdir)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("Done")
}
