package ginclient

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/G-Node/gin-cli/util"
	"github.com/G-Node/gin-cli/web"
	"github.com/gogits/go-gogs-client"
	// its a bit unfortunate that we have that import now
	// but its only temporary...
)

var defaultHostname = "(unknown)"

// MakeSessionKey creates a private+public key pair.
// The private key is saved in the user's configuration directory, to be used for git commands.
// The public key is added to the GIN server for the current logged in user.
func (gincl *Client) MakeSessionKey() error {
	keyPair, err := util.MakeKeyPair()
	if err != nil {
		return err
	}

	hostname, err := os.Hostname()
	if err != nil {
		util.LogWrite("Could not retrieve hostname")
		hostname = defaultHostname
	}
	description := fmt.Sprintf("GIN Client: %s@%s", gincl.Username, hostname)
	pubkey := fmt.Sprintf("%s %s", strings.TrimSpace(keyPair.Public), description)
	err = gincl.AddKey(pubkey, description, true)
	if err != nil {
		return err
	}

	privKeyFile := util.PrivKeyPath(gincl.Username)
	_ = ioutil.WriteFile(privKeyFile, []byte(keyPair.Private), 0600)

	return nil
}

// GetRepo retrieves the information of a repository.
func (gincl *Client) GetRepo(repoPath string) (gogs.Repository, error) {
	fn := fmt.Sprintf("GetRepo(%s)", repoPath)
	util.LogWrite("GetRepo")
	var repo gogs.Repository

	res, err := gincl.Get(fmt.Sprintf("/api/v1/repos/%s", repoPath))
	if err != nil {
		return repo, err // return error from Get() directly
	}
	switch code := res.StatusCode; {
	case code == http.StatusNotFound:
		return repo, ginerror{UError: res.Status, Origin: fn, Description: fmt.Sprintf("repository '%s' does not exist", repoPath)}
	case code == http.StatusUnauthorized:
		return repo, ginerror{UError: res.Status, Origin: fn, Description: "authorisation failed"}
	case code == http.StatusInternalServerError:
		return repo, ginerror{UError: res.Status, Origin: fn, Description: "server error"}
	case code != http.StatusOK:
		return repo, ginerror{UError: res.Status, Origin: fn} // Unexpected error
	}
	defer web.CloseRes(res.Body)
	b, err := ioutil.ReadAll(res.Body) // ignore potential read error on res.Body; catch later when trying to unmarshal
	if err != nil {
		return repo, ginerror{UError: err.Error(), Origin: fn, Description: "failed to read response body"}
	}
	err = json.Unmarshal(b, &repo)
	if err != nil {
		return repo, ginerror{UError: err.Error(), Origin: fn, Description: "failed to parse response body"}
	}
	return repo, nil
}

// ListRepos gets a list of repositories (public or user specific)
func (gincl *Client) ListRepos(user string) ([]gogs.Repository, error) {
	fn := fmt.Sprintf("ListRepos(%s)", user)
	util.LogWrite("Retrieving repo list")
	var repoList []gogs.Repository
	var res *http.Response
	var err error
	res, err = gincl.Get(fmt.Sprintf("/api/v1/users/%s/repos", user))
	if err != nil {
		return nil, err // return error from Get() directly
	}
	switch code := res.StatusCode; {
	case code == http.StatusNotFound:
		return nil, ginerror{UError: res.Status, Origin: fn, Description: fmt.Sprintf("user '%s' does not exist", user)}
	case code == http.StatusUnauthorized:
		return nil, ginerror{UError: res.Status, Origin: fn, Description: "authorisation failed"}
	case code == http.StatusInternalServerError:
		return nil, ginerror{UError: res.Status, Origin: fn, Description: "server error"}
	case code != http.StatusOK:
		return nil, ginerror{UError: res.Status, Origin: fn} // Unexpected error
	}
	defer web.CloseRes(res.Body)
	b, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, ginerror{UError: err.Error(), Origin: fn, Description: "failed to read response body"}
	}
	err = json.Unmarshal(b, &repoList)
	if err != nil {
		return nil, ginerror{UError: err.Error(), Origin: fn, Description: "failed to parse response body"}
	}
	return repoList, nil
}

// CreateRepo creates a repository on the server.
func (gincl *Client) CreateRepo(name, description string) error {
	fn := fmt.Sprintf("CreateRepo(name)")
	util.LogWrite("Creating repository")
	newrepo := gogs.CreateRepoOption{Name: name, Description: description, Private: true}
	util.LogWrite("Name: %s :: Description: %s", name, description)
	res, err := gincl.Post("/api/v1/user/repos", newrepo)
	if err != nil {
		return err // return error from Post() directly
	}
	switch code := res.StatusCode; {
	case code == http.StatusUnprocessableEntity:
		return ginerror{UError: res.Status, Origin: fn, Description: "invalid repository name or repository with the same name already exists"}
	case code == http.StatusUnauthorized:
		return ginerror{UError: res.Status, Origin: fn, Description: "authorisation failed"}
	case code == http.StatusInternalServerError:
		return ginerror{UError: res.Status, Origin: fn, Description: "server error"}
	case code != http.StatusCreated:
		return ginerror{UError: res.Status, Origin: fn} // Unexpected error
	}
	web.CloseRes(res.Body)
	util.LogWrite("Repository created")
	return nil
}

// DelRepo deletes a repository from the server.
func (gincl *Client) DelRepo(name string) error {
	fn := fmt.Sprintf("DelRepo(%s)", name)
	util.LogWrite("Deleting repository")
	res, err := gincl.Delete(fmt.Sprintf("/api/v1/repos/%s", name))
	if err != nil {
		return err // return error from Post() directly
	}
	switch code := res.StatusCode; {
	case code == http.StatusForbidden:
		return ginerror{UError: res.Status, Origin: fn, Description: "failed to delete repository (forbidden)"}
	case code == http.StatusNotFound:
		return ginerror{UError: res.Status, Origin: fn, Description: fmt.Sprintf("repository '%s' does not exist", name)}
	case code == http.StatusUnauthorized:
		return ginerror{UError: res.Status, Origin: fn, Description: "authorisation failed"}
	case code == http.StatusInternalServerError:
		return ginerror{UError: res.Status, Origin: fn, Description: "server error"}
	case code != http.StatusNoContent:
		return ginerror{UError: res.Status, Origin: fn} // Unexpected error
	}
	web.CloseRes(res.Body)
	util.LogWrite("Repository deleted")
	return nil
}

// RepoFileStatus describes the status of files when being added to the repo or transfered to/from remotes.
type RepoFileStatus struct {
	// The name of the file.
	FileName string `json:"filename"`
	// The state of the operation.
	State string `json:"state"`
	// Progress of the operation, if available. If partial progress isn't available or applicable, this will be empty.
	Progress string `json:"progress"`
	// The data rate, if available.
	Rate string `json:"rate"`
	// Errors
	Err error `json:"err"`
}

// MarshalJSON overrides the default marshalling of RepoFileStatus to return the error string for the Err field.
func (s RepoFileStatus) MarshalJSON() ([]byte, error) {
	type RFSAlias RepoFileStatus
	errmsg := ""
	if s.Err != nil {
		errmsg = s.Err.Error()
	}
	return json.Marshal(struct {
		RFSAlias
		Err string `json:"err"`
	}{
		RFSAlias: RFSAlias(s),
		Err:      errmsg,
	})
}

// Upload adds files to a repository and uploads them.
// The status channel 'uploadchan' is closed when this function returns.
func (gincl *Client) Upload(paths []string, commitmsg string, uploadchan chan<- RepoFileStatus) {
	defer close(uploadchan)
	util.LogWrite("Upload")

	paths, err := util.ExpandGlobs(paths)
	if err != nil {
		uploadchan <- RepoFileStatus{Err: err}
		return
	}

	if len(paths) > 0 {
		// Run git annex add using exclusion filters and then add the rest to git
		addchan := make(chan RepoFileStatus)
		go AnnexAdd(paths, addchan)
		for addstat := range addchan {
			// Send UploadStatus
			uploadchan <- addstat
		}

		addchan = make(chan RepoFileStatus)
		go GitAdd(paths, addchan)
		for addstat := range addchan {
			// Send UploadStatus
			uploadchan <- addstat
		}
	}

	annexpushchan := make(chan RepoFileStatus)
	go AnnexPush(paths, commitmsg, annexpushchan)
	for stat := range annexpushchan {
		uploadchan <- stat
	}
	return
}

// GetContent downloads the contents of placeholder files in a checked out repository.
// The status channel 'getcontchan' is closed when this function returns.
func (gincl *Client) GetContent(paths []string, getcontchan chan<- RepoFileStatus) {
	defer close(getcontchan)
	util.LogWrite("GetContent")

	paths, err := util.ExpandGlobs(paths)

	if err != nil {
		getcontchan <- RepoFileStatus{Err: err}
		return
	}

	annexgetchan := make(chan RepoFileStatus)
	go AnnexGet(paths, annexgetchan)
	for stat := range annexgetchan {
		getcontchan <- stat
	}
	return
}

// RemoveContent removes the contents of local files, turning them into placeholders but only if the content is available on a remote.
// The status channel 'rmcchan' is closed when this function returns.
func (gincl *Client) RemoveContent(paths []string, rmcchan chan<- RepoFileStatus) {
	defer close(rmcchan)
	util.LogWrite("RemoveContent")

	paths, err := util.ExpandGlobs(paths)
	if err != nil {
		rmcchan <- RepoFileStatus{Err: err}
		return
	}

	dropchan := make(chan RepoFileStatus)
	go AnnexDrop(paths, dropchan)
	for stat := range dropchan {
		rmcchan <- stat
	}
	return
}

// LockContent locks local files, turning them into symlinks (if supported by the filesystem).
// The status channel 'lockchan' is closed when this function returns.
func (gincl *Client) LockContent(paths []string, lcchan chan<- RepoFileStatus) {
	defer close(lcchan)
	util.LogWrite("LockContent")

	paths, err := util.ExpandGlobs(paths)
	if err != nil {
		lcchan <- RepoFileStatus{Err: err}
		return
	}

	lockchan := make(chan RepoFileStatus)
	go AnnexLock(paths, lockchan)
	for stat := range lockchan {
		lcchan <- stat
	}
	return
}

// UnlockContent unlocks local files turning them into normal files, if the content is locally available.
// The status channel 'unlockchan' is closed when this function returns.
func (gincl *Client) UnlockContent(paths []string, ulcchan chan<- RepoFileStatus) {
	defer close(ulcchan)
	util.LogWrite("UnlockContent")

	paths, err := util.ExpandGlobs(paths)
	if err != nil {
		ulcchan <- RepoFileStatus{Err: err}
		return
	}

	unlockchan := make(chan RepoFileStatus)
	go AnnexUnlock(paths, unlockchan)
	for stat := range unlockchan {
		ulcchan <- stat
	}
	return
}

// Download downloads changes and placeholder files in an already checked out repository.
// Setting the Workingdir package global affects the working directory in which the command is executed.
func (gincl *Client) Download() error {
	util.LogWrite("Download")
	return AnnexPull()
}

// CloneRepo clones a remote repository and initialises annex.
// The status channel 'clonechan' is closed when this function returns.
func (gincl *Client) CloneRepo(repoPath string, clonechan chan<- RepoFileStatus) {
	defer close(clonechan)
	util.LogWrite("CloneRepo")
	clonestatus := make(chan RepoFileStatus)
	go gincl.Clone(repoPath, clonestatus)
	for stat := range clonestatus {
		clonechan <- stat
		if stat.Err != nil {
			return
		}
	}
	_, repoName := splitRepoParts(repoPath)
	Workingdir = repoName

	initstatus := make(chan RepoFileStatus)
	go gincl.InitDir(repoPath, initstatus)
	for stat := range initstatus {
		clonechan <- stat
		if stat.Err != nil {
			return
		}
	}
	return
}

// CheckoutVersion checks out all files specified by paths from the revision with the specified commithash.
func CheckoutVersion(commithash string, paths []string) error {
	return GitCheckout(commithash, paths)
}

// CheckoutFileCopies checks out copies of files specified by path from the revision with the specified commithash.
// The checked out files are stored in the location specified by outpath.
// The timestamp of the revision is appended to the original filenames.
func CheckoutFileCopies(commithash string, paths []string, outpath string) error {
	// TODO: Needs progress/status output (per file)
	objects, err := GitLsTree(commithash, paths)
	if err != nil {
		return err
	}

	for _, obj := range objects {
		if obj.Type == "blob" {
			outfilename := obj.Name + "-old" // TODO: append timestamp (before extension)
			outfile := filepath.Join(outpath, outfilename)
			// determine if it's an annexed link
			content, cerr := GitCatFileContents(commithash, obj.Name)
			if cerr != nil {
				return cerr
			}
			if obj.Mode == "120000" {
				if isAnnexPath(string(content)) {
					fmt.Printf("Checking out annex file %s from revision %s and copying to %s\n", obj.Name, commithash, outfile)
				} else {
					fmt.Printf("%s is a link to %s and is not an annexed file. Cannot recover\n", obj.Name, content)
				}
			} else if obj.Mode == "100755" || obj.Mode == "100644" {
				fmt.Printf("Checking out git file %s from revision %s and copying to %s\n", obj.Name, commithash, outfile)
			}
		}
	}
	return nil
}

// InitDir initialises the local directory with the default remote and annex configuration.
// The status channel 'initchan' is closed when this function returns.
func (gincl *Client) InitDir(repoPath string, initchan chan<- RepoFileStatus) {
	fn := fmt.Sprintf("InitDir")
	defer close(initchan)
	initerr := ginerror{Origin: fn, Description: "Error initialising local directory"}
	remotePath := fmt.Sprintf("ssh://%s@%s/%s", gincl.GitUser, gincl.GitHost, repoPath)

	var stat RepoFileStatus
	stat.State = "Initialising local storage"
	initchan <- stat
	if !IsRepo() {
		cmd := GitCommand("init")
		stdout, stderr, err := cmd.OutputError()
		if err != nil {
			util.LogWrite("Error during Init command: %s", string(stderr))
			logstd(stdout, stderr)
			initerr.UError = err.Error()
			initchan <- RepoFileStatus{Err: initerr}
			return
		}
		Workingdir = "."
	}

	stat.Progress = "10%"
	initchan <- stat

	hostname, err := os.Hostname()
	if err != nil {
		hostname = defaultHostname
	}
	description := fmt.Sprintf("%s@%s", gincl.Username, hostname)

	// If there is no global git user.name or user.email set local ones
	cmd := GitCommand("config", "--global", "user.name")
	globalGitName, _ := cmd.Output()
	cmd = GitCommand("config", "--global", "user.email")
	globalGitEmail, _ := cmd.Output()
	if len(globalGitName) == 0 && len(globalGitEmail) == 0 {
		info, ierr := gincl.RequestAccount(gincl.Username)
		name := info.FullName
		if ierr != nil || name == "" {
			name = gincl.Username
		}
		ierr = SetGitUser(name, info.Email)
		if ierr != nil {
			util.LogWrite("Failed to set local git user configuration")
		}
	}
	stat.Progress = "20%"
	initchan <- stat

	if runtime.GOOS == "windows" {
		// force disable symlinks even if user can create them
		// see https://git-annex.branchable.com/bugs/Symlink_support_on_Windows_10_Creators_Update_with_Developer_Mode/
		GitCommand("config", "--local", "core.symlinks", "false").Run()
	}

	// If there are no commits, create the initial commit.
	// While this isn't strictly necessary, it sets the active remote with commits that makes it easier to work with.
	new, err := CommitIfNew()
	if err != nil {
		initchan <- RepoFileStatus{Err: err}
		return
	}
	stat.Progress = "30%"
	initchan <- stat

	err = AnnexInit(description)
	if err != nil {
		initchan <- RepoFileStatus{Err: err}
		return
	}
	stat.Progress = "40%"
	initchan <- stat

	err = AddRemote("origin", remotePath)
	// Ignore if it already exists
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		initchan <- RepoFileStatus{Err: err}
		return
	}
	stat.Progress = "50%"
	initchan <- stat

	if new {
		// Push initial commit and set default remote
		cmd := GitCommand("push", "--set-upstream", "origin", "master")
		stdout, stderr, err := cmd.OutputError()
		if err != nil {
			logstd(stdout, stderr)
			initchan <- RepoFileStatus{Err: initerr}
			return
		}

		// Sync if an initial commit was created
		syncchan := make(chan RepoFileStatus)
		go AnnexSync(false, syncchan)
		for syncstat := range syncchan {
			if len(syncstat.Progress) > 0 {
				progstr := strings.TrimSuffix(syncstat.Progress, "%")
				progint, converr := strconv.ParseInt(progstr, 10, 32)
				if converr != nil {
					continue
				}
				stat.Progress = fmt.Sprintf("%d%%", 50+progint/2)
			}
			initchan <- stat
		}
		if err != nil {
			initchan <- RepoFileStatus{Err: initerr}
			return
		}
	}
	stat.Progress = "100%"
	initchan <- stat
	return
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
	// Removed indicates that a (previously) tracked file has been deleted or moved
	Removed
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
	case fs == Removed:
		return "Removed"
	case fs == Untracked:
		return "Untracked"
	default:
		return "Unknown"
	}
}

// Abbrev returns the two-letter abbrevation of the file status
// OK (Synced), NC (NoContent), MD (Modified), LC (LocalUpdates), RC (RemoteUpdates), UL (Unlocked), RM (Removed), ?? (Untracked)
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
	case fs == Removed:
		return "RM"
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

	wichan := make(chan AnnexWhereisRes)
	go AnnexWhereis(paths, wichan)
	for wiInfo := range wichan {
		if wiInfo.Err != nil {
			continue
		}
		fname := wiInfo.File
		for _, remote := range wiInfo.Whereis {
			// if no remotes are "here", the file is NoContent
			statuses[fname] = NoContent
			if remote.Here {
				if len(wiInfo.Whereis) > 1 {
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

	statuschan := make(chan AnnexStatusRes)
	go AnnexStatus(asargs, statuschan)
	for item := range statuschan {
		if item.Err != nil {
			return nil, item.Err
		}
		if item.Status == "?" {
			statuses[item.File] = Untracked
		} else if item.Status == "M" {
			statuses[item.File] = Modified
		} else if item.Status == "D" {
			statuses[item.File] = Removed
		}
	}

	// Unmodified files that are checked into git (not annex) do not show up
	// Need to run git ls-files and add only files that haven't been added yet
	lschan := make(chan string)
	go GitLsFiles(paths, lschan)
	for fname := range lschan {
		if _, ok := statuses[fname]; !ok {
			statuses[fname] = Synced
		}
	}

	return statuses, nil
}

func lfIndirect(paths ...string) (map[string]FileStatus, error) {
	statuses := make(map[string]FileStatus)

	cachedchan := make(chan string)
	var cachedfiles, modifiedfiles, untrackedfiles, deletedfiles []string
	// Collect checked in files
	lsfilesargs := append([]string{"--cached"}, paths...)
	go GitLsFiles(lsfilesargs, cachedchan)

	// Collect modified files
	modifiedchan := make(chan string)
	lsfilesargs = append([]string{"--modified"}, paths...)
	go GitLsFiles(lsfilesargs, modifiedchan)

	// Collect untracked files
	otherschan := make(chan string)
	lsfilesargs = append([]string{"--others"}, paths...)
	go GitLsFiles(lsfilesargs, otherschan)

	// Collect deleted files
	deletedchan := make(chan string)
	lsfilesargs = append([]string{"--deleted"}, paths...)
	go GitLsFiles(lsfilesargs, deletedchan)

	for {
		select {
		case fname, ok := <-cachedchan:
			if ok {
				cachedfiles = append(cachedfiles, fname)
			} else {
				cachedchan = nil
			}
		case fname, ok := <-modifiedchan:
			if ok {
				modifiedfiles = append(modifiedfiles, fname)
			} else {
				modifiedchan = nil
			}
		case fname, ok := <-otherschan:
			if ok {
				untrackedfiles = append(untrackedfiles, fname)
			} else {
				otherschan = nil
			}
		case fname, ok := <-deletedchan:
			if ok {
				deletedfiles = append(deletedfiles, fname)
			} else {
				deletedchan = nil
			}
		}
		if cachedchan == nil && modifiedchan == nil && otherschan == nil && deletedchan == nil {
			break
		}
	}

	// Run whereis on cached files
	wichan := make(chan AnnexWhereisRes)
	go AnnexWhereis(cachedfiles, wichan)
	for wiInfo := range wichan {
		if wiInfo.Err != nil {
			continue
		}
		fname := wiInfo.File
		for _, remote := range wiInfo.Whereis {
			// if no remotes are "here", the file is NoContent
			statuses[fname] = NoContent
			if remote.Here {
				if len(wiInfo.Whereis) > 1 {
					statuses[fname] = Synced
				} else {
					statuses[fname] = LocalChanges
				}
				break
			}
		}
	}

	// If cached files are diff from upstream, mark as LocalChanges
	diffargs := []string{"diff", "-z", "--name-only", "--relative", "@{upstream}"}
	diffargs = append(diffargs, cachedfiles...)
	cmd := GitCommand(diffargs...)
	stdout, stderr, err := cmd.OutputError()
	if err != nil {
		util.LogWrite("Error during diff command for status")
		logstd(stdout, stderr)
		// ignoring error and continuing
	}

	diffresults := strings.Split(string(stdout), "\000")
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
	if len(modifiedfiles) > 0 {
		statuschan := make(chan AnnexStatusRes)
		go AnnexStatus(modifiedfiles, statuschan)
		for item := range statuschan {
			if item.Err != nil {
				util.LogWrite("Error during annex status while searching for unlocked files")
				// lockchan <- RepoFileStatus{Err: item.Err}
			}
			if item.Status == "T" {
				statuses[item.File] = Unlocked
			}
		}
	}

	// Add untracked files to the map
	for _, fname := range untrackedfiles {
		statuses[fname] = Untracked
	}

	// Add deleted files to the map
	for _, fname := range deletedfiles {
		statuses[fname] = Removed
	}

	return statuses, nil
}

// ListFiles lists the files and directories specified by paths and their sync status.
// Setting the Workingdir package global affects the working directory in which the command is executed.
func (gincl *Client) ListFiles(paths ...string) (map[string]FileStatus, error) {
	paths, err := util.ExpandGlobs(paths)
	if err != nil {
		return nil, err
	}
	if IsDirect() {
		return lfDirect(paths...)
	}
	return lfIndirect(paths...)
}
