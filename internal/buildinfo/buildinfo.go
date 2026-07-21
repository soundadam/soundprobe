package buildinfo

// These values are overridden with -ldflags in release builds.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)
