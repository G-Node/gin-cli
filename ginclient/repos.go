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

const unknownhostname = "(unknownhost)"

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
	keyPair, err := MakeKeyPair()
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

	res, err := gincl.web.Get(fmt.Sprintf("/api/v1/repos/%s", repoPath))
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
	res, err = gincl.web.Get(fmt.Sprintf("/api/v1/users/%s/repos", user))
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
	res, err := gincl.web.Post("/api/v1/user/repos", newrepo)
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
	res, err := gincl.web.Delete(fmt.Sprintf("/api/v1/repos/%s", name))
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
// Returns a channel that should be read to get the states of the operation
// while it runs.
func Add(paths []string) chan git.RepoFileStatus {
	addchan := make(chan git.RepoFileStatus)
	if len(paths) == 0 {
		close(addchan)
		return addchan
	}
	go func() {
		defer close(addchan)
		paths, err := expandglobs(paths, false)
		if err != nil {
			addchan <- git.RepoFileStatus{Err: err}
			return
		}
		gitaddpaths := make([]string, 0) // most times, this wont be used, so start with 0
		gr := git.New(".")
		statuschan := gr.AnnexStatus(paths)
		for stat := range statuschan {
			if stat.Status == "D" {
				// deleted files match but weren't added
				// this can happen when the annex filters don't match a file
				// and it doesn't go through to get added to git
				gitaddpaths = append(gitaddpaths, stat.File)
			}
		}

		// Run git add on deleted files only
		if len(gitaddpaths) > 0 {
			gitaddchan := gr.Add(gitaddpaths)
			for addstat := range gitaddchan {
				addstat.State = "Removing"
				addchan <- addstat
			}
		}

		conf := config.Read()

		// Run git annex add using exclusion filters
		// Files matching filters are automatically added to git
		annexaddchan := gr.AnnexAdd(paths, conf.Annex.MinSize, conf.Annex.Exclude)
		for addstat := range annexaddchan {
			addchan <- addstat
		}
	}()
	return addchan
}

// Upload transfers locally recorded changes to a remote.
// The status channel 'uploadchan' is closed when this function returns.
func (gincl *Client) Upload(paths []string, remotes []string) chan git.RepoFileStatus {
	// TODO: Does this need to be a Client method?
	log.Write("Upload")
	uploadchan := make(chan git.RepoFileStatus)

	gr := git.New(".")
	gr.SSHCmd = sshopts(gincl.srvalias)

	go func() {
		defer close(uploadchan)
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

		confremotes, err := gr.RemoteShow()
		if err != nil || len(confremotes) == 0 {
			uploadchan <- git.RepoFileStatus{Err: fmt.Errorf("failed to validate remote configuration (no configured remotes?)")}
		}

		for _, remote := range remotes {
			if _, ok := confremotes[remote]; !ok {
				uploadchan <- git.RepoFileStatus{FileName: remote, Err: fmt.Errorf("unknown remote name '%s': skipping", remote)}
				continue
			}

			gitpushchan := gr.Push(remote)
			for stat := range gitpushchan {
				uploadchan <- stat
			}

			annexpushchan := gr.AnnexPush(paths, remote)
			for stat := range annexpushchan {
				uploadchan <- stat
			}
		}
	}()
	return uploadchan
}

// GetContent downloads the contents of placeholder files in a checked out repository.
// The status channel 'getcontchan' is closed when this function returns.
func (gincl *Client) GetContent(paths []string) chan git.RepoFileStatus {
	getcontchan := make(chan git.RepoFileStatus)
	log.Write("GetContent")

	go func() {
		defer close(getcontchan)
		paths, err := expandglobs(paths, true)
		if err != nil {
			getcontchan <- git.RepoFileStatus{Err: err}
			return
		}

		gitcl := git.New(gincl.srvalias)
		gitcl.SSHCmd = sshopts(gincl.srvalias)
		annexgetchan := gitcl.AnnexGet(paths)
		for stat := range annexgetchan {
			getcontchan <- stat
		}
	}()
	return getcontchan
}

// RemoveContent removes the contents of local files, turning them into placeholders but only if the content is available on a remote.
// The status channel 'rmcchan' is closed when this function returns.
func (gincl *Client) RemoveContent(paths []string) chan git.RepoFileStatus {
	rmcchan := make(chan git.RepoFileStatus)

	go func() {
		defer close(rmcchan)
		log.Write("RemoveContent")
		paths, err := expandglobs(paths, true)
		if err != nil {
			rmcchan <- git.RepoFileStatus{Err: err}
			return
		}
		gitcl := git.New(gincl.srvalias)
		gitcl.SSHCmd = sshopts(gincl.srvalias)
		dropchan := gitcl.AnnexDrop(paths)
		for stat := range dropchan {
			rmcchan <- stat
		}
	}()
	return rmcchan
}

// LockContent locks local files, turning them into symlinks (if supported by the filesystem).
// The status channel 'lockchan' is closed when this function returns.
func (gincl *Client) LockContent(paths []string) chan git.RepoFileStatus {
	lcchan := make(chan git.RepoFileStatus)
	log.Write("LockContent")
	gr := git.New(".")

	go func() {
		defer close(lcchan)
		paths, err := expandglobs(paths, true)
		if err != nil {
			lcchan <- git.RepoFileStatus{Err: err}
			return
		}

		lockchan := gr.AnnexLock(paths)
		for stat := range lockchan {
			lcchan <- stat
		}
	}()
	return lcchan
}

// UnlockContent unlocks local files turning them into normal files, if the content is locally available.
// The status channel 'unlockchan' is closed when this function returns.
func (gincl *Client) UnlockContent(paths []string) chan git.RepoFileStatus {
	ulcchan := make(chan git.RepoFileStatus)
	log.Write("UnlockContent")
	gr := git.New(".")

	go func() {
		defer close(ulcchan)
		paths, err := expandglobs(paths, true)
		if err != nil {
			ulcchan <- git.RepoFileStatus{Err: err}
			return
		}

		unlockchan := gr.AnnexUnlock(paths)
		for stat := range unlockchan {
			ulcchan <- stat
		}
	}()
	return ulcchan
}

// Download downloads changes and placeholder files in an already checked out repository.
func (gincl *Client) Download(remote string) error {
	log.Write("Download: %q", remote)
	gitcl := git.New(gincl.srvalias)
	gitcl.SSHCmd = sshopts(gincl.srvalias)
	return gitcl.AnnexPull(remote)
}

// Sync synchronises changes bidirectionally (uploads and downloads),
// optionally transferring content between remotes and the local clone.
func (gincl *Client) Sync(content bool) error {
	log.Write("Sync %t", content)
	gitcl := git.New(gincl.srvalias)
	gitcl.SSHCmd = sshopts(gincl.srvalias)
	return gitcl.AnnexSync(content)
}

// CloneRepo clones a remote repository and initialises annex.
// The status channel 'clonechan' is closed when this function returns.
func (gincl *Client) CloneRepo(repopath string) chan git.RepoFileStatus {
	clonechan := make(chan git.RepoFileStatus)
	log.Write("CloneRepo")
	go func() {
		defer close(clonechan)
		remotepath := fmt.Sprintf("%s/%s", gincl.GitAddress(), repopath)
		repoPathParts := strings.SplitN(repopath, "/", 2)
		repoName := repoPathParts[1]

		here, _ := os.Getwd()
		cloneloc := filepath.Join(here, repoName)
		gitcl := git.New(cloneloc)
		gitcl.SSHCmd = sshopts(gincl.srvalias)
		clonestatus := gitcl.Clone(remotepath, repopath)
		for stat := range clonestatus {
			clonechan <- stat
			if stat.Err != nil {
				return
			}
		}

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
	}()
	return clonechan
}

// CommitIfNew creates an empty initial git commit if the current repository is completely new.
// If a new commit is created and a default remote exists, the new commit is pushed to initialise the remote as well.
// Returns 'true' if (and only if) a commit was created.
func CommitIfNew() (bool, error) {
	if git.Checkwd() == git.NotRepository {
		// Other errors allowed
		return false, fmt.Errorf("not a repository")
	}
	gr := git.New(".")
	_, err := gr.RevParse("HEAD")
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
	if err = gr.CommitEmpty(initmsg); err != nil {
		log.Write("Error while creating initial commit")
		return false, err
	}

	return true, nil
}

// DefaultRemote returns the name of the configured default gin remote.
// If a remote is not set in the config, the remote of the default git upstream is set and returned.
func DefaultRemote() (string, error) {
	gr := git.New(".")
	defremote, err := gr.ConfigGet("gin.remote")
	if err == nil {
		return defremote, nil
	}
	log.Write("Default remote not set. Checking master remote.")
	defremote, err = gr.ConfigGet("branch.master.remote")
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
	gr := git.New(".")
	remotes, err := gr.RemoteShow()
	if err != nil {
		return fmt.Errorf("failed to determine configured remotes")
	}
	if _, ok := remotes[remote]; !ok {
		return fmt.Errorf("no such remote: %s", remote)
	}
	err = gr.ConfigSet("gin.remote", remote)
	if err != nil {
		return fmt.Errorf("failed to set default remote: %s", err)
	}
	return nil
}

// UnsetDefaultRemote unsets the default gin remote in the git configuration.
func UnsetDefaultRemote() error {
	gr := git.New(".")
	err := gr.ConfigUnset("gin.remote")
	if err != nil {
		return fmt.Errorf("failed to unset default remote: %s", err)
	}
	return nil
}

// RemoveRemote removes a remote from the repository configuration.
func RemoveRemote(remote string) error {
	gr := git.New(".")
	remotes, err := gr.RemoteShow()
	if err != nil {
		return fmt.Errorf("failed to determine configured remotes")
	}
	if _, ok := remotes[remote]; !ok {
		return fmt.Errorf("no such remote: %s", remote)
	}
	err = gr.RemoteRemove(remote)
	return err
}

// CheckoutVersion checks out all files specified by paths from the revision with the specified commithash.
func CheckoutVersion(commithash string, paths []string) error {
	gr := git.New(".")
	err := gr.Checkout(commithash, paths)
	if err != nil {
		return err
	}

	return gr.AnnexFsck(paths)
}

// CheckoutFileCopies checks out copies of files specified by path from the revision with the specified commithash.
// The checked out files are stored in the location specified by outpath.
// The timestamp of the revision is appended to the original filenames (before the extension).
func (gincl *Client) CheckoutFileCopies(commithash string, paths []string, outpath string, suffix string) chan FileCheckoutStatus {
	cochan := make(chan FileCheckoutStatus)
	gr := git.New(".")
	go func() {
		defer close(cochan)
		objects, err := gr.LsTree(commithash, paths)
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
				content, cerr := gr.CatFileContents(commithash, obj.Name)
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
					contentloc, err := gr.AnnexContentLocation(key)
					if err != nil {
						gitcl := git.New(gincl.srvalias)
						gitcl.SSHCmd = sshopts(gincl.srvalias)
						getchan := gitcl.AnnexGetKey(key)
						for range getchan {
						}
						contentloc, err = gr.AnnexContentLocation(key)
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
	}()
	return cochan
}

// InitDir initialises the local directory with the default remote and git (and annex) configuration options.
// Optionally initialised as a bare repository (for annex directory remotes).
func (gincl *Client) InitDir(bare bool) error {
	initerr := ginerror{Origin: "InitDir", Description: "Error initialising local directory"}
	gr := git.New(".")
	if git.Checkwd() == git.NotRepository {
		err := gr.Init(".", bare)
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
	cmd := gr.Command("config", "user.name")
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
		ierr = gr.SetGitUser(name, "")
		if ierr != nil {
			log.Write("Failed to set local git user configuration")
		}
	}
	// Disable quotepath: when enabled prints escape sequences for files with
	// unicode characters making it hard to work with, can break JSON
	// formatting, and sometimes impossible to reference specific files.
	gr.ConfigSet("core.quotepath", "false")
	if runtime.GOOS == "windows" {
		// force disable symlinks even if user can create them
		// see https://git-annex.branchable.com/bugs/Symlink_support_on_Windows_10_Creators_Update_with_Developer_Mode/
		gr.ConfigSet("core.symlinks", "false")
	}

	if !bare {
		_, err = CommitIfNew()
		if err != nil {
			initerr.UError = err.Error()
			return initerr
		}
	}

	gitcl := git.New(".")
	gitcl.SSHCmd = sshopts(gincl.srvalias)
	err = gitcl.AnnexInit(description)
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

	gr := git.New(".")
	wichan := gr.AnnexWhereis(paths)
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

	statuschan := gr.AnnexStatus(asargs)
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
	gr.SetBare(false)
	defer gr.SetBare(true)
	lschan := gr.LsFiles(paths)
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
		remote, err := DefaultRemote()
		if err == nil {
			upstream := fmt.Sprintf("%s/master", remote)
			diffchan := gr.DiffUpstream(gitfiles, upstream)
			for fname := range diffchan {
				statuses[filepath.Clean(fname)] = LocalChanges
			}
		}
	}
	return statuses, nil
}

func lfIndirect(paths ...string) (map[string]FileStatus, error) {
	gr := git.New(".")

	// TODO: Determine if added files (LocalChanges) are new or not (new status needed?)
	statuses := make(map[string]FileStatus)

	var cachedfiles, modifiedfiles, untrackedfiles, deletedfiles []string
	// Collect checked in files
	lsfilesargs := append([]string{"--cached"}, paths...)
	cachedchan := gr.LsFiles(lsfilesargs)

	// Collect modified files
	lsfilesargs = append([]string{"--modified"}, paths...)
	modifiedchan := gr.LsFiles(lsfilesargs)

	// Collect untracked files
	lsfilesargs = append([]string{"--others"}, paths...)
	otherschan := gr.LsFiles(lsfilesargs)

	// Collect deleted files
	lsfilesargs = append([]string{"--deleted"}, paths...)
	deletedchan := gr.LsFiles(lsfilesargs)

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
		noremotes := true
		remote, rerr := DefaultRemote()
		if rerr == nil {
			noremotes = false // default remote set
			remoterefs, lserr := gr.LsRemote(remote)
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
			diffchan := gr.DiffUpstream(cachedfiles, upstream)
			for fname := range diffchan {
				fname = filepath.Clean(fname)
				// Two notes:
				//		1. There will definitely be overlap here with the same status in annex (not a problem)
				//		2. The diff might be due to remote or local changes, but for now we're going to assume local
				statuses[fname] = LocalChanges
			}
		}

		// Run whereis on cached files (if any) to see if content is synced for annexed files
		wichan := gr.AnnexWhereis(cachedfiles)
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
	statuschan := gr.AnnexStatus(paths)
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
	gr := git.New(".")
	paths, err := expandglobs(paths, false)
	if err != nil {
		return nil, err
	}
	if gr.IsDirect() {
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
