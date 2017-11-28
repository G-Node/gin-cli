package ginclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/G-Node/gin-cli/util"
	"github.com/G-Node/gin-cli/web"
	"github.com/dustin/go-humanize"
)

// Workingdir sets the directory for shell commands
var Workingdir = "."

// **************** //

// Git commands

// SetGitUser sets the user.name and user.email configuration values for the local git repository.
func SetGitUser(name, email string) error {
	if !IsRepo() {
		return fmt.Errorf("Not a repository")
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
	cmd, err := RunGitCommand("remote", "add", name, url)
	if err != nil {
		return err
	}
	err = cmd.Wait()
	if err != nil {
		util.LogWrite("Error during remote add command")
		cmd.LogStdOutErr()
		stderr := cmd.ErrPipe.ReadAll()
		if strings.Contains(stderr, "already exists") {
			return fmt.Errorf("Remote with name %s already exists", name)
		}
	}
	return err
}

// CommitIfNew creates an empty initial git commit if the current repository is completely new.
// Returns 'true' if (and only if) a commit was created.
// Setting the Workingdir package global affects the working directory in which the command is executed.
func CommitIfNew() (bool, error) {
	if !IsRepo() {
		return false, fmt.Errorf("Not a repository")
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
// (git clone ...)
func (gincl *Client) Clone(repoPath string) error {
	remotePath := fmt.Sprintf("ssh://%s@%s/%s", gincl.GitUser, gincl.GitHost, repoPath)
	cmd, err := RunGitCommand("clone", remotePath)
	// TODO: Parse output and print progress
	if err != nil || cmd.Wait() != nil {
		util.LogWrite("Error during clone command")
		cmd.LogStdOutErr()
		repoOwner, repoName := splitRepoParts(repoPath)

		stderr := cmd.ErrPipe.ReadAll()
		if strings.Contains(stderr, "Server returned non-OK status: 404") {
			return fmt.Errorf("Error retrieving repository.\n"+
				"Please make sure you typed the repository path correctly.\n"+
				"Type 'gin repos %s' to see if the repository exists and if you have access to it.",
				repoOwner)
		} else if strings.Contains(stderr, "already exists and is not an empty directory") {
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

// AnnexInit initialises the repository for annex.
// Setting the Workingdir package global affects the working directory in which the command is executed.
// (git annex init)
func AnnexInit(description string) error {
	args := []string{"init", description}
	cmd, err := RunAnnexCommand(args...)
	cmd.LogStdOutErr()
	if err != nil || cmd.Wait() != nil {
		initError := fmt.Errorf("Repository annex initialisation failed.")
		util.LogWrite(initError.Error())
		return initError
	}
	cmd, err = RunGitCommand("config", "annex.backends", "MD5")
	if err != nil || cmd.Wait() != nil {
		util.LogWrite("Failed to set default annex backend MD5")
		util.LogWrite("[Error]: %v", err)
		cmd.LogStdOutErr()
	}
	return nil
}

// AnnexPull downloads all annexed files. Optionally also downloads all file content.
// Setting the Workingdir package global affects the working directory in which the command is executed.
// The satus channel 'pullchan' is closed when this function returns.
// (git annex sync --no-push [--content])
func AnnexPull(content bool, pullchan chan<- RepoFileStatus) {
	defer close(pullchan)
	args := []string{"sync", "--no-push"}
	if content {
		args = append(args, "--content")
	}
	cmd, err := RunAnnexCommand(args...)
	if err != nil {
		pullchan <- RepoFileStatus{Err: err}
		return
	}
	var status RepoFileStatus
	status.State = "Downloading"
	for {
		line, rerr := cmd.OutPipe.ReadLine()
		if rerr != nil {
			break
		}
		line = util.CleanSpaces(line)
		if strings.HasPrefix(line, "get") {
			words := strings.Split(line, " ")
			status.FileName = strings.TrimSpace(words[1])
			// new file - reset Progress and Rate
			status.Progress = ""
			status.Rate = ""
			if !strings.HasSuffix(line, "ok") {
				// if the copy line ends with ok, the file is already done (no upload needed)
				// so we shouldn't send the status to the caller
				pullchan <- status
			}
		} else if strings.Contains(line, "%") {
			words := strings.Split(line, " ")
			status.Progress = words[1]
			status.Rate = words[2]
			pullchan <- status
		}
	}

	if err != nil || cmd.Wait() != nil {
		util.LogWrite("Error during AnnexPull.")
		util.LogWrite("[Error]: %v", err)
		cmd.LogStdOutErr()
		pullchan <- RepoFileStatus{Err: fmt.Errorf("Error downloading files")}
	}
	return
}

// AnnexSync synchronises the local repository with the remote.
// Optionally synchronises content if content=True
// Setting the Workingdir package global affects the working directory in which the command is executed.
// (git annex sync [--content])
func AnnexSync(content bool) error {
	args := []string{"sync"}
	if content {
		args = append(args, "--content")
	}
	cmd, err := RunAnnexCommand(args...)
	// TODO: Parse output
	if err != nil || cmd.Wait() != nil {
		util.LogWrite("Error during AnnexSync")
		util.LogWrite("[Error]: %v", err)
		cmd.LogStdOutErr()
		return fmt.Errorf("Error synchronising files")
	}
	return nil
}

// AnnexPush uploads all annexed files.
// Setting the Workingdir package global affects the working directory in which the command is executed.
// The status channel 'pushchan' is closed when this function returns.
// (git annex sync --no-pull --content)
func AnnexPush(paths []string, commitMsg string, pushchan chan<- RepoFileStatus) {
	defer close(pushchan)
	cmdargs := []string{"sync", "--no-pull", "--commit", fmt.Sprintf("--message=%s", commitMsg)}
	cmd, err := RunAnnexCommand(cmdargs...)
	if err != nil {
		pushchan <- RepoFileStatus{Err: err}
		return
	}
	util.LogWrite("annex sync output") // remove me
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
		pushchan <- RepoFileStatus{Err: fmt.Errorf("Error uploading files")}
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
		line = util.CleanSpaces(line)
		if strings.HasPrefix(line, "copy") {
			words := strings.Split(line, " ")
			status.FileName = strings.TrimSpace(words[1])
			// new file - reset Progress and Rate
			status.Progress = ""
			status.Rate = ""
			if !strings.HasSuffix(line, "ok") {
				// if the copy line ends with ok, the file is already done (no upload needed)
				// so we shouldn't send the status to the caller
				pushchan <- status
			}
		} else if strings.Contains(line, "%") {
			words := strings.Split(line, " ")
			status.Progress = words[1]
			status.Rate = words[2]
			pushchan <- status
		}
	}
	if err = cmd.Wait(); err != nil {
		util.LogWrite("Error during AnnexPush (copy)")
		util.LogWrite("[Error]: %v", err)
		cmd.LogStdOutErr()
		pushchan <- RepoFileStatus{Err: fmt.Errorf("Error uploading files")}
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
		line = util.CleanSpaces(line)
		if strings.HasPrefix(line, "get") {
			words := strings.Split(line, " ")
			status.FileName = strings.TrimSpace(words[1])
			// new file - reset Progress and Rate
			status.Progress = ""
			status.Rate = ""
			if !strings.HasSuffix(line, "ok") {
				// if the copy line ends with ok, the file is already done (no upload needed)
				// so we shouldn't send the status to the caller
				getchan <- status
			}
		} else if strings.Contains(line, "%") {
			words := strings.Split(line, " ")
			status.Progress = words[1]
			status.Rate = words[2]
			getchan <- status
		}
	}
	if err = cmd.Wait(); err != nil {
		util.LogWrite("Error during AnnexGet")
		util.LogWrite("[Error]: %v", err)
		cmd.LogStdOutErr()
		getchan <- RepoFileStatus{Err: fmt.Errorf("Error downloading files")}
	}
	cmd.LogStdOutErr()
	return
}

// AnnexDrop drops the content of specified files.
// Setting the Workingdir package global affects the working directory in which the command is executed.
// (git annex drop)
func AnnexDrop(filepaths []string) error {
	// TODO: Print success for each file as it finishes
	cmdargs := append([]string{"drop"}, filepaths...)
	cmd, err := RunAnnexCommand(cmdargs...)
	if err != nil {
		return err
	}
	// TODO: Parse output
	if err = cmd.Wait(); err != nil {
		util.LogWrite("Error during AnnexDrop")
		util.LogWrite("[Error]: %v", err)
		cmd.LogStdOutErr()
		return fmt.Errorf("Error removing files")
	}
	return nil
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
// Arguments passed to this function are directly passed on to the 'ls-files' command.
// (git ls-files)
func GitLsFiles(args []string) ([]string, error) {
	if IsDirect() {
		// Set bare false and revert at the end of the function
		err := setBare(false)
		if err != nil {
			return nil, fmt.Errorf("Error during ls-files. Unable to toggle repository bare mode.")
		}
		defer setBare(true)
	}

	cmdargs := append([]string{"ls-files"}, args...)
	cmd, err := RunGitCommand(cmdargs...)
	// TODO: Parse output
	if err != nil {
		return nil, err
	}
	if err = cmd.Wait(); err != nil {
		util.LogWrite("Error during GitLsFiles")
		cmd.LogStdOutErr()
		return nil, fmt.Errorf("Error listing files in repository")
	}

	var filelist []string
	// TODO: Buffered reading
	stdout := cmd.OutPipe.ReadAll()
	for _, fl := range strings.Split(stdout, "\n") {
		fl = strings.TrimSpace(fl)
		if fl != "" {
			filelist = append(filelist, fl)
		}
	}
	return filelist, nil
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
			addchan <- RepoFileStatus{Err: fmt.Errorf("Error adding files to repository. Unable to toggle repository bare mode.")}
			return
		}
		defer setBare(true)
		whereisInfo, err := AnnexWhereis(filepaths)
		if err != nil {
			addchan <- RepoFileStatus{Err: fmt.Errorf("Error querying file annex status.")}
			return
		}
		annexfiles := make([]string, len(whereisInfo))
		for idx, wi := range whereisInfo {
			annexfiles[idx] = wi.File
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
	for {
		line, rerr := cmd.OutPipe.ReadLine()
		if rerr != nil {
			break
		}
		fname := strings.TrimSpace(line)
		fname = strings.TrimPrefix(fname, "add '")
		fname = strings.TrimSuffix(fname, "'")
		util.LogWrite("%s added to git", fname)
		addchan <- RepoFileStatus{FileName: fname, State: "Added"}
	}
	if err = cmd.Wait(); err != nil {
		util.LogWrite("Error during GitAdd")
		cmd.LogStdOutErr()
		addchan <- RepoFileStatus{Err: fmt.Errorf("Error adding files to repository")}
	}
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

	// build exclusion argument list
	// files < annex.minsize or matching exclusion extensions will not be annexed and
	// will instead be handled by git
	var exclargs []string
	if util.Config.Annex.MinSize != "" {
		sizefilterarg := fmt.Sprintf("--largerthan=%s", util.Config.Annex.MinSize)
		exclargs = append(exclargs, sizefilterarg)
	}

	for _, pattern := range util.Config.Annex.Exclude {
		arg := fmt.Sprintf("--exclude=%s", pattern)
		exclargs = append(exclargs, arg)
	}

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

	for {
		line, rerr := cmd.OutPipe.ReadLine()
		if rerr != nil {
			break
		}
		// Send file name
		err = json.Unmarshal([]byte(line), &annexAddRes)
		if err != nil {
			addchan <- RepoFileStatus{Err: err}
			return
		}
		if annexAddRes.Success {
			// Send file name to outchan
			util.LogWrite("%s added to annex", annexAddRes.File)
			addchan <- RepoFileStatus{FileName: annexAddRes.File, State: "Added"}
		} else {
			util.LogWrite("Error adding %s", annexAddRes.File)
			addchan <- RepoFileStatus{Err: fmt.Errorf("Error adding files to repository: Failed to add %s", annexAddRes.File)}
		}
	}
	if err = cmd.Wait(); err != nil {
		util.LogWrite("Error during AnnexAdd")
		cmd.LogStdOutErr()
		addchan <- RepoFileStatus{Err: fmt.Errorf("Error adding files to repository.")}
	}
	return
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
// Setting the Workingdir package global affects the working directory in which the command is executed.
// (git annex whereis)
func AnnexWhereis(paths []string) ([]AnnexWhereisResult, error) {
	cmdargs := []string{"whereis", "--json"}
	cmdargs = append(cmdargs, paths...)
	cmd, err := RunAnnexCommand(cmdargs...)
	// TODO: Parse output
	if err != nil || cmd.Wait() != nil {
		util.LogWrite("Error during AnnexWhereis")
		cmd.LogStdOutErr()
		return nil, fmt.Errorf("Error getting file status from server")
	}

	// TODO: Buffered reading
	stdout := cmd.OutPipe.ReadAll()
	resultsJSON := strings.Split(stdout, "\n")
	results := make([]AnnexWhereisResult, 0, len(resultsJSON))
	for _, resJSON := range resultsJSON {
		if len(resJSON) == 0 {
			continue
		}
		var res AnnexWhereisResult
		err := json.Unmarshal([]byte(resJSON), &res)
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
// Setting the Workingdir package global affects the working directory in which the command is executed.
func AnnexStatus(paths ...string) ([]AnnexStatusResult, error) {
	cmdargs := []string{"status", "--json"}
	cmdargs = append(cmdargs, paths...)
	cmd, err := RunAnnexCommand(cmdargs...)
	// TODO: Parse output
	if err != nil || cmd.Wait() != nil {
		util.LogWrite("Error during DescribeChanges")
		cmd.LogStdOutErr()
		return nil, fmt.Errorf("Error retrieving file status")
	}

	// TODO: Buffered reading
	stdout := cmd.OutPipe.ReadAll()
	files := strings.Split(stdout, "\n")

	statuses := make([]AnnexStatusResult, 0, len(files))
	var outStruct AnnexStatusResult
	for _, f := range files {
		if len(f) == 0 {
			// can return empty lines
			continue
		}
		err := json.Unmarshal([]byte(f), &outStruct)
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
	// TODO: 'git annex status' doesn't list added (A) files wnen in direct mode.
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
	statuses, err := AnnexStatus()
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
// Note that this function uses 'git annex add' to lock files, but only if they are marked as unlocked (T) by git annex.
// Attempting to lock an untracked file, or a file in any state other than T will have no effect.
// Setting the Workingdir package global affects the working directory in which the command is executed.
// (git annex add)
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

	if len(unlockedfiles) == 0 {
		util.LogWrite("No files to lock")
		return nil
	}
	cmdargs := []string{"add"}
	cmdargs = append(cmdargs, unlockedfiles...)
	cmd, err := RunAnnexCommand(cmdargs...)
	// TODO: Parse output
	if err != nil || cmd.Wait() != nil {
		util.LogWrite("Error during AnnexLock")
		cmd.LogStdOutErr()
		return fmt.Errorf("Error locking files")
	}
	return nil
}

// AnnexUnlock unlocks the specified files and directory contents if they are annexed
// Setting the Workingdir package global affects the working directory in which the command is executed.
// (git annex unlock)
func AnnexUnlock(paths ...string) error {
	cmdargs := []string{"unlock"}
	cmdargs = append(cmdargs, paths...)
	cmd, err := RunAnnexCommand(cmdargs...)
	if err != nil || cmd.Wait() != nil {
		util.LogWrite("Error during AnnexUnlock")
		cmd.LogStdOutErr()
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
// Setting the Workingdir package global affects the working directory in which the command is executed.
// (git annex info)
func AnnexInfo() (AnnexInfoResult, error) {
	cmd, err := RunAnnexCommand("info", "--json")
	if err != nil || cmd.Wait() != nil {
		util.LogWrite("Error during AnnexInfo")
		cmd.LogStdOutErr()
		return AnnexInfoResult{}, fmt.Errorf("Error retrieving annex info")
	}

	// TODO: Buffered reading
	stdout := cmd.OutPipe.ReadAll()
	var info AnnexInfoResult
	err = json.Unmarshal([]byte(stdout), &info)
	return info, err
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
		// Don't catch this result
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
	env := os.Environ()
	token := web.UserToken{}
	_ = token.LoadToken()
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
	annexsshopt := util.AnnexSSHOpt(token.Username)
	cmd.Args = append(cmd.Args, "-c", annexsshopt)
	util.LogWrite("Running shell command (Dir: %s): %s", Workingdir, strings.Join(cmd.Args, " "))
	err := cmd.Start()
	return cmd, err
}

// selectGitOrAnnex splits a list of paths into two: the first to be added to git proper and the second to be added to git annex.
// The selection is made based on the file type (extension) and size, both of which are configurable.
func selectGitOrAnnex(paths []string) (gitpaths []string, annexpaths []string) {
	minsize, err := humanize.ParseBytes(util.Config.Annex.MinSize)
	if err != nil {
		util.LogWrite("Invalid minsize string found in config. Defaulting to 10 MiB")
		minsize, _ = humanize.ParseBytes("10 MiB")
	}
	excludes := util.Config.Annex.Exclude

	util.LogWrite("Using minsize %v", minsize)
	util.LogWrite("Using exclude list %v", excludes)

	var fsize uint64
	for _, p := range paths {
		fstat, err := os.Stat(p)
		if err != nil {
			util.LogWrite("Cannot stat file [%s]: %s", p, err.Error())
			fsize = math.MaxUint64
		} else {
			fsize = uint64(fstat.Size())
		}
		if fsize < minsize {
			for _, pattern := range excludes {
				match, err := filepath.Match(pattern, p)
				if match {
					if err != nil {
						util.LogWrite("Bad pattern found in annex exclusion list %s", excludes)
						continue
					}
					gitpaths = append(gitpaths, p)
					util.LogWrite("Adding %v to git paths", p)
					continue
				}
			}
		}
		util.LogWrite("Adding %v to annex paths", p)
		annexpaths = append(annexpaths, p)
	}

	return
}

// GetAnnexVersion returns the version string of the system's git-annex.
func GetAnnexVersion() (string, error) {
	cmd, err := RunAnnexCommand("version", "--raw")
	if err != nil {
		util.LogWrite("Error while checking git-annex version")
		return "", err
	}
	if err = cmd.Wait(); err != nil {
		util.LogWrite("Error while checking git-annex version")
		cmd.LogStdOutErr()
		if strings.Contains(cmd.ErrPipe.ReadAll(), "command not found") {
			return "", fmt.Errorf("Error: git-annex command not found")
		}
		return "", err
	}

	stdout := cmd.OutPipe.ReadAll()
	return stdout, nil
}
