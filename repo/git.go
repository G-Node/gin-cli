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

// ListFiles lists the files and directories specified by paths and their sync status.
func ListFiles(paths ...string) (map[string]FileStatus, error) {
	statuses := make(map[string]FileStatus)

	gitlsfiles := func(option string) []string {
		gitargs := []string{"ls-files", option}
		gitargs = append(gitargs, paths...)
		stdout, stderr, err := RunGitCommand(".", gitargs...)
		if err != nil {
			util.LogWrite("Error during git ls-files %s", option)
			util.LogWrite("[stdout]\r\n%s", stdout.String())
			util.LogWrite("[stderr]\r\n%s", stderr.String())
			// ignoring error and continuing
		}
		var fnames []string
		for _, fname := range strings.Split(stdout.String(), "\n") {
			// filter out emtpty lines
			if len(fname) > 0 {
				fnames = append(fnames, fname)
			}
		}
		return fnames
	}

	// Collect checked in files
	cachedfiles := gitlsfiles("--cached")

	// Collect modified files
	modifiedfiles := gitlsfiles("--modified")

	// Collect untracked files
	untrackedfiles := gitlsfiles("--others")

	// Run whereis on cached files
	wiResults, err := AnnexWhereis(cachedfiles)
	if err == nil {
		for _, status := range wiResults {
			fname := status.File
			for _, remote := range status.Whereis {
				// if no remotes are "here", the file is NoContent
				statuses[fname] = NoContent
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
	}

	// If cached files are diff from upstream, mark as LocalChanges
	diffargs := []string{"diff", "--name-only", "--relative", "@{upstream}"}
	diffargs = append(diffargs, cachedfiles...)
	stdout, stderr, err := RunGitCommand(".", diffargs...)
	if err != nil {
		util.LogWrite("Error during diff command for status")
		util.LogWrite("[stdout]\r\n%s", stdout.String())
		util.LogWrite("[stderr]\r\n%s", stderr.String())
		// ignoring error and continuing
	}

	diffresults := strings.Split(stdout.String(), "\n")
	for _, fname := range diffresults {
		// Two notes:
		//		1. There will definitely be overlap here with the same status in annex (not a problem)
		//		2. The diff might be due to remote or local changes, but for now we're going to assume local
		if strings.TrimSpace(fname) != "" {
			statuses[fname] = LocalChanges
		}
	}

	// Add leftover cached files to the map
	for _, fname := range cachedfiles {
		if _, ok := statuses[fname]; !ok {
			statuses[fname] = Synced
		}
	}

	// Add modified and untracked files to the map
	for _, fname := range modifiedfiles {
		statuses[fname] = Modified
	}

	for _, fname := range untrackedfiles {
		statuses[fname] = Untracked
	}

	return statuses, nil
}

// Git commands

// CommitIfNew creates an empty initial git commit if the current repository is completely new.
func CommitIfNew(path string) error {
	if !IsRepo(path) {
		return fmt.Errorf("Not a repository")
	}

	_, _, err := RunGitCommand(path, "rev-parse", "HEAD")
	if err == nil {
		// All good. No need to do anything
		return nil
	}

	// Create an empty initial commit and run annex sync to synchronise everything
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "(unknown)"
	}
	stdout, stderr, err := RunGitCommand(path, "commit", "--allow-empty", "-m", fmt.Sprintf("Initial commit: Repository initialised on %s", hostname))
	if err != nil {
		util.LogWrite("Error while creating initial commit")
		util.LogWrite("[stdout]\r\n%s", stdout.String())
		util.LogWrite("[stderr]\r\n%s", stderr.String())
		return err
	}

	return AnnexSync(path, true)
}

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
	stdout, stderr, err := RunGitCommand(".", "clone", remotePath)
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
func AnnexPush(paths []string, commitMsg string) error {

	contarg := make([]string, len(paths))
	for idx, p := range paths {
		contarg[idx] = fmt.Sprintf("--content-of=%s", p)
	}
	cmdargs := []string{"sync", "--no-pull", "--content", "--commit", fmt.Sprintf("--message=%s", commitMsg)}
	cmdargs = append(cmdargs, contarg...)
	stdout, stderr, err := RunAnnexCommand(".", cmdargs...)

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
	cmdargs := append([]string{"get"}, filepaths...)
	stdout, stderr, err := RunAnnexCommand(".", cmdargs...)
	if err != nil {
		util.LogWrite("Error during AnnexGet")
		util.LogWrite("[Error]: %v", err)
		util.LogWrite("[stdout]\r\n%s", stdout.String())
		util.LogWrite("[stderr]\r\n%s", stderr.String())
		return fmt.Errorf("Error downloading files")
	}
	return nil
}

// AnnexDrop drops the content of specified files.
func AnnexDrop(filepaths []string) error {
	// TODO: Print success for each file as it finishes
	cmdargs := append([]string{"drop"}, filepaths...)
	stdout, stderr, err := RunAnnexCommand(".", cmdargs...)
	if err != nil {
		util.LogWrite("Error during AnnexDrop")
		util.LogWrite("[Error]: %v", err)
		util.LogWrite("[stdout]\r\n%s", stdout.String())
		util.LogWrite("[stderr]\r\n%s", stderr.String())
		return fmt.Errorf("Error removing files")
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
func AnnexAdd(filepaths []string) ([]string, error) {
	cmdargs := []string{"--json", "add"}
	cmdargs = append(cmdargs, filepaths...)
	stdout, stderr, err := RunAnnexCommand(".", cmdargs...)
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
	cmdargs := []string{"whereis", "--json"}
	cmdargs = append(cmdargs, paths...)
	stdout, stderr, err := RunAnnexCommand(".", cmdargs...)
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
func AnnexStatus(paths ...string) ([]AnnexStatusResult, error) {
	cmdargs := []string{"status", "--json"}
	cmdargs = append(cmdargs, paths...)
	stdout, stderr, err := RunAnnexCommand(".", cmdargs...)
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

// DescribeIndexShort returns a string which represents a condensed form of the git (annex) index.
// It is constructed using the result of 'git annex status'.
// The description is composed of the file count for each status: added, modified, deleted
func DescribeIndexShort() (string, error) {
	statuses, err := AnnexStatus()
	if err != nil {
		return "", err
	}

	statusmap := make(map[string]int)
	for _, item := range statuses {
		statusmap[item.Status]++
	}
	var changesBuffer bytes.Buffer
	if statusmap["A"] > 0 {
		_, _ = changesBuffer.WriteString(fmt.Sprintf("New files: %d", statusmap["A"]))
	}
	if statusmap["M"] > 0 {
		_, _ = changesBuffer.WriteString(fmt.Sprintf("Modified files: %d", statusmap["M"]))
	}
	if statusmap["D"] > 0 {
		_, _ = changesBuffer.WriteString(fmt.Sprintf("Deleted files: %d", statusmap["D"]))
	}
	return changesBuffer.String(), nil
}

// DescribeIndex returns a string which describes the git (annex) index.
// It is constructed using the result of 'git annex status'.
// The resulting message can be used to inform the user of changes
// that are about to be uploaded and as a long commit message.
func DescribeIndex(localPath string) (string, error) {
	statuses, err := AnnexStatus(localPath)
	if err != nil {
		return "", err
	}

	statusmap := make(map[string][]string)
	for _, item := range statuses {
		statusmap[item.Status] = append(statusmap[item.Status], item.File)
	}

	var changesBuffer bytes.Buffer
	_, _ = changesBuffer.WriteString(makeFileList("New files", statusmap["A"]))
	_, _ = changesBuffer.WriteString(makeFileList("Modified files", statusmap["M"]))
	_, _ = changesBuffer.WriteString(makeFileList("Deleted files", statusmap["D"]))
	_, _ = changesBuffer.WriteString(makeFileList("Type modified files", statusmap["T"]))
	_, _ = changesBuffer.WriteString(makeFileList("Untracked files ", statusmap["?"]))

	return changesBuffer.String(), nil
}

func makeFileList(header string, fnames []string) string {
	if len(fnames) == 0 {
		return ""
	}
	var filelist bytes.Buffer
	_, _ = filelist.WriteString(fmt.Sprintf("%s (%d)\n", header, len(fnames)))
	for idx, name := range fnames {
		_, _ = filelist.WriteString(fmt.Sprintf("  %d: %s\n", idx+1, name))
	}
	_, _ = filelist.WriteString("\n")
	return filelist.String()
}

// AnnexLock locks the specified files and directory contents if they are annexed.
func AnnexLock(paths ...string) error {
	// Annex lock doesn't work like it used to. It's better to instead annex add, but only the files that are already known to annex.
	// To find these files, we can do a 'git-annex status paths...'and look for Type changes (T)
	statuses, err := AnnexStatus(paths...)
	if err != nil {
		return err
	}
	unlockedfiles := make([]string, 0, len(paths))
	for _, stat := range statuses {
		if stat.Status == "T" {
			unlockedfiles = append(unlockedfiles, stat.File)
		}
	}

	cmdargs := []string{"add"}
	cmdargs = append(cmdargs, unlockedfiles...)
	stdout, stderr, err := RunAnnexCommand(".", cmdargs...)
	if err != nil {
		util.LogWrite("Error during AnnexLock")
		util.LogWrite("[stdout]\r\n%s", stdout.String())
		util.LogWrite("[stderr]\r\n%s", stderr.String())
		return fmt.Errorf("Error locking files")
	}
	return nil
}

// AnnexUnlock unlocks the specified files and directory contents if they are annexed
func AnnexUnlock(paths ...string) error {
	cmdargs := []string{"unlock"}
	cmdargs = append(cmdargs, paths...)
	stdout, stderr, err := RunAnnexCommand(".", cmdargs...)
	if err != nil {
		util.LogWrite("Error during AnnexUnlock")
		util.LogWrite("[stdout]\r\n%s", stdout.String())
		util.LogWrite("[stderr]\r\n%s", stderr.String())
		return fmt.Errorf("Error unlocking files")
	}
	return nil
}

// AnnexInfoResult holds the information returned by AnnexInfo
type AnnexInfoResult struct {
	TransfersInProgress             []interface{} `json:"transfers in progress"`
	LocalAnnexKeys                  int           `json:"local annex keys"`
	AvailableLocalDiskSpace         string        `json:"available local disk space"`
	AnnexedFilesInWorkingTree       int           `json:"annexed files in working tree"`
	File                            interface{}   `json:"file"`
	TrustedRepositories             []interface{} `json:"trusted repositories"`
	SizeOfAnnexedFilesInWorkingTree string        `json:"size of annexed files in working tree"`
	LocalAnnexSize                  string        `json:"local annex size"`
	Command                         string        `json:"command"`
	UntrustedRepositories           []interface{} `json:"untrusted repositories"`
	SemitrustedRepositories         []struct {
		Description string `json:"description"`
		Here        bool   `json:"here"`
		UUID        string `json:"uuid"`
	} `json:"semitrusted repositories"`
	Success         bool   `json:"success"`
	BloomFilterSize string `json:"bloom filter size"`
	BackendUsage    struct {
		SHA256E int `json:"SHA256E"`
		WORM    int `json:"WORM"`
	} `json:"backend usage"`
	RepositoryMode string `json:"repository mode"`
}

// AnnexInfo returns the annex information for a given repository
func AnnexInfo(path string) (AnnexInfoResult, error) {
	stdout, stderr, err := RunAnnexCommand(path, "info", "--json")
	if err != nil {
		util.LogWrite("Error during AnnexInfo")
		util.LogWrite("[stdout]\r\n%s", stdout.String())
		util.LogWrite("[stderr]\r\n%s", stderr.String())
		return AnnexInfoResult{}, fmt.Errorf("Error retrieving annex info")
	}

	var info AnnexInfoResult
	err = json.Unmarshal(stdout.Bytes(), &info)
	return info, err
}

// IsDirect returns true if the repository in a given path is working in git annex 'direct' mode.
// If path is not a repository, or is not an initialised annex repository, the result defaults to false.
func IsDirect(path string) bool {
	info, err := AnnexInfo(".")
	if err != nil {
		util.LogWrite(err.Error())
		return false
	}
	if info.RepositoryMode == "direct" {
		return true
	}
	return false
}

// File locking and unlocking utility functions

// LockAllFiles locks all annexed files which is necessary for most git annex operations. This has no effect in Direct mode.
func LockAllFiles(path string) {
	if IsRepo(path) && !IsDirect(path) {
		_ = AnnexLock(path)
	}
}

// UnlockAllFiles unlocks all annexed files. This has no effect in Direct mode.
func UnlockAllFiles(path string) {
	if IsRepo(path) && !IsDirect(path) {
		_ = AnnexUnlock(path)
	}
}

// Utility functions for shelling out

// RunGitCommand executes a external git command with the provided arguments and returns stdout and stderr
func RunGitCommand(path string, args ...string) (bytes.Buffer, bytes.Buffer, error) {
	gitbin := util.Config.Bin.Git
	cmd := exec.Command(gitbin)
	cmd.Dir = path
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
