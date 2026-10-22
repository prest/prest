package helpers

import (
	"runtime/debug"
	"testing"
)

func TestPrestReleaseVersion(t *testing.T) {
	// Save original values to restore in cleanup.
	originalVersion := Version
	originalPrestVersionNumber := PrestVersionNumber
	originalReadBuildInfo := readBuildInfo

	t.Cleanup(func() {
		Version = originalVersion
		PrestVersionNumber = originalPrestVersionNumber
		readBuildInfo = originalReadBuildInfo
	})

	tests := []struct {
		name        string
		version     string
		buildInfo   *debug.BuildInfo
		buildInfoOk bool
		expected    string
	}{
		{
			name:        "ldflags Version wins",
			version:     "v2.5.0",
			buildInfo:   &debug.BuildInfo{Main: debug.Module{Version: "v2.5.1"}},
			buildInfoOk: true,
			expected:    "v2.5.0",
		},
		{
			name:        "build info v2.5.1 returned when Version is empty",
			version:     "",
			buildInfo:   &debug.BuildInfo{Main: debug.Module{Version: "v2.5.1"}},
			buildInfoOk: true,
			expected:    "v2.5.1",
		},
		{
			name:        "devel returned when build info is (devel)",
			version:     "",
			buildInfo:   &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}},
			buildInfoOk: true,
			expected:    "devel",
		},
		{
			name:        "devel returned when ok=false",
			version:     "",
			buildInfo:   nil,
			buildInfoOk: false,
			expected:    "devel",
		},
		{
			name:        "devel returned when build info Main.Version is empty",
			version:     "",
			buildInfo:   &debug.BuildInfo{Main: debug.Module{Version: ""}},
			buildInfoOk: true,
			expected:    "devel",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up test state
			Version = tt.version
			PrestVersionNumber = "devel"

			if tt.buildInfoOk {
				readBuildInfo = func() (*debug.BuildInfo, bool) {
					return tt.buildInfo, true
				}
			} else {
				readBuildInfo = func() (*debug.BuildInfo, bool) {
					return nil, false
				}
			}

			result := PrestReleaseVersion()
			if result != tt.expected {
				t.Errorf("PrestReleaseVersion() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestPrestCommit(t *testing.T) {
	// Save original values to restore in cleanup.
	originalCommit := Commit
	originalCommitHash := CommitHash
	originalReadBuildInfo := readBuildInfo

	t.Cleanup(func() {
		Commit = originalCommit
		CommitHash = originalCommitHash
		readBuildInfo = originalReadBuildInfo
	})

	tests := []struct {
		name        string
		commit      string
		commitHash  string
		buildInfo   *debug.BuildInfo
		buildInfoOk bool
		expected    string
	}{
		{
			name:       "ldflags Commit wins",
			commit:     "abc123",
			commitHash: "def456",
			buildInfo: &debug.BuildInfo{
				Settings: []debug.BuildSetting{
					{Key: "vcs.revision", Value: "xyz789"},
				},
			},
			buildInfoOk: true,
			expected:    "abc123",
		},
		{
			name:       "CommitHash wins over vcs.revision",
			commit:     "",
			commitHash: "def456",
			buildInfo: &debug.BuildInfo{
				Settings: []debug.BuildSetting{
					{Key: "vcs.revision", Value: "xyz789"},
				},
			},
			buildInfoOk: true,
			expected:    "def456",
		},
		{
			name:       "falls back to vcs.revision when CommitHash unset",
			commit:     "",
			commitHash: "",
			buildInfo: &debug.BuildInfo{
				Settings: []debug.BuildSetting{
					{Key: "vcs.revision", Value: "xyz789"},
				},
			},
			buildInfoOk: true,
			expected:    "xyz789",
		},
		{
			name:        "falls back to CommitHash when build info unavailable",
			commit:      "",
			commitHash:  "def456",
			buildInfo:   nil,
			buildInfoOk: false,
			expected:    "def456",
		},
		{
			name:       "returns empty string when all sources empty",
			commit:     "",
			commitHash: "",
			buildInfo: &debug.BuildInfo{
				Settings: []debug.BuildSetting{},
			},
			buildInfoOk: true,
			expected:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up test state
			Commit = tt.commit
			CommitHash = tt.commitHash

			if tt.buildInfoOk {
				readBuildInfo = func() (*debug.BuildInfo, bool) {
					return tt.buildInfo, true
				}
			} else {
				readBuildInfo = func() (*debug.BuildInfo, bool) {
					return nil, false
				}
			}

			result := PrestCommit()
			if result != tt.expected {
				t.Errorf("PrestCommit() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestPrestBuildDate(t *testing.T) {
	// Save original values to restore in cleanup.
	originalDate := Date
	originalReadBuildInfo := readBuildInfo

	t.Cleanup(func() {
		Date = originalDate
		readBuildInfo = originalReadBuildInfo
	})

	tests := []struct {
		name        string
		date        string
		buildInfo   *debug.BuildInfo
		buildInfoOk bool
		expected    string
	}{
		{
			name: "ldflags Date wins",
			date: "2026-01-01T00:00:00Z",
			buildInfo: &debug.BuildInfo{
				Settings: []debug.BuildSetting{
					{Key: "vcs.time", Value: "2025-01-01T00:00:00Z"},
				},
			},
			buildInfoOk: true,
			expected:    "2026-01-01T00:00:00Z",
		},
		{
			// vcs.time is the commit time, not the build time.
			name: "ignores vcs.time when Date unset",
			date: "",
			buildInfo: &debug.BuildInfo{
				Settings: []debug.BuildSetting{
					{Key: "vcs.time", Value: "2025-01-01T00:00:00Z"},
				},
			},
			buildInfoOk: true,
			expected:    "",
		},
		{
			name:        "returns empty string when both sources empty",
			date:        "",
			buildInfo:   nil,
			buildInfoOk: false,
			expected:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up test state
			Date = tt.date

			if tt.buildInfoOk {
				readBuildInfo = func() (*debug.BuildInfo, bool) {
					return tt.buildInfo, true
				}
			} else {
				readBuildInfo = func() (*debug.BuildInfo, bool) {
					return nil, false
				}
			}

			result := PrestBuildDate()
			if result != tt.expected {
				t.Errorf("PrestBuildDate() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestBuildSetting(t *testing.T) {
	// Save original value to restore in cleanup.
	originalReadBuildInfo := readBuildInfo

	t.Cleanup(func() {
		readBuildInfo = originalReadBuildInfo
	})

	readBuildInfo = func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{
			Settings: []debug.BuildSetting{
				{Key: "vcs.revision", Value: "abc123def456"},
				{Key: "vcs.time", Value: "2025-01-01T12:00:00Z"},
			},
		}, true
	}

	tests := []struct {
		name     string
		key      string
		expected string
	}{
		{
			name:     "finds vcs.revision",
			key:      "vcs.revision",
			expected: "abc123def456",
		},
		{
			name:     "finds vcs.time",
			key:      "vcs.time",
			expected: "2025-01-01T12:00:00Z",
		},
		{
			name:     "returns empty for missing key",
			key:      "nonexistent",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildSetting(tt.key)
			if result != tt.expected {
				t.Errorf("buildSetting(%q) = %q, want %q", tt.key, result, tt.expected)
			}
		})
	}
}

func TestBuildSetting_NoInfo(t *testing.T) {
	// Save original value to restore in cleanup.
	originalReadBuildInfo := readBuildInfo

	t.Cleanup(func() {
		readBuildInfo = originalReadBuildInfo
	})

	readBuildInfo = func() (*debug.BuildInfo, bool) {
		return nil, false
	}

	result := buildSetting("vcs.revision")
	if result != "" {
		t.Errorf("buildSetting(\"vcs.revision\") with no info = %q, want \"\"", result)
	}
}
