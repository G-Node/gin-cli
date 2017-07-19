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

// Temporary (SSH key) file handling
var privKeyFile util.TempFile

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

// FileStatus represents the state a file is in with respect to local and remote changes.
type FileStatus uint8

const (
	// Synced indicates that an annexed file is synced between local and remote
	Synced FileStatus = iota
	// NoContent indicates that a file represents an annexed file that has not had its contents synced yet
	NoContent
	// Modified indicatres that a file has local modifications that have not been committed
	Modified
	// LocalChanges indicates that a file has local, committed modifications that have not been pushed
	LocalChanges
	// RemoteChanges indicates that a file has remote modifications that have not been pulled
	RemoteChanges
	// Untracked indicates that a file is not being tracked by neither git nor git annex
	Untracked
)

// Description returns the long description of the file status
func (fs FileStatus) Description() string {
	switch {
	case fs == Synced:
		return "Synced"
	case fs == NoContent:
		return "No local content"
	case fs == Modified:
		return "Locally modified (unsaved)"
	case fs == LocalChanges:
		return "Locally modified (not uploaded)"
	case fs == RemoteChanges:
		return "Remotely modified (not downloaded)"
	case fs == Untracked:
		return "Untracked"
	default:
		return "Unknown"
	}
}

// Abbrev returns the two-letter abbrevation of the file status
// OK (Synced), NC (NoContent), MD (Modified), LC (LocalUpdates), RC (RemoteUpdates), ?? (Untracked)
func (fs FileStatus) Abbrev() string {
	switch {
	case fs == Synced:
		return "OK"
	case fs == NoContent:
		return "NC"
	case fs == Modified:
		return "MD"
	case fs == LocalChanges:
		return "LC"
	case fs == RemoteChanges:
		return "RC"
	case fs == Untracked:
		return "??"
	default:
		return "??"
	}
}

// FileStatusSlice is a slice of FileStatus which implements Len() and Less() to allow sorting.
type FileStatusSlice []FileStatus

// Len is the number of elements in FileStatusSlice.
func (fsSlice FileStatusSlice) Len() int {
	return len(fsSlice)
}

// Swap swaps the elements with intexes i and j.
func (fsSlice FileStatusSlice) Swap(i, j int) {
	fsSlice[i], fsSlice[j] = fsSlice[j], fsSlice[i]
}

// Less reports whether the element with index i should sort before the element with index j.
func (fsSlice FileStatusSlice) Less(i, j int) bool {
	return fsSlice[i] < fsSlice[j]
}

// ListFiles lists the files in the specified directory and their sync status.
func ListFiles(paths []string) (map[string]FileStatus, error) {

	statuses := make(map[string]FileStatus)

	wiResults, err := AnnexWhereis(paths)
	if err != nil {
		return statuses, err
	}
	for _, status := range wiResults {
		fname := status.File
		statuses[fname] = Untracked
		if !status.Success {
			// default to untracked --- perhaps there should be an error state instead
			continue
		}
		for _, remote := range status.Whereis {
			if remote.Here {
				if len(status.Whereis) > 1 {
					statuses[fname] = Synced
				} else {
					statuses[fname] = LocalChanges
				}
				break
			}
		}
	}
	return statuses, nil
	// if err != nil {
	// 	// File does not exist. Different status???
	// 	return Untracked
	// }

	// function only runs for one file
	// if len(wiRes) > 0 {
	// 	for _, remote := range wiRes[0].Whereis {
	// 		if remote.Here {
	// 			return Synced
	// 		}
	// 	}
	// 	return NoContent
	// }

	// not in annex, but AnnexStatus can still tell us the file status
	// annexStat, err := AnnexStatus(filepath)
	// if err == nil && len(annexStat) > 0 {
	// 	switch stat := annexStat[0].Status; {
	// 	case stat == "M" || stat == "A":
	// 		return Modified
	// 	case stat == "?":
	// 		return Untracked
	// 	}
	// }

	// committed but not pushed?
	// TODO: use default remote/branch
	// stdout, stderr, err := RunGitCommand("diff", "--name-only", "origin/master", filepath)
	// if err != nil {
	// 	// Error out?
	// 	util.LogWrite("Error during diff command for status")
	// 	util.LogWrite("[stdout]\r\n%s", stdout.String())
	// 	util.LogWrite("[stderr]\r\n%s", stderr.String())
	// 	return Untracked
	// }
	// if stdout.Len() > 0 {
	// 	return LocalChanges
	// }

	// return Untracked
}

// Git commands

// IsRepo checks whether a given path is (in) a git repository.
// This function assumes path is a directory and will return false for files.
func IsRepo(path string) bool {
	gitbin := util.Config.Bin.Git
	cmd := exec.Command(gitbin, "status")
	cmd.Dir = path
	err := cmd.Run()
	return err == nil
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
	util.LogWrite("Checking connection to git server")
	sshAgent, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK"))
	if err != nil {
		// No agent running - use temp keys
		util.LogWrite("No agent running. Setting up temporary keys")
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
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	util.LogWrite("Attempting connection with key from SSH agent")
	connection, err := ssh.Dial("tcp", repocl.GitHost, sshConfig)
	if err != nil && strings.Contains(err.Error(), "unable to authenticate") {
		// Agent key authentication failed - use temp keys
		util.LogWrite("Auth key authentication failed. Setting up temporary keys")
		_, err = repocl.MakeTempKeyPair()
		if err != nil {
			return fmt.Errorf("Error while creating temporary key for connection: %s", err.Error())
		}
		return nil
	}
	// TODO: Attempt connection again after temp key is set up

	if err != nil {
		// Connection error (other than "unable to auth")
		return fmt.Errorf("Failed to connect to git host: %s", err.Error())
	}
	defer connection.Close()

	session, err := connection.NewSession()
	util.LogWrite("Creating SSH session")
	if err != nil {
		return fmt.Errorf("Failed to create session: %s", err.Error())
	}
	defer session.Close()
	util.LogWrite("Connection to git server OK")
	return nil
}

func splitRepoParts(repoPath string) (repoOwner, repoName string) {
	repoPathParts := strings.SplitN(repoPath, "/", 2)
	repoOwner = repoPathParts[0]
	repoName = repoPathParts[1]
	return
}

// Clone downloads a repository and sets the remote fetch and push urls.
// (git clone ...)
func (repocl *Client) Clone(repoPath string) error {
	remotePath := fmt.Sprintf("ssh://%s@%s/%s", repocl.GitUser, repocl.GitHost, repoPath)
	stdout, stderr, err := RunGitCommand("clone", remotePath)
	if err != nil {
		util.LogWrite("Error during clone command")
		util.LogWrite("[stdout]\r\n%s", stdout.String())
		util.LogWrite("[stderr]\r\n%s", stderr.String())
		repoOwner, repoName := splitRepoParts(repoPath)

		if strings.Contains(stderr.String(), "Server returned non-OK status: 404") {
			return fmt.Errorf("Error retrieving repository.\n"+
				"Please make sure you typed the repository path correctly.\n"+
				"Type 'gin repos %s' to see if the repository exists and if you have access to it.",
				repoOwner)
		} else if strings.Contains(stderr.String(), "already exists and is not an empty directory") {
			return fmt.Errorf("Error retrieving repository.\n"+
				"'%s' already exists in the current directory and is not empty.", repoName)
		} else {
			return fmt.Errorf("Error retrieving repository.\nAn unknown error occured.")
		}
	}
	return nil
}

// **************** //

// Git annex commands

// AnnexInit initialises the repository for annex
// (git annex init)
func AnnexInit(localPath, description string) error {
	stdout, stderr, err := RunAnnexCommand(localPath, "init", description)
	if err != nil {
		initError := fmt.Errorf("Repository annex initialisation failed.")
		util.LogWrite(initError.Error())
		util.LogWrite("[stdout]\r\n%s", stdout.String())
		util.LogWrite("[stderr]\r\n%s", stderr.String())
		return initError
	}
	return nil
}

// AnnexPull downloads all annexed files.
// (git annex sync --no-push --content)
func AnnexPull(localPath string) error {
	stdout, stderr, err := RunAnnexCommand(localPath, "sync", "--no-push", "--content")
	if err != nil {
		util.LogWrite("Error during AnnexPull.")
		util.LogWrite("[stdout]\r\n%s", stdout.String())
		util.LogWrite("[stderr]\r\n%s", stderr.String())
		return fmt.Errorf("Error downloading files")
	}
	return nil
}

// AnnexSync synchronises the local repository with the remote.
// Optionally synchronises content if content=True
// (git annex sync [--content])
func AnnexSync(localPath string, content bool) error {
	var contentarg string
	if content {
		contentarg = "--content"
	}
	stdout, stderr, err := RunAnnexCommand(localPath, "sync", contentarg)

	if err != nil {
		util.LogWrite("Error during AnnexSync")
		util.LogWrite("[stdout]\r\n%s", stdout.String())
		util.LogWrite("[stderr]\r\n%s", stderr.String())
		return fmt.Errorf("Error synchronising files")
	}
	return nil
}

// AnnexPush uploads all annexed files.
// (git annex sync --no-pull --content)
func AnnexPush(localPath, commitMsg string) error {
	stdout, stderr, err := RunAnnexCommand(
		localPath, "sync", "--no-pull", "--content",
		fmt.Sprintf("--content-of=%s", localPath),
		"--commit", fmt.Sprintf("--message=%s", commitMsg),
	)

	if err != nil {
		util.LogWrite("Error during AnnexPush")
		util.LogWrite("[Error]: %v", err)
		util.LogWrite("[stdout]\r\n%s", stdout.String())
		util.LogWrite("[stderr]\r\n%s", stderr.String())
		return fmt.Errorf("Error uploading files")
	}
	return nil
}

// AnnexGet retrieves the content of specified files.
func AnnexGet(filepaths []string) error {
	// TODO: Print success for each file as it finishes
	args := append([]string{"get"}, filepaths...)
	stdout, stderr, err := RunAnnexCommand(".", args...)
	if err != nil {
		util.LogWrite("Error during AnnexGet")
		util.LogWrite("[Error]: %v", err)
		util.LogWrite("[stdout]\r\n%s", stdout.String())
		util.LogWrite("[stderr]\r\n%s", stderr.String())
		return fmt.Errorf("Error uploading files")
	}
	return nil
}

// AnnexAddResult is used to store information about each added file, as returned from the annex command.
type AnnexAddResult struct {
	Command string `json:"command"`
	File    string `json:"file"`
	Key     string `json:"key"`
	Success bool   `json:"success"`
}

// AnnexAdd adds a path to the annex.
// (git annex add)
func AnnexAdd(localPath string) ([]string, error) {
	stdout, stderr, err := RunAnnexCommand(".", "--json", "add", localPath)
	if err != nil {
		util.LogWrite("Error during AnnexAdd")
		util.LogWrite("[stdout]\r\n%s", stdout.String())
		util.LogWrite("[stderr]\r\n%s", stderr.String())
		return nil, fmt.Errorf("Error adding files to repository.")
	}

	var outStruct AnnexAddResult
	files := bytes.Split(stdout.Bytes(), []byte("\n"))
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
// (git annex whereis)
func AnnexWhereis(paths []string) ([]AnnexWhereisResult, error) {
	args := []string{"whereis", "--json"}
	args = append(args, paths...)
	stdout, stderr, err := RunAnnexCommand(".", args...)
	if err != nil {
		util.LogWrite("Error during AnnexWhereis")
		util.LogWrite("[stdout]\r\n%s", stdout.String())
		util.LogWrite("[stderr]\r\n%s", stderr.String())
		return nil, fmt.Errorf("Error getting file status from server")
	}

	resultsJSON := bytes.Split(stdout.Bytes(), []byte("\n"))
	results := make([]AnnexWhereisResult, 0, len(resultsJSON))
	for _, resJSON := range resultsJSON {
		if len(resJSON) == 0 {
			continue
		}
		var res AnnexWhereisResult
		err := json.Unmarshal(resJSON, &res)
		if err != nil {
			return nil, err
		}
		results = append(results, res)
	}
	return results, nil
}

// AnnexStatusResult for getting the (annex) status of individual files
type AnnexStatusResult struct {
	Status string `json:"status"`
	File   string `json:"file"`
}

// AnnexStatus returns the status of a file or files in a directory
func AnnexStatus(path string) ([]AnnexStatusResult, error) {
	stdout, stderr, err := RunAnnexCommand(".", "status", "--json", path)
	if err != nil {
		util.LogWrite("Error during DescribeChanges")
		util.LogWrite("[stdout]\r\n%s", stdout.String())
		util.LogWrite("[stderr]\r\n%s", stderr.String())
		return nil, fmt.Errorf("Error retrieving file status")
	}

	files := bytes.Split(stdout.Bytes(), []byte("\n"))

	statuses := make([]AnnexStatusResult, 0, len(files))
	var outStruct AnnexStatusResult
	for _, f := range files {
		if len(f) == 0 {
			// can return empty lines
			continue
		}
		err := json.Unmarshal(f, &outStruct)
		if err != nil {
			return nil, err
		}
		statuses = append(statuses, outStruct)
	}
	return statuses, nil
}

// DescribeChanges returns a string which describes the status of the files in the working tree
// with respect to git annex. The resulting message can be used to inform the user of changes
// that are about to be uploaded and as a long commit message.
func DescribeChanges(localPath string) (string, error) {
	statuses, err := AnnexStatus(localPath)
	if err != nil {
		return "", err
	}

	statusmap := make(map[string][]string)
	for _, item := range statuses {
		statusmap[item.Status] = append(statusmap[item.Status], item.File)
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

// Utility functions for shelling out

// RunGitCommand executes a external git command with the provided arguments and returns stdout and stderr
func RunGitCommand(args ...string) (bytes.Buffer, bytes.Buffer, error) {
	gitbin := util.Config.Bin.Git
	cmd := exec.Command(gitbin)
	cmd.Args = append(cmd.Args, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if privKeyFile.Active {
		env := os.Environ()
		cmd.Env = append(env, privKeyFile.GitSSHEnv())
	}
	util.LogWrite("Running shell command: %s", strings.Join(cmd.Args, " "))
	err := cmd.Run()
	return stdout, stderr, err
}

// RunAnnexCommand executes a git annex command with the provided arguments and returns stdout and stderr.
// The first argument specifies the working directory inside which the command is executed.
func RunAnnexCommand(path string, args ...string) (bytes.Buffer, bytes.Buffer, error) {
	gitannexbin := util.Config.Bin.GitAnnex
	cmd := exec.Command(gitannexbin, args...)
	cmd.Dir = path
	annexsshopt := "annex.ssh-options=-o StrictHostKeyChecking=no"
	if privKeyFile.Active {
		annexsshopt = fmt.Sprintf("%s -i %s", annexsshopt, privKeyFile.FullPath())
	}
	cmd.Args = append(cmd.Args, "-c", annexsshopt)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	util.LogWrite("Running shell command: %s", strings.Join(cmd.Args, " "))
	err := cmd.Run()
	return stdout, stderr, err
}
