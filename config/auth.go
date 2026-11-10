package config

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// DefaultAuthTokenTTL is how long a token issued by /auth stays valid when
// auth.token_ttl is not set.
const DefaultAuthTokenTTL = 6 * time.Hour

// ErrInvalidAuthTokenTTL is returned when auth.token_ttl is set to something
// other than a duration of at least one second. Startup fails instead of
// falling back to the default, which could hand out tokens that live longer
// than intended.
var ErrInvalidAuthTokenTTL = errors.New("invalid auth.token_ttl")

func setAuthDefaults(v *viper.Viper) {
	v.SetDefault("auth.enabled", false)
	v.SetDefault("auth.username", "username")
	v.SetDefault("auth.password", "password")
	v.SetDefault("auth.schema", "public")
	v.SetDefault("auth.table", "prest_users")
	v.SetDefault("auth.encrypt", "bcrypt")
	v.SetDefault("auth.type", "body")
	v.SetDefault("auth.token_ttl", DefaultAuthTokenTTL.String())
}

func parseAuthConfig(v *viper.Viper, cfg *Prest) error {
	cfg.AuthEnabled = v.GetBool("auth.enabled")
	if v.IsSet("auth.migrate_on_startup") {
		cfg.AuthMigrateOnStartup = v.GetBool("auth.migrate_on_startup")
	} else {
		cfg.AuthMigrateOnStartup = cfg.AuthEnabled
	}
	cfg.AuthSchema = v.GetString("auth.schema")
	cfg.AuthTable = v.GetString("auth.table")
	cfg.AuthUsername = v.GetString("auth.username")
	cfg.AuthPassword = v.GetString("auth.password")
	cfg.AuthEncrypt = v.GetString("auth.encrypt")
	cfg.AuthMetadata = v.GetStringSlice("auth.metadata")
	cfg.AuthType = v.GetString("auth.type")
	ttl, err := parseAuthTokenTTL(v.GetString("auth.token_ttl"))
	if err != nil {
		return err
	}
	cfg.AuthTokenTTL = ttl
	return nil
}

// parseAuthTokenTTL reads auth.token_ttl as a Go duration ("30m", "12h") or,
// like the other timeouts in the config, as a bare number of seconds. viper's
// GetDuration would read a bare number as nanoseconds and issue tokens that
// are already expired, so the value is parsed here. JWT expiry has one-second
// resolution, so anything shorter is rejected. An empty value means the
// default.
func parseAuthTokenTTL(raw string) (time.Duration, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return DefaultAuthTokenTTL, nil
	}
	// Anything unparsable or overflowing leaves ttl at zero.
	var ttl time.Duration
	if secs, err := strconv.ParseInt(raw, 10, 64); err == nil {
		if secs <= math.MaxInt64/int64(time.Second) {
			ttl = time.Duration(secs) * time.Second
		}
	} else {
		ttl, _ = time.ParseDuration(raw)
	}
	if ttl < time.Second {
		return 0, fmt.Errorf(`%w %q: use a duration of at least 1s such as "30m" or "12h", or a number of seconds`,
			ErrInvalidAuthTokenTTL, raw)
	}
	return ttl, nil
}
