package ginerrors

// NotInRepo is returned when a command needs to be run from within a repository
const NotInRepo = "this command must be run from inside a gin repository"

// RequiresLogin is returned when a command requires the user to be logged in
const RequiresLogin = "this operation requires login"
