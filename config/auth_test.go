package config

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestParseAuthConfig_MigrateDefaultDisabled(t *testing.T) {
	t.Parallel()

	v := viper.New()
	v.Set("auth.enabled", false)
	cfg := &Prest{}
	require.NoError(t, parseAuthConfig(v, cfg))

	require.False(t, cfg.AuthEnabled)
	require.False(t, cfg.AuthMigrateOnStartup)
}

func TestParseAuthConfig_MigrateDefaultEnabled(t *testing.T) {
	t.Parallel()

	v := viper.New()
	v.Set("auth.enabled", true)
	cfg := &Prest{}
	require.NoError(t, parseAuthConfig(v, cfg))

	require.True(t, cfg.AuthEnabled)
	require.True(t, cfg.AuthMigrateOnStartup)
}

func TestParseAuthConfig_ExplicitMigrateOnStartup(t *testing.T) {
	t.Parallel()

	v := viper.New()
	v.Set("auth.enabled", true)
	v.Set("auth.migrate_on_startup", false)
	cfg := &Prest{}
	require.NoError(t, parseAuthConfig(v, cfg))

	require.True(t, cfg.AuthEnabled)
	require.False(t, cfg.AuthMigrateOnStartup)
}

func TestParseAuthConfig_TokenTTL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value any
		want  time.Duration
	}{
		{name: "unset uses default", value: nil, want: DefaultAuthTokenTTL},
		{name: "go duration", value: "30m", want: 30 * time.Minute},
		{name: "compound duration", value: "1h30m", want: 90 * time.Minute},
		{name: "bare number is seconds", value: "3600", want: time.Hour},
		{name: "toml integer is seconds", value: 900, want: 15 * time.Minute},
		{name: "surrounding spaces", value: " 2h ", want: 2 * time.Hour},
		{name: "one second is the minimum", value: "1s", want: time.Second},
		{name: "empty uses default", value: "", want: DefaultAuthTokenTTL},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			v := viper.New()
			setAuthDefaults(v)
			if tc.value != nil {
				v.Set("auth.token_ttl", tc.value)
			}
			cfg := &Prest{}
			require.NoError(t, parseAuthConfig(v, cfg))

			require.Equal(t, tc.want, cfg.AuthTokenTTL)
		})
	}
}

func TestParseAuthConfig_TokenTTLInvalid(t *testing.T) {
	t.Parallel()

	// A typo must not silently become the 6h default: that would issue
	// tokens that outlive what the operator asked for.
	for _, value := range []any{"soon", "5mins", "0", "0s", "-1h", "-60", "500ms", "9223372036854775807"} {
		t.Run(fmt.Sprint(value), func(t *testing.T) {
			t.Parallel()

			v := viper.New()
			setAuthDefaults(v)
			v.Set("auth.token_ttl", value)
			cfg := &Prest{}

			err := parseAuthConfig(v, cfg)
			require.ErrorIs(t, err, ErrInvalidAuthTokenTTL)
			require.ErrorContains(t, err, fmt.Sprint(value))
		})
	}
}

func TestLoad_TokenTTLFromEnv(t *testing.T) {
	t.Setenv("PREST_CONF", filepath.Join(t.TempDir(), "missing.toml"))
	t.Setenv("PREST_AUTH_TOKEN_TTL", "45m")

	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, 45*time.Minute, cfg.AuthTokenTTL)
}

func TestLoad_InvalidTokenTTLFailsStartup(t *testing.T) {
	t.Setenv("PREST_CONF", filepath.Join(t.TempDir(), "missing.toml"))
	t.Setenv("PREST_AUTH_TOKEN_TTL", "forever")

	cfg, err := Load()
	require.ErrorIs(t, err, ErrInvalidAuthTokenTTL)
	require.Nil(t, cfg)
}
