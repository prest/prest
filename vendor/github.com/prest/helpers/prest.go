package helpers

import (
	"fmt"
)

const (
	// PrestVersionNumber Major and minor version.
	PrestVersionNumber = 0.3

	// PrestPatchVersion Increment this for bug releases
	PrestPatchVersion = 0
)

var (
	// CommitHash for version
	CommitHash string
)

// PrestReleaseVersion is same as pREST Version.
func PrestReleaseVersion() string {
	return fmt.Sprintf("%.2g.%d", PrestVersionNumber, PrestPatchVersion)
}
