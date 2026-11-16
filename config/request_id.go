package config

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/spf13/viper"
)

// parseRequestIDHeader reads http.request_id_header: the header that carries
// the request (correlation) ID. It returns the name in canonical form, or ""
// when the setting is unset, empty or not a valid header field name; the
// request ID is then turned off rather than failing startup.
func parseRequestIDHeader(v *viper.Viper) string {
	const key = "http.request_id_header"
	name := strings.TrimSpace(v.GetString(key))
	if name == "" {
		return ""
	}
	if !isHeaderFieldName(name) {
		slog.Warn(key+" is not a valid header name, request IDs are disabled", "value", name)
		return ""
	}
	return http.CanonicalHeaderKey(name)
}

// isHeaderFieldName reports whether name is an RFC 9110 field name: a
// non-empty token of letters, digits and !#$%&'*+-.^_`|~.
func isHeaderFieldName(name string) bool {
	if name == "" {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case strings.IndexByte("!#$%&'*+-.^_`|~", c) >= 0:
		default:
			return false
		}
	}
	return true
}
