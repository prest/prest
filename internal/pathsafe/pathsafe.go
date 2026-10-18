// Package pathsafe provides containment checks for file paths built from
// untrusted input, so a caller can confirm a joined path stays under its base
// directory before opening it.
package pathsafe

import (
	"path/filepath"
	"strings"
)

// WithinBase reports whether path lies inside base. Both are expected to be
// cleaned already; a path that escapes yields a relative path of ".." or one
// rooted at "..", and an unrelated volume yields an error from filepath.Rel.
// This is a lexical check only — see ResolvedWithinBase for symlinks.
func WithinBase(base, path string) bool {
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// ResolvedWithinBase repeats the containment check with symlinks resolved, so a
// link inside the base directory cannot point outside it. Both paths must
// already exist: EvalSymlinks fails on a missing path, which is why callers run
// this after confirming the file is there rather than before.
func ResolvedWithinBase(base, path string) bool {
	realBase, err := filepath.EvalSymlinks(base)
	if err != nil {
		return false
	}
	realPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return false
	}
	return WithinBase(filepath.Clean(realBase), filepath.Clean(realPath))
}
