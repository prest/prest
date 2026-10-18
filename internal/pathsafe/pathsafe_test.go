package pathsafe

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithinBase(t *testing.T) {
	t.Parallel()

	base := filepath.Clean("/srv/plugins")
	cases := []struct {
		description string
		path        string
		want        bool
	}{
		{"parent directory itself escapes", filepath.Join(base, ".."), false},
		{"sibling via ../x escapes", filepath.Join(base, "..", "x"), false},
		{"a/../.. collapses above base", filepath.Join(base, "a", "..", ".."), false},
		{"absolute path elsewhere escapes", "/etc/passwd", false},
		{"base itself is within", base, true},
		{"file in base is within", filepath.Join(base, "hello.so"), true},
		{"file in subdir is within", filepath.Join(base, "middlewares", "hello.so"), true},
		{"name starting with dots stays inside", filepath.Join(base, "..hello.so"), true},
	}
	for _, tc := range cases {
		t.Run(tc.description, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, WithinBase(base, tc.path))
		})
	}
}

func TestWithinBaseRelError(t *testing.T) {
	t.Parallel()
	// filepath.Rel cannot relate an absolute base to a relative path.
	assert.False(t, WithinBase("/srv/plugins", "relative/hello.so"))
}

func TestResolvedWithinBase(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	outside := t.TempDir()

	outsideTarget := filepath.Join(outside, "evil.so")
	require.NoError(t, os.WriteFile(outsideTarget, []byte("x"), 0o600))
	insideTarget := filepath.Join(base, "real.so")
	require.NoError(t, os.WriteFile(insideTarget, []byte("x"), 0o600))

	escapeLink := filepath.Join(base, "escape.so")
	require.NoError(t, os.Symlink(outsideTarget, escapeLink))
	insideLink := filepath.Join(base, "inside.so")
	require.NoError(t, os.Symlink(insideTarget, insideLink))

	t.Run("regular file inside base is within", func(t *testing.T) {
		t.Parallel()
		assert.True(t, ResolvedWithinBase(base, insideTarget))
	})
	t.Run("link inside base to outside target is rejected", func(t *testing.T) {
		t.Parallel()
		assert.True(t, WithinBase(base, escapeLink), "lexical check alone is fooled by the link")
		assert.False(t, ResolvedWithinBase(base, escapeLink))
	})
	t.Run("link inside base to inside target is within", func(t *testing.T) {
		t.Parallel()
		assert.True(t, ResolvedWithinBase(base, insideLink))
	})
	t.Run("missing path is rejected", func(t *testing.T) {
		t.Parallel()
		assert.False(t, ResolvedWithinBase(base, filepath.Join(base, "missing.so")))
	})
	t.Run("missing base is rejected", func(t *testing.T) {
		t.Parallel()
		assert.False(t, ResolvedWithinBase(filepath.Join(base, "nope"), insideTarget))
	})
}
