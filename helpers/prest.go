package helpers

import "runtime/debug"

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
	// It should match the version in go.mod when building from source.
	PrestVersionNumber = "devel"
	// CommitHash is kept for compatibility; prefer Commit.
	CommitHash string

	// readBuildInfo is a test seam for reading build info from the Go runtime.
	readBuildInfo = debug.ReadBuildInfo
)

// PrestReleaseVersion returns, in order: the ldflags Version, the module
// version recorded by `go install …@vX`, or "devel".
func PrestReleaseVersion() string {
	if Version != "" {
		return Version
	}
	if v := buildInfoVersion(); v != "" {
		return v
	}
	return PrestVersionNumber
}

// buildInfoVersion returns the version from runtime build info,
// or "" if it is not available, empty, or marked as development.
func buildInfoVersion() string {
	bi, ok := readBuildInfo()
	if !ok || bi.Main.Version == "" || bi.Main.Version == "(devel)" {
		return ""
	}
	return bi.Main.Version
}

// buildSetting returns the value of a build setting from runtime build info,
// or "" if not found.
func buildSetting(key string) string {
	bi, ok := readBuildInfo()
	if !ok {
		return ""
	}
	for _, s := range bi.Settings {
		if s.Key == key {
			return s.Value
		}
	}
	return ""
}

// PrestCommit returns the effective commit hash: the ldflags Commit, then the
// explicit CommitHash, then the vcs.revision build setting.
func PrestCommit() string {
	if Commit != "" {
		return Commit
	}
	if CommitHash != "" {
		return CommitHash
	}
	return buildSetting("vcs.revision")
}

// PrestBuildDate returns the ldflags build date; empty for builds without
// -ldflags (vcs.time is the commit time, not the build time).
func PrestBuildDate() string {
	return Date
}
