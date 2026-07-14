package config

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestParseAuthConfig_MigrateDefaultDisabled(t *testing.T) {
	t.Parallel()

	v := viper.New()
	v.Set("auth.enabled", false)
	cfg := &Prest{}
	parseAuthConfig(v, cfg)

	require.False(t, cfg.AuthEnabled)
	require.False(t, cfg.AuthMigrateOnStartup)
}

func TestParseAuthConfig_MigrateDefaultEnabled(t *testing.T) {
	t.Parallel()

	v := viper.New()
	v.Set("auth.enabled", true)
	cfg := &Prest{}
	parseAuthConfig(v, cfg)

	require.True(t, cfg.AuthEnabled)
	require.True(t, cfg.AuthMigrateOnStartup)
}

func TestParseAuthConfig_ExplicitMigrateOnStartup(t *testing.T) {
	t.Parallel()

	v := viper.New()
	v.Set("auth.enabled", true)
	v.Set("auth.migrate_on_startup", false)
	cfg := &Prest{}
	parseAuthConfig(v, cfg)

	require.True(t, cfg.AuthEnabled)
	require.False(t, cfg.AuthMigrateOnStartup)
}
