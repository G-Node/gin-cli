package ginclient

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/G-Node/gin-cli/ginclient/config"
	"github.com/G-Node/gin-cli/ginclient/log"
	"github.com/G-Node/gin-cli/git"
	"github.com/G-Node/gin-cli/web"
	gogs "github.com/gogits/go-gogs-client"
)

// High level functions for managing repositories.
// These functions either end up performing web calls (using the web package) or git shell commands (using the git package).

const unknownhostname = "(unknown)"

// Types

// FileCheckoutStatus is used to report the status of a CheckoutFileCopies() operation.
type FileCheckoutStatus struct {
	Filename    string
	Type        string
	Destination string
	Err         error
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
	// TypeChange indicates that a file being tracked as locked (unlocked) is now unlocked (locked)
	TypeChange
	// Removed indicates that a (previously) tracked file has been deleted or moved
	Removed
	// Untracked indicates that a file is not being tracked by neither git nor git annex
	Untracked
)

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

//isAnnexPath returns true if a given string represents the path to an annex object.
func isAnnexPath(path string) bool {
	// TODO: Check paths on Windows
	return strings.Contains(path, "/annex/objects")
}

// MakeSessionKey creates a private+public key pair.
// The private key is saved in the user's configuration directory, to be used for git commands.
// The public key is added to the GIN server for the current logged in user.
func (gincl *Client) MakeSessionKey() error {
	keyPair, err := git.MakeKeyPair()
	if err != nil {
		return err
	}

	hostname, err := os.Hostname()
	if err != nil {
		log.Write("Could not retrieve hostname")
		hostname = unknownhostname
	}
	description := fmt.Sprintf("GIN Client: %s@%s", gincl.Username, hostname)
	pubkey := fmt.Sprintf("%s %s", strings.TrimSpace(keyPair.Public), description)
	err = gincl.AddKey(pubkey, description, true)
	if err != nil {
		return err
	}

	configpath, err := config.Path(true)
	if err != nil {
		log.Write("Could not create config directory for private key")
		return err
	}
	keyfilepath := filepath.Join(configpath, fmt.Sprintf("%s.key", gincl.srvalias))
	ioutil.WriteFile(keyfilepath, []byte(keyPair.Private), 0600)

	return nil
}

// GetRepo retrieves the information of a repository.
func (gincl *Client) GetRepo(repoPath string) (gogs.Repository, error) {
	fn := fmt.Sprintf("GetRepo(%s)", repoPath)
	log.Write("GetRepo")
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
	log.Write("Retrieving repo list")
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
	log.Write("Creating repository")
	newrepo := gogs.CreateRepoOption{Name: name, Description: description, Private: true}
	log.Write("Name: %s :: Description: %s", name, description)
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
	log.Write("Repository created")
	return nil
}

// DelRepo deletes a repository from the server.
func (gincl *Client) DelRepo(name string) error {
	fn := fmt.Sprintf("DelRepo(%s)", name)
	log.Write("Deleting repository")
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
	log.Write("Repository deleted")
	return nil
}

// Add updates the index with the changes in the files specified by 'paths'.
// The status channel 'addchan' is closed when this function returns.
func Add(paths []string, addchan chan<- git.RepoFileStatus) {
	defer close(addchan)
	paths, err := expandglobs(paths, false)
	if err != nil {
		addchan <- git.RepoFileStatus{Err: err}
		return
	}

	if len(paths) > 0 {
		// Run git annex add using exclusion filters
		// Files matching filters are automatically added to git
		annexaddchan := make(chan git.RepoFileStatus)
		go git.AnnexAdd(paths, annexaddchan)
		for addstat := range annexaddchan {
			addchan <- addstat
		}

		gitaddchan := make(chan git.RepoFileStatus)
		go git.Add(paths, gitaddchan)
		for addstat := range gitaddchan {
			addchan <- addstat
		}
	}
}

// Upload transfers locally recorded changes to a remote.
// The status channel 'uploadchan' is closed when this function returns.
func (gincl *Client) Upload(paths []string, remotes []string, uploadchan chan<- git.RepoFileStatus) {
	// TODO: Does this need to be a Client method?
	defer close(uploadchan)
	log.Write("Upload")

	paths, err := expandglobs(paths, false)
	if err != nil {
		uploadchan <- git.RepoFileStatus{Err: err}
		return
	}

	if len(remotes) == 0 {
		remote, ierr := DefaultRemote()
		if ierr != nil {
			uploadchan <- git.RepoFileStatus{Err: ierr}
			return
		}
		remotes = []string{remote}
	}

	confremotes, err := git.RemoteShow()
	if err != nil || len(confremotes) == 0 {
		uploadchan <- git.RepoFileStatus{Err: fmt.Errorf("failed to validate remote configuration (no configured remotes?)")}
	}

	for _, remote := range remotes {
		if _, ok := confremotes[remote]; !ok {
			uploadchan <- git.RepoFileStatus{FileName: remote, Err: fmt.Errorf("unknown remote name '%s': skipping", remote)}
			continue
		}

		gitpushchan := make(chan git.RepoFileStatus)
		go git.Push(remote, gitpushchan)
		for stat := range gitpushchan {
			uploadchan <- stat
		}

		annexpushchan := make(chan git.RepoFileStatus)
		go git.AnnexPush(paths, remote, annexpushchan)
		for stat := range annexpushchan {
			uploadchan <- stat
		}
	}
	return
}

// GetContent downloads the contents of placeholder files in a checked out repository.
// The status channel 'getcontchan' is closed when this function returns.
func (gincl *Client) GetContent(paths []string, getcontchan chan<- git.RepoFileStatus) {
	defer close(getcontchan)
	log.Write("GetContent")

	paths, err := expandglobs(paths, true)

	if err != nil {
		getcontchan <- git.RepoFileStatus{Err: err}
		return
	}

	annexgetchan := make(chan git.RepoFileStatus)
	go git.AnnexGet(paths, annexgetchan)
	for stat := range annexgetchan {
		getcontchan <- stat
	}
	return
}

// RemoveContent removes the contents of local files, turning them into placeholders but only if the content is available on a remote.
// The status channel 'rmcchan' is closed when this function returns.
func (gincl *Client) RemoveContent(paths []string, rmcchan chan<- git.RepoFileStatus) {
	defer close(rmcchan)
	log.Write("RemoveContent")

	paths, err := expandglobs(paths, true)
	if err != nil {
		rmcchan <- git.RepoFileStatus{Err: err}
		return
	}

	dropchan := make(chan git.RepoFileStatus)
	go git.AnnexDrop(paths, dropchan)
	for stat := range dropchan {
		rmcchan <- stat
	}
	return
}

// LockContent locks local files, turning them into symlinks (if supported by the filesystem).
// The status channel 'lockchan' is closed when this function returns.
func (gincl *Client) LockContent(paths []string, lcchan chan<- git.RepoFileStatus) {
	defer close(lcchan)
	log.Write("LockContent")

	paths, err := expandglobs(paths, true)
	if err != nil {
		lcchan <- git.RepoFileStatus{Err: err}
		return
	}

	lockchan := make(chan git.RepoFileStatus)
	go git.AnnexLock(paths, lockchan)
	for stat := range lockchan {
		lcchan <- stat
	}
	return
}

// UnlockContent unlocks local files turning them into normal files, if the content is locally available.
// The status channel 'unlockchan' is closed when this function returns.
func (gincl *Client) UnlockContent(paths []string, ulcchan chan<- git.RepoFileStatus) {
	defer close(ulcchan)
	log.Write("UnlockContent")

	paths, err := expandglobs(paths, true)
	if err != nil {
		ulcchan <- git.RepoFileStatus{Err: err}
		return
	}

	unlockchan := make(chan git.RepoFileStatus)
	go git.AnnexUnlock(paths, unlockchan)
	for stat := range unlockchan {
		ulcchan <- stat
	}
	return
}

// Download downloads changes and placeholder files in an already checked out repository.
func (gincl *Client) Download(remote string) error {
	log.Write("Download")
	// err := git.Pull(remote)
	// if err != nil {
	// 	return err
	// }
	return git.AnnexPull(remote)
}

// Sync synchronises changes bidirectionally (uploads and downloads),
// optionally transferring content between remotes and the local clone.
func (gincl *Client) Sync(content bool) error {
	log.Write("Sync %t", content)
	return git.AnnexSync(content)
}

// CloneRepo clones a remote repository and initialises annex.
// The status channel 'clonechan' is closed when this function returns.
func (gincl *Client) CloneRepo(repopath string, clonechan chan<- git.RepoFileStatus) {
	defer close(clonechan)
	log.Write("CloneRepo")
	clonestatus := make(chan git.RepoFileStatus)
	remotepath := fmt.Sprintf("%s/%s", gincl.GitAddress(), repopath)
	go git.Clone(remotepath, repopath, clonestatus)
	for stat := range clonestatus {
		clonechan <- stat
		if stat.Err != nil {
			return
		}
	}

	repoPathParts := strings.SplitN(repopath, "/", 2)
	repoName := repoPathParts[1]

	status := git.RepoFileStatus{State: "Initialising local storage"}
	clonechan <- status
	os.Chdir(repoName)
	err := gincl.InitDir(false)
	if err != nil {
		status.Err = err
		clonechan <- status
		return
	}
	status.Progress = "100%"
	clonechan <- status
	return
}

// CommitIfNew creates an empty initial git commit if the current repository is completely new.
// If a new commit is created and a default remote exists, the new commit is pushed to initialise the remote as well.
// Returns 'true' if (and only if) a commit was created.
func CommitIfNew() (bool, error) {
	if !git.IsRepo() {
		return false, fmt.Errorf("not a repository")
	}
	_, err := git.RevParse("HEAD")
	if err == nil {
		// All good. No need to do anything
		return false, nil
	}

	// Create an empty initial commit
	hostname, err := os.Hostname()
	if err != nil {
		hostname = unknownhostname
	}
	initmsg := fmt.Sprintf("Initial commit: Repository initialised on %s", hostname)
	if err = git.CommitEmpty(initmsg); err != nil {
		log.Write("Error while creating initial commit")
		return false, err
	}

	return true, nil
}

// DefaultRemote returns the name of the configured default gin remote.
// If a remote is not set in the config, the remote of the default git upstream is set and returned.
func DefaultRemote() (string, error) {
	defremote, err := git.ConfigGet("gin.remote")
	if err == nil {
		return defremote, nil
	}
	log.Write("Default remote not set. Checking master remote.")
	defremote, err = git.ConfigGet("branch.master.remote")
	if err == nil {
		SetDefaultRemote(defremote)
		log.Write("Set default remote to %s", defremote)
		return defremote, nil
	}
	err = fmt.Errorf("could not determine default remote")
	return defremote, err
}

// SetDefaultRemote sets the name of the default gin remote.
func SetDefaultRemote(remote string) error {
	remotes, err := git.RemoteShow()
	if err != nil {
		return fmt.Errorf("failed to determine configured remotes")
	}
	if _, ok := remotes[remote]; !ok {
		return fmt.Errorf("no such remote: %s", remote)
	}
	err = git.ConfigSet("gin.remote", remote)
	if err != nil {
		return fmt.Errorf("failed to set default remote: %s", err)
	}
	return nil
}

// UnsetDefaultRemote unsets the default gin remote in the git configuration.
func UnsetDefaultRemote() error {
	err := git.ConfigUnset("gin.remote")
	if err != nil {
		return fmt.Errorf("failed to unset default remote: %s", err)
	}
	return nil
}

// RemoveRemote removes a remote from the repository configuration.
func RemoveRemote(remote string) error {
	remotes, err := git.RemoteShow()
	if err != nil {
		return fmt.Errorf("failed to determine configured remotes")
	}
	if _, ok := remotes[remote]; !ok {
		return fmt.Errorf("no such remote: %s", remote)
	}
	err = git.RemoteRemove(remote)
	return err
}

// CheckoutVersion checks out all files specified by paths from the revision with the specified commithash.
func CheckoutVersion(commithash string, paths []string) error {
	err := git.Checkout(commithash, paths)
	if err != nil {
		return err
	}

	return git.AnnexFsck(paths)
}

// CheckoutFileCopies checks out copies of files specified by path from the revision with the specified commithash.
// The checked out files are stored in the location specified by outpath.
// The timestamp of the revision is appended to the original filenames (before the extension).
func CheckoutFileCopies(commithash string, paths []string, outpath string, suffix string, cochan chan<- FileCheckoutStatus) {
	defer close(cochan)
	objects, err := git.LsTree(commithash, paths)
	if err != nil {
		cochan <- FileCheckoutStatus{Err: err}
		return
	}

	for _, obj := range objects {
		var status FileCheckoutStatus
		if obj.Type == "blob" {
			status.Filename = obj.Name

			filext := filepath.Ext(obj.Name)
			outfilename := fmt.Sprintf("%s-%s%s", strings.TrimSuffix(obj.Name, filext), suffix, filext)
			outfile := filepath.Join(outpath, outfilename)
			status.Destination = outfile

			// determine if it's an annexed link
			content, cerr := git.CatFileContents(commithash, obj.Name)
			if cerr != nil {
				cochan <- FileCheckoutStatus{Err: cerr}
				return
			}
			if mderr := os.MkdirAll(outpath, 0777); mderr != nil {
				cochan <- FileCheckoutStatus{Err: mderr}
				return
			}

			// heuristic check for annexed pointer file:
			// - check if the first 255 bytes of the file (or the entire
			// contents if smaller) contain the string /annex/objects
			maxpathidx := 255
			if len(content) < maxpathidx {
				maxpathidx = len(content)
			}

			if isAnnexPath(string(content[:maxpathidx])) {
				// Pointer file to annexed content
				status.Type = "Annex"
				// strip any newlines from the end of the path
				keypath := strings.TrimSpace(string(content))
				_, key := path.Split(keypath)
				contentloc, err := git.AnnexContentLocation(key)
				if err != nil {
					getchan := make(chan git.RepoFileStatus)
					go git.AnnexGetKey(key, getchan)
					for range getchan {
					}
					contentloc, err = git.AnnexContentLocation(key)
					if err != nil {
						status.Err = fmt.Errorf("Annexed content is not available locally")
						cochan <- status
						continue
					}
					status.Err = nil
				}
				err = git.CopyFile(contentloc, outfile)
				if err != nil {
					status.Err = fmt.Errorf("Error writing %s: %s", outfile, err.Error())
				}
			} else if obj.Mode == "120000" {
				// Plain symlink
				status.Type = "Link"
				status.Destination = string(content)
			} else if obj.Mode == "100755" || obj.Mode == "100644" {
				status.Type = "Git"
				werr := ioutil.WriteFile(outfile, content, 0666)
				if werr != nil {
					status.Err = fmt.Errorf("Error writing %s: %s", outfile, werr.Error())
				}
			} else {
				status.Err = fmt.Errorf("Unexpected object found in tree: %s", obj.Name)
			}
			cochan <- status
		} else if obj.Type == "tree" {
			status.Type = "Tree"
			status.Filename = obj.Name
			status.Destination = filepath.Join(outpath, obj.Name)
			os.MkdirAll(status.Destination, 0777)
			cochan <- status
		}
	}
}

// InitDir initialises the local directory with the default remote and git (and annex) configuration options.
// Optionally initialised as a bare repository (for annex directory remotes).
func (gincl *Client) InitDir(bare bool) error {
	initerr := ginerror{Origin: "InitDir", Description: "Error initialising local directory"}
	if !git.IsRepo() {
		err := git.Init(bare)
		if err != nil {
			initerr.UError = err.Error()
			return initerr
		}
	}

	hostname, err := os.Hostname()
	if err != nil {
		hostname = unknownhostname
	}
	description := fmt.Sprintf("%s@%s", gincl.Username, hostname)

	// If there is no git user.name or user.email set local ones
	cmd := git.Command("config", "user.name")
	globalGitName, _ := cmd.Output()
	if len(globalGitName) == 0 {
		info, ierr := gincl.RequestAccount(gincl.Username)
		name := info.FullName
		if ierr != nil || name == "" {
			name = gincl.Username
		}
		if name == "" { // user might not be logged in; fall back to system user
			u, _ := user.Current()
			name = u.Name
		}
		ierr = git.SetGitUser(name, "")
		if ierr != nil {
			log.Write("Failed to set local git user configuration")
		}
	}
	// Disable quotepath: when enabled prints escape sequences for files with
	// unicode characters making it hard to work with, can break JSON
	// formatting, and sometimes impossible to reference specific files.
	git.ConfigSet("core.quotepath", "false")
	if runtime.GOOS == "windows" {
		// force disable symlinks even if user can create them
		// see https://git-annex.branchable.com/bugs/Symlink_support_on_Windows_10_Creators_Update_with_Developer_Mode/
		git.ConfigSet("core.symlinks", "false")
	}

	if !bare {
		_, err = CommitIfNew()
		if err != nil {
			initerr.UError = err.Error()
			return initerr
		}
	}

	err = git.AnnexInit(description)
	if err != nil {
		initerr.UError = err.Error()
		return initerr
	}

	return nil
}

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
	case fs == TypeChange:
		return "Lock status changed"
	case fs == Removed:
		return "Removed"
	case fs == Untracked:
		return "Untracked"
	default:
		return "Unknown"
	}
}

// Abbrev returns the two-letter abbrevation of the file status
// OK (Synced), NC (NoContent), MD (Modified), LC (LocalUpdates), RC (RemoteUpdates), UL (Unlocked), TC (TypeChange), RM (Removed), ?? (Untracked)
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
	case fs == TypeChange:
		return "TC"
	case fs == Removed:
		return "RM"
	case fs == Untracked:
		return "??"
	default:
		return "??"
	}
}

func lfDirect(paths ...string) (map[string]FileStatus, error) {
	statuses := make(map[string]FileStatus)

	wichan := make(chan git.AnnexWhereisRes)
	go git.AnnexWhereis(paths, wichan)
	for wiInfo := range wichan {
		if wiInfo.Err != nil {
			continue
		}
		fname := filepath.Clean(wiInfo.File)
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

	statuschan := make(chan git.AnnexStatusRes)
	go git.AnnexStatus(asargs, statuschan)
	for item := range statuschan {
		if item.Err != nil {
			return nil, item.Err
		}
		fname := filepath.Clean(item.File)
		if item.Status == "?" {
			statuses[fname] = Untracked
		} else if item.Status == "M" {
			statuses[fname] = Modified
		} else if item.Status == "D" {
			statuses[fname] = Removed
		}
	}

	// Unmodified files that are checked into git (not annex) do not show up
	// Need to run git ls-files, with bare temporarily disabled, and add only files that haven't been added yet
	git.SetBare(false)
	defer git.SetBare(true)
	lschan := make(chan string)
	go git.LsFiles(paths, lschan)
	var gitfiles []string
	for fname := range lschan {
		fname = filepath.Clean(fname)
		if _, ok := statuses[fname]; !ok {
			statuses[fname] = Synced
			gitfiles = append(gitfiles, fname)
		}
	}

	// git files should be checked against upstream (if it exists) for local commits
	if len(gitfiles) > 0 {
		diffchan := make(chan string)
		remote, err := DefaultRemote()
		if err == nil {
			upstream := fmt.Sprintf("%s/master", remote)
			go git.DiffUpstream(gitfiles, upstream, diffchan)
			for fname := range diffchan {
				statuses[filepath.Clean(fname)] = LocalChanges
			}
		}
	}
	return statuses, nil
}

func lfIndirect(paths ...string) (map[string]FileStatus, error) {
	// TODO: Determine if added files (LocalChanges) are new or not (new status needed?)
	statuses := make(map[string]FileStatus)

	cachedchan := make(chan string)
	var cachedfiles, modifiedfiles, untrackedfiles, deletedfiles []string
	// Collect checked in files
	lsfilesargs := append([]string{"--cached"}, paths...)
	go git.LsFiles(lsfilesargs, cachedchan)

	// Collect modified files
	modifiedchan := make(chan string)
	lsfilesargs = append([]string{"--modified"}, paths...)
	go git.LsFiles(lsfilesargs, modifiedchan)

	// Collect untracked files
	otherschan := make(chan string)
	lsfilesargs = append([]string{"--others"}, paths...)
	go git.LsFiles(lsfilesargs, otherschan)

	// Collect deleted files
	deletedchan := make(chan string)
	lsfilesargs = append([]string{"--deleted"}, paths...)
	go git.LsFiles(lsfilesargs, deletedchan)

	// TODO: Use a WaitGroup
	for {
		select {
		case fname, ok := <-cachedchan:
			if ok {
				cachedfiles = append(cachedfiles, filepath.Clean(fname))
			} else {
				cachedchan = nil
			}
		case fname, ok := <-modifiedchan:
			if ok {
				modifiedfiles = append(modifiedfiles, filepath.Clean(fname))
			} else {
				modifiedchan = nil
			}
		case fname, ok := <-otherschan:
			if ok {
				untrackedfiles = append(untrackedfiles, filepath.Clean(fname))
			} else {
				otherschan = nil
			}
		case fname, ok := <-deletedchan:
			if ok {
				deletedfiles = append(deletedfiles, filepath.Clean(fname))
			} else {
				deletedchan = nil
			}
		}
		if cachedchan == nil && modifiedchan == nil && otherschan == nil && deletedchan == nil {
			break
		}
	}

	if len(cachedfiles) > 0 {
		// Check for git diffs with upstream
		diffchan := make(chan string)
		noremotes := true
		remote, rerr := DefaultRemote()
		if rerr == nil {
			noremotes = false // default remote set
			remoterefs, lserr := git.LsRemote(remote)
			if lserr == nil && remoterefs == "" {
				noremotes = true // default remote is uninitialised; treat as missing
			}
		}
		if noremotes {
			for _, fname := range cachedfiles {
				statuses[fname] = LocalChanges
			}
		} else if rerr == nil {
			upstream := fmt.Sprintf("%s/master", remote) // TODO: Don't assume master; use current branch name
			go git.DiffUpstream(cachedfiles, upstream, diffchan)
			for fname := range diffchan {
				fname = filepath.Clean(fname)
				// Two notes:
				//		1. There will definitely be overlap here with the same status in annex (not a problem)
				//		2. The diff might be due to remote or local changes, but for now we're going to assume local
				statuses[fname] = LocalChanges
			}
		}

		// Run whereis on cached files (if any) to see if content is synced for annexed files
		wichan := make(chan git.AnnexWhereisRes)
		go git.AnnexWhereis(cachedfiles, wichan)
		for wiInfo := range wichan {
			if wiInfo.Err != nil {
				continue
			}
			fname := filepath.Clean(wiInfo.File)
			// if no content location for this file is "here", the status is NoContent
			statuses[fname] = NoContent
			for _, remote := range wiInfo.Whereis {
				if remote.Here {
					if len(wiInfo.Whereis) > 1 {
						// content is here and in one other location: Synced
						statuses[fname] = Synced
					} else {
						// content is here only: LocalChanges (not uploaded)
						statuses[fname] = LocalChanges
					}
					break
				}
			}
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

	// Check if there are any TypeChange files (lock state change)
	statuschan := make(chan git.AnnexStatusRes)
	go git.AnnexStatus(paths, statuschan)
	for item := range statuschan {
		if item.Err != nil {
			log.Write("Error during annex status while searching for unlocked files")
		}
		if item.Status == "T" {
			statuses[filepath.Clean(item.File)] = TypeChange
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
func (gincl *Client) ListFiles(paths ...string) (map[string]FileStatus, error) {
	paths, err := expandglobs(paths, false)
	if err != nil {
		return nil, err
	}
	if git.IsDirect() {
		return lfDirect(paths...)
	}
	return lfIndirect(paths...)
}

// expandglobs expands a list of globs into paths (files and directories).
// If strictmatch is true, an error is returned if at least one element of the input slice does not match a real path,
// otherwise the pattern itself is returned when it matches no existing path.
func expandglobs(paths []string, strictmatch bool) (globexppaths []string, err error) {
	if len(paths) == 0 {
		// Nothing to do
		globexppaths = paths
		return
	}
	// expand potential globs
	for _, p := range paths {
		log.Write("ExpandGlobs: Checking for glob expansion for %s", p)
		exp, globerr := filepath.Glob(p)
		if globerr != nil {
			log.Write(globerr.Error())
			log.Write("Bad file pattern %s", p)
			return nil, globerr
		}
		if exp == nil {
			log.Write("ExpandGlobs: No files matched")
			if strictmatch {
				return nil, fmt.Errorf("No files matched %v", p)
			}
			exp = []string{p}
		}
		globexppaths = append(globexppaths, exp...)
	}
	return
}
