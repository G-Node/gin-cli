package ginclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
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
	_, _, err := RunGitCommand("config", "--local", "user.name", name)
	if err != nil {
		return err
	}
	_, _, err = RunGitCommand("config", "--local", "user.email", email)
	return err
}

// CommitIfNew creates an empty initial git commit if the current repository is completely new.
// Returns 'true' if (and only if) a commit was created.
// Setting the Workingdir package global affects the working directory in which the command is executed.
func CommitIfNew() (bool, error) {
	if !IsRepo() {
		return false, fmt.Errorf("Not a repository")
	}
	_, _, err := RunGitCommand("rev-parse", "HEAD")
	if err == nil {
		// All good. No need to do anything
		return false, nil
	}

	// Create an empty initial commit and run annex sync to synchronise everything
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "(unknown)"
	}
	commitargs := []string{"commit", "--allow-empty", "-m", fmt.Sprintf("Initial commit: Repository initialised on %s", hostname)}
	stdout, stderr, err := RunGitCommand(commitargs...)
	if err != nil {
		util.LogWrite("Error while creating initial commit")
		util.LogWrite("[stdout]\r\n%s", stdout.String())
		util.LogWrite("[stderr]\r\n%s", stderr.String())
		return false, err
	}
	return true, nil
}

// IsRepo checks whether the current working directory is in a git repository.
// This function will also return true for bare repositories that use git annex (direct mode).
// Setting the Workingdir package global affects the working directory in which the command is executed.
func IsRepo() bool {
	util.LogWrite("IsRepo '%s'?", Workingdir)
	_, _, err := RunGitCommand("status")
	yes := err == nil
	if !yes {
		// Maybe it's an annex repo in direct mode?
		_, _, err = RunAnnexCommand("status")
		yes = err == nil
	}
	util.LogWrite("IsRepo: %v", yes)
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

// AnnexInit initialises the repository for annex.
// Setting the Workingdir package global affects the working directory in which the command is executed.
// (git annex init)
func AnnexInit(description string) error {
	args := []string{"init", description}
	stdout, stderr, err := RunAnnexCommand(args...)
	util.LogWrite("[stdout]\r\n%s", stdout.String())
	util.LogWrite("[stderr]\r\n%s", stderr.String())
	if err != nil {
		initError := fmt.Errorf("Repository annex initialisation failed.")
		util.LogWrite(initError.Error())
		return initError
	}
	stdout, stderr, err = RunGitCommand("config", "annex.backends", "MD5")
	if err != nil {
		util.LogWrite("Failed to set default annex backend MD5")
		util.LogWrite("[Error]: %v", err)
		util.LogWrite("[stdout]\r\n%s", stdout.String())
		util.LogWrite("[stderr]\r\n%s", stderr.String())
	}
	return nil
}

// AnnexPull downloads all annexed files.
// Setting the Workingdir package global affects the working directory in which the command is executed.
// (git annex sync --no-push [--content])
func AnnexPull(content bool) error {
	args := []string{"sync", "--no-push"}
	if content {
		args = append(args, "--content")
	}
	stdout, stderr, err := RunAnnexCommand(args...)
	if err != nil {
		util.LogWrite("Error during AnnexPull.")
		util.LogWrite("[Error]: %v", err)
		util.LogWrite("[stdout]\r\n%s", stdout.String())
		util.LogWrite("[stderr]\r\n%s", stderr.String())
		return fmt.Errorf("Error downloading files")
	}
	return nil
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
	stdout, stderr, err := RunAnnexCommand(args...)

	if err != nil {
		util.LogWrite("Error during AnnexSync")
		util.LogWrite("[Error]: %v", err)
		util.LogWrite("[stdout]\r\n%s", stdout.String())
		util.LogWrite("[stderr]\r\n%s", stderr.String())
		return fmt.Errorf("Error synchronising files")
	}
	return nil
}

// AnnexPush uploads all annexed files.
// Setting the Workingdir package global affects the working directory in which the command is executed.
// (git annex sync --no-pull --content)
func AnnexPush(paths []string, commitMsg string) error {
	// contarg := make([]string, len(paths))
	// for idx, p := range paths {
	// 	contarg[idx] = fmt.Sprintf("--content-of=%s", p)
	// }
	cmdargs := []string{"sync", "--no-pull", "--commit", fmt.Sprintf("--message=%s", commitMsg)}
	// cmdargs = append(cmdargs, contarg...)
	stdout, stderr, err := RunAnnexCommand(cmdargs...)

	if err != nil {
		util.LogWrite("Error during AnnexPush (sync --no-pull)")
		util.LogWrite("[Error]: %v", err)
		util.LogWrite("[stdout]\r\n%s", stdout.String())
		util.LogWrite("[stderr]\r\n%s", stderr.String())
		return fmt.Errorf("Error uploading files")
	}

	cmdargs = []string{"copy"}
	cmdargs = append(cmdargs, paths...)
	// NOTE: Using origin which is the conventional default remote. This should be fixed.
	cmdargs = append(cmdargs, "--to=origin")
	stdout, stderr, err = RunAnnexCommand(cmdargs...)

	if err != nil {
		util.LogWrite("Error during AnnexPush (copy)")
		util.LogWrite("[Error]: %v", err)
		util.LogWrite("[stdout]\r\n%s", stdout.String())
		util.LogWrite("[stderr]\r\n%s", stderr.String())
		return fmt.Errorf("Error uploading files")
	}
	return nil
}

// AnnexGet retrieves the content of specified files.
// Setting the Workingdir package global affects the working directory in which the command is executed.
// (git annex get)
func AnnexGet(filepaths []string) error {
	// TODO: Print success for each file as it finishes
	cmdargs := append([]string{"get"}, filepaths...)
	stdout, stderr, err := RunAnnexCommand(cmdargs...)
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
// Setting the Workingdir package global affects the working directory in which the command is executed.
// (git annex drop)
func AnnexDrop(filepaths []string) error {
	// TODO: Print success for each file as it finishes
	cmdargs := append([]string{"drop"}, filepaths...)
	stdout, stderr, err := RunAnnexCommand(cmdargs...)
	if err != nil {
		util.LogWrite("Error during AnnexDrop")
		util.LogWrite("[Error]: %v", err)
		util.LogWrite("[stdout]\r\n%s", stdout.String())
		util.LogWrite("[stderr]\r\n%s", stderr.String())
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
	stdout, stderr, err := RunGitCommand("config", "--local", "--bool", "core.bare", statestr)
	if err != nil {
		util.LogWrite("Error switching bare status to %s", statestr)
		util.LogWrite("[stdout]\r\n%s", stdout.String())
		util.LogWrite("[stderr]\r\n%s", stderr.String())
	}
	return err
}

// GitAdd adds paths to git directly (not annex).
// In direct mode, files that are already in the annex are explicitly ignored.
// In indirect mode, adding annexed files to git has no effect.
// Setting the Workingdir package global affects the working directory in which the command is executed.
// (git add)
func GitAdd(filepaths []string) ([]string, error) {
	if len(filepaths) == 0 {
		util.LogWrite("No paths to add to git. Nothing to do.")
		return nil, nil
	}

	if IsDirect() {
		// Set bare false and revert at the end of the function
		err := setBare(false)
		if err != nil {
			return nil, fmt.Errorf("Error adding files to repository. Unable to toggle repository bare mode.")
		}
		defer setBare(true)
		whereisInfo, err := AnnexWhereis(filepaths)
		if err != nil {
			return nil, fmt.Errorf("Error querying file annex status.")
		}
		annexfiles := make([]string, len(whereisInfo))
		for idx, wi := range whereisInfo {
			annexfiles[idx] = wi.File
		}
		filepaths = util.FilterPaths(filepaths, annexfiles)
	}

	cmdargs := append([]string{"add", "--verbose"}, filepaths...)
	stdout, stderr, err := RunGitCommand(cmdargs...)
	if err != nil {
		util.LogWrite("Error during GitAdd")
		util.LogWrite("[stdout]\r\n%s", stdout.String())
		util.LogWrite("[stderr]\r\n%s", stderr.String())
		return nil, fmt.Errorf("Error adding files to repository")
	}

	var added []string
	for _, line := range strings.Split(stdout.String(), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		line = strings.TrimPrefix(line, "add '")
		line = strings.TrimSuffix(line, "'")
		added = append(added, line)
	}

	util.LogWrite("Files added to git: %v", added)
	return added, nil
}

// AnnexAddResult is used to store information about each added file, as returned from the annex command.
type AnnexAddResult struct {
	Command string `json:"command"`
	File    string `json:"file"`
	Key     string `json:"key"`
	Success bool   `json:"success"`
}

// AnnexAdd adds paths to the annex.
// Files specified for exclusion in the configuration are ignored automatically.
// Setting the Workingdir package global affects the working directory in which the command is executed.
// (git annex add)
func AnnexAdd(filepaths []string) ([]string, error) {
	if len(filepaths) == 0 {
		util.LogWrite("No paths to add to annex. Nothing to do.")
		return nil, nil
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

	stdout, stderr, err := RunAnnexCommand(cmdargs...)
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
	util.LogWrite("Files added to annex: %v", added)

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
// Setting the Workingdir package global affects the working directory in which the command is executed.
// (git annex whereis)
func AnnexWhereis(paths []string) ([]AnnexWhereisResult, error) {
	cmdargs := []string{"whereis", "--json"}
	cmdargs = append(cmdargs, paths...)
	stdout, stderr, err := RunAnnexCommand(cmdargs...)
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
// Setting the Workingdir package global affects the working directory in which the command is executed.
func AnnexStatus(paths ...string) ([]AnnexStatusResult, error) {
	cmdargs := []string{"status", "--json"}
	cmdargs = append(cmdargs, paths...)
	stdout, stderr, err := RunAnnexCommand(cmdargs...)
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
	stdout, stderr, err := RunAnnexCommand(cmdargs...)
	if err != nil {
		util.LogWrite("Error during AnnexLock")
		util.LogWrite("[stdout]\r\n%s", stdout.String())
		util.LogWrite("[stderr]\r\n%s", stderr.String())
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
	stdout, stderr, err := RunAnnexCommand(cmdargs...)
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
// Setting the Workingdir package global affects the working directory in which the command is executed.
// (git annex info)
func AnnexInfo() (AnnexInfoResult, error) {
	stdout, stderr, err := RunAnnexCommand("info", "--json")
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

var modecache = make(map[string]bool)

// IsDirect returns true if the repository in a given path is working in git annex 'direct' mode.
// If path is not a repository, or is not an initialised annex repository, the result defaults to false.
// If the path is a repository and no error was raised, the result it cached so that subsequent checks are faster.
// Setting the Workingdir package global affects the working directory in which the command is executed.
func IsDirect() bool {
	if mode, ok := modecache[Workingdir]; ok {
		return mode
	}
	stdout, _, err := RunGitCommand("config", "--local", "annex.direct")
	if err != nil {
		// Don't cache this result
		return false
	}
	if strings.TrimSpace(stdout.String()) == "true" {
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
	stdout, stderr, err := RunGitCommand("config", "--local", "--get", "annex.version")
	if err != nil {
		util.LogWrite("Error while checking repository annex version")
		util.LogWrite("[stdout]\r\n%s", stdout.String())
		util.LogWrite("[stderr]\r\n%s", stderr.String())
		return false
	}
	ver := strings.TrimSpace(stdout.String())
	util.LogWrite("Annex version is %s", ver)
	return ver == "6"
}

// Utility functions for shelling out

// RunGitCommand executes an external git command with the provided arguments and returns stdout and stderr.
// Setting the Workingdir package global affects the working directory in which the command is executed.
func RunGitCommand(args ...string) (bytes.Buffer, bytes.Buffer, error) {
	gitbin := util.Config.Bin.Git
	cmd := exec.Command(gitbin)
	cmd.Dir = Workingdir
	cmd.Args = append(cmd.Args, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	env := os.Environ()
	token := web.UserToken{}
	_ = token.LoadToken()
	cmd.Env = append(env, util.GitSSHEnv(token.Username))
	util.LogWrite("Running shell command (Dir: %s): %s", Workingdir, strings.Join(cmd.Args, " "))
	err := cmd.Run()
	return stdout, stderr, err
}

// RunAnnexCommand executes a git annex command with the provided arguments and returns stdout and stderr.
// Setting the Workingdir package global affects the working directory in which the command is executed.
func RunAnnexCommand(args ...string) (bytes.Buffer, bytes.Buffer, error) {
	gitannexbin := util.Config.Bin.GitAnnex
	cmd := exec.Command(gitannexbin, args...)
	cmd.Dir = Workingdir
	token := web.UserToken{}
	_ = token.LoadToken()
	annexsshopt := util.AnnexSSHOpt(token.Username)
	cmd.Args = append(cmd.Args, "-c", annexsshopt)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	util.LogWrite("Running shell command (Dir: %s): %s", Workingdir, strings.Join(cmd.Args, " "))
	err := cmd.Run()
	return stdout, stderr, err
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
