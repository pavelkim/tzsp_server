package version

// Version is the current application version
// This value is injected at build time using -ldflags
var Version = "dev"

// GetVersion returns the current application version
func GetVersion() string {
	return Version
}
