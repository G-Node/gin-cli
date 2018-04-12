package ginclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/G-Node/gin-cli/util"
	"github.com/G-Node/gin-cli/web"
	humanize "github.com/dustin/go-humanize"
)

// Workingdir sets the directory for shell commands
var Workingdir = "."

const progcomplete = "100%"

// **************** //

// Git commands

// SetGitUser sets the user.name and user.email configuration values for the local git repository.
func SetGitUser(name, email string) error {
	if !IsRepo() {
		return fmt.Errorf("not a repository")
	}
	cmd := GitCommand("config", "--local", "user.name", name)
	err := cmd.Run()
	if err != nil {
		return err
	}
	cmd = GitCommand("config", "--local", "user.email", email)
	return cmd.Run()
}

// AddRemote adds a remote named name for the repository at url.
func AddRemote(name, url string) error {
	fn := fmt.Sprintf("AddRemote(%s, %s)", name, url)
	cmd := GitCommand("remote", "add", name, url)
	stdout, stderr, err := cmd.OutputError()
	if err != nil {
		gerr := ginerror{UError: err.Error(), Origin: fn}
		util.LogWrite("Error during remote add command")
		logstd(stdout, stderr)
		if strings.Contains(string(stderr), "already exists") {
			gerr.Description = fmt.Sprintf("remote with name '%s' already exists", name)
			return gerr
		}
	}
	return err
}

// CommitIfNew creates an empty initial git commit if the current repository is completely new.
// Returns 'true' if (and only if) a commit was created.
// Setting the Workingdir package global affects the working directory in which the command is executed.
func CommitIfNew() (bool, error) {
	if !IsRepo() {
		return false, fmt.Errorf("not a repository")
	}
	cmd := GitCommand("rev-parse", "HEAD")
	err := cmd.Wait()
	if err == nil {
		// All good. No need to do anything
		return false, nil
	}

	// Create an empty initial commit and run annex sync to synchronise everything
	hostname, err := os.Hostname()
	if err != nil {
		hostname = defaultHostname
	}
	commitargs := []string{"commit", "--allow-empty", "-m", fmt.Sprintf("Initial commit: Repository initialised on %s", hostname)}
	cmd = GitCommand(commitargs...)
	stdout, stderr, err := cmd.OutputError()
	if err != nil {
		util.LogWrite("Error while creating initial commit")
		logstd(stdout, stderr)
		return false, fmt.Errorf(string(stderr))
	}
	return true, nil
}

// IsRepo checks whether the current working directory is in a git repository.
// This function will also return true for bare repositories that use git annex (direct mode).
// Setting the Workingdir package global affects the working directory in which the command is executed.
func IsRepo() bool {
	util.LogWrite("IsRepo '%s'?", Workingdir)
	_, err := util.FindRepoRoot(Workingdir)
	yes := err == nil
	util.LogWrite("%v", yes)
	return yes
}

// Clone downloads a repository and sets the remote fetch and push urls.
// Setting the Workingdir package global affects the working directory in which the command is executed.
// The status channel 'clonechan' is closed when this function returns.
// (git clone ...)
func (gincl *Client) Clone(repoPath string, clonechan chan<- RepoFileStatus) {
	fn := fmt.Sprintf("Clone(%s)", repoPath)
	defer close(clonechan)
	remotePath := fmt.Sprintf("ssh://%s@%s/%s", gincl.GitUser, gincl.GitHost, repoPath)
	args := []string{"clone", "--progress", remotePath}
	if runtime.GOOS == "windows" {
		// force disable symlinks even if user can create them
		// see https://git-annex.branchable.com/bugs/Symlink_support_on_Windows_10_Creators_Update_with_Developer_Mode/
		args = append([]string{"-c", "core.symlinks=false"}, args...)
	}
	cmd := GitCommand(args...)
	err := cmd.Start()
	if err != nil {
		clonechan <- RepoFileStatus{Err: ginerror{UError: err.Error(), Origin: fn}}
		return
	}

	var line string
	var stderr []byte
	var status RepoFileStatus
	status.State = "Downloading repository"
	clonechan <- status
	var rerr error
	readbuffer := make([]byte, 1024)
	var nread, errhead int
	var eob, eof bool
	// git clone progress prints to stderr
	for eof = false; !eof; nread, rerr = cmd.ErrReader.Read(readbuffer) {
		if rerr != nil && errhead == len(stderr) {
			eof = true
		}
		stderr = append(stderr, readbuffer[:nread]...)
		for eob = false; !eob || errhead < len(stderr); line, eob = cutline(stderr[errhead:]) {
			if len(line) == 0 {
				errhead++
				break
			}
			errhead += len(line) + 1
			words := strings.Fields(line)
			status.FileName = repoPath
			if strings.HasPrefix(line, "Receiving objects") {
				if len(words) > 2 {
					status.Progress = words[2]
				}
				if len(words) > 8 {
					rate := fmt.Sprintf("%s%s", words[7], words[8])
					if strings.HasSuffix(rate, ",") {
						rate = strings.TrimSuffix(rate, ",")
					}
					status.Rate = rate
				}
			}
			clonechan <- status
		}
	}
	errstring := string(stderr)
	if err = cmd.Wait(); err != nil {
		util.LogWrite("Error during clone command")
		repoOwner, repoName := splitRepoParts(repoPath)
		gerr := ginerror{UError: errstring, Origin: fn}
		if strings.Contains(errstring, "does not exist") {
			gerr.Description = fmt.Sprintf("Repository download failed\n"+
				"Make sure you typed the repository path correctly\n"+
				"Type 'gin repos %s' to see if the repository exists and if you have access to it",
				repoOwner)
		} else if strings.Contains(errstring, "already exists and is not an empty directory") {
			gerr.Description = fmt.Sprintf("Repository download failed.\n"+
				"'%s' already exists in the current directory and is not empty.", repoName)
		} else if strings.Contains(errstring, "Host key verification failed") {
			gerr.Description = "Server key does not match known/configured host key."
		} else {
			gerr.Description = fmt.Sprintf("Repository download failed. Internal git command returned: %s", errstring)
		}
		status.Err = gerr
		clonechan <- status
		// doesn't really need to break here, but let's not send the progcomplete
		return
	}
	// Progress doesn't show 100% if cloning an empty repository, so let's force it
	status.Progress = progcomplete
	clonechan <- status
	return
}

// **************** //

// Git annex commands

type annexAction struct {
	Command string `json:"command"`
	Note    string `json:"note"`
	Success bool   `json:"success"`
	Key     string `json:"key"`
	File    string `json:"file"`
}

type annexProgress struct {
	Action          annexAction `json:"action"`
	ByteProgress    int         `json:"byte-progress"`
	TotalSize       int         `json:"total-size"`
	PercentProgress string      `json:"percent-progress"`
}

// AnnexInit initialises the repository for annex.
// Setting the Workingdir package global affects the working directory in which the command is executed.
// (git annex init)
func AnnexInit(description string) error {
	args := []string{"init", description}
	cmd := AnnexCommand(args...)
	stdout, stderr, err := cmd.OutputError()
	if err != nil {
		initError := fmt.Errorf("Repository annex initialisation failed.\n%s", string(stderr))
		logstd(stdout, stderr)
		return initError
	}
	cmd = GitCommand("config", "annex.backends", "MD5")
	stdout, stderr, err = cmd.OutputError()
	if err != nil {
		util.LogWrite("Failed to set default annex backend MD5")
		util.LogWrite("[Error]: %v", string(stderr))
		logstd(stdout, stderr)
	}
	return nil
}

// AnnexPull downloads all annexed files. Optionally also downloads all file content.
// Setting the Workingdir package global affects the working directory in which the command is executed.
// (git annex sync --no-push [--content])
func AnnexPull() error {
	args := []string{"sync", "--no-push", "--no-commit"}
	cmd := AnnexCommand(args...)
	stdout, stderr, err := cmd.OutputError()
	if err != nil {
		util.LogWrite("Error during AnnexPull.")
		util.LogWrite("[Error]: %v", err)
		logstd(stdout, stderr)
		errmsg := "failed"
		sstderr := string(stderr)
		if strings.Contains(sstderr, "Permission denied") {
			errmsg = "download failed: permission denied"
		} else if strings.Contains(sstderr, "Host key verification failed") {
			errmsg = "download failed: server key does not match known host key"
		} else if strings.Contains(sstderr, "would be overwritten by merge") {
			errmsg = "download failed: local modified or untracked file would be overwritten by download"
			// TODO: Which file
		}
		err = fmt.Errorf(errmsg)
	}
	return err
}

// AnnexSync synchronises the local repository with the remote.
// Optionally synchronises content if content=True.
// Setting the Workingdir package global affects the working directory in which the command is executed.
// The status channel 'syncchan' is closed when this function returns.
// (git annex sync [--content])
func AnnexSync(content bool, syncchan chan<- RepoFileStatus) {
	defer close(syncchan)
	args := []string{"sync"}
	if content {
		args = append(args, "--content")
	}
	cmd := AnnexCommand(args...)
	cmd.Start()
	var status RepoFileStatus
	status.State = "Synchronising repository"
	syncchan <- status
	var line string
	var rerr error
	for rerr = nil; rerr == nil; line, rerr = cmd.OutReader.ReadString('\n') {
		line = strings.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		words := strings.Fields(line)
		if words[0] == "copy" || words[0] == "get" {
			status.FileName = strings.TrimSpace(words[1])
			// new file - reset Progress and Rate
			status.Progress = ""
			status.Rate = ""
			if words[len(words)-1] != "ok" {
				// if the copy line ends with ok, the file is already done (no upload needed)
				// so we shouldn't send the status to the caller
				syncchan <- status
			}
		} else if strings.Contains(line, "%") {
			status.Progress = words[1]
			status.Rate = words[2]
			syncchan <- status
		}
	}

	var stderr string
	for rerr = nil; rerr == nil; line, rerr = cmd.ErrReader.ReadString('\000') {
		stderr += line
	}
	if err := cmd.Wait(); err != nil {
		util.LogWrite("Error during AnnexSync")
		util.LogWrite("[stderr]\n%s", stderr)
		util.LogWrite("[Error]: %v", err)
	}
	status.Progress = progcomplete
	syncchan <- status
	return
}

// AnnexPush uploads all annexed files.
// Setting the Workingdir package global affects the working directory in which the command is executed.
// The status channel 'pushchan' is closed when this function returns.
// (git annex sync --no-pull; git annex copy --to=origin)
func AnnexPush(paths []string, commitmsg string, pushchan chan<- RepoFileStatus) {
	defer close(pushchan)
	// NOTE: Using origin which is the conventional default remote. This should change to work with alternate remotes.
	remote := "origin"
	cmdargs := []string{"sync", "--no-pull", "--commit", fmt.Sprintf("--message=%s", commitmsg)}
	cmd := AnnexCommand(cmdargs...)
	stdout, stderr, err := cmd.OutputError()
	// TODO: Parse git push output for progress
	if err != nil {
		util.LogWrite("Error during AnnexPush (sync --no-pull)")
		util.LogWrite("[Error]: %v", err)
		logstd(stdout, stderr)
		errmsg := "failed"
		sstderr := string(stderr)
		if strings.Contains(sstderr, "Permission denied") {
			errmsg = "upload failed: permission denied"
		} else if strings.Contains(sstderr, "Host key verification failed") {
			errmsg = "upload failed: server key does not match known host key"
		} else if strings.Contains(sstderr, "rejected") {
			errmsg = "upload failed: changes were made on the server that have not been downloaded; run 'gin download' to update local copies"
		}
		pushchan <- RepoFileStatus{Err: fmt.Errorf(errmsg)}
		return
	}

	cmdargs = []string{"copy", "--json-progress", fmt.Sprintf("--to=%s", remote)}
	cmdargs = append(cmdargs, paths...)
	cmd = AnnexCommand(cmdargs...)
	err = cmd.Start()
	if err != nil {
		pushchan <- RepoFileStatus{Err: err}
		return
	}

	var status RepoFileStatus
	status.State = "Uploading"

	var outline []byte
	var rerr error
	var progress annexProgress
	var getresult annexAction

	var prevByteProgress int
	var prevT time.Time
	for rerr = nil; rerr == nil; outline, rerr = cmd.OutReader.ReadBytes('\n') {
		if len(outline) == 0 {
			// skip empty lines
			continue
		}
		err := json.Unmarshal(outline, &progress)
		if err != nil || progress == (annexProgress{}) {
			// File done? Check if succeeded and continue to next line
			err = json.Unmarshal(outline, &getresult)
			if err != nil || getresult == (annexAction{}) {
				// Couldn't parse output
				util.LogWrite("Could not parse 'git annex copy' output")
				util.LogWrite(string(outline))
				util.LogWrite(err.Error())
				// TODO: Print error at the end: Command succeeded but there was an error understanding the output
				continue
			}
			status.FileName = getresult.File
			if getresult.Success {
				status.Progress = progcomplete
				status.Err = nil
			} else {
				errmsg := getresult.Note
				if strings.Contains(errmsg, "Unable to access") {
					errmsg = "authorisation failed or remote storage unavailable"
				}
				status.Err = fmt.Errorf("failed: %s", errmsg)
			}
		} else {
			status.FileName = progress.Action.File
			status.Progress = progress.PercentProgress

			dbytes := progress.ByteProgress - prevByteProgress
			now := time.Now()
			dt := now.Sub(prevT)
			status.Rate = calcRate(dbytes, dt)
			prevByteProgress = progress.ByteProgress
			prevT = now
			status.Err = nil
		}

		pushchan <- status
	}
	if cmd.Wait() != nil {
		var stderr, errline []byte
		for rerr = nil; rerr == nil; errline, rerr = cmd.OutReader.ReadBytes('\000') {
			stderr = append(stderr, errline...)
		}
		util.LogWrite("Error during AnnexGet")
		util.LogWrite(string(stderr))
	}
	return
}

// AnnexCommit performs a commit by calling git-annex-sync, passing a commit message, and disabling both pull and push, so no actual synchronisation happens.
// Setting the Workingdir package global affects the working directory in which the command is executed.
// The status channel 'commitchan' is closed when this function returns.
// (git annex sync --no-push --no-pull)
func AnnexCommit(commitmsg string, commitchan chan<- RepoFileStatus) {
	defer close(commitchan)
	cmdargs := []string{"sync", "--no-pull", "--no-push", "--commit", fmt.Sprintf("--message=%s", commitmsg)}
	cmd := AnnexCommand(cmdargs...)
	var status RepoFileStatus
	status.State = "Recording changes"
	commitchan <- status
	stdout, stderr, err := cmd.OutputError()

	if err != nil {
		util.LogWrite("Error during AnnexCommit")
		logstd(stdout, stderr)
		status.Err = fmt.Errorf(string(stderr))
		commitchan <- status
		return
	}
	status.Progress = progcomplete
	commitchan <- status
	return
}

// AnnexGet retrieves the content of specified files.
// Setting the Workingdir package global affects the working directory in which the command is executed.
// The status channel 'getchan' is closed when this function returns.
// (git annex get)
func AnnexGet(filepaths []string, getchan chan<- RepoFileStatus) {
	defer close(getchan)
	cmdargs := append([]string{"get", "--json-progress"}, filepaths...)
	cmd := AnnexCommand(cmdargs...)
	if err := cmd.Start(); err != nil {
		getchan <- RepoFileStatus{Err: err}
		return
	}

	var status RepoFileStatus
	status.State = "Downloading"

	var outline []byte
	var rerr error
	var progress annexProgress
	var getresult annexAction
	var prevByteProgress int
	var prevT time.Time
	for rerr = nil; rerr == nil; outline, rerr = cmd.OutReader.ReadBytes('\n') {
		if len(outline) == 0 {
			// skip empty lines
			continue
		}
		err := json.Unmarshal(outline, &progress)
		if err != nil || progress == (annexProgress{}) {
			// File done? Check if succeeded and continue to next line
			err = json.Unmarshal(outline, &getresult)
			if err != nil || getresult == (annexAction{}) {
				// Couldn't parse output
				util.LogWrite("Could not parse 'git annex get' output")
				util.LogWrite(string(outline))
				util.LogWrite(err.Error())
				// TODO: Print error at the end: Command succeeded but there was an error understanding the output
				continue
			}
			status.FileName = getresult.File
			if getresult.Success {
				status.Progress = progcomplete
				status.Err = nil
			} else {
				errmsg := getresult.Note
				if strings.Contains(errmsg, "Unable to access") {
					errmsg = "authorisation failed or remote storage unavailable"
				}
				status.Err = fmt.Errorf("failed: %s", errmsg)
			}
		} else {
			status.FileName = progress.Action.File
			status.Progress = progress.PercentProgress
			dbytes := progress.ByteProgress - prevByteProgress
			now := time.Now()
			dt := now.Sub(prevT)
			status.Rate = calcRate(dbytes, dt)
			prevByteProgress = progress.ByteProgress
			prevT = now
			status.Err = nil
		}

		getchan <- status
	}
	if cmd.Wait() != nil {
		var stderr, errline []byte
		for rerr = nil; rerr == nil; errline, rerr = cmd.OutReader.ReadBytes('\000') {
			stderr = append(stderr, errline...)
		}
		util.LogWrite("Error during AnnexGet")
		util.LogWrite(string(stderr))
	}
	return
}

// AnnexDrop drops the content of specified files.
// Setting the Workingdir package global affects the working directory in which the command is executed.
// The status channel 'dropchan' is closed when this function returns.
// (git annex drop)
func AnnexDrop(filepaths []string, dropchan chan<- RepoFileStatus) {
	defer close(dropchan)
	cmdargs := append([]string{"drop", "--json"}, filepaths...)
	cmd := AnnexCommand(cmdargs...)
	err := cmd.Start()
	if err != nil {
		dropchan <- RepoFileStatus{Err: err}
		return
	}
	var status RepoFileStatus
	var annexDropRes struct {
		Command string `json:"command"`
		File    string `json:"file"`
		Key     string `json:"key"`
		Success bool   `json:"success"`
		Note    string `json:"note"`
	}

	status.State = "Removing content"
	var line string
	var rerr error
	for rerr = nil; rerr == nil; line, rerr = cmd.OutReader.ReadString('\n') {
		line = strings.TrimSpace(line)
		if len(line) == 0 {
			// Empty line output. Ignore
			continue
		}
		// Send file name
		err = json.Unmarshal([]byte(line), &annexDropRes)
		if err != nil {
			dropchan <- RepoFileStatus{Err: err}
			return
		}
		status.FileName = annexDropRes.File
		if annexDropRes.Success {
			util.LogWrite("%s content dropped", annexDropRes.File)
			status.Err = nil
		} else {
			util.LogWrite("Error dropping %s", annexDropRes.File)
			errmsg := annexDropRes.Note
			if strings.Contains(errmsg, "unsafe") {
				errmsg = "failed (unsafe): could not verify remote copy"
			}
			status.Err = fmt.Errorf(errmsg)
		}
		status.Progress = progcomplete
		dropchan <- status
	}
	if cmd.Wait() != nil {
		var stderr, errline []byte
		for rerr = nil; rerr == nil; errline, rerr = cmd.OutReader.ReadBytes('\000') {
			stderr = append(stderr, errline...)
		}
		util.LogWrite("Error during AnnexDrop")
		util.LogWrite("[stderr]\n%s", string(stderr))
	}
	return
}

// GitLsFiles lists all files known to git.
// In direct mode, the bare flag is temporarily switched off before running the command.
// The output channel 'lschan' is closed when this function returns.
// (git ls-files)
func GitLsFiles(args []string, lschan chan<- string) {
	defer close(lschan)
	cmdargs := append([]string{"ls-files"}, args...)
	cmd := GitCommand(cmdargs...)
	err := cmd.Start()
	if err != nil {
		util.LogWrite("ls-files command set up failed: %s", err)
		return
	}
	var line string
	var rerr error
	for rerr = nil; rerr == nil; line, rerr = cmd.OutReader.ReadString('\n') {
		line = strings.TrimSuffix(line, "\n")
		if line != "" {
			lschan <- line
		}
	}

	var stderr, errline []byte
	if cmd.Wait() != nil {
		for rerr = nil; rerr == nil; errline, rerr = cmd.OutReader.ReadBytes('\000') {
			stderr = append(stderr, errline...)
		}
		util.LogWrite("Error during GitLsFiles")
		logstd(nil, stderr)
	}
	return
}

// GitAdd adds paths to git directly (not annex).
// In direct mode, files that are already in the annex are explicitly ignored.
// In indirect mode, adding annexed files to git has no effect.
// Setting the Workingdir package global affects the working directory in which the command is executed.
// The status channel 'addchan' is closed when this function returns.
// (git add)
func GitAdd(filepaths []string, addchan chan<- RepoFileStatus) {
	defer close(addchan)
	if len(filepaths) == 0 {
		util.LogWrite("No paths to add to git. Nothing to do.")
		return
	}

	if IsDirect() {
		// Set bare false and revert at the end of the function
		err := setBare(false)
		if err != nil {
			addchan <- RepoFileStatus{Err: fmt.Errorf("failed to toggle repository bare mode")}
			return
		}
		defer setBare(true)
		wichan := make(chan AnnexWhereisRes)
		go AnnexWhereis(filepaths, wichan)
		var annexfiles []string
		for wiInfo := range wichan {
			if wiInfo.Err != nil {
				continue
			}
			annexfiles = append(annexfiles, wiInfo.File)
		}
		filepaths = util.FilterPaths(filepaths, annexfiles)
	}

	cmdargs := append([]string{"add", "--verbose", "--"}, filepaths...)
	cmd := GitCommand(cmdargs...)
	err := cmd.Start()
	if err != nil {
		addchan <- RepoFileStatus{Err: err}
		return
	}
	var status RepoFileStatus
	var line string
	var rerr error
	for rerr = nil; rerr == nil; line, rerr = cmd.OutReader.ReadString('\n') {
		fname := strings.TrimSpace(line)
		if len(fname) == 0 {
			// skip empty lines
			continue
		}
		if strings.HasPrefix(fname, "add") {
			status.State = "Adding (git)  "
			fname = strings.TrimPrefix(fname, "add '")
		} else if strings.HasPrefix(fname, "remove") {
			status.State = "Removing"
			fname = strings.TrimPrefix(fname, "remove '")
		}
		fname = strings.TrimSuffix(fname, "'")
		status.FileName = fname
		util.LogWrite("'%s' added to git", fname)
		// Error conditions?
		status.Progress = progcomplete
		addchan <- status
	}
	var stderr, errline []byte
	if cmd.Wait() != nil {
		for rerr = nil; rerr == nil; errline, rerr = cmd.OutReader.ReadBytes('\000') {
			stderr = append(stderr, errline...)
		}
		util.LogWrite("Error during GitAdd")
		logstd(nil, stderr)
	}
	return
}

// build exclusion argument list
// files < annex.minsize or matching exclusion extensions will not be annexed and
// will instead be handled by git
func annexExclArgs() (exclargs []string) {
	if util.Config.Annex.MinSize != "" {
		sizefilterarg := fmt.Sprintf("--largerthan=%s", util.Config.Annex.MinSize)
		exclargs = append(exclargs, sizefilterarg)
	}

	for _, pattern := range util.Config.Annex.Exclude {
		arg := fmt.Sprintf("--exclude=%s", pattern)
		exclargs = append(exclargs, arg)
	}

	// explicitly exclude config file
	exclargs = append(exclargs, "--exclude=config.yml")
	return
}

// annexAddCommon is the common function that serves both AnnexAdd() and AnnexLock().
// AnnexLock() is performed by passing true to 'update'.
func annexAddCommon(filepaths []string, update bool, addchan chan<- RepoFileStatus) {
	defer close(addchan)
	if len(filepaths) == 0 {
		util.LogWrite("No paths to add to annex. Nothing to do.")
		return
	}
	cmdargs := []string{"add", "--json"}
	if update {
		cmdargs = append(cmdargs, "--update")
	}
	cmdargs = append(cmdargs, filepaths...)

	exclargs := annexExclArgs()
	if len(exclargs) > 0 {
		cmdargs = append(cmdargs, exclargs...)
	}

	cmd := AnnexCommand(cmdargs...)
	err := cmd.Start()
	if err != nil {
		addchan <- RepoFileStatus{Err: err}
		return
	}

	var outline []byte
	var rerr error
	var status RepoFileStatus
	var addresult annexAction
	status.State = "Adding (annex)"
	if update {
		status.State = "Locking"
	}
	for rerr = nil; rerr == nil; outline, rerr = cmd.OutReader.ReadBytes('\n') {
		if len(outline) == 0 {
			// Empty line output. Ignore
			continue
		}
		err := json.Unmarshal(outline, &addresult)
		if err != nil || addresult == (annexAction{}) {
			// Couldn't parse output
			util.LogWrite("Could not parse 'git annex add' output")
			util.LogWrite(string(outline))
			util.LogWrite(err.Error())
			// TODO: Print error at the end: Command succeeded but there was an error understanding the output
			continue
		}
		status.FileName = addresult.File
		if addresult.Success {
			util.LogWrite("%s added to annex", addresult.File)
			status.Err = nil
		} else {
			util.LogWrite("Error adding %s", addresult.File)
			status.Err = fmt.Errorf("failed")
		}
		status.Progress = progcomplete
		addchan <- status
	}
	var stderr, errline []byte
	if cmd.Wait() != nil {
		for rerr = nil; rerr == nil; errline, rerr = cmd.OutReader.ReadBytes('\000') {
			stderr = append(stderr, errline...)
		}
		util.LogWrite("Error during AnnexAdd")
		logstd(nil, stderr)
	}
	return
}

// AnnexAdd adds paths to the annex.
// Files specified for exclusion in the configuration are ignored automatically.
// Setting the Workingdir package global affects the working directory in which the command is executed.
// The status channel 'addchan' is closed when this function returns.
// (git annex add)
func AnnexAdd(filepaths []string, addchan chan<- RepoFileStatus) {
	annexAddCommon(filepaths, false, addchan)
}

// AnnexWhereisRes holds the output of a "git annex whereis" command
type AnnexWhereisRes struct {
	File      string   `json:"file"`
	Command   string   `json:"command"`
	Note      string   `json:"note"`
	Success   bool     `json:"success"`
	Untrusted []string `json:"untrusted"`
	Key       string   `json:"key"`
	Whereis   []struct {
		Here        bool     `json:"here"`
		UUID        string   `json:"uuid"`
		URLs        []string `json:"urls"`
		Description string   `json:"description"`
	}
	Err error `json:"err"`
}

// AnnexWhereis returns information about annexed files in the repository
// Setting the Workingdir package global affects the working directory in which the command is executed.
// The output channel 'wichan' is closed when this function returns.
// (git annex whereis)
func AnnexWhereis(paths []string, wichan chan<- AnnexWhereisRes) {
	defer close(wichan)
	cmdargs := []string{"whereis", "--json"}
	cmdargs = append(cmdargs, paths...)
	cmd := AnnexCommand(cmdargs...)
	err := cmd.Start()
	if err != nil {
		util.LogWrite("Error during AnnexWhereis")
		wichan <- AnnexWhereisRes{Err: fmt.Errorf("Failed to run git-annex whereis: %s", err)}
		return
	}

	var line string
	var rerr error
	var info AnnexWhereisRes
	for rerr = nil; rerr == nil; line, rerr = cmd.OutReader.ReadString('\n') {
		line = strings.TrimSpace(line)
		if len(line) == 0 {
			// Empty line output. Ignore
			continue
		}
		jsonerr := json.Unmarshal([]byte(line), &info)
		info.Err = jsonerr
		wichan <- info
	}
	return
}

// AnnexStatusRes for getting the (annex) status of individual files
type AnnexStatusRes struct {
	Status string `json:"status"`
	File   string `json:"file"`
	Err    error  `json:"err"`
}

// AnnexStatus returns the status of a file or files in a directory
// Setting the Workingdir package global affects the working directory in which the command is executed.
// The output channel 'statuschan' is closed when this function returns.
// (git annex status)
func AnnexStatus(paths []string, statuschan chan<- AnnexStatusRes) {
	defer close(statuschan)
	cmdargs := []string{"status", "--json"}
	cmdargs = append(cmdargs, paths...)
	cmd := AnnexCommand(cmdargs...)
	// TODO: Parse output
	err := cmd.Start()
	if err != nil {
		util.LogWrite("Error setting up git-annex status")
		statuschan <- AnnexStatusRes{Err: fmt.Errorf("Failed to run git-annex status: %s", err)}
		return
	}

	var line string
	var rerr error
	var status AnnexStatusRes
	for rerr = nil; rerr == nil; line, rerr = cmd.OutReader.ReadString('\n') {
		line = strings.TrimSpace(line)
		if len(line) == 0 {
			// Empty line output. Ignore
			continue
		}
		jsonerr := json.Unmarshal([]byte(line), &status)
		status.Err = jsonerr
		statuschan <- status
	}
	return
}

// DescribeIndexShort returns a string which represents a condensed form of the git (annex) index.
// It is constructed using the result of 'git annex status'.
// The description is composed of the file count for each status: added, modified, deleted
func DescribeIndexShort() (string, error) {
	// TODO: 'git annex status' doesn't list added (A) files when in direct mode.
	statuschan := make(chan AnnexStatusRes)
	go AnnexStatus([]string{}, statuschan)
	statusmap := make(map[string]int)
	for item := range statuschan {
		if item.Err != nil {
			return "", item.Err
		}
		statusmap[item.Status]++
	}

	var changesBuffer bytes.Buffer
	if statusmap["A"] > 0 {
		_, _ = changesBuffer.WriteString(fmt.Sprintf("New files: %d\n", statusmap["A"]))
	}
	if statusmap["M"] > 0 {
		_, _ = changesBuffer.WriteString(fmt.Sprintf("Modified files: %d\n", statusmap["M"]))
	}
	if statusmap["D"] > 0 {
		_, _ = changesBuffer.WriteString(fmt.Sprintf("Deleted files: %d\n", statusmap["D"]))
	}
	return changesBuffer.String(), nil
}

// DescribeIndex returns a string which describes the git (annex) index.
// It is constructed using the result of 'git annex status'.
// The resulting message can be used to inform the user of changes
// that are about to be uploaded and as a long commit message.
func DescribeIndex() (string, error) {
	statuschan := make(chan AnnexStatusRes)
	go AnnexStatus([]string{}, statuschan)
	statusmap := make(map[string][]string)
	for item := range statuschan {
		if item.Err != nil {
			return "", item.Err
		}
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

// AnnexLock locks the specified files and directory contents if they are annexed.
// Note that this function uses 'git annex add' to lock files, but only if they are marked as unlocked (T) by git annex.
// Attempting to lock an untracked file, or a file in any state other than T will have no effect.
// Setting the Workingdir package global affects the working directory in which the command is executed.
// The status channel 'lockchan' is closed when this function returns.
// (git annex add --update)
func AnnexLock(filepaths []string, lockchan chan<- RepoFileStatus) {
	annexAddCommon(filepaths, true, lockchan)
}

// AnnexUnlock unlocks the specified files and directory contents if they are annexed
// Setting the Workingdir package global affects the working directory in which the command is executed.
// The status channel 'unlockchan' is closed when this function returns.
// (git annex unlock)
func AnnexUnlock(filepaths []string, unlockchan chan<- RepoFileStatus) {
	defer close(unlockchan)
	cmdargs := []string{"unlock", "--json"}
	cmdargs = append(cmdargs, filepaths...)
	cmd := AnnexCommand(cmdargs...)
	err := cmd.Start()
	if err != nil {
		unlockchan <- RepoFileStatus{Err: err}
		return
	}
	var status RepoFileStatus
	status.State = "Unlocking"

	var outline []byte
	var rerr error
	var unlockres annexAction
	for rerr = nil; rerr == nil; outline, rerr = cmd.OutReader.ReadBytes('\n') {
		if len(outline) == 0 {
			// Empty line output. Ignore
			continue
		}
		// Send file name
		err = json.Unmarshal(outline, &unlockres)
		if err != nil || unlockres == (annexAction{}) {
			// Couldn't parse output
			util.LogWrite("Could not parse 'git annex unlock' output")
			util.LogWrite(string(outline))
			util.LogWrite(err.Error())
			// TODO: Print error at the end: Command succeeded but there was an error understanding the output
			continue
		}
		status.FileName = unlockres.File
		if unlockres.Success {
			util.LogWrite("%s unlocked", unlockres.File)
			status.Err = nil
		} else {
			util.LogWrite("Error unlocking %s", unlockres.File)
			status.Err = fmt.Errorf("Content not available locally\nUse 'gin get-content' to download")
		}
		status.Progress = progcomplete
		unlockchan <- status
	}
	var stderr, errline []byte
	if cmd.Wait() != nil {
		for rerr = nil; rerr == nil; errline, rerr = cmd.OutReader.ReadBytes('\000') {
			stderr = append(stderr, errline...)
		}
		util.LogWrite("Error during AnnexUnlock")
		logstd(nil, stderr)
	}
	status.Progress = progcomplete
	return
}

// AnnexInfoRes holds the information returned by AnnexInfo
type AnnexInfoRes struct {
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
// Setting the Workingdir package global affects the working directory in which the command is executed.
// (git annex info)
func AnnexInfo() (AnnexInfoRes, error) {
	cmd := AnnexCommand("info", "--json")
	stdout, stderr, err := cmd.OutputError()
	if err != nil || cmd.Wait() != nil {
		util.LogWrite("Error during AnnexInfo")
		logstd(stdout, stderr)
		return AnnexInfoRes{}, fmt.Errorf("Error retrieving annex info")
	}

	var info AnnexInfoRes
	err = json.Unmarshal(stdout, &info)
	return info, err
}

// GinCommit describes a commit, retrieved from the git log.
type GinCommit struct {
	Hash            string    `json:"hash"`
	AbbreviatedHash string    `json:"abbrevhash"`
	AuthorName      string    `json:"authorname"`
	AuthorEmail     string    `json:"authoremail"`
	Date            time.Time `json:"date"`
	Subject         string    `json:"subject"`
	Body            string    `json:"body"`
	FileStats       DiffStat
}

// GitLog returns the commit logs for the repository.
// The number of commits can be limited by the count argument.
// If count <= 0, the entire commit history is returned.
// Revisions which match only the deletion of the matching paths can be filtered using the showdeletes argument.
func GitLog(count uint, revrange string, paths []string, showdeletes bool) ([]GinCommit, error) {
	logformat := `{"hash":"%H","abbrevhash":"%h","authorname":"%an","authoremail":"%ae","date":"%aI","subject":"%s","body":"%b"}`
	cmdargs := []string{"log", "-z", fmt.Sprintf("--format=%s", logformat)}
	if count > 0 {
		cmdargs = append(cmdargs, fmt.Sprintf("--max-count=%d", count))
	}
	if !showdeletes {
		cmdargs = append(cmdargs, "--diff-filter=d")
	}
	if revrange != "" {
		cmdargs = append(cmdargs, revrange)
	}

	cmdargs = append(cmdargs, "--") // separate revisions from paths, even if there are no paths
	if paths != nil && len(paths) > 0 {
		cmdargs = append(cmdargs, paths...)
	}
	cmd := GitCommand(cmdargs...)
	err := cmd.Start()
	if err != nil {
		util.LogWrite("Error setting up git log command")
		return nil, fmt.Errorf("error retrieving version logs - malformed git log command")
	}

	var line []byte
	var rerr error
	var commits []GinCommit
	for rerr = nil; rerr == nil; line, rerr = cmd.OutReader.ReadBytes('\000') {
		line = bytes.TrimSuffix(line, []byte("\000"))
		if len(line) == 0 {
			continue
		}
		// Escape newlines and tabs (from body)
		line = bytes.Replace(line, []byte("\n"), []byte("\\n"), -1)
		line = bytes.Replace(line, []byte("\t"), []byte("\\t"), -1)
		var commit GinCommit
		ierr := json.Unmarshal(line, &commit)
		if ierr != nil {
			util.LogWrite("Error parsing git log")
			util.LogWrite(string(line))
			util.LogWrite(ierr.Error())
			continue
		}
		// Trim potential newline or spaces at end of body
		commit.Body = strings.TrimSpace(commit.Body)
		commits = append(commits, commit)
	}

	var stderr, errline []byte
	if cmd.Wait() != nil {
		for rerr = nil; rerr == nil; errline, rerr = cmd.OutReader.ReadBytes('\000') {
			stderr = append(stderr, errline...)
		}
		util.LogWrite("Error getting git log")
		errmsg := string(stderr)
		if strings.Contains(errmsg, "bad revision") {
			errmsg = fmt.Sprintf("'%s' does not match a known version ID or name", revrange)
		}
		return nil, fmt.Errorf(errmsg)
	}

	// TODO: Combine diffstats into first git log invocation
	logstats, err := GitLogDiffstat(count, paths, showdeletes)
	if err != nil {
		util.LogWrite("Failed to get diff stats")
		return commits, nil
	}

	for idx, commit := range commits {
		commits[idx].FileStats = logstats[commit.Hash]
	}

	return commits, nil
}

type DiffStat struct {
	NewFiles      []string
	DeletedFiles  []string
	ModifiedFiles []string
}

func GitLogDiffstat(count uint, paths []string, showdeletes bool) (map[string]DiffStat, error) {
	logformat := `::%H`
	cmdargs := []string{"log", fmt.Sprintf("--format=%s", logformat), "--name-status"}
	if count > 0 {
		cmdargs = append(cmdargs, fmt.Sprintf("--max-count=%d", count))
	}
	if !showdeletes {
		cmdargs = append(cmdargs, "--diff-filter=d")
	}
	cmdargs = append(cmdargs, "--") // separate revisions from paths, even if there are no paths
	if paths != nil && len(paths) > 0 {
		cmdargs = append(cmdargs, paths...)
	}
	cmd := GitCommand(cmdargs...)
	err := cmd.Start()
	if err != nil {
		util.LogWrite("Error during GitLogDiffstat")
		return nil, err
	}

	stats := make(map[string]DiffStat)
	var curhash string
	var curstat DiffStat

	var line string
	var rerr error
	for rerr = nil; rerr == nil; line, rerr = cmd.OutReader.ReadString('\n') {
		// Avoid trimming spaces at end of filenames
		line = strings.TrimSuffix(line, "\n")
		if len(strings.TrimSpace(line)) == 0 { // but still check if the line is only spaces
			continue
		}
		if strings.HasPrefix(line, "::") {
			curhash = strings.TrimPrefix(line, "::")
			curstat = DiffStat{}
		} else {
			// parse name-status
			fstat := strings.SplitN(line, "\t", 2) // stat (A, M, or D) and filename
			if len(fstat) < 2 {
				continue
			}
			stat := fstat[0]
			fname := fstat[1]
			switch stat {
			case "A":
				nf := curstat.NewFiles
				curstat.NewFiles = append(nf, fname)
			case "M":
				mf := curstat.ModifiedFiles
				curstat.ModifiedFiles = append(mf, fname)
			case "D":
				df := curstat.DeletedFiles
				curstat.DeletedFiles = append(df, fname)
			case "R100":
				// Ignore renames
			default:
				util.LogWrite("Could not parse diffstat line")
				util.LogWrite(line)
			}
			stats[curhash] = curstat
		}
	}

	return stats, nil
}

// GitCheckout performs a git checkout of a specific commit.
// Individual files or directories may be specified, otherwise the entire tree is checked out.
func GitCheckout(hash string, paths []string) error {
	cmdargs := []string{"checkout", hash, "--"}
	if paths == nil || len(paths) == 0 {
		reporoot, _ := util.FindRepoRoot(".")
		Workingdir = reporoot
		paths = []string{"."}
	}
	cmdargs = append(cmdargs, paths...)

	cmd := GitCommand(cmdargs...)
	stdout, stderr, err := cmd.OutputError()
	if err != nil {
		util.LogWrite("Error during GitCheckout")
		logstd(stdout, stderr)
		return fmt.Errorf(string(stderr))
	}
	return nil
}

// GitObject contains the information for a tree or blob object in git
type GitObject struct {
	Name string
	Hash string
	Type string
	Mode string
}

// GitLsTree performs a recursive git ls-tree with a given revision (hash) and a list of paths.
// For each item, it returns a struct which contains the type (blob, tree), the mode, the hash, and the absolute (repo rooted) path to the object (name).
// Setting the Workingdir package global affects the working directory in which the command is executed.
func GitLsTree(revision string, paths []string) ([]GitObject, error) {
	cmdargs := []string{"ls-tree", "--full-tree", "-z", "-t", "-r", revision}
	cmdargs = append(cmdargs, paths...)
	cmd := GitCommand(cmdargs...)
	// This command doesn't need to be read line-by-line
	err := cmd.Start()
	if err != nil {
		return nil, err
	}

	var objects []GitObject
	var line string
	var rerr error
	for rerr = nil; rerr == nil; line, rerr = cmd.OutReader.ReadString('\000') {
		line = strings.TrimSuffix(line, "\000")
		if len(line) == 0 {
			continue
		}
		words := strings.Fields(line)
		fnamesplit := strings.SplitN(line, "\t", 2)
		if len(words) < 4 || len(fnamesplit) < 2 {
			continue
		}
		obj := GitObject{
			Mode: words[0],
			Type: words[1],
			Hash: words[2],
			Name: fnamesplit[1],
		}
		objects = append(objects, obj)
	}

	var stderr, errline []byte
	if cmd.Wait() != nil {
		for rerr = nil; rerr == nil; errline, rerr = cmd.OutReader.ReadBytes('\000') {
			stderr = append(stderr, errline...)
		}
		util.LogWrite("Error during GitLsTree")
		logstd(nil, stderr)
		return nil, fmt.Errorf(string(stderr))
	}

	return objects, nil
}

// GitCatFileContents performs a git-cat-file of a specific file from a specific commit and returns the file contents (as bytes).
// Setting the Workingdir package global affects the working directory in which the command is executed.
func GitCatFileContents(revision, filepath string) ([]byte, error) {
	cmd := GitCommand("cat-file", "blob", fmt.Sprintf("%s:./%s", revision, filepath))
	stdout, stderr, err := cmd.OutputError()
	if err != nil {
		util.LogWrite("Error during GitCatFile (Contents)")
		logstd(nil, stderr)
		return nil, fmt.Errorf(string(stderr))
	}
	return stdout, nil
}

// GitCatFileType returns the type of a given object at a given revision (blob, tree, or commit)
// Setting the Workingdir package global affects the working directory in which the command is executed.
func GitCatFileType(object string) (string, error) {
	cmd := GitCommand("cat-file", "-t", object)
	stdout, stderr, err := cmd.OutputError()
	if err != nil {
		util.LogWrite("Error during GitCatFile (Type)")
		logstd(stdout, stderr)
		err = fmt.Errorf(string(stderr))
		return "", err
	}
	return string(stdout), nil
}

// AnnexFromKey creates an Annex placeholder file at a given location with a specific key.
// The creation is forced, so there is no guarantee that the key refers to valid repository content, nor that the content is still available in any of the remotes.
// The location where the file is to be created must be available (no directories are created).
// Setting the Workingdir package global affects the working directory in which the command is executed.
// (git annex fromkey --force)
func AnnexFromKey(key, filepath string) error {
	cmd := AnnexCommand("fromkey", "--force", key, filepath)
	stdout, stderr, err := cmd.OutputError()
	if err != nil {
		logstd(stdout, stderr)
		return fmt.Errorf(string(stderr))
	}
	return nil
}

// GitRevCount returns the number of commits between two revisions.
// Setting the Workingdir package global affects the working directory in which the command is executed.
func GitRevCount(a, b string) (int, error) {
	cmd := GitCommand("rev-list", "--count", fmt.Sprintf("%s..%s", a, b))
	stdout, stderr, err := cmd.OutputError()
	if err != nil {
		logstd(stdout, stderr)
		return 0, fmt.Errorf(string(stderr))
	}
	return strconv.Atoi(string(stdout))
}

var annexmodecache = make(map[string]bool)

// IsDirect returns true if the repository in a given path is working in git annex 'direct' mode.
// If path is not a repository, or is not an initialised annex repository, the result defaults to false.
// If the path is a repository and no error was raised, the result it cached so that subsequent checks are faster.
// Setting the Workingdir package global affects the working directory in which the command is executed.
func IsDirect() bool {
	if mode, ok := annexmodecache[Workingdir]; ok {
		return mode
	}
	cmd := GitCommand("config", "--local", "annex.direct")
	stdout, _, err := cmd.OutputError()
	if err != nil {
		// Don't cache this result
		return false
	}

	if strings.TrimSpace(string(stdout)) == "true" {
		annexmodecache[Workingdir] = true
		return true
	}
	annexmodecache[Workingdir] = false
	return false
}

// IsVersion6 returns true if the repository in a given path is working in git annex 'direct' mode.
// If path is not a repository, or is not an initialised annex repository, the result defaults to false.
// Setting the Workingdir package global affects the working directory in which the command is executed.
func IsVersion6() bool {
	cmd := GitCommand("config", "--local", "--get", "annex.version")
	stdout, stderr, err := cmd.OutputError()
	if err != nil {
		util.LogWrite("Error while checking repository annex version")
		logstd(stdout, stderr)
		return false
	}
	ver := strings.TrimSpace(string(stdout))
	util.LogWrite("Annex version is %s", ver)
	return ver == "6"
}

// Utility functions for shelling out

// GitCommand sets up an external git command with the provided arguments and returns a GinCmd struct.
// Setting the Workingdir package global affects the working directory in which the command will be executed.
func GitCommand(args ...string) util.GinCmd {
	gitbin := util.Config.Bin.Git
	cmd := util.Command(gitbin)
	cmd.Dir = Workingdir
	cmd.Args = append(cmd.Args, args...)
	token := web.UserToken{}
	_ = token.LoadToken()
	env := os.Environ()
	cmd.Env = append(env, util.GitSSHEnv(token.Username))
	util.LogWrite("Running shell command (Dir: %s): %s", Workingdir, strings.Join(cmd.Args, " "))
	return cmd
}

// AnnexCommand sets up a git annex command with the provided arguments and returns a GinCmd struct.
// Setting the Workingdir package global affects the working directory in which the command will be executed.
func AnnexCommand(args ...string) util.GinCmd {
	gitannexbin := util.Config.Bin.GitAnnex
	cmd := util.Command(gitannexbin, args...)
	cmd.Dir = Workingdir
	token := web.UserToken{}
	_ = token.LoadToken()
	env := os.Environ()
	cmd.Env = append(env, util.GitSSHEnv(token.Username))
	cmd.Env = append(cmd.Env, "GIT_ANNEX_USE_GIT_SSH=1")
	util.LogWrite("Running shell command (Dir: %s): %s", Workingdir, strings.Join(cmd.Args, " "))
	return cmd
}

// GetAnnexVersion returns the version string of the system's git-annex.
func GetAnnexVersion() (string, error) {
	cmd := AnnexCommand("version", "--raw")
	stdout, _, err := cmd.OutputError()
	if err != nil {
		util.LogWrite("Error while preparing git-annex version command")
		if strings.Contains(err.Error(), "executable file not found") ||
			strings.Contains(err.Error(), "no such file or directory") {
			return "", fmt.Errorf("git-annex executable '%s' not found", util.Config.Bin.GitAnnex)
		}
		return "", err
	}
	return string(stdout), nil
}

// Local utility functions

func logstd(out, err []byte) {
	util.LogWrite("[stdout]\n%s\n[stderr]\n%s", string(out), string(err))
}

func splitRepoParts(repoPath string) (repoOwner, repoName string) {
	repoPathParts := strings.SplitN(repoPath, "/", 2)
	repoOwner = repoPathParts[0]
	repoName = repoPathParts[1]
	return
}

func cutline(b []byte) (string, bool) {
	idx := -1
	cridx := bytes.IndexByte(b, '\r')
	nlidx := bytes.IndexByte(b, '\n')
	if cridx > 0 {
		idx = cridx
	} else {
		cridx = len(b) + 1
	}
	if nlidx > 0 && nlidx < cridx {
		idx = nlidx
	}
	if idx <= 0 {
		return string(b), true
	}
	return string(b[:idx]), false
}

func setBare(state bool) error {
	var statestr string
	if state {
		statestr = "true"
	} else {
		statestr = "false"
	}
	cmd := GitCommand("config", "--local", "--bool", "core.bare", statestr)
	stdout, stderr, err := cmd.OutputError()
	if err != nil {
		util.LogWrite("Error switching bare status to %s", statestr)
		logstd(stdout, stderr)
		err = fmt.Errorf(string(stderr))
	}
	return err
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

//isAnnexPath returns true if a given string represents the path to an annex object.
func isAnnexPath(path string) bool {
	// TODO: Check paths on Windows
	return strings.Contains(path, ".git/annex/objects")
}

func calcRate(dbytes int, dt time.Duration) string {
	dtns := dt.Nanoseconds()
	if dtns <= 0 || dbytes <= 0 {
		return ""
	}
	rate := int64(dbytes) * 1000000000 / dtns
	return fmt.Sprintf("%s/s", humanize.IBytes(uint64(rate)))
}
