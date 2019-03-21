package git

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/G-Node/gin-cli/ginclient/config"
	"github.com/G-Node/gin-cli/ginclient/log"
	"github.com/G-Node/gin-cli/git/shell"
)

// Git annex commands

// Determine if json or normal command is used
var JsonBool bool = true

// Types (private)
type annexAction struct {
	Command string   `json:"command"`
	Note    string   `json:"note"`
	Success bool     `json:"success"`
	Key     string   `json:"key"`
	File    string   `json:"file"`
	Errors  []string `json:"error-messages"`
}

type annexProgress struct {
	Action          annexAction `json:"action"`
	ByteProgress    int         `json:"byte-progress"`
	TotalSize       int         `json:"total-size"`
	PercentProgress string      `json:"percent-progress"`
}

type annexMetadata struct {
	annexAction
	Fields struct {
		Lastchanged    []string
		Ginfilename    []string
		GinefilenameLC []string `json:"ginfilename-lastchanged"`
	}
}

type annexFilenameDate struct {
	Key      string
	FileName string
	ModTime  time.Time
}

// Types (public)

// AnnexFindRes holds the result of a git annex find invocation for one file.
type AnnexFindRes struct {
	Hashdirlower string
	Hashdirmixed string
	Key          string
	Humansize    string
	Backend      string
	File         string
	Keyname      string
	Bytesize     string
	Mtime        string
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

// AnnexStatusRes for getting the (annex) status of individual files
type AnnexStatusRes struct {
	Status string `json:"status"`
	File   string `json:"file"`
	Err    error  `json:"err"`
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

// AnnexInit initialises the repository for annex.
// (git annex init)
func AnnexInit(description string) error {
	err := ConfigSet("annex.backends", "MD5")
	if err != nil {
		log.Write("Failed to set default annex backend MD5")
	}
	err = ConfigSet("annex.addunlocked", "true")
	if err != nil {
		log.Write("Failed to initialise annex in unlocked mode")
		return err
	}
	args := []string{"init", "--version=7", description}
	cmd := AnnexCommand(args...)
	stdout, stderr, err := cmd.OutputError()
	if err != nil {
		initError := fmt.Errorf("Repository annex initialisation failed.\n%s", string(stderr))
		logstd(stdout, stderr)
		return initError
	}

	cmd = Command("checkout", "master")
	stdout, stderr, err = cmd.OutputError()
	if err != nil {
		logstd(stdout, stderr)
	}

	return nil
}

// AnnexPull downloads all annexed files. Optionally also downloads all file content.
// (git annex sync --no-push [--content])
func AnnexPull(remote string) error {
	args := []string{"sync", "--verbose", "--no-push", "--no-commit", remote}
	cmd := AnnexCommand(args...)
	stdout, stderr, err := cmd.OutputError()
	cmd.Wait()
	sstdout := string(stdout)
	sstderr := string(stderr)
	if err != nil {
		log.Write("Error during AnnexPull.")
		log.Write("[Error]: %v", err)
		logstd(stdout, stderr)
		mergeAbort() // abort a potential failed merge attempt
		// TODO: Use giterror
		if strings.Contains(sstderr, "Permission denied") {
			return fmt.Errorf("download failed: permission denied")
		} else if strings.Contains(sstderr, "Host key verification failed") {
			// Bad host key configured
			return fmt.Errorf("download failed: server key does not match known host key")
		} else {
			err = checkMergeErrors(sstdout, sstderr)
			if err == nil {
				err = fmt.Errorf("failed")
			}
			return err
		}
	}

	// some conflicts are resolved automatically and don't produce an error in some combinations
	return checkMergeErrors(sstdout, sstderr)
}

func checkMergeErrors(stdout, stderr string) error {
	messages := strings.ToLower(stdout + stderr)
	if strings.Contains(messages, "would be overwritten by merge") {
		// Untracked local file conflicts with file being pulled
		return fmt.Errorf("download failed: local modified or untracked files would be overwritten by download:\n  %s", strings.Join(parseFilesOverwrite(messages), ", "))
	} else if strings.Contains(messages, "unresolved conflict") {
		// Merge conflict in git files
		return fmt.Errorf("download failed: files changed locally and remotely and cannot be automatically merged (merge conflict):\n %s", strings.Join(parseFilesConflict(messages), ", "))
		// abort merge
	} else if strings.Contains(messages, "merge conflict was automatically resolved") {
		// Merge conflict in annex files (automatically resolved by keeping both copies)
		return fmt.Errorf("files changed locally and remotely. Both files have been kept:\n %s", strings.Join(parseFilesAnnexConflict(stdout), ", "))
		// TODO: This should probably instead become a warning or notice, instead of a full error
	}
	return nil
}

// AnnexSync performs a bidirectional synchronisation between local and remote
// repositories, automatically resolving merge conflicts.
// (git annex sync --resolvemerge)
func AnnexSync(content bool) error {
	cmdargs := []string{"sync", "--verbose", "--resolvemerge"}
	if content {
		cmdargs = append(cmdargs, "--content")
	}
	cmd := AnnexCommand(cmdargs...)
	stdout, stderr, err := cmd.OutputError()
	cmd.Wait()
	if err != nil {
		log.Write("Error during AnnexSync.")
		log.Write("[Error]: %v", err)
		logstd(stdout, stderr)
		return fmt.Errorf(string(stderr))
	}
	return nil
}

func parseFilesConflict(errmsg string) []string {
	lines := strings.Split(errmsg, "\n")
	var filenames []string
	delim := "merge conflict in "
	for _, l := range lines {
		if idx := strings.Index(l, delim); idx > -1 {
			filenames = append(filenames, l[idx+len(delim):])
		}
	}
	return filenames
}

func parseFilesAnnexConflict(errmsg string) []string {
	lines := strings.Split(errmsg, "\n")
	var filenames []string
	delim := ": needs merge"
	for _, l := range lines {
		if idx := strings.Index(l, delim); idx > -1 {
			filenames = append(filenames, l[0:idx])
		}
	}
	return filenames
}

func parseFilesOverwrite(errmsg string) []string {
	lines := strings.Split(errmsg, "\n")
	var filenames []string
	start := false
	for _, l := range lines {
		if strings.Contains(l, "error: the following") || strings.Contains(l, "error: your local") {
			start = true
			continue
		}
		if strings.Contains(l, "please move or remove") || strings.Contains(l, "please commit your changes") {
			break
		}
		if start {
			filenames = append(filenames, strings.TrimSpace(l))
		}
	}
	return filenames

}

// AnnexPush uploads all changes and new content to the default remote.
// The status channel 'pushchan' is closed when this function returns.
// (git annex sync --no-pull; git annex copy --to=<defaultremote>)
func AnnexPush(paths []string, remote string, pushchan chan<- RepoFileStatus) {
	defer close(pushchan)
	cmd := AnnexCommand("sync", "--verbose", "--no-pull", "--no-commit", remote) // NEVER commit changes when doing annex-sync
	stdout, stderr, err := cmd.OutputError()
	if err != nil {
		log.Write("Error during AnnexPush (sync --no-pull)")
		log.Write("[Error]: %v", err)
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

	// check which files are annexed
	wichan := make(chan AnnexWhereisRes)
	go AnnexWhereis(paths, wichan)

	// collect annex paths for annex copy command
	annexpaths := make([]string, 0, len(paths))
	for info := range wichan {
		annexpaths = append(annexpaths, info.File)
	}

	if len(annexpaths) == 0 {
		return
	}

	var args []string
	if JsonBool {
		args = []string{"copy", "--json-progress", fmt.Sprintf("--to=%s", remote)}
	} else {
		args = []string{"copy", "--verbose", fmt.Sprintf("--to=%s", remote)}
	}
	if len(paths) == 0 {
		paths = []string{"--all"}
	}
	args = append(args, paths...)

	cmd = AnnexCommand(args...)
	err = cmd.Start()
	if err != nil {
		pushchan <- RepoFileStatus{Err: err}
		return
	}

	var status RepoFileStatus
	status.State = fmt.Sprintf("Uploading (to: %s)", remote)

	var outline []byte
	var rerr error
	var progress annexProgress
	var getresult annexAction

	var prevByteProgress int
	var prevT time.Time

	// 'git-annex copy --all' copies all local keys to the server.
	// When no filenames are specified, the command doesn't print filenames, just keys.
	// getAnnexMetadataName gives us the original filename and the time it was set.
	var currentkey = ""
	for rerr = nil; rerr == nil; outline, rerr = cmd.OutReader.ReadBytes('\n') {
		if len(outline) == 0 {
			// skip empty lines
			continue
		}
		if !JsonBool {
			status.RawOutput = string(outline)
			lineInput := cmd.Args
			input := strings.Join(lineInput, " ")
			status.RawInput = input
			pushchan <- status
			continue
		}
		err := json.Unmarshal(outline, &progress)
		if err != nil || progress.Action.Command == "" {
			time.Sleep(1 * time.Second)
			// File done? Check if succeeded and continue to next line
			err = json.Unmarshal(outline, &getresult)
			if err != nil || getresult.Command == "" {
				// Couldn't parse output
				log.Write("Could not parse 'git annex copy' output")
				log.Write(string(outline))
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
			key := progress.Action.Key
			if currentkey != key {
				if md := getAnnexMetadataName(key); md.FileName != "" {
					timestamp := md.ModTime.Format("2006-01-02 15:04:05")
					status.FileName = fmt.Sprintf("%s (version: %s)", md.FileName, timestamp)
				} else {
					status.FileName = "(unknown)"
				}
				currentkey = key
			}
			// otherwise the same name as before is used
			status.Progress = progress.PercentProgress

			dbytes := progress.ByteProgress - prevByteProgress
			now := time.Now()
			dt := now.Sub(prevT)
			status.Rate = calcRate(dbytes, dt)
			prevByteProgress = progress.ByteProgress
			prevT = now
			status.Err = nil
		}

		// Don't push message if no filename was set
		if status.FileName != "" {
			pushchan <- status
		}
	}
	if cmd.Wait() != nil {
		var stderr, errline []byte
		for rerr = nil; rerr == nil; errline, rerr = cmd.OutReader.ReadBytes('\000') {
			stderr = append(stderr, errline...)
		}
		log.Write("Error during AnnexPush")
		log.Write(string(stderr))
		pushchan <- RepoFileStatus{Err: fmt.Errorf(string(stderr))}
	}
	return
}

func baseAnnexGet(cmdargs []string, getchan chan<- RepoFileStatus) {
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

		if !JsonBool {
			lineInput := cmd.Args
			input := strings.Join(lineInput, " ")
			status.RawInput = input
			status.RawOutput = string(outline)
			getchan <- status
			continue
		}
		err := json.Unmarshal(outline, &progress)
		if err != nil || progress.Action.Command == "" {
			// File done? Check if succeeded and continue to next line
			err = json.Unmarshal(outline, &getresult)
			if err != nil || getresult.Command == "" {
				// Couldn't parse output
				log.Write("Could not parse 'git annex get' output")
				log.Write(string(outline))
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
		log.Write("Error during AnnexGet")
		log.Write(string(stderr))
	}
	return
}

// AnnexGet retrieves the content of specified files.
// The status channel 'getchan' is closed when this function returns.
// (git annex get)
func AnnexGet(filepaths []string, getchan chan<- RepoFileStatus) {
	defer close(getchan)
	var cmdargs []string
	if JsonBool {
		cmdargs = append([]string{"get", "--json-progress"}, filepaths...)
	} else {
		cmdargs = append([]string{"get"}, filepaths...)
	}
	baseAnnexGet(cmdargs, getchan)
}

// AnnexGetKey retrieves the content of a single specified key.
// The status channel 'getchan' is closed when this function returns.
// (git annex get)
func AnnexGetKey(key string, getchan chan<- RepoFileStatus) {
	defer close(getchan)
	cmdargs := []string{"get", "--json-progress", fmt.Sprintf("--key=%s", key)}
	baseAnnexGet(cmdargs, getchan)
	return
}

// AnnexDrop drops the content of specified files.
// The status channel 'dropchan' is closed when this function returns.
// (git annex drop)
func AnnexDrop(filepaths []string, dropchan chan<- RepoFileStatus) {
	defer close(dropchan)
	var cmdargs []string
	if JsonBool {
		cmdargs = append([]string{"drop", "--json"}, filepaths...)
	} else {
		cmdargs = append([]string{"drop"}, filepaths...)
	}
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

		if !JsonBool {
			lineInput := cmd.Args
			input := strings.Join(lineInput, " ")
			status.RawInput = input
			status.RawOutput = line
			dropchan <- status
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
			log.Write("%s content dropped", annexDropRes.File)
			status.Err = nil
		} else {
			log.Write("Error dropping %s", annexDropRes.File)
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
		log.Write("Error during AnnexDrop")
		log.Write("[stderr]\n%s", string(stderr))
	}
	return
}

// getAnnexMetadataName returns the filename, key, and last modification time stored in the metadata of an annexed file given the key.
// If an unused key does not have a name associated with it, the filename will be empty.
func getAnnexMetadataName(key string) annexFilenameDate {
	var cmdargs []string
	cmdargs = []string{"metadata", "--json", fmt.Sprintf("--key=%s", key)}
	cmd := AnnexCommand(cmdargs...)
	stdout, stderr, err := cmd.OutputError()
	if err != nil {
		log.Write("Error retrieving annexed content metadata")
		logstd(stdout, stderr)
		return annexFilenameDate{}
	}
	var annexmd annexMetadata
	json.Unmarshal(bytes.TrimSpace(stdout), &annexmd)
	if len(annexmd.Fields.Ginfilename) > 0 {
		name := annexmd.Fields.Ginfilename[0]
		modtime, _ := time.Parse("2006-01-02@15-04-05", annexmd.Fields.GinefilenameLC[0])
		return annexFilenameDate{Key: key, FileName: name, ModTime: modtime}
	}
	return annexFilenameDate{Key: key, FileName: annexmd.File}
}

// AnnexWhereis returns information about annexed files in the repository
// The output channel 'wichan' is closed when this function returns.
// (git annex whereis)
func AnnexWhereis(paths []string, wichan chan<- AnnexWhereisRes) {
	defer close(wichan)
	cmdargs := []string{"whereis", "--json"}
	cmdargs = append(cmdargs, paths...)
	cmd := AnnexCommand(cmdargs...)
	err := cmd.Start()
	if err != nil {
		log.Write("Error during AnnexWhereis")
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

// AnnexStatus returns the status of a file or files in a directory
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
		log.Write("Error setting up git-annex status")
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

// AnnexDescribe changes the description of a repository.
// (git annex describe)
func AnnexDescribe(repository, description string) error {
	fn := fmt.Sprintf("AnnexDescribe(%s, %s)", repository, description)
	cmd := AnnexCommand("describe", repository, description)
	stdout, stderr, err := cmd.OutputError()
	if err != nil {
		log.Write("Error during Describe")
		logstd(stdout, stderr)
		return giterror{Origin: fn, UError: string(stderr)}
	}
	return nil
}

// AnnexInfo returns the annex information for a given repository
// (git annex info)
func AnnexInfo() (AnnexInfoRes, error) {
	cmd := AnnexCommand("info", "--json")
	stdout, stderr, err := cmd.OutputError()
	if err != nil {
		log.Write("Error during AnnexInfo")
		logstd(stdout, stderr)
		return AnnexInfoRes{}, fmt.Errorf("Error retrieving annex info")
	}

	var info AnnexInfoRes
	err = json.Unmarshal(stdout, &info)
	return info, err
}

// AnnexLock locks the specified files and directory contents if they are annexed.
// If an unlocked file has modifications, it wont be locked and an error will be returned for that file.
// The status channel 'lockchan' is closed when this function returns.
// (git annex lock)
func AnnexLock(filepaths []string, lockchan chan<- RepoFileStatus) {
	defer close(lockchan)
	if len(filepaths) == 0 {
		log.Write("No paths to lock. Nothing to do.")
		return
	}
	var cmdargs []string
	if JsonBool {
		cmdargs = []string{"lock", "--json-error-messages"}
	} else {
		cmdargs = []string{"lock"}
	}

	cmdargs = append(cmdargs, filepaths...)
	cmd := AnnexCommand(cmdargs...)
	err := cmd.Start()
	if err != nil {
		lockchan <- RepoFileStatus{Err: err}
		return
	}

	var outline []byte
	var rerr error
	var status RepoFileStatus
	var addresult annexAction
	status.State = "Locking"

	var filenames []string
	for rerr = nil; rerr == nil; outline, rerr = cmd.OutReader.ReadBytes('\n') {
		if len(outline) == 0 {
			// Empty line output. Ignore
			continue
		}
		if !JsonBool {
			status.RawOutput = string(outline)
			lineInput := cmd.Args
			input := strings.Join(lineInput, " ")
			status.RawInput = input
			lockchan <- status
			continue
		}
		err := json.Unmarshal(outline, &addresult)
		if err != nil || addresult.Command == "" {
			// Couldn't parse output
			log.Write("Could not parse 'git annex lock' output")
			log.Write(string(outline))
			// TODO: Print error at the end: Command succeeded but there was an error understanding the output
			continue
		}
		status.FileName = addresult.File
		lineInput := cmd.Args
		input := strings.Join(lineInput, " ")
		status.RawInput = input
		status.RawOutput = string(outline)
		if addresult.Success {
			log.Write("%s locked", addresult.File)
			status.Err = nil
			filenames = append(filenames, status.FileName)
		} else {
			errmsgs := strings.Join(addresult.Errors, " | ")
			log.Write("Error locking %s: %s", addresult.File, errmsgs)
			if strings.Contains(errmsgs, "Locking this file would discard any changes") {
				errmsgs = "Locking this file would discard any changes you have made to it. Commit (with 'gin commit') or discard any changes before locking."
			}
			status.Err = fmt.Errorf(errmsgs)
		}
		status.Progress = progcomplete
		lockchan <- status
	}
	var stderr, errline []byte
	if cmd.Wait() != nil {
		for rerr = nil; rerr == nil; errline, rerr = cmd.OutReader.ReadBytes('\000') {
			stderr = append(stderr, errline...)
		}
		log.Write("Error during lock")
		logstd(nil, stderr)
	}
	// Add metadata
	for _, fname := range filenames {
		setAnnexMetadataName(fname)
	}
	return
}

// AnnexUnlock unlocks the specified files and directory contents if they are annexed
// The status channel 'unlockchan' is closed when this function returns.
// (git annex unlock)
func AnnexUnlock(filepaths []string, unlockchan chan<- RepoFileStatus) {
	defer close(unlockchan)
	var cmdargs []string
	if JsonBool {
		cmdargs = []string{"unlock", "--json"}
	} else {
		cmdargs = []string{"unlock"}
	}
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
		if !JsonBool {
			lineInput := cmd.Args
			input := strings.Join(lineInput, " ")
			status.RawInput = input
			status.RawOutput = string(outline)
			unlockchan <- status
			continue
		}
		// Send file name
		err = json.Unmarshal(outline, &unlockres)
		if err != nil || unlockres.Command == "" {
			// Couldn't parse output
			log.Write("Could not parse 'git annex unlock' output")
			log.Write(string(outline))
			// TODO: Print error at the end: Command succeeded but there was an error understanding the output
			continue
		}
		status.FileName = unlockres.File
		if unlockres.Success {
			log.Write("%s unlocked", unlockres.File)
			status.Err = nil
		} else {
			log.Write("Error unlocking %s", unlockres.File)
			status.Err = fmt.Errorf("Content not available locally. Use 'gin get-content' to download")
		}
		status.Progress = progcomplete
		unlockchan <- status
	}
	var stderr, errline []byte
	if cmd.Wait() != nil {
		for rerr = nil; rerr == nil; errline, rerr = cmd.OutReader.ReadBytes('\000') {
			stderr = append(stderr, errline...)
		}
		log.Write("Error during AnnexUnlock")
		logstd(nil, stderr)
	}
	status.Progress = progcomplete
	return
}

// AnnexFind lists available annexed files in the current directory.
// Specifying 'paths' limits the search to files matching a given path.
// Returned items are indexed by their annex key.
// (git annex find)
func AnnexFind(paths []string) (map[string]AnnexFindRes, error) {
	cmdargs := []string{"find", "--json"}
	if len(paths) > 0 {
		cmdargs = append(cmdargs, paths...)
	}
	cmd := AnnexCommand(cmdargs...)
	stdout, stderr, err := cmd.OutputError()
	if err != nil {
		logstd(stdout, stderr)
		return nil, fmt.Errorf(string(stderr))
	}

	outlines := bytes.Split(stdout, []byte("\n"))
	items := make(map[string]AnnexFindRes, len(outlines))
	for _, line := range outlines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			// Empty line output. Ignore
			continue
		}
		var afr AnnexFindRes
		json.Unmarshal(line, &afr)
		items[afr.Key] = afr
	}
	return items, nil
}

// AnnexFromKey creates an Annex placeholder file at a given location with a specific key.
// The creation is forced, so there is no guarantee that the key refers to valid repository content, nor that the content is still available in any of the remotes.
// The location where the file is to be created must be available (no directories are created).
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

// AnnexContentLocation returns the location of the content for a given annex
// key. This is the location of the content file in the object store. If the
// annexed content is not available locally, the function returns an error.
func AnnexContentLocation(key string) (string, error) {
	cmd := AnnexCommand("contentlocation", key)
	stdout, stderr, err := cmd.OutputError()
	if err != nil {
		logstd(stdout, stderr)
		errmsg := "content not available locally"
		if len(stderr) > 0 {
			errmsg = string(stderr)
		}
		return "", fmt.Errorf(errmsg)
	}
	sstdout := string(stdout)
	sstdout = strings.TrimSpace(sstdout)
	return sstdout, nil
}

// AnnexFsck runs fsck (filesystem check) on the specified files, fixing any
// issues with the annexed files in the working tree.
func AnnexFsck(paths []string) error {
	cmdargs := []string{"fsck"}
	cmdargs = append(cmdargs, paths...)
	cmd := AnnexCommand(cmdargs...)
	stdout, stderr, err := cmd.OutputError()
	if err != nil {
		logstd(stdout, stderr)
		return fmt.Errorf("error fixing working tree files: %s", string(stderr))
	}
	return nil
}

// build exclusion argument list
// files < annex.minsize or matching exclusion extensions will not be annexed and
// will instead be handled by git
func annexExclArgs() string {
	var expbuilder strings.Builder
	config := config.Read()
	if config.Annex.MinSize != "" {
		largerthan := fmt.Sprintf("(largerthan=%s)", config.Annex.MinSize)
		expbuilder.WriteString(largerthan)
	}

	for _, pattern := range config.Annex.Exclude {
		exclarg := fmt.Sprintf(" and (exclude=%s)", pattern)
		expbuilder.WriteString(exclarg)
	}

	// explicitly exclude config file
	expbuilder.WriteString(fmt.Sprintf(" and (exclude=config.yml)"))
	return fmt.Sprintf("annex.largefiles=(%s)", expbuilder.String())
}

// AnnexAdd adds paths to the annex.
// Files specified for exclusion in the configuration are ignored automatically.
// The status channel 'addchan' is closed when this function returns.
// (git annex add)
func AnnexAdd(filepaths []string, addchan chan<- RepoFileStatus) {
	defer close(addchan)
	if len(filepaths) == 0 {
		log.Write("No paths to add to annex. Nothing to do.")
		return
	}
	var cmdargs []string
	if JsonBool {
		cmdargs = []string{"add", "--json"}
	} else {
		cmdargs = []string{"add"}
	}
	exclargs := annexExclArgs()
	if len(exclargs) > 0 {
		cmdargs = append(cmdargs, "-c", exclargs)
	}

	cmdargs = append(cmdargs, filepaths...)
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
	// NOTE Can differentiate "git" and "annex" files from note in JSON struct
	status.State = "Adding"
	var filenames []string
	for rerr = nil; rerr == nil; outline, rerr = cmd.OutReader.ReadBytes('\n') {
		if len(outline) == 0 {
			// Empty line output. Ignore
			continue
		}
		if !JsonBool {
			status.RawOutput = string(outline)
			lineInput := cmd.Args
			input := strings.Join(lineInput, " ")
			status.RawInput = input
			addchan <- status
			continue
		}
		err := json.Unmarshal(outline, &addresult)
		if err != nil || addresult.Command == "" {
			// Couldn't parse output
			log.Write("Could not parse 'git annex add' output")
			log.Write(string(outline))
			// TODO: Print error at the end: Command succeeded but there was an error understanding the output
			continue
		}
		status.FileName = addresult.File
		lineInput := cmd.Args
		input := strings.Join(lineInput, " ")
		status.RawInput = input
		status.RawOutput = string(outline)
		if addresult.Success {
			log.Write("%s added to annex", addresult.File)
			status.Err = nil
			filenames = append(filenames, status.FileName)
		} else {
			log.Write("Error adding %s", addresult.File)
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
		log.Write("Error during AnnexAdd")
		logstd(nil, stderr)
	}
	// Add metadata
	for _, fname := range filenames {
		setAnnexMetadataName(fname)
	}
	return
}

// setAnnexMetadataName starts a routine and waits for input on the provided channel.
// For each path specified, the name of the file is added to the metadata of the annexed file.
// The function exits when the channel is closed.
func setAnnexMetadataName(path string) {
	_, fname := filepath.Split(path)
	cmd := AnnexCommand("metadata", fmt.Sprintf("--set=ginfilename=%s", fname), path)
	stdout, stderr, err := cmd.OutputError()
	if err != nil {
		logstd(stdout, stderr)
	} else {
		log.Write("ginfilename metadata key set to %s", fname)
	}
	return
}

// GetAnnexVersion returns the version string of the system's git-annex.
func GetAnnexVersion() (string, error) {
	cmd := AnnexCommand("version", "--raw")
	stdout, stderr, err := cmd.OutputError()
	if err != nil {
		errmsg := string(stderr)
		log.Write("Error while preparing git-annex version command")
		if strings.Contains(err.Error(), "executable file not found") {
			return "", fmt.Errorf("git-annex executable not found: %s", err.Error())
		}
		if strings.Contains(errmsg, "no such file or directory") {
			return "", fmt.Errorf("git-annex executable not found: %s", errmsg)
		}
		if errmsg != "" {
			return "", fmt.Errorf(errmsg)
		}
		return "", err

	}
	return string(stdout), nil
}

// AnnexCommand sets up a git annex command with the provided arguments and returns a GinCmd struct.
func AnnexCommand(args ...string) shell.Cmd {
	config := config.Read()
	// gitannexbin := config.Bin.GitAnnex
	gitbin := config.Bin.Git
	gitannexpath := config.Bin.GitAnnexPath
	cmdargs := []string{"annex"}
	cmdargs = append(cmdargs, args...)
	cmd := shell.Command(gitbin, cmdargs...)
	env := os.Environ()
	cmd.Env = env
	if gitannexpath != "" {
		syspath := os.Getenv("PATH")
		syspath += string(os.PathListSeparator) + gitannexpath
		cmd.Env = append(cmd.Env, syspath)
	}
	cmd.Env = append(cmd.Env, sshEnv())
	cmd.Env = append(cmd.Env, "GIT_ANNEX_USE_GIT_SSH=1")
	workingdir, _ := filepath.Abs(".")
	log.Write("Running shell command (Dir: %s): %s", workingdir, strings.Join(cmd.Args, " "))
	return cmd
}
