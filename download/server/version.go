package server

// GetVersion returns the current version of the application.
// This should be set at build time via ldflags.
func GetVersion() string {
	// Import main package to access Version variable
	// We'll need to pass this in from the main package
	// For now, return a placeholder that will be set by the server
	return ""
}
