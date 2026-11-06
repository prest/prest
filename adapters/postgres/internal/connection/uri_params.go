package connection

import (
	"net/url"
	"strings"
)

// appendApplicationName returns uri with name as lib/pq's
// fallback_application_name, so each pooled connection shows up under that
// name in pg_stat_activity and in the server log.
//
// It is a fallback: lib/pq uses it only when nothing else supplies
// application_name, so a name in the URI or in PGAPPNAME still wins. A URI
// that already sets either spelling is returned unchanged, as is an empty
// name or an empty URI.
//
// The name is added when the connection is opened, not in GetURI, so the
// pool stays keyed by the database address alone.
func appendApplicationName(uri, name string) string {
	if name == "" || hasParam(uri, "application_name", "fallback_application_name") {
		return uri
	}
	return appendParam(uri, "fallback_application_name", name)
}

// appendParam returns uri with key=value added: as a query parameter on a
// postgres:// URL, or as a trailing pair on a key=value URI (as BuildURI
// writes one). An empty uri is returned unchanged. It does not check for an
// existing key; callers use hasParam first.
func appendParam(uri, key, value string) string {
	if uri == "" {
		return uri
	}
	if strings.HasPrefix(uri, "postgres://") || strings.HasPrefix(uri, "postgresql://") {
		separator := "?"
		if strings.Contains(uri, "?") {
			separator = "&"
		}
		return uri + separator + key + "=" + url.QueryEscape(value)
	}
	return uri + " " + key + "=" + quoteURIValue(value)
}

// hasParam reports whether uri already sets any of keys as a parameter. Text
// that only appears inside a value, such as a password, does not count: a
// parameter starts the URI or follows a space (key=value URI) or ? or & (URL
// query).
func hasParam(uri string, keys ...string) bool {
	for _, key := range keys {
		needle := key + "="
		for rest, offset := uri, 0; ; {
			at := strings.Index(rest, needle)
			if at < 0 {
				break
			}
			before := uri[:offset+at]
			if before == "" || strings.HasSuffix(before, " ") ||
				strings.HasSuffix(before, "?") || strings.HasSuffix(before, "&") {
				return true
			}
			step := at + len(needle)
			rest, offset = rest[step:], offset+step
		}
	}
	return false
}

// quoteURIValue quotes a value for a key=value URI the way libpq reads one:
// single quotes, with ' and \ escaped by a backslash.
func quoteURIValue(v string) string {
	if v != "" && !strings.ContainsAny(v, ` '\`) {
		return v
	}
	return "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(v) + "'"
}
