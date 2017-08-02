package repo

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/G-Node/gin-cli/util"
	"github.com/G-Node/gin-cli/web"
	"github.com/gogits/go-gogs-client"
	// its a bit unfortunate that we have that import now
	// but its only temporary...
	"github.com/G-Node/gin-cli/auth"
)

// Client is a client interface to the repo server. Embeds web.Client.
type Client struct {
	*web.Client
	KeyHost string
	GitHost string
	GitUser string
}

// NewClient returns a new client for the repo server.
func NewClient(host string) *Client {
	return &Client{Client: web.NewClient(host)}
}

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

// GetRepo retrieves the information of a repository.
func (repocl *Client) GetRepo(repoPath string) (gogs.Repository, error) {
	defer CleanUpTemp()
	util.LogWrite("GetRepo")
	var repo gogs.Repository

	res, err := repocl.Get(fmt.Sprintf("/api/v1/repos/%s", repoPath))
	if err != nil {
		return repo, err
	} else if res.StatusCode == http.StatusNotFound {
		return repo, fmt.Errorf("Not found. Check repository owner and name.")
	} else if res.StatusCode == http.StatusUnauthorized {
		return repo, fmt.Errorf("You are not authorised to access repository.")
	} else if res.StatusCode != http.StatusOK {
		return repo, fmt.Errorf("Server returned %s", res.Body)
	}
	defer web.CloseRes(res.Body)
	b, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return repo, err
	}
	err = json.Unmarshal(b, &repo)
	return repo, err
}

// ListRepos gets a list of repositories (public or user specific)
func (repocl *Client) ListRepos(user string) ([]gogs.Repository, error) {
	util.LogWrite("Retrieving repo list")
	var repoList []gogs.Repository
	var res *http.Response
	var err error
	res, err = repocl.Get("/api/v1/user/repos")
	if err != nil {
		return repoList, err
	}
	defer web.CloseRes(res.Body)
	b, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return repoList, err
	}
	err = json.Unmarshal(b, &repoList)
	return repoList, err
}

// CreateRepo creates a repository on the server.
func (repocl *Client) CreateRepo(name, description string) error {
	util.LogWrite("Creating repository")
	err := repocl.LoadToken()
	if err != nil {
		return fmt.Errorf("[Create repository] This action requires login")
	}

	newrepo := gogs.Repository{Name: name, Description: description}
	util.LogWrite("Name: %s :: Description: %s", name, description)
	res, err := repocl.Post("/api/v1/user/repos", newrepo)
	if err != nil {
		return err
	} else if res.StatusCode != http.StatusCreated {
		return fmt.Errorf("[Create repository] Failed. Server returned %s", res.Status)
	}
	web.CloseRes(res.Body)
	util.LogWrite("Repository created")
	return nil
}

// DelRepo deletes a repository from the server.
func (repocl *Client) DelRepo(name string) error {
	util.LogWrite("Deleting repository")
	err := repocl.LoadToken()
	if err != nil {
		return fmt.Errorf("[Delete repository] This action requires login")
	}

	res, err := repocl.Delete(fmt.Sprintf("/api/v1/repos/%s", name))
	if err != nil {
		return err
	} else if res.StatusCode != http.StatusNoContent {
		return fmt.Errorf("[Delete repository] Failed. Server returned %s", res.Status)
	}
	web.CloseRes(res.Body)
	util.LogWrite("Repository deleted")
	return nil
}

// Upload adds files to a repository and uploads them.
func (repocl *Client) Upload(paths []string) error {
	defer auth.NewClient(repocl.Host).DeleteTmpKeys()
	defer CleanUpTemp()
	util.LogWrite("Upload")

	err := repocl.Connect()
	if err != nil {
		return err
	}

	_, err = AnnexAdd(paths)
	if err != nil {
		return err
	}

	changes, err := DescribeIndexShort()
	if err != nil {
		return err
	}
	// add header commit line
	hostname, err := os.Hostname()
	if err != nil {
		util.LogWrite("Could not retrieve hostname")
		hostname = "(unknown)"
	}
	if changes == "" {
		changes = "No changes recorded"
	}
	changes = fmt.Sprintf("gin upload from %s\n\n%s", hostname, changes)
	if err != nil {
		return err
	}

	err = AnnexPush(paths, changes)
	return err
}

// DownloadRepo downloads the files in an already checked out repository.
// Setting the Workingdir package global affects the working directory in which the command is executed.
func (repocl *Client) DownloadRepo() error {
	defer auth.NewClient(repocl.Host).DeleteTmpKeys()
	defer CleanUpTemp()
	util.LogWrite("DownloadRepo")

	err := repocl.Connect()
	if err != nil {
		return err
	}
	err = AnnexPull()
	return err
}

// GetContent retrieves the contents of placeholder files in a checked out repository.
func (repocl *Client) GetContent(filepaths []string) error {
	defer CleanUpTemp()
	util.LogWrite("GetContent")

	err := repocl.Connect()
	if err != nil {
		return err
	}
	err = AnnexGet(filepaths)
	return err
}

// RmContent removes the contents of local files, turning them into placeholders, but ONLY IF the content is available on a remote
func (repocl *Client) RmContent(filepaths []string) error {
	defer CleanUpTemp()
	util.LogWrite("RmContent")

	err := repocl.Connect()
	if err != nil {
		return err
	}
	err = AnnexDrop(filepaths)
	return err
}

// CloneRepo clones a remote repository and initialises annex.
// Returns the name of the directory in which the repository is cloned.
func (repocl *Client) CloneRepo(repoPath string) (string, error) {
	authcl := auth.NewClient(repocl.Host)
	defer authcl.DeleteTmpKeys()
	defer CleanUpTemp()
	util.LogWrite("CloneRepo")

	err := repocl.Connect()
	if err != nil {
		return "", err
	}

	_, repoName := splitRepoParts(repoPath)
	fmt.Printf("Fetching repository '%s'... ", repoPath)
	err = repocl.Clone(repoPath)
	if err != nil {
		return "", err
	}
	fmt.Printf("done.\n")

	err = repocl.LoadToken()
	if err != nil {
		return "", err
	}

	// Following shell commands performed from within the repository root
	Workingdir = repoName
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "localhost"
	}
	description := fmt.Sprintf("%s@%s", repocl.Username, hostname)

	// If there is no global git user.name or user.email set local ones
	globalGitName, _, _ := RunGitCommand("config", "--global", "user.name")
	globalGitEmail, _, _ := RunGitCommand("config", "--global", "user.Email")
	if globalGitName.Len() == 0 && globalGitEmail.Len() == 0 {
		info, err := authcl.RequestAccount(repocl.Username)
		name := info.FullName
		if err != nil {
			name = repocl.Username
		}
		// NOTE: Add user email too?
		err = SetGitUser(name, "")
		if err != nil {
			util.LogWrite("Failed to set local git user configuration")
		}
	}

	// If there are no commits, create the initial commit.
	// While this isn't strictly necessary, it sets the active remote with commits that makes it easier to work with.
	new, err := CommitIfNew()
	if err != nil {
		return "", err
	}

	err = AnnexInit(description)
	if err != nil {
		return "", err
	}

	if new {
		// Sync if an initial commit was created
		err = AnnexSync(false)
		if err != nil {
			return "", err
		}
	}

	return repoName, nil
}

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
	// Unlocked indicates that a file is being tracked and is unlocked for editing
	Unlocked
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
	case fs == Unlocked:
		return "Unlocked for editing"
	case fs == Untracked:
		return "Untracked"
	default:
		return "Unknown"
	}
}

// Abbrev returns the two-letter abbrevation of the file status
// OK (Synced), NC (NoContent), MD (Modified), LC (LocalUpdates), RC (RemoteUpdates), UL (Unlocked), ?? (Untracked)
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
	case fs == Unlocked:
		return "UL"
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

func lfDirect(paths ...string) (map[string]FileStatus, error) {
	statuses := make(map[string]FileStatus)

	wiResults, _ := AnnexWhereis(paths)
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

	asargs := paths
	if len(asargs) == 0 {
		// AnnexStatus with no arguments defaults to root directory, so we should use "." instead
		asargs = []string{"."}
	}
	annexstatuses, _ := AnnexStatus(asargs...)
	for _, stat := range annexstatuses {
		if stat.Status == "?" {
			statuses[stat.File] = Untracked
		} else if stat.Status == "M" {
			statuses[stat.File] = Modified
		}
	}

	return statuses, nil
}

func lfIndirect(paths ...string) (map[string]FileStatus, error) {
	statuses := make(map[string]FileStatus)

	gitlsfiles := func(option string) []string {
		gitargs := []string{"ls-files", option}
		gitargs = append(gitargs, paths...)
		stdout, stderr, err := RunGitCommand(gitargs...)
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
	stdout, stderr, err := RunGitCommand(diffargs...)
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

	// Check if modified files are actually annex unlocked instead
	mdfilestatus, err := AnnexStatus(modifiedfiles...)
	if err != nil {
		util.LogWrite("Error during annex status while searching for unlocked files")
	}
	for _, stat := range mdfilestatus {
		if stat.Status == "T" {
			statuses[stat.File] = Unlocked
		}
	}

	// Add untracked files to the map
	for _, fname := range untrackedfiles {
		statuses[fname] = Untracked
	}

	return statuses, nil
}

// ListFiles lists the files and directories specified by paths and their sync status.
// Setting the Workingdir package global affects the working directory in which the command is executed.
func (repocl *Client) ListFiles(paths ...string) (map[string]FileStatus, error) {
	defer auth.NewClient(repocl.Host).DeleteTmpKeys()
	defer CleanUpTemp()
	err := repocl.Connect()
	if err != nil {
		return nil, err
	}
	if IsDirect() {
		return lfDirect(paths...)
	}
	return lfIndirect(paths...)
}
