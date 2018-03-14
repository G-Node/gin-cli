package ginclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/G-Node/gin-cli/util"
	"github.com/G-Node/gin-cli/web"
)

// Workingdir sets the directory for shell commands
var Workingdir = "."
var progcomplete = "100%"

// **************** //

// Git commands

// SetGitUser sets the user.name and user.email configuration values for the local git repository.
func SetGitUser(name, email string) error {
	if !IsRepo() {
		return fmt.Errorf("not a repository")
	}
	cmd, err := RunGitCommand("config", "--local", "user.name", name)
	if err != nil {
		return err
	}
	err = cmd.Wait()
	if err != nil {
		return err
	}
	cmd, err = RunGitCommand("config", "--local", "user.email", email)
	if err != nil {
		return err
	}
	return cmd.Wait()
}

// AddRemote adds a remote named name for the repository at url.
func AddRemote(name, url string) error {
	fn := fmt.Sprintf("AddRemote(%s, %s)", name, url)
	cmd, err := RunGitCommand("remote", "add", name, url)
	if err != nil {
		return err
	}
	err = cmd.Wait()
	if err != nil {
		gerr := ginerror{UError: err.Error(), Origin: fn}
		util.LogWrite("Error during remote add command")
		cmd.LogStdOutErr()
		stderr := cmd.ErrPipe.ReadAll()
		if strings.Contains(stderr, "already exists") {
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
	cmd, err := RunGitCommand("rev-parse", "HEAD")
	if err == nil && cmd.Wait() == nil {
		// All good. No need to do anything
		return false, nil
	}

	// Create an empty initial commit and run annex sync to synchronise everything
	hostname, err := os.Hostname()
	if err != nil {
		hostname = defaultHostname
	}
	commitargs := []string{"commit", "--allow-empty", "-m", fmt.Sprintf("Initial commit: Repository initialised on %s", hostname)}
	cmd, err = RunGitCommand(commitargs...)
	if err != nil {
		util.LogWrite("Error while creating initial commit")
		return false, err
	}
	if err = cmd.Wait(); err != nil {
		util.LogWrite("Error while creating initial commit")
		cmd.LogStdOutErr()
		return false, err
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

func splitRepoParts(repoPath string) (repoOwner, repoName string) {
	repoPathParts := strings.SplitN(repoPath, "/", 2)
	repoOwner = repoPathParts[0]
	repoName = repoPathParts[1]
	return
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
	cmd, err := RunGitCommand(args...)
	if err != nil {
		clonechan <- RepoFileStatus{Err: ginerror{UError: err.Error(), Origin: fn}}
		return
	}
	var status RepoFileStatus
	status.State = "Downloading repository"
	for {
		// git clone progress prints to stderr
		line, rerr := cmd.ErrPipe.ReadLine()
		if rerr != nil {
			break
		}
		words := strings.Fields(line)
		status.FileName = repoPath
		if words[0] == "Receiving" && words[1] == "objects" {
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
	if err != nil || cmd.Wait() != nil {
		util.LogWrite("Error during clone command")
		cmd.LogStdOutErr()
		repoOwner, repoName := splitRepoParts(repoPath)

		stderr := cmd.ErrPipe.ReadAll()
		gerr := ginerror{UError: stderr, Origin: fn}
		if strings.Contains(stderr, "does not exist") {
			gerr.Description = fmt.Sprintf("Repository download failed\n"+
				"Make sure you typed the repository path correctly\n"+
				"Type 'gin repos %s' to see if the repository exists and if you have access to it",
				repoOwner)
		} else if strings.Contains(stderr, "already exists and is not an empty directory") {
			gerr.Description = fmt.Sprintf("Repository download failed.\n"+
				"'%s' already exists in the current directory and is not empty.", repoName)
		} else if strings.Contains(stderr, "Host key verification failed") {
			gerr.Description = "Server key does not match known/configured host key."
		} else {
			gerr.Description = fmt.Sprintf("Repository download failed. Internal git command returned: %s", stderr)
			clonechan <- RepoFileStatus{Err: gerr}
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

// AnnexInit initialises the repository for annex.
// Setting the Workingdir package global affects the working directory in which the command is executed.
// (git annex init)
func AnnexInit(description string) error {
	args := []string{"init", description}
	cmd, err := RunAnnexCommand(args...)
	cmd.LogStdOutErr()
	if err != nil || cmd.Wait() != nil {
		initError := fmt.Errorf("Repository annex initialisation failed.\n%s", cmd.ErrPipe.ReadAll())
		util.LogWrite(initError.Error())
		return initError
	}
	cmd, err = RunGitCommand("config", "annex.backends", "MD5")
	if err != nil || cmd.Wait() != nil {
		util.LogWrite("Failed to set default annex backend MD5")
		util.LogWrite("[Error]: %v", cmd.ErrPipe.ReadAll())
		cmd.LogStdOutErr()
	}
	return nil
}

// AnnexPull downloads all annexed files. Optionally also downloads all file content.
// Setting the Workingdir package global affects the working directory in which the command is executed.
// (git annex sync --no-push [--content])
func AnnexPull() error {
	args := []string{"sync", "--no-push", "--no-commit"}
	cmd, err := RunAnnexCommand(args...)
	if err != nil {
		return err
	}
	err = cmd.Wait()
	if err != nil {
		util.LogWrite("Error during AnnexPull.")
		util.LogWrite("[Error]: %v", err)
		cmd.LogStdOutErr()
		stderr := cmd.ErrPipe.ReadAll()
		if strings.Contains(stderr, "Permission denied") {
			return fmt.Errorf("download failed: permission denied")
		} else if strings.Contains(stderr, "Host key verification failed") {
			return fmt.Errorf("download failed: server key does not match known host key")
		}
	}
	return err
}

// AnnexSync synchronises the local repository with the remote.
// Optionally synchronises content if content=True
// Setting the Workingdir package global affects the working directory in which the command is executed.
// The status channel 'syncchan' is closed when this function returns.
// (git annex sync [--content])
func AnnexSync(content bool, syncchan chan<- RepoFileStatus) {
	defer close(syncchan)
	args := []string{"sync"}
	if content {
		args = append(args, "--content")
	}
	cmd, err := RunAnnexCommand(args...)
	var status RepoFileStatus
	status.State = "Synchronising repository"
	syncchan <- status
	for {
		line, rerr := cmd.OutPipe.ReadLine()
		if rerr != nil {
			break
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
	if err != nil || cmd.Wait() != nil {
		util.LogWrite("Error during AnnexSync")
		util.LogWrite("[Error]: %v", err)
		cmd.LogStdOutErr()
	}
	status.Progress = progcomplete
	syncchan <- status
	return
}

// AnnexPush uploads all annexed files.
// Setting the Workingdir package global affects the working directory in which the command is executed.
// The status channel 'pushchan' is closed when this function returns.
// (git annex sync --no-pull --content)
func AnnexPush(paths []string, commitmsg string, pushchan chan<- RepoFileStatus) {
	defer close(pushchan)
	cmdargs := []string{"sync", "--no-pull", "--commit", fmt.Sprintf("--message=%s", commitmsg)}
	cmd, err := RunAnnexCommand(cmdargs...)
	if err != nil {
		pushchan <- RepoFileStatus{Err: err}
		return
	}
	for {
		line, rerr := cmd.OutPipe.ReadLine()
		if rerr != nil {
			break
		}
		// TODO: Parse git output to return git file upload status
		util.LogWrite(line)
	}
	if err = cmd.Wait(); err != nil {
		util.LogWrite("Error during AnnexPush (sync --no-pull)")
		util.LogWrite("[Error]: %v", err)
		cmd.LogStdOutErr()
		stderr := cmd.ErrPipe.ReadAll()
		errmsg := "failed"
		if strings.Contains(stderr, "Permission denied") {
			errmsg = "upload failed: permission denied"
		} else if strings.Contains(stderr, "Host key verification failed") {
			errmsg = "upload failed: server key does not match known host key"
		}
		pushchan <- RepoFileStatus{Err: fmt.Errorf(errmsg)}
		return
	}

	cmdargs = []string{"copy"}
	cmdargs = append(cmdargs, paths...)
	// NOTE: Using origin which is the conventional default remote. This should be fixed.
	cmdargs = append(cmdargs, "--to=origin")
	cmd, err = RunAnnexCommand(cmdargs...)
	if err != nil {
		pushchan <- RepoFileStatus{Err: err}
		return
	}
	var status RepoFileStatus
	status.State = "Uploading"
	for {
		line, rerr := cmd.OutPipe.ReadLine()
		if rerr != nil {
			break
		}
		words := strings.Fields(line)
		if words[0] == "copy" {
			status.FileName = words[1] // NOTE: doesn't work for files with spaces in the name; fix with --json-progress
			// new file - reset Progress and Rate
			status.Progress = ""
			status.Rate = ""
			if words[len(words)-1] != "ok" {
				// if the copy line ends with ok, the file is already done (no upload needed)
				// so we shouldn't send the status to the caller
				pushchan <- status
			}
		} else if strings.Contains(line, "%") {
			status.Progress = words[1]
			status.Rate = words[2]
			pushchan <- status
		}
	}
	if err = cmd.Wait(); err != nil {
		util.LogWrite("Error during AnnexPush (copy)")
		util.LogWrite("[Error]: %v", err)
		cmd.LogStdOutErr()
	}
	return
}

// AnnexGet retrieves the content of specified files.
// Setting the Workingdir package global affects the working directory in which the command is executed.
// The status channel 'getchan' is closed when this function returns.
// (git annex get)
func AnnexGet(filepaths []string, getchan chan<- RepoFileStatus) {
	defer close(getchan)
	cmdargs := append([]string{"get"}, filepaths...)
	cmd, err := RunAnnexCommand(cmdargs...)
	if err != nil {
		getchan <- RepoFileStatus{Err: err}
		return
	}
	var status RepoFileStatus
	status.State = "Downloading"
	for {
		line, rerr := cmd.OutPipe.ReadLine()
		if rerr != nil {
			break
		}
		words := strings.Fields(line)
		lastword := words[len(words)-1]
		if words[0] == "get" {
			status.FileName = words[1] // NOTE: Fix with --json-progress
			// new file - reset Progress, Rate, and Err
			status.Progress = ""
			status.Rate = ""
			status.Err = nil
			if lastword != "ok" {
				// if the copy line ends with ok, the file is already done (no upload needed)
				// so we shouldn't send the status to the caller
				getchan <- status
			}
		} else if lastword == "failed" {
			// determine error type
			errline := cmd.ErrPipe.ReadAll()
			if strings.Contains(errline, "Permission denied") {
				status.Err = fmt.Errorf("Authentication failed: try logging in again")
			} else {
				// TODO: Other reasons?
				status.Err = fmt.Errorf("Content or server unavailable")
			}
			getchan <- status
		} else if strings.Contains(line, "%") {
			status.Progress = words[1]
			status.Rate = words[2]
			getchan <- status
		}
	}
	if err = cmd.Wait(); err != nil {
		util.LogWrite("Error during AnnexGet")
		util.LogWrite("[Error]: %v", err)
		cmd.LogStdOutErr()
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
	cmd, err := RunAnnexCommand(cmdargs...)
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
	for {
		line, rerr := cmd.OutPipe.ReadLine()
		if rerr != nil {
			break
		}
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
			status.Err = fmt.Errorf("failed")
		}
		status.Progress = progcomplete
		dropchan <- status
	}
	if err = cmd.Wait(); err != nil {
		util.LogWrite("Error during AnnexDrop")
		util.LogWrite("[Error]: %v", err)
		cmd.LogStdOutErr()
	}
	return
}

func setBare(state bool) error {
	var statestr string
	if state {
		statestr = "true"
	} else {
		statestr = "false"
	}
	cmd, err := RunGitCommand("config", "--local", "--bool", "core.bare", statestr)
	if err != nil || cmd.Wait() != nil {
		util.LogWrite("Error switching bare status to %s", statestr)
		cmd.LogStdOutErr()
	}
	return err
}

// GitLsFiles lists all files known to git.
// In direct mode, the bare flag is temporarily switched off before running the command.
// The output channel 'lschan' is closed when this function returns.
// (git ls-files)
func GitLsFiles(args []string, lschan chan<- string) {
	defer close(lschan)
	cmdargs := append([]string{"ls-files"}, args...)
	cmd, err := RunGitCommand(cmdargs...)
	if err != nil {
		util.LogWrite("ls-files command set up failed: %s", err)
		return
	}
	for {
		line, rerr := cmd.OutPipe.ReadLine()
		if rerr != nil {
			break
		}
		line = strings.TrimSpace(line)
		if line != "" {
			lschan <- line
		}
	}

	if err = cmd.Wait(); err != nil {
		util.LogWrite("Error during GitLsFiles")
		cmd.LogStdOutErr()
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

	cmdargs := append([]string{"add", "--verbose"}, filepaths...)
	cmd, err := RunGitCommand(cmdargs...)
	if err != nil {
		addchan <- RepoFileStatus{Err: err}
		return
	}
	// TODO: Parse output
	var status RepoFileStatus
	for {
		line, rerr := cmd.OutPipe.ReadLine()
		if rerr != nil {
			break
		}
		fname := strings.TrimSpace(line)
		if strings.HasPrefix(fname, "add") {
			status.State = "Adding"
			fname = strings.TrimPrefix(fname, "add '")
		} else if strings.HasPrefix(fname, "remove") {
			status.State = "Removing"
			fname = strings.TrimPrefix(fname, "remove '")
		}
		fname = strings.TrimSuffix(fname, "'")
		status.FileName = fname
		util.LogWrite("%s added to git", fname)
		// Error conditions?
		status.Progress = progcomplete
		addchan <- status
	}
	if err = cmd.Wait(); err != nil {
		util.LogWrite("Error during GitAdd")
		cmd.LogStdOutErr()
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

// AnnexAdd adds paths to the annex.
// Files specified for exclusion in the configuration are ignored automatically.
// Setting the Workingdir package global affects the working directory in which the command is executed.
// The status channel 'addchan' is closed when this function returns.
// (git annex add)
func AnnexAdd(filepaths []string, addchan chan<- RepoFileStatus) {
	defer close(addchan)
	if len(filepaths) == 0 {
		util.LogWrite("No paths to add to annex. Nothing to do.")
		return
	}
	cmdargs := []string{"--json", "add"}
	cmdargs = append(cmdargs, filepaths...)

	exclargs := annexExclArgs()
	if len(exclargs) > 0 {
		cmdargs = append(cmdargs, exclargs...)
	}

	cmd, err := RunAnnexCommand(cmdargs...)
	if err != nil {
		addchan <- RepoFileStatus{Err: err}
		return
	}

	var annexAddRes struct {
		Command string `json:"command"`
		File    string `json:"file"`
		Key     string `json:"key"`
		Success bool   `json:"success"`
	}
	var status RepoFileStatus
	status.State = "Adding"
	for {
		line, rerr := cmd.OutPipe.ReadLine()
		if rerr != nil {
			break
		}
		line = strings.TrimSpace(line)
		if len(line) == 0 {
			// Empty line output. Ignore
			continue
		}
		// Send file name
		err = json.Unmarshal([]byte(line), &annexAddRes)
		if err != nil {
			addchan <- RepoFileStatus{Err: err}
			return
		}
		status.FileName = annexAddRes.File
		if annexAddRes.Success {
			util.LogWrite("%s added to annex", annexAddRes.File)
			status.Err = nil
		} else {
			util.LogWrite("Error adding %s", annexAddRes.File)
			status.Err = fmt.Errorf("failed")
		}
		status.Progress = progcomplete
		addchan <- status
	}
	if err = cmd.Wait(); err != nil {
		util.LogWrite("Error during AnnexAdd")
		cmd.LogStdOutErr()
	}
	return
}

// AnnexWhereisRes holds the output of a "git annex whereis" command
type AnnexWhereisRes struct {
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
	Err error  `json:"err"`
}

// AnnexWhereis returns information about annexed files in the repository
// Setting the Workingdir package global affects the working directory in which the command is executed.
// The output channel 'wichan' is closed when this function returns.
// (git annex whereis)
func AnnexWhereis(paths []string, wichan chan<- AnnexWhereisRes) {
	defer close(wichan)
	cmdargs := []string{"whereis", "--json"}
	cmdargs = append(cmdargs, paths...)
	cmd, err := RunAnnexCommand(cmdargs...)
	if err != nil {
		util.LogWrite("Error during AnnexWhereis")
		cmd.LogStdOutErr()
		wichan <- AnnexWhereisRes{Err: fmt.Errorf("Failed to run git-annex whereis: %s", err)}
		return
	}

	var info AnnexWhereisRes
	for {
		line, rerr := cmd.OutPipe.ReadLine()
		if rerr != nil {
			break
		}
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
	cmd, err := RunAnnexCommand(cmdargs...)
	// TODO: Parse output
	if err != nil {
		util.LogWrite("Error setting up git-annex status")
		cmd.LogStdOutErr()
		statuschan <- AnnexStatusRes{Err: fmt.Errorf("Failed to run git-annex status: %s", err)}
		return
	}

	var status AnnexStatusRes
	for {
		line, rerr := cmd.OutPipe.ReadLine()
		if rerr != nil {
			break
		}
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
// Note that this function uses 'git annex add' to lock files, but only if they are marked as unlocked (T) by git annex.
// Attempting to lock an untracked file, or a file in any state other than T will have no effect.
// Setting the Workingdir package global affects the working directory in which the command is executed.
// The status channel 'lockchan' is closed when this function returns.
// (git annex add --update)
func AnnexLock(filepaths []string, lockchan chan<- RepoFileStatus) {
	defer close(lockchan)
	// Annex lock doesn't work like it used to. It's better to instead annex add, but only the files that are already known to annex (handled by --update).
	var status RepoFileStatus
	status.State = "Locking"

	cmdargs := []string{"add", "--json", "--update"}
	exclargs := annexExclArgs()
	if len(exclargs) > 0 {
		cmdargs = append(cmdargs, exclargs...)
	}

	cmdargs = append(cmdargs, filepaths...)
	cmd, err := RunAnnexCommand(cmdargs...)
	if err != nil {
		lockchan <- RepoFileStatus{Err: err}
		return
	}

	var annexAddRes struct {
		Command string `json:"command"`
		File    string `json:"file"`
		Key     string `json:"key"`
		Success bool   `json:"success"`
	}
	for {
		line, rerr := cmd.OutPipe.ReadLine()
		if rerr != nil {
			break
		}
		line = strings.TrimSpace(line)
		if len(line) == 0 {
			// Empty line output. Ignore
			continue
		}
		// Send file name
		err = json.Unmarshal([]byte(line), &annexAddRes)
		if err != nil {
			lockchan <- RepoFileStatus{Err: err}
			return
		}
		status.FileName = annexAddRes.File
		if annexAddRes.Success {
			util.LogWrite("%s locked", annexAddRes.File)
			status.Err = nil
		} else {
			util.LogWrite("Error locking %s", annexAddRes.File)
			status.Err = fmt.Errorf("failed")
		}
		status.Progress = progcomplete
		lockchan <- status
	}
	if err != nil || cmd.Wait() != nil {
		util.LogWrite("Error during AnnexLock")
		cmd.LogStdOutErr()
	}
	status.Progress = progcomplete
	return
}

// AnnexUnlock unlocks the specified files and directory contents if they are annexed
// Setting the Workingdir package global affects the working directory in which the command is executed.
// The status channel 'unlockchan' is closed when this function returns.
// (git annex unlock)
func AnnexUnlock(filepaths []string, unlockchan chan<- RepoFileStatus) {
	defer close(unlockchan)
	cmdargs := []string{"unlock", "--json"}
	cmdargs = append(cmdargs, filepaths...)
	cmd, err := RunAnnexCommand(cmdargs...)
	if err != nil {
		unlockchan <- RepoFileStatus{Err: err}
		return
	}
	var status RepoFileStatus
	status.State = "Unlocking"

	var annexUnlockRes struct {
		Command string `json:"command"`
		File    string `json:"file"`
		Key     string `json:"key"`
		Success bool   `json:"success"`
		Note    string `json:"note"`
	}
	for {
		line, rerr := cmd.OutPipe.ReadLine()
		if rerr != nil {
			break
		}
		line = strings.TrimSpace(line)
		if len(line) == 0 {
			// Empty line output. Ignore
			continue
		}
		// Send file name
		err = json.Unmarshal([]byte(line), &annexUnlockRes)
		if err != nil {
			unlockchan <- RepoFileStatus{Err: err}
			return
		}
		status.FileName = annexUnlockRes.File
		if annexUnlockRes.Success {
			util.LogWrite("%s unlocked", annexUnlockRes.File)
			status.Err = nil
		} else {
			util.LogWrite("Error unlocking %s", annexUnlockRes.File)
			status.Err = fmt.Errorf("Content not available locally\nUse 'gin get-content' to download")
		}
		status.Progress = progcomplete
		unlockchan <- status
	}
	if err != nil || cmd.Wait() != nil {
		util.LogWrite("Error during AnnexUnlock")
		cmd.LogStdOutErr()
		return
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
	cmd, err := RunAnnexCommand("info", "--json")
	if err != nil || cmd.Wait() != nil {
		util.LogWrite("Error during AnnexInfo")
		cmd.LogStdOutErr()
		return AnnexInfoRes{}, fmt.Errorf("Error retrieving annex info")
	}

	stdout := cmd.OutPipe.ReadAll()
	stdout = strings.TrimSpace(stdout)
	var info AnnexInfoRes
	if len(stdout) == 0 {
		// empty output - error?
		return info, nil
	}
	err = json.Unmarshal([]byte(stdout), &info)
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
	// TODO: Use git log -z and split stdout on NULL (\x00)
	logformat := `{"hash":"%H","abbrevhash":"%h","authorname":"%an","authoremail":"%ae","date":"%aI","subject":"%s","body":""}`
	cmdargs := []string{"log", fmt.Sprintf("--format=%s", logformat)}
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
	cmd, err := RunGitCommand(cmdargs...)
	if err != nil {
		util.LogWrite("Error setting up git log command")
		cmd.LogStdOutErr()
		return nil, fmt.Errorf("error retrieving version logs - malformed git log command")
	}

	var commits []GinCommit
	for {
		line, rerr := cmd.OutPipe.ReadLine()
		if rerr != nil {
			break
		}
		var commit GinCommit
		ierr := json.Unmarshal([]byte(line), &commit)
		if ierr != nil {
			util.LogWrite("Error parsing git log")
			util.LogWrite(ierr.Error())
			continue
		}
		commits = append(commits, commit)
	}

	err = cmd.Wait() // should be done by now
	if err != nil {
		util.LogWrite("Error getting git log")
		cmd.LogStdOutErr()
		stderr := cmd.ErrPipe.ReadAll()
		if strings.Contains(stderr, "bad revision") {
			stderr = fmt.Sprintf("'%s' does not match a known version ID or name", revrange)
		}
		return nil, fmt.Errorf(stderr)
	}

	// TODO: Combine diffstats into first git log invocation
	logstats, err := GitLogDiffstat(count, paths)
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

func GitLogDiffstat(count uint, paths []string) (map[string]DiffStat, error) {
	logformat := `::%H`
	cmdargs := []string{"log", fmt.Sprintf("--format=%s", logformat), "--name-status"}
	if count > 0 {
		cmdargs = append(cmdargs, fmt.Sprintf("--max-count=%d", count))
	}
	cmdargs = append(cmdargs, "--") // separate revisions from paths, even if there are no paths
	if paths != nil && len(paths) > 0 {
		cmdargs = append(cmdargs, paths...)
	}
	cmd, err := RunGitCommand(cmdargs...)
	if err != nil {
		util.LogWrite("Error during GitLogDiffstat")
		cmd.LogStdOutErr()
		return nil, err
	}

	stats := make(map[string]DiffStat)
	var curhash string
	var curstat DiffStat
	for {
		line, rerr := cmd.OutPipe.ReadLine()
		if rerr != nil {
			break
		}
		line = util.CleanSpaces(line)
		if strings.HasPrefix(line, "::") {
			curhash = strings.TrimPrefix(line, "::")
			curstat = DiffStat{}
		} else if len(line) == 0 {
			continue // Skip empty lines
		} else {
			// parse name-status
			fstat := strings.SplitN(line, " ", 2) // stat (A, M, or D) and filename
			stat, fname := fstat[0], fstat[1]
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

	cmd, err := RunGitCommand(cmdargs...)
	if err != nil || cmd.Wait() != nil {
		util.LogWrite("Error during GitCheckout")
		cmd.LogStdOutErr()
		return fmt.Errorf(cmd.ErrPipe.ReadAll())
	}

	fmt.Print(cmd.OutPipe.ReadAll())
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
	cmdargs := []string{"ls-tree", "--full-tree", "-t", "-r"}
	cmdargs = append(cmdargs, paths...)
	cmd, err := RunGitCommand(cmdargs...)
	if err != nil || cmd.Wait() != nil {
		util.LogWrite("Error during GitLsTree")
		cmd.LogStdOutErr()
		err = fmt.Errorf(cmd.ErrPipe.ReadAll())
		return nil, err
	}

	var objects []GitObject
	for {
		line, rerr := cmd.OutPipe.ReadLine()
		if rerr != nil {
			break
		}
		words := strings.Fields(line)
		fnamesplit := strings.SplitN(line, "   ", 2)
		obj := GitObject{
			Mode: words[0],
			Type: words[1],
			Hash: words[2],
			Name: fnamesplit[len(fnamesplit)-1],
		}
		objects = append(objects, obj)
	}
	return objects, nil
}

// GitCatFileContents performs a git-cat-file of a specific file from a specific commit and returns the file contents (as bytes).
// Setting the Workingdir package global affects the working directory in which the command is executed.
func GitCatFileContents(revision, filepath string) ([]byte, error) {
	cmd, err := RunGitCommand("cat-file", "blob", fmt.Sprintf("%s:./%s", revision, filepath))
	if err != nil || cmd.Wait() != nil {
		util.LogWrite("Error during GitCatFile")
		cmd.LogStdOutErr()
		err = fmt.Errorf(cmd.ErrPipe.ReadAll())
		return nil, err
	}
	output := cmd.OutPipe.ReadAll()
	return []byte(output), nil
}

// GitCatFileType returns the type of a given object at a given revision (blob, tree, or commit)
// Setting the Workingdir package global affects the working directory in which the command is executed.
func GitCatFileType(object string) (string, error) {
	cmd, err := RunGitCommand("cat-file", "-t", object)
	if err != nil || cmd.Wait() != nil {
		util.LogWrite("Error during GitCatFile")
		cmd.LogStdOutErr()
		err = fmt.Errorf(cmd.ErrPipe.ReadAll())
		return "", err
	}
	return cmd.OutPipe.ReadAll(), nil
}

var modecache = make(map[string]bool)

// IsDirect returns true if the repository in a given path is working in git annex 'direct' mode.
// If path is not a repository, or is not an initialised annex repository, the result defaults to false.
// If the path is a repository and no error was raised, the result it cached so that subsequent checks are faster.
// Setting the Workingdir package global affects the working directory in which the command is executed.
func IsDirect() bool {
	if mode, ok := modecache[Workingdir]; ok {
		return mode
	}
	cmd, err := RunGitCommand("config", "--local", "annex.direct")
	if err != nil || cmd.Wait() != nil {
		// Don't cache this result
		return false
	}

	stdout := cmd.OutPipe.ReadAll()
	if strings.TrimSpace(stdout) == "true" {
		modecache[Workingdir] = true
		return true
	}
	modecache[Workingdir] = false
	return false
}

// IsVersion6 returns true if the repository in a given path is working in git annex 'direct' mode.
// If path is not a repository, or is not an initialised annex repository, the result defaults to false.
// Setting the Workingdir package global affects the working directory in which the command is executed.
func IsVersion6() bool {
	cmd, err := RunGitCommand("config", "--local", "--get", "annex.version")
	if err != nil || cmd.Wait() != nil {
		util.LogWrite("Error while checking repository annex version")
		cmd.LogStdOutErr()
		return false
	}
	stdout := cmd.OutPipe.ReadAll()
	ver := strings.TrimSpace(stdout)
	util.LogWrite("Annex version is %s", ver)
	return ver == "6"
}

// Utility functions for shelling out

// RunGitCommand executes an external git command with the provided arguments and returns a GinCmd struct.
// The command is started with Start() before returning.
// Setting the Workingdir package global affects the working directory in which the command is executed.
func RunGitCommand(args ...string) (util.GinCmd, error) {
	gitbin := util.Config.Bin.Git
	cmd := util.Command(gitbin)
	cmd.Dir = Workingdir
	cmd.Args = append(cmd.Args, args...)
	token := web.UserToken{}
	_ = token.LoadToken()
	env := os.Environ()
	cmd.Env = append(env, util.GitSSHEnv(token.Username))
	util.LogWrite("Running shell command (Dir: %s): %s", Workingdir, strings.Join(cmd.Args, " "))
	err := cmd.Start()
	return cmd, err
}

// RunAnnexCommand executes a git annex command with the provided arguments and returns a GinCmd struct.
// The command is started with Start() before returning.
// Setting the Workingdir package global affects the working directory in which the command is executed.
func RunAnnexCommand(args ...string) (util.GinCmd, error) {
	gitannexbin := util.Config.Bin.GitAnnex
	cmd := util.Command(gitannexbin, args...)
	cmd.Dir = Workingdir
	token := web.UserToken{}
	_ = token.LoadToken()
	env := os.Environ()
	cmd.Env = append(env, util.GitSSHEnv(token.Username))
	cmd.Env = append(cmd.Env, "GIT_ANNEX_USE_GIT_SSH=1")
	util.LogWrite("Running shell command (Dir: %s): %s", Workingdir, strings.Join(cmd.Args, " "))
	err := cmd.Start()
	return cmd, err
}

// GetAnnexVersion returns the version string of the system's git-annex.
func GetAnnexVersion() (string, error) {
	cmd, err := RunAnnexCommand("version", "--raw")
	if err != nil {
		util.LogWrite("Error while preparing git-annex version command")
		if strings.Contains(err.Error(), "executable file not found") ||
			strings.Contains(err.Error(), "no such file or directory") {
			return "", fmt.Errorf("git-annex executable '%s' not found", util.Config.Bin.GitAnnex)
		}
		return "", err
	}
	if err = cmd.Wait(); err != nil {
		util.LogWrite("Error while checking git-annex version")
		cmd.LogStdOutErr()
		return "", err
	}

	stdout := cmd.OutPipe.ReadAll()
	return stdout, nil
}
