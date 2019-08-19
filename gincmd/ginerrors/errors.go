package ginerrors

const (
	// NotInRepo is returned when a command needs to be run from within a repository
	NotInRepo = "this command must be run from inside a gin repository"

	// RequiresLogin is returned when a command requires the user to be logged in
	RequiresLogin = "this operation requires login"

	// BadPort is returned when a port number is outside the valid range (1..65535)
	BadPort = "port must be between 0 and 65535 (inclusive)"

	// MissingURLScheme is returned when a string is assumed to be a URL but does not contain a scheme (missing ://)
	MissingURLScheme = "could not determine protocol scheme (no ://)"

	// MissingGitUser is returned when a string is assumed to be a git configuration but does not contain a user (missing @)
	MissingGitUser = "could not determine git username (no @)"

	// MissingAnnex is returned when a repository doesn't have annex initialised (can also be used as a warning)
	MissingAnnex = "no annex information found: run 'gin init' to initialise annex"
)
