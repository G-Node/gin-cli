package gincmd

import (
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"
	"time"

	ginclient "github.com/G-Node/gin-cli/ginclient"
	"github.com/G-Node/gin-cli/ginclient/config"
	"github.com/spf13/cobra"
)

func keys(cmd *cobra.Command, args []string) {
	flags := cmd.Flags()
	if flags.NFlag() > 1 {
		usageDie(cmd)
	}

	// TODO: add server flag
	conf := config.Read()
	gincl := ginclient.New(conf.DefaultServer)
	requirelogin(cmd, gincl, true)

	keyfilename, _ := flags.GetString("add")
	keyidx, _ := flags.GetInt("delete")
	verbose, _ := flags.GetBool("verbose")

	if keyfilename != "" {
		addKey(gincl, keyfilename)
		return
	}
	if keyidx > 0 {
		delKey(gincl, keyidx)
		return
	}
	printKeys(gincl, verbose)
}

func printKeys(gincl *ginclient.Client, verbose bool) {
	keys, err := gincl.GetUserKeys()
	CheckError(err)

	nkeys := len(keys)
	var plural string
	if nkeys == 1 {
		plural = ""
	} else {
		plural = "s"
	}

	var nkeysStr string
	if nkeys == 0 {
		nkeysStr = "no"
	} else {
		nkeysStr = fmt.Sprintf("%d", nkeys)
	}
	fmt.Printf("You have %s key%s associated with your account.\n\n", nkeysStr, plural)
	for idx, key := range keys {
		fmt.Printf("[%v] \"%s\"\n", idx+1, key.Title)
		if verbose {
			fmt.Printf("--- Key ---\n%s\n", key.Key)
		}
	}
}

func addKey(gincl *ginclient.Client, filename string) {
	keyBytes, err := ioutil.ReadFile(filename)
	CheckError(err)
	key := string(keyBytes)
	strSlice := strings.Split(key, " ")
	var description string
	if len(strSlice) > 2 {
		description = strings.TrimSpace(strSlice[2])
	} else {
		description = fmt.Sprintf("%s@%s", gincl.Username, strconv.FormatInt(time.Now().Unix(), 10))
	}

	err = gincl.AddKey(string(keyBytes), description, false)
	CheckError(err)
	fmt.Printf("New key added '%s'\n", description)
}

func delKey(gincl *ginclient.Client, idx int) {
	name, err := gincl.DeletePubKeyByIdx(idx)
	CheckError(err)
	fmt.Printf("Deleted key with name '%s'\n", name)
}

// KeysCmd sets up the 'keys' list, add, delete subcommand(s)
func KeysCmd() *cobra.Command {
	description := "List, add, or delete SSH keys. If no argument is provided, a numbered list of key names is printed. The key number can be used with the '--delete' flag to remove a key from the server.\n\nThe command can also be used to add a public key to your account from an existing filename (see '--add' flag)."
	examples := map[string]string{
		"Add a public key to your account, as generated from the default ssh-keygen command": "$ gin keys --add ~/.ssh/id_rsa.pub",
	}
	var cmd = &cobra.Command{
		Use:     "keys [--add <filename> | --delete <keynum> | --verbose | -v]",
		Short:   "List, add, or delete public keys on the GIN services",
		Long:    formatdesc(description, nil),
		Example: formatexamples(examples),
		Args:    cobra.MaximumNArgs(1),
		Run:     keys,
		DisableFlagsInUseLine: true,
	}
	cmd.Flags().String("add", "", "Specify a `filename` which contains a public key to be added to the GIN server.")
	cmd.Flags().Int("delete", 0, "Specify a `number` to delete the corresponding key from the server. Use 'gin keys' to get the numbered listing of keys.")
	cmd.Flags().BoolP("verbose", "v", false, "Verbose printing. Prints the entire public key.")
	return cmd
}
