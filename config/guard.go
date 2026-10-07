package config

import (
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"

	"github.com/spf13/viper"
)

const (
	defaultGuardRateLimitWindow = 60
	defaultGuardMaxBodyBytes    = 1 << 20 // 1 MiB
	defaultGuardRedisPrefix     = "prest_guard"
)

// setGuardDefaults registers the [guard] section defaults. Every knob starts
// off: the guard is opt-in and pREST behaves exactly as before unless the
// operator enables it.
func setGuardDefaults(v *viper.Viper) {
	v.SetDefault("guard.enabled", false)
	v.SetDefault("guard.passive", false)
	v.SetDefault("guard.rate_limit", 0)
	v.SetDefault("guard.rate_limit_window", defaultGuardRateLimitWindow)
	v.SetDefault("guard.max_body_bytes", defaultGuardMaxBodyBytes)
	v.SetDefault("guard.blacklist", []string{})
	v.SetDefault("guard.whitelist", []string{})
	v.SetDefault("guard.exclude_paths", []string{})
	v.SetDefault("guard.trusted_proxies", []string{})
	v.SetDefault("guard.redis_url", "")
	v.SetDefault("guard.redis_prefix", defaultGuardRedisPrefix)
	v.SetDefault("guard.block_cloud_providers", []string{})
}

// GuardConf holds opt-in request security settings backed by the guard-core
// engine (per-client rate limits, request payload inspection, IP policy).
// Everything here is disabled by default: when Enabled is false pREST builds
// no engine and behaves exactly as before.
type GuardConf struct {
	// Enabled turns the guard middleware on. Default false.
	Enabled bool
	// Invalid carries a config error that makes the guard unusable, such as a
	// PREST_GUARD_ENABLED value that is not a valid boolean. Parse cannot
	// return errors, so invalidity is carried here and GuardMiddleware fails
	// closed (every request answers 500) instead of silently running without
	// the security the operator asked for. Nil means the config parsed fine.
	Invalid error
	// Passive makes the guard log what it would have blocked instead of
	// rejecting requests. Use it to preview rules before enforcing.
	Passive bool
	// RateLimit is the maximum number of requests per RateLimitWindow seconds
	// per client IP. 0 (default) disables rate limiting.
	RateLimit int
	// RateLimitWindow is the rate limit window in seconds. Default 60.
	RateLimitWindow int
	// MaxBodyBytes caps how much of a request body the inspection layer
	// reads. Default 1 MiB.
	MaxBodyBytes int64
	// Blacklist blocks these IPs/CIDRs. Whitelist allows them unconditionally
	// (both empty by default).
	Blacklist []string
	Whitelist []string
	// ExcludePaths skips all guard checks (including rate limits) for these
	// paths and their subtrees.
	ExcludePaths []string
	// TrustedProxies lists proxy IPs/CIDRs whose X-Forwarded-For header is
	// trusted when resolving the real client IP. Empty (default) means
	// forwarded headers are ignored and the direct peer is used as client.
	TrustedProxies []string
	// RedisURL, when set, shares rate limit and ban state across instances.
	// When empty, state is per-instance (in memory).
	RedisURL string
	// RedisPrefix namespaces guard keys in Redis. Default "prest_guard".
	RedisPrefix string
	// BlockCloudProviders blocks datacenter ranges (e.g. ["AWS", "GCP",
	// "Azure"]). Off by default; range refresh uses Redis when configured.
	BlockCloudProviders []string
}

// guardRawBool reads key strictly. Viper's GetBool silently discards
// conversion errors, which would turn a typo like PREST_GUARD_PASSIVE=yes
// into "passive off"; an explicitly set but unparseable value instead marks
// the guard config invalid so the middleware fails closed.
func guardRawBool(v *viper.Viper, key string, g *GuardConf) (bool, bool) {
	switch raw := v.Get(key).(type) {
	case nil:
		return false, true
	case bool:
		return raw, true
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(raw))
		if err != nil {
			g.Invalid = fmt.Errorf("%s: %q is not a valid boolean", key, raw)
			return false, false
		}
		return parsed, true
	default:
		g.Invalid = fmt.Errorf("%s: unsupported value %v", key, raw)
		return false, false
	}
}

// guardRawInt reads key strictly, reporting whether the key was set at all
// so unset keys keep their defaults while explicitly invalid values fail
// closed. Viper's GetInt turns a typo like PREST_GUARD_RATE_LIMIT=100/min
// into 0, silently disabling rate limiting.
func guardRawInt(v *viper.Viper, key string, g *GuardConf) (int64, bool, bool) {
	switch raw := v.Get(key).(type) {
	case nil:
		return 0, false, true
	case int:
		return int64(raw), true, true
	case int64:
		return raw, true, true
	case float64:
		if raw != math.Trunc(raw) {
			g.Invalid = fmt.Errorf("%s: %v is not an integer", key, raw)
			return 0, true, false
		}
		return int64(raw), true, true
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
		if err != nil {
			g.Invalid = fmt.Errorf("%s: %q is not a valid integer", key, raw)
			return 0, true, false
		}
		return parsed, true, true
	default:
		g.Invalid = fmt.Errorf("%s: unsupported value %v", key, raw)
		return 0, true, false
	}
}

// guardRawList reads key as a string list. Env values are comma-separated:
// viper's GetStringSlice splits environment values on whitespace, which
// would read PREST_GUARD_BLACKLIST=1.2.3.4,5.6.7.8 as the single entry
// "1.2.3.4,5.6.7.8" and block nothing. Config-file arrays must contain
// only strings.
func guardRawList(v *viper.Viper, key string, g *GuardConf) ([]string, bool) {
	switch raw := v.Get(key).(type) {
	case nil:
		return nil, true
	case []string:
		return raw, true
	case []interface{}:
		out := make([]string, 0, len(raw))
		for _, item := range raw {
			s, ok := item.(string)
			if !ok {
				g.Invalid = fmt.Errorf("%s: non-string entry %v", key, item)
				return nil, false
			}
			out = append(out, s)
		}
		return out, true
	case string:
		parts := strings.Split(raw, ",")
		out := make([]string, 0, len(parts))
		for _, part := range parts {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				out = append(out, trimmed)
			}
		}
		return out, true
	default:
		g.Invalid = fmt.Errorf("%s: unsupported value %v", key, raw)
		return nil, false
	}
}

// parseGuardConfig reads the [guard] section. Env overrides use the
// PREST_GUARD_* prefix. Every security-relevant key is validated from its
// raw value: anything explicitly set but unparseable marks the whole guard
// config invalid so the middleware fails closed instead of silently running
// with a weaker posture.
func parseGuardConfig(v *viper.Viper, cfg *Prest) {
	g := &cfg.Guard
	fail := func() {
		slog.Error("invalid guard config, guard requests will fail closed", "err", g.Invalid)
	}

	enabled, ok := guardRawBool(v, "guard.enabled", g)
	if !ok {
		fail()
		return
	}
	g.Enabled = enabled

	passive, ok := guardRawBool(v, "guard.passive", g)
	if !ok {
		fail()
		return
	}
	g.Passive = passive

	rateLimit, set, ok := guardRawInt(v, "guard.rate_limit", g)
	if !ok {
		fail()
		return
	}
	if set && rateLimit < 0 {
		g.Invalid = fmt.Errorf("guard.rate_limit: %d is negative", rateLimit)
		fail()
		return
	}
	g.RateLimit = int(rateLimit)

	window, set, ok := guardRawInt(v, "guard.rate_limit_window", g)
	if !ok {
		fail()
		return
	}
	if set && window <= 0 {
		g.Invalid = fmt.Errorf("guard.rate_limit_window: %d is not positive", window)
		fail()
		return
	}
	if !set {
		window = defaultGuardRateLimitWindow
	}
	g.RateLimitWindow = int(window)

	maxBody, set, ok := guardRawInt(v, "guard.max_body_bytes", g)
	if !ok {
		fail()
		return
	}
	if set && maxBody <= 0 {
		g.Invalid = fmt.Errorf("guard.max_body_bytes: %d is not positive", maxBody)
		fail()
		return
	}
	if !set {
		maxBody = defaultGuardMaxBodyBytes
	}
	g.MaxBodyBytes = maxBody

	lists := []struct {
		key string
		dst *[]string
	}{
		{"guard.blacklist", &g.Blacklist},
		{"guard.whitelist", &g.Whitelist},
		{"guard.exclude_paths", &g.ExcludePaths},
		{"guard.trusted_proxies", &g.TrustedProxies},
		{"guard.block_cloud_providers", &g.BlockCloudProviders},
	}
	for _, list := range lists {
		values, ok := guardRawList(v, list.key, g)
		if !ok {
			fail()
			return
		}
		*list.dst = values
	}

	g.RedisURL = v.GetString("guard.redis_url")
	if g.RedisPrefix = v.GetString("guard.redis_prefix"); g.RedisPrefix == "" {
		g.RedisPrefix = defaultGuardRedisPrefix
	}
}
