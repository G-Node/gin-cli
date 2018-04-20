package git

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/G-Node/gin-cli/ginclient/config"
	"github.com/G-Node/gin-cli/ginclient/log"
	"github.com/G-Node/gin-cli/git/shell"
	"github.com/G-Node/gin-cli/util"
	"github.com/G-Node/gin-cli/web"
)

// Workingdir sets the directory for shell commands
var Workingdir = "."

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
	// Errors
	Err error `json:"err"`
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
		RFSAlias
		Err string `json:"err"`
	}{
		RFSAlias: RFSAlias(s),
		Err:      errmsg,
	})
}

// Git commands

// Clone downloads a repository and sets the remote fetch and push urls.
// Setting the Workingdir package global affects the working directory in which the command is executed.
// The status channel 'clonechan' is closed when this function returns.
// (git clone ...)
func Clone(remotepath string, repopath string, clonechan chan<- RepoFileStatus) {
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
				}
			}
			clonechan <- status
		}
	}

	errstring := string(stderr)
	if err = cmd.Wait(); err != nil {
		log.LogWrite("Error during clone command")
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

// Add adds paths to git directly (not annex).
// In direct mode, files that are already in the annex are explicitly ignored.
// In indirect mode, adding annexed files to git has no effect.
// Setting the Workingdir package global affects the working directory in which the command is executed.
// The status channel 'addchan' is closed when this function returns.
// (git add)
func Add(filepaths []string, addchan chan<- RepoFileStatus) {
	defer close(addchan)
	if len(filepaths) == 0 {
		log.LogWrite("No paths to add to git. Nothing to do.")
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
	cmd := Command(cmdargs...)
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
		log.LogWrite("'%s' added to git", fname)
		// Error conditions?
		status.Progress = progcomplete
		addchan <- status
	}
	var stderr, errline []byte
	if cmd.Wait() != nil {
		for rerr = nil; rerr == nil; errline, rerr = cmd.OutReader.ReadBytes('\000') {
			stderr = append(stderr, errline...)
		}
		log.LogWrite("Error during GitAdd")
		logstd(nil, stderr)
	}
	return
}

// SetGitUser sets the user.name and user.email configuration values for the local git repository.
func SetGitUser(name, email string) error {
	if !IsRepo() {
		return fmt.Errorf("not a repository")
	}
	cmd := Command("config", "--local", "user.name", name)
	err := cmd.Run()
	if err != nil {
		return err
	}
	cmd = Command("config", "--local", "user.email", email)
	return cmd.Run()
}

// AddRemote adds a remote named name for the repository at url.
func AddRemote(name, url string) error {
	fn := fmt.Sprintf("AddRemote(%s, %s)", name, url)
	cmd := Command("remote", "add", name, url)
	stdout, stderr, err := cmd.OutputError()
	if err != nil {
		gerr := giterror{UError: err.Error(), Origin: fn}
		log.LogWrite("Error during remote add command")
		logstd(stdout, stderr)
		if strings.Contains(string(stderr), "already exists") {
			gerr.Description = fmt.Sprintf("remote with name '%s' already exists", name)
			return gerr
		}
	}
	return err
}

// CommitIfNew creates an empty initial git commit if the current repository is completely new.
// If 'upstream' is not an empty string, and an initial commit was created, it sets the current branch to track the same-named branch at the specified remote.
// Returns 'true' if (and only if) a commit was created.
// Setting the Workingdir package global affects the working directory in which the command is executed.
func CommitIfNew(upstream string) (bool, error) {
	if !IsRepo() {
		return false, fmt.Errorf("not a repository")
	}
	cmd := Command("rev-parse", "HEAD")
	err := cmd.Run()
	if err == nil {
		// All good. No need to do anything
		return false, nil
	}

	// Create an empty initial commit and run annex sync to synchronise everything
	hostname, err := os.Hostname()
	if err != nil {
		hostname = unknownhostname
	}
	cmd = Command("commit", "--allow-empty", "-m", fmt.Sprintf("Initial commit: Repository initialised on %s", hostname))
	stdout, stderr, err := cmd.OutputError()
	if err != nil {
		log.LogWrite("Error while creating initial commit")
		logstd(stdout, stderr)
		return false, fmt.Errorf(string(stderr))
	}

	if upstream == "" {
		return true, nil
	}

	cmd = Command("push", "--set-upstream", upstream, "HEAD")
	stdout, stderr, err = cmd.OutputError()
	if err != nil {
		log.LogWrite("Error while creating initial commit")
		logstd(stdout, stderr)
		return false, fmt.Errorf(string(stderr))
	}
	return true, nil
}

// IsRepo checks whether the current working directory is in a git repository.
// This function will also return true for bare repositories that use git annex (direct mode).
// Setting the Workingdir package global affects the working directory in which the command is executed.
func IsRepo() bool {
	log.LogWrite("IsRepo '%s'?", Workingdir)
	_, err := FindRepoRoot(Workingdir)
	yes := err == nil
	log.LogWrite("%v", yes)
	return yes
}

// **************** //

// Commit records changes that have been added to the repository with a given message.
// Setting the Workingdir package global affects the working directory in which the command is executed.
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
		if strings.Contains(string(stdout), "nothing to commit") {
			// eat the error
			log.LogWrite("Nothing to commit")
			return nil
		}
		log.LogWrite("Error during GitCommit")
		logstd(stdout, stderr)
		return fmt.Errorf(string(stderr))
	}
	return nil
}

// LsFiles lists all files known to git.
// In direct mode, the bare flag is temporarily switched off before running the command.
// The output channel 'lschan' is closed when this function returns.
// (git ls-files)
func LsFiles(args []string, lschan chan<- string) {
	defer close(lschan)
	cmdargs := append([]string{"ls-files"}, args...)
	cmd := Command(cmdargs...)
	err := cmd.Start()
	if err != nil {
		log.LogWrite("ls-files command set up failed: %s", err)
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
		log.LogWrite("Error during GitLsFiles")
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
		log.LogWrite("Error setting up git log command")
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
			log.LogWrite("Error parsing git log")
			log.LogWrite(string(line))
			log.LogWrite(ierr.Error())
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
		log.LogWrite("Error getting git log")
		errmsg := string(stderr)
		if strings.Contains(errmsg, "bad revision") {
			errmsg = fmt.Sprintf("'%s' does not match a known version ID or name", revrange)
		}
		return nil, fmt.Errorf(errmsg)
	}

	// TODO: Combine diffstats into first git log invocation
	logstats, err := LogDiffStat(count, paths, showdeletes)
	if err != nil {
		log.LogWrite("Failed to get diff stats")
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
		log.LogWrite("Error during LogDiffstat")
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
				log.LogWrite("Could not parse diffstat line")
				log.LogWrite(line)
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
		Workingdir = reporoot
		paths = []string{"."}
	}
	cmdargs = append(cmdargs, paths...)

	cmd := Command(cmdargs...)
	stdout, stderr, err := cmd.OutputError()
	if err != nil {
		log.LogWrite("Error during GitCheckout")
		logstd(stdout, stderr)
		return fmt.Errorf(string(stderr))
	}
	return nil
}

// LsTree performs a recursive git ls-tree with a given revision (hash) and a list of paths.
// For each item, it returns a struct which contains the type (blob, tree), the mode, the hash, and the absolute (repo rooted) path to the object (name).
// Setting the Workingdir package global affects the working directory in which the command is executed.
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
		log.LogWrite("Error during GitLsTree")
		logstd(nil, stderr)
		return nil, fmt.Errorf(string(stderr))
	}

	return objects, nil
}

// CatFileContents performs a git-cat-file of a specific file from a specific commit and returns the file contents (as bytes).
// Setting the Workingdir package global affects the working directory in which the command is executed.
func CatFileContents(revision, filepath string) ([]byte, error) {
	cmd := Command("cat-file", "blob", fmt.Sprintf("%s:./%s", revision, filepath))
	stdout, stderr, err := cmd.OutputError()
	if err != nil {
		log.LogWrite("Error during GitCatFile (Contents)")
		logstd(nil, stderr)
		return nil, fmt.Errorf(string(stderr))
	}
	return stdout, nil
}

// CatFileType returns the type of a given object at a given revision (blob, tree, or commit)
// Setting the Workingdir package global affects the working directory in which the command is executed.
func CatFileType(object string) (string, error) {
	cmd := Command("cat-file", "-t", object)
	stdout, stderr, err := cmd.OutputError()
	if err != nil {
		log.LogWrite("Error during GitCatFile (Type)")
		logstd(stdout, stderr)
		err = fmt.Errorf(string(stderr))
		return "", err
	}
	return string(stdout), nil
}

// RevCount returns the number of commits between two revisions.
// Setting the Workingdir package global affects the working directory in which the command is executed.
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
// Setting the Workingdir package global affects the working directory in which the command is executed.
func IsDirect() bool {
	if mode, ok := annexmodecache[Workingdir]; ok {
		return mode
	}
	cmd := Command("config", "--local", "annex.direct")
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
	cmd := Command("config", "--local", "--get", "annex.version")
	stdout, stderr, err := cmd.OutputError()
	if err != nil {
		log.LogWrite("Error while checking repository annex version")
		logstd(stdout, stderr)
		return false
	}
	ver := strings.TrimSpace(string(stdout))
	log.LogWrite("Annex version is %s", ver)
	return ver == "6"
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
		log.LogWrite("Error switching bare status to %s", statestr)
		logstd(stdout, stderr)
		err = fmt.Errorf(string(stderr))
	}
	return err
}

// Command sets up an external git command with the provided arguments and returns a GinCmd struct.
// Setting the Workingdir package global affects the working directory in which the command will be executed.
func Command(args ...string) shell.Cmd {
	gitbin := config.Config.Bin.Git
	cmd := shell.Command(gitbin)
	cmd.Dir = Workingdir
	cmd.Args = append(cmd.Args, args...)
	token := web.UserToken{}
	_ = token.LoadToken()
	env := os.Environ()
	cmd.Env = append(env, GitSSHEnv(token.Username))
	log.LogWrite("Running shell command (Dir: %s): %s", Workingdir, strings.Join(cmd.Args, " "))
	return cmd
}
