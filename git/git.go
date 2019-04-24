package git

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/G-Node/gin-cli/ginclient/config"
	"github.com/G-Node/gin-cli/ginclient/log"
	"github.com/G-Node/gin-cli/git/shell"
)

const progcomplete = "100%"
const unknownhostname = "(unknown)"

// giterror convenience alias to util.Error
type giterror = shell.Error

// **************** //

// Types

// RepoFileStatus describes the status of files when being added to the repo or transferred to/from remotes.
type RepoFileStatus struct {
	// The name of the file.
	FileName string `json:"filename"`
	// The state of the operation.
	State string `json:"state"`
	// Progress of the operation, if available. If partial progress isn't available or applicable, this will be empty.
	Progress string `json:"progress"`
	// The data rate, if available.
	Rate string `json:"rate"`
	// original cmd input
	RawInput string `json:"rawinput"`
	// original command output
	RawOutput string `json:"rawoutput"`
	// Errors
	Err error
}

// TODO: Create structs to accommodate extra information for other operations

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

type DiffStat struct {
	NewFiles      []string
	DeletedFiles  []string
	ModifiedFiles []string
}

// Object contains the information for a tree or blob object in git
type Object struct {
	Name string
	Hash string
	Type string
	Mode string
}

// MarshalJSON overrides the default marshalling of RepoFileStatus to return the error string for the Err field.
func (s RepoFileStatus) MarshalJSON() ([]byte, error) {
	type RFSAlias RepoFileStatus
	errmsg := ""
	if s.Err != nil {
		errmsg = s.Err.Error()
	}
	return json.Marshal(struct {
		Err string `json:"err"`
		RFSAlias
	}{
		Err:      errmsg,
		RFSAlias: RFSAlias(s),
	})
}

// Git commands

// Init initialises the current directory as a git repository.
// The repository is optionally initialised as bare.
// (git init [--bare])
func Init(bare bool) error {
	fn := fmt.Sprintf("Init(%v)", bare)
	args := []string{"init"}
	if bare {
		args = append(args, "--bare")
	}
	cmd := Command(args...)
	stdout, stderr, err := cmd.OutputError()
	if err != nil {
		log.Write("Error during init command")
		logstd(stdout, stderr)
		gerr := giterror{UError: string(stderr), Origin: fn}
		return gerr
	}
	return nil
}

// Clone downloads a repository and sets the remote fetch and push urls.
// The status channel 'clonechan' is closed when this function returns.
// (git clone ...)
func Clone(remotepath string, repopath string, clonechan chan<- RepoFileStatus) {
	// TODO: This function is crazy huge - simplify
	fn := fmt.Sprintf("Clone(%s)", remotepath)
	defer close(clonechan)
	args := []string{"clone", "--progress", remotepath}
	if runtime.GOOS == "windows" {
		// force disable symlinks even if user can create them
		// see https://git-annex.branchable.com/bugs/Symlink_support_on_Windows_10_Creators_Update_with_Developer_Mode/
		args = append([]string{"-c", "core.symlinks=false"}, args...)
	}
	cmd := Command(args...)
	err := cmd.Start()
	if err != nil {
		clonechan <- RepoFileStatus{Err: giterror{UError: err.Error(), Origin: fn}}
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
	lineInput := cmd.Args
	input := strings.Join(lineInput, " ")
	status.RawInput = input
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
			status.FileName = repopath
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
					status.RawOutput = line
				}
			}
			clonechan <- status
		}
	}

	errstring := string(stderr)
	if err = cmd.Wait(); err != nil {
		log.Write("Error during clone command")
		repoPathParts := strings.SplitN(repopath, "/", 2)
		repoOwner := repoPathParts[0]
		repoName := repoPathParts[1]
		gerr := giterror{UError: errstring, Origin: fn}
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

// Pull downloads all small (git) files from the server.
// (git pull --ff-only)
func Pull(remote string) error {
	// TODO: Common output handling with Push
	cmd := Command("pull", "--ff-only", remote)
	stdout, stderr, err := cmd.OutputError()

	if err != nil {
		logstd(stdout, stderr)
		return fmt.Errorf("download command failed: %s", string(stderr))
	}
	return nil
}

// Push uploads all small (git) files to the server.
// (git push)
func Push(remote string, pushchan chan<- RepoFileStatus) {
	defer close(pushchan)

	if IsDirect() {
		// Set bare false and revert at the end of the function
		err := setBare(false)
		if err != nil {
			pushchan <- RepoFileStatus{Err: fmt.Errorf("failed to toggle repository bare mode")}
			return
		}
		defer setBare(true)
	}

	cmd := Command("push", "--progress", remote)
	err := cmd.Start()
	if err != nil {
		pushchan <- RepoFileStatus{Err: err}
	}

	var status RepoFileStatus
	var line string
	var rerr error
	re := regexp.MustCompile(`(?P<state>Compressing|Writing) objects:\s+(?P<progress>[0-9]{2,3})% \((?P<n>[0-9]+)/(?P<N>[0-9]+)\)`)
	lineInput := cmd.Args
	input := strings.Join(lineInput, " ")
	status.RawInput = input
	for rerr = nil; rerr == nil; line, rerr = cmd.ErrReader.ReadString('\r') {
		if !re.MatchString(line) {
			continue
		}
		match := re.FindStringSubmatch(line)
		status.State = match[1]
		if status.State == "Writing" {
			status.State = fmt.Sprintf("Uploading git files (to: %s)", remote)
		}
		status.Progress = fmt.Sprintf("%s%%", match[2])
		status.RawOutput = line
		pushchan <- status
	}
	return
}

// Add adds paths to git directly (not annex).
// In direct mode, files that are already in the annex are explicitly ignored.
// In indirect mode, adding annexed files to git has no effect.
// The status channel 'addchan' is closed when this function returns.
// (git add)
func Add(filepaths []string, addchan chan<- RepoFileStatus) {
	defer close(addchan)
	if len(filepaths) == 0 {
		log.Write("No paths to add to git. Nothing to do.")
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
		// Call addPathsDirect to collect filenames not in annex and deleted files
		filepaths = gitAddDirect(filepaths)
	}

	// exclargs := annexExclArgs()
	cmdargs := []string{"add", "--verbose", "--"}
	cmdargs = append(cmdargs, filepaths...)
	cmd := Command(cmdargs...)
	err := cmd.Start()
	if err != nil {
		addchan <- RepoFileStatus{Err: err}
		return
	}
	var status RepoFileStatus
	var line string
	var rerr error
	lineInput := cmd.Args
	input := strings.Join(lineInput, " ")
	status.RawInput = input
	for rerr = nil; rerr == nil; line, rerr = cmd.OutReader.ReadString('\n') {
		fname := strings.TrimSpace(line)
		status.RawOutput = line
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
		log.Write("'%s' added to git", fname)
		// Error conditions?
		status.Progress = progcomplete
		addchan <- status
	}
	var stderr, errline []byte
	if cmd.Wait() != nil {
		for rerr = nil; rerr == nil; errline, rerr = cmd.OutReader.ReadBytes('\000') {
			stderr = append(stderr, errline...)
		}
		log.Write("Error during GitAdd")
		logstd(nil, stderr)
	}
	return
}

// SetGitUser sets the user.name and user.email configuration values for the local git repository.
func SetGitUser(name, email string) error {
	if !IsRepo() {
		return fmt.Errorf("not a repository")
	}
	err := ConfigSet("user.name", name)
	if err != nil {
		return err
	}
	return ConfigSet("user.email", email)
}

// ConfigGet returns the value of a given git configuration key.
// The returned key is always a string.
// (git config --get)
func ConfigGet(key string) (string, error) {
	fn := fmt.Sprintf("ConfigGet(%s)", key)
	cmd := Command("config", "--get", key)
	stdout, stderr, err := cmd.OutputError()
	if err != nil {
		gerr := giterror{UError: string(stderr), Origin: fn}
		log.Write("Error during config get")
		logstd(stdout, stderr)
		return "", gerr
	}
	value := string(stdout)
	value = strings.TrimSpace(value)
	return value, nil
}

// ConfigSet sets a configuration value in the local git config.
// (git config --local)
func ConfigSet(key, value string) error {
	fn := fmt.Sprintf("ConfigSet(%s, %s)", key, value)
	cmd := Command("config", "--local", key, value)
	stdout, stderr, err := cmd.OutputError()
	if err != nil {
		gerr := giterror{UError: string(stderr), Origin: fn}
		log.Write("Error during config set")
		logstd(stdout, stderr)
		return gerr
	}
	return nil
}

// ConfigUnset unsets a configuration value in the local git config.
// (git config unset --local)
func ConfigUnset(key string) error {
	fn := fmt.Sprintf("ConfigUnset(%s)", key)
	cmd := Command("config", "--unset", "--local", key)
	stdout, stderr, err := cmd.OutputError()
	if err != nil {
		gerr := giterror{UError: string(stderr), Origin: fn}
		log.Write("Error during config unset")
		logstd(stdout, stderr)
		return gerr
	}
	return nil
}

// RemoteShow returns the configured remotes and their URL.
// (git remote -v show -n)
func RemoteShow() (map[string]string, error) {
	fn := "RemoteShow()"
	cmd := Command("remote", "-v", "show", "-n")
	stdout, stderr, err := cmd.OutputError()
	if err != nil {
		sstderr := string(stderr)
		gerr := giterror{UError: sstderr, Origin: fn}
		log.Write("Error during remote show command")
		logstd(stdout, stderr)
		return nil, gerr
	}
	remotes := make(map[string]string)
	sstdout := string(stdout)
	for _, line := range strings.Split(sstdout, "\n") {
		line = strings.TrimSuffix(line, "\n")
		if len(strings.TrimSpace(line)) == 0 {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) != 3 {
			log.Write("Unexpected output: %s", line)
			continue
		}
		remotes[parts[0]] = parts[1]
	}

	return remotes, nil
}

// RemoteAdd adds a remote named name for the repository at URL.
func RemoteAdd(name, url string) error {
	fn := fmt.Sprintf("RemoteAdd(%s, %s)", name, url)
	cmd := Command("remote", "add", name, url)
	stdout, stderr, err := cmd.OutputError()
	if err != nil {
		sstderr := string(stderr)
		gerr := giterror{UError: sstderr, Origin: fn}
		log.Write("Error during remote add command")
		logstd(stdout, stderr)
		if strings.Contains(sstderr, "already exists") {
			gerr.Description = fmt.Sprintf("remote with name '%s' already exists", name)
		}
		return gerr
	}
	// Performing fetch after adding remote to retrieve references
	// Any errors are logged and ignored
	cmd = Command("fetch", name)
	stdout, stderr, err = cmd.OutputError()
	if err != nil {
		logstd(stdout, stderr)
	}
	return nil
}

// RemoteRemove removes the remote named name from the repository configuration.
func RemoteRemove(name string) error {
	fn := fmt.Sprintf("RemoteRm(%s)", name)
	cmd := Command("remote", "remove", name)
	stdout, stderr, err := cmd.OutputError()
	if err != nil {
		sstderr := string(stderr)
		gerr := giterror{UError: sstderr, Origin: fn}
		log.Write("Error during remote remove command")
		logstd(stdout, stderr)
		if strings.Contains(sstderr, "No such remote") {
			gerr.Description = fmt.Sprintf("remote with name '%s' does not exist", name)
		}
		return gerr
	}
	return nil
}

// BranchSetUpstream sets the default upstream remote for the current branch.
// (git branch --set-upstream-to=)
func BranchSetUpstream(name string) error {
	fn := fmt.Sprintf("BranchSetUpstream(%s)", name)
	cmd := Command("branch", fmt.Sprintf("--set-upstream-to=%s/master", name))
	stdout, stderr, err := cmd.OutputError()
	if err != nil {
		gerr := giterror{UError: string(stderr), Origin: fn}
		log.Write("Error during branch set-upstream-to")
		logstd(stdout, stderr)
		return gerr
	}
	return nil
}

// LsRemote performs a git ls-remote of a specific remote.
// The argument can be a name or a URL.
// (git ls-remote)
func LsRemote(remote string) (string, error) {
	fn := fmt.Sprintf("LsRemote(%s)", remote)
	cmd := Command("ls-remote", remote)
	stdout, stderr, err := cmd.OutputError()
	if err != nil {
		sstderr := string(stderr)
		gerr := giterror{UError: sstderr, Origin: fn}
		if strings.Contains(sstderr, "does not exist") || strings.Contains(sstderr, "Permission denied") {
			gerr.Description = fmt.Sprintf("remote %s does not exist", remote)
		}
		log.Write("Error during ls-remote command")
		logstd(stdout, stderr)
		return "", gerr
	}

	return string(stdout), nil
}

// RevParse parses an argument and returns the unambiguous, SHA1 representation.
// (git rev-parse)
func RevParse(rev string) (string, error) {
	fn := fmt.Sprintf("RevParse(%s)", rev)
	cmd := Command("rev-parse", rev)
	stdout, stderr, err := cmd.OutputError()
	if err != nil {
		log.Write("Error during rev-parse command")
		logstd(stdout, stderr)
		gerr := giterror{UError: string(stderr), Origin: fn}
		return "", gerr
	}
	return string(stdout), nil
}

// IsRepo checks whether the current working directory is in a git repository.
// This function will also return true for bare repositories that use git annex (direct mode).
func IsRepo() bool {
	path, _ := filepath.Abs(".")
	log.Write("IsRepo '%s'?", path)
	_, err := FindRepoRoot(path)
	yes := err == nil
	log.Write("%v", yes)
	return yes
}

// **************** //

// Commit records changes that have been added to the repository with a given message.
// (git commit)
func Commit(commitmsg string) error {
	if IsDirect() {
		// Set bare false and revert at the end of the function
		err := setBare(false)
		if err != nil {
			return fmt.Errorf("failed to toggle repository bare mode")
		}
		defer setBare(true)
	}

	cmd := Command("commit", fmt.Sprintf("--message=%s", commitmsg))
	stdout, stderr, err := cmd.OutputError()

	if err != nil {
		sstdout := string(stdout)
		if strings.Contains(sstdout, "nothing to commit") || strings.Contains(sstdout, "nothing added to commit") || strings.Contains(sstdout, "no changes added to commit") {
			// Return special error
			log.Write("Nothing to commit")
			return fmt.Errorf("Nothing to commit")
		}
		log.Write("Error during GitCommit")
		logstd(stdout, stderr)
		return fmt.Errorf(string(stderr))
	}
	return nil
}

// CommitEmpty performs a commit even when there are no new changes added to the index.
// This is useful for initialising new repositories with a usable HEAD.
// In indirect mode (non-bare repositories) simply uses git commit with the '--allow-empty' flag.
// In direct mode it uses git-annex sync.
// (git commit --allow-empty or git annex sync --commit)
func CommitEmpty(commitmsg string) error {
	msgarg := fmt.Sprintf("--message=%s", commitmsg)
	var cmd shell.Cmd
	if !IsDirect() {
		cmd = Command("commit", "--allow-empty", msgarg)
	} else {
		cmd = AnnexCommand("sync", "--commit", msgarg)
	}
	stdout, stderr, err := cmd.OutputError()
	if err != nil {
		log.Write("Error during CommitEmpty")
		logstd(stdout, stderr)
		return fmt.Errorf(string(stderr))
	}
	return nil
}

// DiffUpstream returns, through the provided channel, the names of all files that differ from the default remote branch.
// The output channel 'diffchan' is closed when this function returns.
// (git diff --name-only --relative @{upstream})
func DiffUpstream(paths []string, upstream string, diffchan chan<- string) {
	defer close(diffchan)
	diffargs := []string{"diff", "-z", "--name-only", "--relative", upstream, "--"}
	diffargs = append(diffargs, paths...)
	cmd := Command(diffargs...)
	err := cmd.Start()
	if err != nil {
		log.Write("ls-files command set up failed: %s", err)
		return
	}
	var line []byte
	var rerr error
	for rerr = nil; rerr == nil; line, rerr = cmd.OutReader.ReadBytes('\000') {
		line = bytes.TrimSuffix(line, []byte("\000"))
		if len(line) > 0 {
			diffchan <- string(line)
		}
	}

	var stderr, errline []byte
	if cmd.Wait() != nil {
		for rerr = nil; rerr == nil; errline, rerr = cmd.OutReader.ReadBytes('\000') {
			stderr = append(stderr, errline...)
		}
		log.Write("Error during DiffUpstream")
		logstd(nil, stderr)
	}
	return
}

// LsFiles lists all files known to git.
// The output channel 'lschan' is closed when this function returns.
// (git ls-files)
func LsFiles(args []string, lschan chan<- string) {
	defer close(lschan)
	cmdargs := append([]string{"ls-files"}, args...)
	cmd := Command(cmdargs...)
	err := cmd.Start()
	if err != nil {
		log.Write("ls-files command set up failed: %s", err)
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
		log.Write("Error during GitLsFiles")
		logstd(nil, stderr)
	}
	return
}

// DescribeIndexShort returns a string which represents a condensed form of the git (annex) index.
// It is constructed using the result of 'git annex status'.
// The description is composed of the file count for each status: added, modified, deleted
// If 'paths' are specified, the status output is limited to files and directories matching those paths.
func DescribeIndexShort(paths []string) (string, error) {
	// TODO: 'git annex status' doesn't list added (A) files when in direct mode.
	statuschan := make(chan AnnexStatusRes)
	go AnnexStatus(paths, statuschan)
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

// Log returns the commit logs for the repository.
// The number of commits can be limited by the count argument.
// If count <= 0, the entire commit history is returned.
// Revisions which match only the deletion of the matching paths can be filtered using the showdeletes argument.
func Log(count uint, revrange string, paths []string, showdeletes bool) ([]GinCommit, error) {
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
	cmd := Command(cmdargs...)
	err := cmd.Start()
	if err != nil {
		log.Write("Error setting up git log command")
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
			log.Write("Error parsing git log")
			log.Write(string(line))
			log.Write(ierr.Error())
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
		log.Write("Error getting git log")
		errmsg := string(stderr)
		if strings.Contains(errmsg, "bad revision") {
			errmsg = fmt.Sprintf("'%s' does not match a known version ID or name", revrange)
		}
		return nil, fmt.Errorf(errmsg)
	}

	// TODO: Combine diffstats into first git log invocation
	logstats, err := LogDiffStat(count, paths, showdeletes)
	if err != nil {
		log.Write("Failed to get diff stats")
		return commits, nil
	}

	for idx, commit := range commits {
		commits[idx].FileStats = logstats[commit.Hash]
	}

	return commits, nil
}

func LogDiffStat(count uint, paths []string, showdeletes bool) (map[string]DiffStat, error) {
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
	cmd := Command(cmdargs...)
	err := cmd.Start()
	if err != nil {
		log.Write("Error during LogDiffstat")
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
				log.Write("Could not parse diffstat line")
				log.Write(line)
			}
			stats[curhash] = curstat
		}
	}

	return stats, nil
}

// Checkout performs a git checkout of a specific commit.
// Individual files or directories may be specified, otherwise the entire tree is checked out.
func Checkout(hash string, paths []string) error {
	cmdargs := []string{"checkout", hash, "--"}
	if paths == nil || len(paths) == 0 {
		reporoot, _ := FindRepoRoot(".")
		os.Chdir(reporoot)
		paths = []string{"."}
	}
	cmdargs = append(cmdargs, paths...)

	cmd := Command(cmdargs...)
	stdout, stderr, err := cmd.OutputError()
	if err != nil {
		log.Write("Error during GitCheckout")
		logstd(stdout, stderr)
		return fmt.Errorf(string(stderr))
	}
	return nil
}

// LsTree performs a recursive git ls-tree with a given revision (hash) and a list of paths.
// For each item, it returns a struct which contains the type (blob, tree), the mode, the hash, and the absolute (repo rooted) path to the object (name).
func LsTree(revision string, paths []string) ([]Object, error) {
	cmdargs := []string{"ls-tree", "--full-tree", "-z", "-t", "-r", revision}
	cmdargs = append(cmdargs, paths...)
	cmd := Command(cmdargs...)
	// This command doesn't need to be read line-by-line
	err := cmd.Start()
	if err != nil {
		return nil, err
	}

	var objects []Object
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
		obj := Object{
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
		log.Write("Error during GitLsTree")
		logstd(nil, stderr)
		return nil, fmt.Errorf(string(stderr))
	}

	return objects, nil
}

// CatFileContents performs a git-cat-file of a specific file from a specific commit and returns the file contents (as bytes).
func CatFileContents(revision, filepath string) ([]byte, error) {
	cmd := Command("cat-file", "blob", fmt.Sprintf("%s:%s", revision, filepath))
	stdout, stderr, err := cmd.OutputError()
	if err != nil {
		log.Write("Error during GitCatFile (Contents)")
		logstd(nil, stderr)
		return nil, fmt.Errorf(string(stderr))
	}
	return stdout, nil
}

// CatFileType returns the type of a given object at a given revision (blob, tree, or commit)
func CatFileType(object string) (string, error) {
	cmd := Command("cat-file", "-t", object)
	stdout, stderr, err := cmd.OutputError()
	if err != nil {
		log.Write("Error during GitCatFile (Type)")
		logstd(stdout, stderr)
		err = fmt.Errorf(string(stderr))
		return "", err
	}
	return string(stdout), nil
}

// RevCount returns the number of commits between two revisions.
func RevCount(a, b string) (int, error) {
	cmd := Command("rev-list", "--count", fmt.Sprintf("%s..%s", a, b))
	stdout, stderr, err := cmd.OutputError()
	if err != nil {
		logstd(stdout, stderr)
		return 0, fmt.Errorf(string(stderr))
	}
	return strconv.Atoi(string(stdout))
}

// IsDirect returns true if the repository in a given path is working in git annex 'direct' mode.
// If path is not a repository, or is not an initialised annex repository, the result defaults to false.
// If the path is a repository and no error was raised, the result it cached so that subsequent checks are faster.
func IsDirect() bool {
	abspath, _ := filepath.Abs(".")
	if mode, ok := annexmodecache[abspath]; ok {
		return mode
	}
	cmd := Command("config", "--local", "annex.direct")
	stdout, _, err := cmd.OutputError()
	if err != nil {
		// Don't cache this result
		return false
	}

	if strings.TrimSpace(string(stdout)) == "true" {
		annexmodecache[abspath] = true
		return true
	}
	annexmodecache[abspath] = false
	return false
}

// IsVersion6 returns true if the repository in a given path is working in git annex 'direct' mode.
// If path is not a repository, or is not an initialised annex repository, the result defaults to false.
func IsVersion6() bool {
	cmd := Command("config", "--local", "--get", "annex.version")
	stdout, stderr, err := cmd.OutputError()
	if err != nil {
		log.Write("Error while checking repository annex version")
		logstd(stdout, stderr)
		return false
	}
	ver := strings.TrimSpace(string(stdout))
	log.Write("Annex version is %s", ver)
	return ver == "6"
}

// mergeAbort aborts an unfinished git merge.
func mergeAbort() {
	// Here, we run a git status without checking any part of the result. It
	// seems git-annex performs some cleanup or consistency fixes to the index
	// when git status is run and before that, the merge --abort fails.
	Command("status").Run()
	cmd := Command("merge", "--abort")
	stdout, stderr, err := cmd.OutputError()
	if err != nil {
		// log error but do nothing
		logstd(stdout, stderr)
	}
}

func SetBare(state bool) {
	setBare(state)
}

func setBare(state bool) error {
	var statestr string
	if state {
		statestr = "true"
	} else {
		statestr = "false"
	}
	cmd := Command("config", "--local", "--bool", "core.bare", statestr)
	stdout, stderr, err := cmd.OutputError()
	if err != nil {
		log.Write("Error switching bare status to %s", statestr)
		logstd(stdout, stderr)
		err = fmt.Errorf(string(stderr))
	}
	return err
}

// gitAddDirect determines which files to be added to git when in direct mode.
// In direct mode, in order to perform a 'git add' operation, the client temporarily disables bare mode in the repository.
// This has a side effect that annexed files change type (from symlinks to "direct" files), since now git's view of the repository is not modified by annex.
// This function filters out any files known to annex to avoid re-adding them to git as files.
// The filtering is done twice:
// Once against the provided paths in the current directory (recursively) and once more against the output of 'git ls-files <paths>', in order to include any files that might have been deleted.
func gitAddDirect(paths []string) (filtered []string) {
	wichan := make(chan AnnexWhereisRes)
	go AnnexWhereis(paths, wichan)
	var annexfiles []string
	for wiInfo := range wichan {
		if wiInfo.Err != nil {
			continue
		}
		annexfiles = append(annexfiles, filepath.Clean(wiInfo.File))
	}
	filtered = filterpaths(paths, annexfiles)

	lschan := make(chan string)
	go LsFiles(paths, lschan)
	for gitfile := range lschan {
		gitfile = filepath.Clean(gitfile)
		if !stringInSlice(gitfile, annexfiles) && !stringInSlice(gitfile, filtered) {
			filtered = append(filtered, gitfile)
		}
	}

	return
}

// GetGitVersion returns the version string of the system's git binary.
func GetGitVersion() (string, error) {
	cmd := Command("version")
	stdout, stderr, err := cmd.OutputError()
	if err != nil {
		errmsg := string(stderr)
		log.Write("Error while preparing git version command")
		if strings.Contains(err.Error(), "executable file not found") {
			return "", fmt.Errorf("git executable not found: %s", err.Error())
		}
		if strings.Contains(errmsg, "no such file or directory") {
			return "", fmt.Errorf("git executable not found: %s", errmsg)
		}
		if errmsg != "" {
			return "", fmt.Errorf(errmsg)
		}
		return "", err

	}
	verstr := string(stdout)
	verstr = strings.TrimSpace(verstr)
	verstr = strings.TrimPrefix(verstr, "git version ")
	return verstr, nil
}

// Command sets up an external git command with the provided arguments and returns a GinCmd struct.
func Command(args ...string) shell.Cmd {
	config := config.Read()
	gitbin := config.Bin.Git
	cmd := shell.Command(gitbin)
	cmd.Args = append(cmd.Args, args...)
	env := os.Environ()
	cmd.Env = append(env, sshEnv())
	workingdir, _ := filepath.Abs(".")
	log.Write("Running shell command (Dir: %s): %s", workingdir, strings.Join(cmd.Args, " "))
	return cmd
}
