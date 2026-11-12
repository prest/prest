package config

import (
	"bytes"
	"log/slog"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// captureSlog swaps the process-wide slog default for a text handler writing to
// the returned buffer. Callers must not use t.Parallel.
func captureSlog(t *testing.T) *bytes.Buffer {
	t.Helper()
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	return &buf
}

const tzWarn = "config key invalid, using default"

// TestValidTimezone pins the only validity rule for the session TimeZone: an
// IANA name made of DSN-safe bytes. Anything else warns once and is dropped, so
// it can never reach a connection string.
func TestValidTimezone(t *testing.T) {
	tests := []struct {
		name  string
		alias string
		in    string
		want  string
		ok    bool
	}{
		{name: "empty is unset, no log", in: "", want: "", ok: false},
		{name: "blank is unset, no log", in: "   ", want: "", ok: false},
		{name: "UTC", in: "UTC", want: "UTC", ok: true},
		{name: "region/city", in: "America/Sao_Paulo", want: "America/Sao_Paulo", ok: true},
		{name: "trimmed", in: "  Asia/Tokyo\t", want: "Asia/Tokyo", ok: true},
		{name: "etc offset with plus", in: "Etc/GMT+3", want: "Etc/GMT+3", ok: true},
		{name: "etc offset with minus", in: "Etc/GMT-3", want: "Etc/GMT-3", ok: true},
		{name: "unknown zone", in: "Mars/Olympus", ok: false},
		{name: "dsn injection with space", in: "UTC password=x", ok: false},
		{name: "quote", in: "UTC'", ok: false},
		{name: "equals", in: "UTC=x", ok: false},
		{name: "backslash", in: `UTC\`, ok: false},
		{name: "newline inside", in: "UTC\nx", ok: false},
		{name: "Local", in: "Local", ok: false},
		{name: "local any case", in: "lOcAl", ok: false},
		{name: "65 bytes", in: strings.Repeat("a", 65), ok: false},
		{name: "path traversal", in: "../etc/passwd", ok: false},
		{name: "absolute path", in: "/etc/passwd", ok: false},
		{name: "non ascii", in: "Europe/Zürich", ok: false},
		{name: "alias logged", alias: "tenant-a", in: "Mars/Olympus", ok: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := captureSlog(t)
			got, ok := validTimezone("pg.timezone", tt.alias, tt.in)
			require.Equal(t, tt.ok, ok)
			require.Equal(t, tt.want, got)

			logged := buf.String()
			if ok || strings.TrimSpace(tt.in) == "" {
				require.Empty(t, logged, "valid or empty input must not log")
				return
			}
			require.Equal(t, 1, strings.Count(logged, tzWarn), "exactly one warning")
			require.Contains(t, logged, "key=pg.timezone")
			if tt.alias != "" {
				require.Contains(t, logged, "alias="+tt.alias)
			} else {
				require.NotContains(t, logged, "alias=")
			}
		})
	}
}

// A long invalid value is logged truncated to 64 bytes so a hostile env var
// cannot flood the log.
func TestValidTimezone_TruncatesLoggedValue(t *testing.T) {
	buf := captureSlog(t)
	in := strings.Repeat("a", 64) + "TAIL"
	_, ok := validTimezone("databases.timezone", "x", in)
	require.False(t, ok)
	require.Contains(t, buf.String(), strings.Repeat("a", 64))
	require.NotContains(t, buf.String(), "TAIL")
}

// timezoneFromQuery matches the key case-insensitively and is deterministic
// when several spellings are present.
func TestTimezoneFromQuery(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		q    url.Values
		want string
	}{
		{name: "nil", q: nil, want: ""},
		{name: "absent", q: url.Values{"sslmode": {"require"}}, want: ""},
		{name: "lower", q: url.Values{"timezone": {"Asia/Tokyo"}}, want: "Asia/Tokyo"},
		{name: "mixed case", q: url.Values{"TimeZone": {"Asia/Tokyo"}}, want: "Asia/Tokyo"},
		{name: "first value", q: url.Values{"timezone": {"UTC", "Asia/Tokyo"}}, want: "UTC"},
		{name: "sorted keys decide", q: url.Values{"timezone": {"UTC"}, "TIMEZONE": {"Asia/Tokyo"}}, want: "Asia/Tokyo"},
		{name: "empty value list", q: url.Values{"timezone": {}}, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, timezoneFromQuery(tt.q))
		})
	}
}
