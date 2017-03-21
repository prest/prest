package helpers

import "fmt"

const (
	// Major and minor version.
	PrestVersionNumber = 0.1

	// Increment this for bug releases
	PrestPatchVersion = 6
)

var CommitHash string

// PrestReleaseVersion is same as pREST Version.
func PrestReleaseVersion() string {
	return fmt.Sprintf("%.2g.%d", PrestVersionNumber, PrestPatchVersion)
}
