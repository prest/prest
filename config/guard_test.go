package config

import (
	"math"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestGuardRawHelpers(t *testing.T) {
	t.Run("bool", func(t *testing.T) {
		cases := []struct {
			name    string
			set     any
			want    bool
			ok      bool
			invalid string
		}{
			{"unset", nil, false, true, ""},
			{"native bool", true, true, true, ""},
			{"string true", " true ", true, true, ""},
			{"bad string", "yes", false, false, `guard.k: "yes" is not a valid boolean`},
			{"unsupported type", 3, false, false, "guard.k: unsupported value 3"},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				v := viper.New()
				if c.set != nil {
					v.Set("guard.k", c.set)
				}
				g := &GuardConf{}
				got, ok := guardRawBool(v, "guard.k", g)
				require.Equal(t, c.want, got)
				require.Equal(t, c.ok, ok)
				if c.invalid == "" {
					require.NoError(t, g.Invalid)
				} else {
					require.EqualError(t, g.Invalid, c.invalid)
				}
			})
		}
	})

	t.Run("int", func(t *testing.T) {
		cases := []struct {
			name    string
			set     any
			want    int64
			isSet   bool
			ok      bool
			invalid string
		}{
			{"unset", nil, 0, false, true, ""},
			{"int", 7, 7, true, true, ""},
			{"int64", int64(8), 8, true, true, ""},
			{"whole float", float64(9), 9, true, true, ""},
			{"fractional float", 1.5, 0, true, false, "guard.k: 1.5 is not an integer"},
			{"nan float", math.NaN(), 0, true, false, "guard.k: NaN is not an integer"},
			{"string", " 10 ", 10, true, true, ""},
			{"bad string", "100/min", 0, true, false, `guard.k: "100/min" is not a valid integer`},
			{"unsupported type", true, 0, true, false, "guard.k: unsupported value true"},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				v := viper.New()
				if c.set != nil {
					v.Set("guard.k", c.set)
				}
				g := &GuardConf{}
				got, isSet, ok := guardRawInt(v, "guard.k", g)
				require.Equal(t, c.want, got)
				require.Equal(t, c.isSet, isSet)
				require.Equal(t, c.ok, ok)
				if c.invalid == "" {
					require.NoError(t, g.Invalid)
				} else {
					require.EqualError(t, g.Invalid, c.invalid)
				}
			})
		}
	})

	t.Run("list", func(t *testing.T) {
		cases := []struct {
			name    string
			set     any
			want    []string
			ok      bool
			invalid string
		}{
			{"unset", nil, nil, true, ""},
			{"string slice", []string{"a", "b"}, []string{"a", "b"}, true, ""},
			{"interface slice", []interface{}{"a", "b"}, []string{"a", "b"}, true, ""},
			{"non-string entry", []interface{}{"a", 5}, nil, false, "guard.k: non-string entry 5"},
			{"comma separated string", " 1.2.3.4, ,5.6.7.8 ", []string{"1.2.3.4", "5.6.7.8"}, true, ""},
			{"unsupported type", 4, nil, false, "guard.k: unsupported value 4"},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				v := viper.New()
				if c.set != nil {
					v.Set("guard.k", c.set)
				}
				g := &GuardConf{}
				got, ok := guardRawList(v, "guard.k", g)
				require.Equal(t, c.want, got)
				require.Equal(t, c.ok, ok)
				if c.invalid == "" {
					require.NoError(t, g.Invalid)
				} else {
					require.EqualError(t, g.Invalid, c.invalid)
				}
			})
		}
	})

	t.Run("string", func(t *testing.T) {
		v := viper.New()
		g := &GuardConf{}
		got, ok := guardRawString(v, "guard.k", g)
		require.True(t, ok)
		require.Equal(t, "", got)

		v.Set("guard.k", "redis://x")
		got, ok = guardRawString(v, "guard.k", g)
		require.True(t, ok)
		require.Equal(t, "redis://x", got)
		require.NoError(t, g.Invalid)

		v.Set("guard.k", 12)
		_, ok = guardRawString(v, "guard.k", g)
		require.False(t, ok)
		require.EqualError(t, g.Invalid, "guard.k: unsupported value 12")
	})
}

// TestParseGuardConfigFailsClosedPerKey sets each guard key to an unusable
// value and requires the whole config to be marked invalid, so no key can be
// silently ignored.
func TestParseGuardConfigFailsClosedPerKey(t *testing.T) {
	cases := []struct {
		key   string
		value any
		want  string
	}{
		{"guard.enabled", "yes", "guard.enabled"},
		{"guard.passive", "yes", "guard.passive"},
		{"guard.rate_limit", "100/min", "guard.rate_limit"},
		{"guard.rate_limit", -1, "guard.rate_limit: -1 is negative"},
		{"guard.rate_limit_window", "soon", "guard.rate_limit_window"},
		{"guard.rate_limit_window", 0, "guard.rate_limit_window: 0 is not positive"},
		{"guard.max_body_bytes", "big", "guard.max_body_bytes"},
		{"guard.max_body_bytes", 0, "guard.max_body_bytes: 0 is not positive"},
		{"guard.blacklist", 4, "guard.blacklist"},
		{"guard.whitelist", []interface{}{1}, "guard.whitelist"},
		{"guard.exclude_paths", 4, "guard.exclude_paths"},
		{"guard.trusted_proxies", 4, "guard.trusted_proxies"},
		{"guard.block_cloud_providers", 4, "guard.block_cloud_providers"},
		{"guard.auto_ban", "yes", "guard.auto_ban"},
		{"guard.penetration_detection", "yes", "guard.penetration_detection"},
		{"guard.redis_fail_open", "yes", "guard.redis_fail_open"},
		{"guard.redis_url", 5, "guard.redis_url"},
		{"guard.redis_prefix", 5, "guard.redis_prefix"},
	}
	for _, c := range cases {
		t.Run(c.key+"="+c.want, func(t *testing.T) {
			v := viper.New()
			setGuardDefaults(v)
			v.Set(c.key, c.value)
			cfg := &Prest{}
			parseGuardConfig(v, cfg)
			require.Error(t, cfg.Guard.Invalid)
			require.Contains(t, cfg.Guard.Invalid.Error(), c.want)
		})
	}
}

func TestParseGuardConfigValidValues(t *testing.T) {
	v := viper.New()
	setGuardDefaults(v)
	v.Set("guard.enabled", true)
	v.Set("guard.passive", "true")
	v.Set("guard.rate_limit", 50)
	v.Set("guard.rate_limit_window", 30)
	v.Set("guard.max_body_bytes", 4096)
	v.Set("guard.blacklist", "203.0.113.0/24,198.51.100.7")
	v.Set("guard.redis_prefix", "")
	v.Set("guard.auto_ban", true)
	cfg := &Prest{}
	parseGuardConfig(v, cfg)
	g := cfg.Guard
	require.NoError(t, g.Invalid)
	require.True(t, g.Enabled)
	require.True(t, g.Passive)
	require.Equal(t, 50, g.RateLimit)
	require.Equal(t, 30, g.RateLimitWindow)
	require.Equal(t, int64(4096), g.MaxBodyBytes)
	require.Equal(t, []string{"203.0.113.0/24", "198.51.100.7"}, g.Blacklist)
	require.Equal(t, defaultGuardRedisPrefix, g.RedisPrefix, "empty prefix falls back to the default")
	require.True(t, g.AutoBan)
	require.True(t, g.PenetrationDetection, "penetration detection defaults on")
	require.True(t, g.RedisFailOpen, "redis fail-open defaults on")
}

// TestParseGuardConfigWithoutRegisteredDefaults proves the window and body
// size fall back to their defaults even when viper holds no default for them.
func TestParseGuardConfigWithoutRegisteredDefaults(t *testing.T) {
	cfg := &Prest{}
	parseGuardConfig(viper.New(), cfg)
	require.NoError(t, cfg.Guard.Invalid)
	require.Equal(t, defaultGuardRateLimitWindow, cfg.Guard.RateLimitWindow)
	require.Equal(t, int64(defaultGuardMaxBodyBytes), cfg.Guard.MaxBodyBytes)
	require.Equal(t, defaultGuardRedisPrefix, cfg.Guard.RedisPrefix)
}
