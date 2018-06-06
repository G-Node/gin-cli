package ginerrors

const (
	// NotInRepo is returned when a command needs to be run from within a repository
	NotInRepo = "this command must be run from inside a gin repository"

	// RequiresLogin is returned when a command requires the user to be logged in
	RequiresLogin = "this operation requires login"

	// BadPort is returned when a port number is outside the valid range (1..65535)
	BadPort = "port must be between 0 and 65535 (inclusive)"
)
