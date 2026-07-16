package helpers

// Build metadata. Release builds set these via -ldflags:
//
//	-X github.com/prest/prest/v2/helpers.Version=...
//	-X github.com/prest/prest/v2/helpers.Commit=...
//	-X github.com/prest/prest/v2/helpers.Date=...
var (
	// Version is the release version injected at link time.
	Version string
	// Commit is the git commit SHA injected at link time.
	Commit string
	// Date is the build date injected at link time.
	Date string
	// BuiltBy identifies the build system (e.g. goreleaser).
	BuiltBy string

	// PrestVersionNumber is the fallback version when Version is unset.
	PrestVersionNumber = "2.0.0"
	// CommitHash is kept for compatibility; prefer Commit.
	CommitHash string
)

// PrestReleaseVersion returns the effective pREST version string.
func PrestReleaseVersion() string {
	if Version != "" {
		return Version
	}
	return PrestVersionNumber
}

// PrestCommit returns the effective commit hash.
func PrestCommit() string {
	if Commit != "" {
		return Commit
	}
	return CommitHash
}

// PrestBuildDate returns the effective build date.
func PrestBuildDate() string {
	return Date
}
