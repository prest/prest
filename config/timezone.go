package config

import (
	"log/slog"
	"net/url"
	"slices"
	"strings"
	"time"
)

// maxTimezoneLen bounds a session TimeZone name; real IANA names are far shorter.
const maxTimezoneLen = 64

// validTimezone is the only validity rule for a PostgreSQL session TimeZone.
// The trimmed zone is valid when it is at most 64 bytes of [A-Za-z0-9_+-/],
// does not start with '/', is not "Local" (any case) and time.LoadLocation
// knows it. The byte set keeps the value safe to emit unquoted in a lib/pq DSN.
// Empty input means unset and is not logged. Any other invalid input logs one
// warning naming key (and alias when set) and returns ("", false).
func validTimezone(key, alias, zone string) (string, bool) {
	zone = strings.TrimSpace(zone)
	if zone == "" {
		return "", false
	}
	if isTimezoneName(zone) {
		if _, err := time.LoadLocation(zone); err == nil {
			return zone, true
		}
	}
	logged := zone
	if len(logged) > maxTimezoneLen {
		logged = logged[:maxTimezoneLen]
	}
	args := []any{"key", key}
	if alias != "" {
		args = append(args, "alias", alias)
	}
	args = append(args, "value", logged)
	slog.Warn("config key invalid, using default", args...)
	return "", false
}

// isTimezoneName applies the syntactic checks of validTimezone.
func isTimezoneName(zone string) bool {
	if len(zone) > maxTimezoneLen || zone[0] == '/' || strings.EqualFold(zone, "Local") {
		return false
	}
	for i := 0; i < len(zone); i++ {
		if !isTimezoneByte(zone[i]) {
			return false
		}
	}
	return true
}

func isTimezoneByte(c byte) bool {
	switch {
	case 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z', '0' <= c && c <= '9':
		return true
	case c == '_', c == '+', c == '-', c == '/':
		return true
	}
	return false
}

// timezoneFromQuery returns the first value of the query key matching
// "timezone" case-insensitively, scanning keys in sorted order so the result
// is deterministic when several spellings are present; "" when absent.
func timezoneFromQuery(q url.Values) string {
	keys := make([]string, 0, len(q))
	for k := range q {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		if strings.EqualFold(k, "timezone") && len(q[k]) > 0 {
			return q[k][0]
		}
	}
	return ""
}

// fillTimezoneDefault validates an entry's own zone and, for non-MySQL entries
// left unset, inherits the validated global pg.timezone. MySQL entries never
// inherit it: the zone is a PostgreSQL session parameter.
func fillTimezoneDefault(db *DatabaseConf, cfg *Prest) {
	if db.Timezone != "" {
		db.Timezone, _ = validTimezone("databases.timezone", db.Alias, db.Timezone)
	}
	if EffectiveEngine(db, cfg) == EngineMySQL {
		return
	}
	if db.Timezone == "" && cfg != nil {
		db.Timezone = cfg.PGTimezone
	}
}

// applyURLTimezone sets *dst from the URL query timezone when it is valid; an
// invalid one warns and leaves *dst unchanged.
func applyURLTimezone(dst *string, key, alias string, q url.Values) {
	if tz, ok := validTimezone(key, alias, timezoneFromQuery(q)); ok {
		*dst = tz
	}
}
