package repo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/G-Node/gin-cli/auth"
	"github.com/G-Node/gin-cli/util"
)

// Keys

// Temporary (SSH key) file handling

var privKeyFile *util.TempFile

// MakeTempKeyPair creates a temporary key pair and stores it in a temporary directory.
// It also sets the global tempFile for use by the annex commands. The key pair is returned directly.
func (repocl *Client) MakeTempKeyPair() (*util.KeyPair, error) {
	tempKeyPair, err := util.MakeKeyPair()
	if err != nil {
		return nil, err
	}

	description := fmt.Sprintf("tmpkey@%s", strconv.FormatInt(time.Now().Unix(), 10))
	pubkey := fmt.Sprintf("%s %s", strings.TrimSpace(tempKeyPair.Public), description)
	authcl := auth.NewClient(repocl.KeyHost)
	err = authcl.AddKey(pubkey, description, true)
	if err != nil {
		return tempKeyPair, err
	}

	privKeyFile, err = util.SaveTempKeyFile(tempKeyPair.Private)
	if err != nil {
		return tempKeyPair, err
	}

	privKeyFile.Active = true

	return tempKeyPair, nil
}

// CleanUpTemp deletes the temporary directory which holds the temporary private key if it exists.
func CleanUpTemp() {
	if privKeyFile.Active {
		privKeyFile.Delete()
	}
}

// **************** //

// Git commands

// IsRepo checks whether a given path is a git repository.
func IsRepo(path string) bool {
	err := exec.Command("git", "status").Run()
	if err != nil {
		return false
	}
	return true
}

func publicKeyFile(file string) ssh.AuthMethod {
	buffer, err := ioutil.ReadFile(file)
	if err != nil {
		return nil
	}

	key, err := ssh.ParsePrivateKey(buffer)
	if err != nil {
		return nil
	}
	return ssh.PublicKeys(key)
}

// Connect opens a connection to the git server. This is used to validate credentials
// and generate temporary keys on demand, without performing a git operation.
// On Unix systems, the function will attempt to use the system's SSH agent.
// If no agent is running or the keys offered by the agent are not valid for the server,
// a temporary key pair is generated, the public key is uploaded to the auth server,
// and the private key is stored internally, to be used for subsequent functions.
func (repocl *Client) Connect() error {
	sshAgent, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK"))
	if err != nil {
		// No agent running - use temp keys
		_, err = repocl.MakeTempKeyPair()
		if err != nil {
			return fmt.Errorf("Error while creating temporary key for connection: %s", err.Error())
		}
		return nil
	}

	agent := ssh.PublicKeysCallback(agent.NewClient(sshAgent).Signers)

	sshConfig := &ssh.ClientConfig{
		User: repocl.GitUser,
		Auth: []ssh.AuthMethod{
			agent,
		},
	}

	connection, err := ssh.Dial("tcp", repocl.GitHost, sshConfig)
	if err != nil && strings.Contains(err.Error(), "unable to authenticate") {
		// Agent key authentication failed - use temp keys
		_, err = repocl.MakeTempKeyPair()
		if err != nil {
			return fmt.Errorf("Error while creating temporary key for connection: %s", err.Error())
		}
		return nil
	}
	// TODO: Attempt connection again after temp key is set up

	if err != nil {
		// Connection error (other than "unable to auth")
		return fmt.Errorf("Failed to dial: %s\n", err.Error())
	}
	defer connection.Close()

	session, err := connection.NewSession()
	if err != nil {
		return fmt.Errorf("Failed to create session: %s", err.Error())
	}
	defer session.Close()
	return nil
}

// Clone downloads a repository and sets the remote fetch and push urls.
// (git clone ...)
func (repocl *Client) Clone(repopath string) error {
	remotePath := fmt.Sprintf("ssh://%s@%s/%s", repocl.GitUser, repocl.GitHost, repopath)
	var cmd *exec.Cmd
	if privKeyFile.Active {
		cmd = exec.Command("git", "-c", privKeyFile.GitSSHOpt())
	} else {
		cmd = exec.Command("git")
	}
	cmd.Args = append(cmd.Args, "clone", remotePath)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	util.LogWrite("Running shell command: %s", strings.Join(cmd.Args, " "))
	err := cmd.Run()
	if err != nil {
		util.LogWrite("Error during clone command. [stderr] %s", stderr.String())
		return fmt.Errorf("Error retrieving repository")
	}
	return nil
}

// **************** //

// Git annex commands

// AnnexInit initialises the repository for annex
// (git annex init)
func AnnexInit(localPath string) error {
	initError := fmt.Errorf("Repository annex initialisation failed.")
	cmd := exec.Command("git", "-C", localPath, "annex", "init", "--version=6")
	if privKeyFile.Active {
		cmd.Args = append(cmd.Args, "-c", privKeyFile.AnnexSSHOpt())
	}
	util.LogWrite("Running shell command: %s", strings.Join(cmd.Args, " "))
	err := cmd.Run()
	if err != nil {
		// TODO: Collect and print stderr
		util.LogWrite("%s [stderr] %s", initError, "")
		return initError
	}

	cmd = exec.Command("git", "-C", localPath, "config", "annex.addunlocked", "true")
	util.LogWrite("Running shell command: %s", strings.Join(cmd.Args, " "))
	err = cmd.Run()
	if err != nil {
		return initError
	}

	// list of extensions that are added to git (not annex)
	exclfilters := util.Config.Annex.Exclude
	includes := make([]string, len(exclfilters))
	for idx, ext := range exclfilters {
		includes[idx] = fmt.Sprintf("include=*.%s", ext)
	}
	sizethreshold := util.Config.Annex.MinSize
	lfvalue := fmt.Sprintf("largerthan=%s and not (%s)", sizethreshold, strings.Join(includes, " or "))
	cmd = exec.Command("git", "-C", localPath, "config", "annex.largefiles", lfvalue)
	util.LogWrite("Running shell command: %s", strings.Join(cmd.Args, " "))
	err = cmd.Run()
	if err != nil {
		return initError
	}
	cmd = exec.Command("git", "-C", localPath, "config", "annex.backends", "WORM")
	util.LogWrite("Running shell command: %s", strings.Join(cmd.Args, " "))
	err = cmd.Run()
	if err != nil {
		return initError
	}
	cmd = exec.Command("git", "-C", localPath, "config", "annex.thin", "true")
	util.LogWrite("Running shell command: %s", strings.Join(cmd.Args, " "))
	err = cmd.Run()
	if err != nil {
		return initError
	}
	return nil
}

// AnnexPull downloads all annexed files.
// (git annex sync --no-push --content)
func AnnexPull(localPath string) error {
	cmd := exec.Command("git", "-C", localPath, "annex", "sync", "--no-push", "--content")
	if privKeyFile.Active {
		cmd.Args = append(cmd.Args, "-c", privKeyFile.AnnexSSHOpt())
	}
	util.LogWrite("Running shell command: %s", strings.Join(cmd.Args, " "))
	err := cmd.Run()

	if err != nil {
		// TODO: Collect and print stderr to log
		util.LogWrite("Error during AnnexPull. [stderr] %s", "")
		return fmt.Errorf("Error downloading files")
	}
	return nil
}

// AnnexSync synchronises the local repository with the remote.
// (git annex sync --content)
func AnnexSync(localPath string) error {
	cmd := exec.Command("git", "-C", localPath, "annex", "sync", "--content")
	if privKeyFile.Active {
		cmd.Args = append(cmd.Args, "-c", privKeyFile.AnnexSSHOpt())
	}
	util.LogWrite("Running shell command: %s", strings.Join(cmd.Args, " "))
	err := cmd.Run()

	if err != nil {
		// TODO: Collect and print stderr to log
		util.LogWrite("Error during AnnexSync. [stderr] %s", "")
		return fmt.Errorf("Error synchronising files")
	}
	return nil
}

// AnnexPush uploads all annexed files.
// (git annex sync --no-pull --content)
func AnnexPush(localPath, commitMsg string) error {
	cmd := exec.Command("git", "-C", localPath, "annex", "sync", "--no-pull", "--content", "--commit", fmt.Sprintf("--message=%s", commitMsg))
	if privKeyFile.Active {
		cmd.Args = append(cmd.Args, "-c", privKeyFile.AnnexSSHOpt())
	}
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	util.LogWrite("Running shell command: %s", strings.Join(cmd.Args, " "))
	err := cmd.Run()

	if err != nil {
		util.LogWrite("Error during AnnexPush. [stderr] %s", stderr.String())
		return fmt.Errorf("Error uploading files")
	}
	return nil
}

// AnnexAddResult ...
type AnnexAddResult struct {
	Command string `json:"command"`
	File    string `json:"file"`
	Key     string `json:"key"`
	Success bool   `json:"success"`
}

// AnnexAdd adds a path to the annex.
// (git annex add)
func AnnexAdd(localPath string) ([]string, error) {
	cmd := exec.Command("git", "annex", "--json", "add", localPath)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	util.LogWrite("Running shell command: %s", strings.Join(cmd.Args, " "))
	err := cmd.Run()

	if err != nil {
		util.LogWrite("Error during AnnexAdd. [stderr] %s", stderr.String())
		return nil, fmt.Errorf("Error adding files to repository.")
	}

	var outStruct AnnexAddResult
	files := bytes.Split(out.Bytes(), []byte("\n"))
	added := make([]string, 0, len(files))
	for _, f := range files {
		if len(f) == 0 {
			continue
		}
		err := json.Unmarshal(f, &outStruct)
		if err != nil {
			return nil, err
		}
		if !outStruct.Success {
			return nil, fmt.Errorf("Error adding files to repository: Failed to add %s", outStruct.File)
		}
		added = append(added, outStruct.File)
	}

	return added, nil
}

// AnnexWhereisResult holds the JSON output of a "git annex whereis" command
type AnnexWhereisResult struct {
	File      string   `json:"file"`
	Command   string   `json:"command"`
	Note      string   `json:"note"`
	Success   bool     `json:"success"`
	Untrusted []string `json:"untrusted"`
	Whereis   []struct {
		Here        bool     `json:"here"`
		UUID        string   `json:"uuid"`
		URLs        []string `json:"urls"`
		Description string   `json:"description"`
	}
	Key string `json:"key"`
}

// AnnexWhereis returns information about annexed files in the repository
// (git annex find)
func AnnexWhereis(localPath string) ([]AnnexWhereisResult, error) {
	cmd := exec.Command("git", "-C", localPath, "annex", "whereis", "--json")
	if privKeyFile.Active {
		cmd.Args = append(cmd.Args, "-c", privKeyFile.AnnexSSHOpt())
	}
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	util.LogWrite("Running shell command: %s", strings.Join(cmd.Args, " "))
	err := cmd.Run()

	if err != nil {
		util.LogWrite("Error during AnnexWhereis. [stderr] %s", stderr.String())
		return nil, fmt.Errorf("Error getting file status from server")
	}

	resultsJSON := bytes.Split(out.Bytes(), []byte("\n"))
	results := make([]AnnexWhereisResult, 0, len(resultsJSON))
	var res AnnexWhereisResult
	for _, resJSON := range resultsJSON {
		if len(resJSON) == 0 {
			continue
		}
		err := json.Unmarshal(resJSON, &res)
		if err != nil {
			return nil, err
		}
		results = append(results, res)
	}
	return results, nil
}

// AnnexStatusResult ...
type AnnexStatusResult struct {
	Status string `json:"status"`
	File   string `json:"file"`
}

// DescribeChanges returns a string which describes the status of the files in the working tree
// with respect to git annex. The resulting message can be used to inform the user of changes
// that are about to be uploaded and as a long commit message.
func DescribeChanges(localPath string) (string, error) {
	cmd := exec.Command("git", "-C", localPath, "annex", "status", "--json")
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	util.LogWrite("Running shell command: %s", strings.Join(cmd.Args, " "))
	err := cmd.Run()

	if err != nil {
		util.LogWrite("Error during DescribeChanges. [stderr] %s", stderr.String())
		return "", fmt.Errorf("Error retrieving file status")
	}

	var outStruct AnnexStatusResult
	files := bytes.Split(out.Bytes(), []byte("\n"))

	statusmap := make(map[string][]string)
	for _, f := range files {
		if len(f) == 0 {
			continue
		}
		err := json.Unmarshal(f, &outStruct)
		if err != nil {
			return "", err
		}
		statusmap[outStruct.Status] = append(statusmap[outStruct.Status], outStruct.File)
	}

	var changeList string
	changeList += makeFileList("New files", statusmap["A"])
	changeList += makeFileList("Modified files", statusmap["M"])
	changeList += makeFileList("Deleted files", statusmap["D"])
	changeList += makeFileList("Type modified files", statusmap["T"])
	changeList += makeFileList("Untracked files ", statusmap["?"])

	return changeList, nil
}

func makeFileList(header string, fnames []string) (list string) {
	if len(fnames) == 0 {
		return
	}
	list += fmt.Sprint(header) + "\n"
	for idx, name := range fnames {
		list += fmt.Sprintf("  %d: %s\n", idx+1, name)
	}
	list += "\n"
	return
}
