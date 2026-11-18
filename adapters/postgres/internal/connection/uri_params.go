package connection

import (
	"net/url"
	"strings"
	"unicode"

	"github.com/lib/pq"
)

// appendApplicationName returns uri with name as lib/pq's
// fallback_application_name, so each pooled connection shows up under that
// name in pg_stat_activity and in the server log.
//
// It is a fallback: lib/pq uses it only when nothing else supplies
// application_name. Whether something already does is decided by lib/pq's
// own parser, so the URI, PGAPPNAME and an explicit fallback are read exactly
// as the driver reads them when it dials. A URI lib/pq cannot parse is
// returned unchanged, as is an empty name or an empty URI; dialling reports
// the parse error.
//
// The name is added when the connection is opened, not in GetURI, so the
// pool stays keyed by the database address alone.
func appendApplicationName(uri, name string) string {
	if uri == "" || name == "" {
		return uri
	}
	cfg, err := pq.NewConfig(uri)
	if err != nil || cfg.ApplicationName != "" {
		return uri
	}
	return appendParam(uri, "fallback_application_name", name)
}

// appendParam returns uri with key=value added: as a query parameter on a
// postgres:// URL, placed before any #fragment, or as a trailing pair on a
// key=value URI (as BuildURI writes one). An empty uri is returned
// unchanged. It does not check for an existing key.
func appendParam(uri, key, value string) string {
	if uri == "" {
		return uri
	}
	if strings.HasPrefix(uri, "postgres://") || strings.HasPrefix(uri, "postgresql://") {
		base, fragment := uri, ""
		if i := strings.IndexByte(uri, '#'); i >= 0 {
			base, fragment = uri[:i], uri[i:]
		}
		separator := "?"
		if strings.Contains(base, "?") {
			separator = "&"
		}
		return base + separator + key + "=" + url.QueryEscape(value) + fragment
	}
	return uri + " " + key + "=" + quoteURIValue(value)
}

// quoteURIValue quotes a value for a key=value URI the way libpq reads one:
// single quotes, with ' and \ escaped by a backslash. An empty value, or one
// holding any Unicode white space, a quote or a backslash, is quoted.
func quoteURIValue(v string) string {
	if v != "" && !strings.ContainsAny(v, `'\`) && strings.IndexFunc(v, unicode.IsSpace) < 0 {
		return v
	}
	return "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(v) + "'"
}
