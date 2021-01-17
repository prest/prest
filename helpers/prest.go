package helpers

var (
	// PrestVersionNumber repesemts prest version.
	PrestVersionNumber = "1.0.5"
	// CommitHash for version
	CommitHash string
)

// PrestReleaseVersion is same as pREST Version.
func PrestReleaseVersion() string {
	return PrestVersionNumber
}
