package config

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/spf13/viper"
)

const (
	defaultGuardRateLimitWindow = 60
	defaultGuardMaxBodyBytes    = 1 << 20 // 1 MiB
	defaultGuardRedisPrefix     = "prest_guard"
)

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

// parseGuardConfig reads the [guard] section. Env overrides use the
// PREST_GUARD_* prefix.
func parseGuardConfig(v *viper.Viper, cfg *Prest) {
	g := &cfg.Guard

	// viper's GetBool silently discards conversion errors, which would turn a
	// typo like PREST_GUARD_ENABLED=yes into "guard off". Validate the raw
	// value instead: an explicitly set but non-boolean value marks the whole
	// guard config invalid so the middleware fails closed.
	switch raw := v.Get("guard.enabled").(type) {
	case nil:
		// Nothing set anywhere: keep the default (disabled).
	case bool:
		g.Enabled = raw
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(raw))
		if err != nil {
			g.Invalid = fmt.Errorf("guard.enabled: %q is not a valid boolean", raw)
			slog.Error("invalid guard config, guard requests will fail closed", "err", g.Invalid)
			return
		}
		g.Enabled = parsed
	default:
		g.Invalid = fmt.Errorf("guard.enabled: unsupported value %v", raw)
		slog.Error("invalid guard config, guard requests will fail closed", "err", g.Invalid)
		return
	}

	g.Passive = v.GetBool("guard.passive")
	g.RateLimit = v.GetInt("guard.rate_limit")
	g.RateLimitWindow = v.GetInt("guard.rate_limit_window")
	g.MaxBodyBytes = v.GetInt64("guard.max_body_bytes")
	g.Blacklist = v.GetStringSlice("guard.blacklist")
	g.Whitelist = v.GetStringSlice("guard.whitelist")
	g.ExcludePaths = v.GetStringSlice("guard.exclude_paths")
	g.TrustedProxies = v.GetStringSlice("guard.trusted_proxies")
	g.RedisURL = v.GetString("guard.redis_url")
	g.RedisPrefix = v.GetString("guard.redis_prefix")
	g.BlockCloudProviders = v.GetStringSlice("guard.block_cloud_providers")

	if g.RateLimitWindow <= 0 {
		slog.Warn("guard.rate_limit_window must be positive, using default", "value", g.RateLimitWindow)
		g.RateLimitWindow = defaultGuardRateLimitWindow
	}
	if g.MaxBodyBytes <= 0 {
		g.MaxBodyBytes = defaultGuardMaxBodyBytes
	}
	if g.RedisPrefix == "" {
		g.RedisPrefix = defaultGuardRedisPrefix
	}
}
