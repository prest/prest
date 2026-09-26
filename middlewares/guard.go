package middlewares

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strings"

	guardcore "github.com/rennf93/guard-core-go/v4/guardcore"
	nethttpguard "github.com/rennf93/nethttp-guard"
	"github.com/urfave/negroni/v3"

	"github.com/prest/prest/v2/config"
)

// GuardMiddleware wraps the request chain with the guard-core engine
// (per-client rate limits, request payload inspection, IP policy). The
// middleware sits after CORS so blocked responses keep CORS headers, and
// before JWT auth so the auth endpoint is covered by rate limits.
//
// Opt-in via [guard] config (PREST_GUARD_* env vars); DisabledByDefault means
// New() never builds an engine unless guard.enabled is true.
func GuardMiddleware(conf config.GuardConf) (negroni.Handler, error) {
	// An invalid config (e.g. a non-boolean guard.enabled) must fail closed
	// here so New() mounts invalidGuardConfigMiddleware instead of serving
	// requests without the security the operator asked for.
	if conf.Invalid != nil {
		return nil, fmt.Errorf("guard config invalid: %w", conf.Invalid)
	}

	trustedProxies, err := newTrustedProxySet(conf.TrustedProxies)
	if err != nil {
		return nil, fmt.Errorf("guard config invalid: %w", err)
	}

	// MaxBodyBytes comes from config (default 1 MiB via parseGuardConfig),
	// but fall back to the shim default so a zero value never silently
	// disables the body pre-scan.
	maxBodyBytes := conf.MaxBodyBytes
	if maxBodyBytes <= 0 {
		maxBodyBytes = nethttpguard.DefaultMaxBodyBytes
	}

	engineCfg, err := guardcore.NewSecurityConfig(func(c *guardcore.SecurityConfig) {
		c.EnableRedis = conf.RedisURL != ""
		c.RedisURL = conf.RedisURL
		c.RedisPrefix = conf.RedisPrefix
		// When redis_url is explicitly configured a Redis outage must not
		// silently drop shared rate limit and ban state: fail closed instead.
		c.RedisFailOpen = false
		// Records what the guard blocks (and, in passive mode, what it would
		// block) for rate_limit and suspicious_activity checks.
		c.OnBlock = logOnBlock

		c.PassiveMode = conf.Passive
		c.EnableRateLimiting = conf.RateLimit > 0
		if conf.RateLimit > 0 {
			c.RateLimit = conf.RateLimit
			c.RateLimitWindow = conf.RateLimitWindow
		}
		c.EnablePenetrationDetection = true
		c.EnableIPBanning = !conf.Passive

		c.Whitelist = conf.Whitelist
		c.Blacklist = conf.Blacklist
		c.BlockCloudProviders = conf.BlockCloudProviders
		if len(conf.ExcludePaths) > 0 {
			c.ExcludePaths = conf.ExcludePaths
		}
	})
	if err != nil {
		return nil, fmt.Errorf("guard config invalid: %w", err)
	}

	engine, err := guardcore.NewEngine(engineCfg)
	if err != nil {
		return nil, fmt.Errorf("guard engine init failed: %w", err)
	}
	if err = engine.Initialize(); err != nil {
		return nil, fmt.Errorf("guard engine startup failed: %w", err)
	}

	wrap, err := nethttpguard.New(engine, nethttpguard.WithMaxBodyBytes(maxBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("guard middleware init failed: %w", err)
	}

	exclusions := newPathExclusions(conf.ExcludePaths)
	whitelist := newIPList(conf.Whitelist)

	return negroni.HandlerFunc(func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		// The engine still enforces some checks (rate limits, IP policy) on
		// excluded paths; exclude_paths promises "skips guard checks", so
		// short-circuit before the engine entirely.
		if exclusions.matches(r.URL.Path) {
			next(w, r)
			return
		}

		applyTrustedProxy(r, trustedProxies)

		// The engine runs before the body scan so its verdicts take
		// precedence: whitelisted clients keep full trust, blacklisted peers
		// get 403, over-limit clients get 429, and only then does a body-only
		// payload earn the 400. guard-core-go v4 has no exported API to feed
		// these findings into its violation counter, so body-only threats do
		// not contribute to auto-ban accounting.
		wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Whitelisted clients are fully trusted by the engine (its IP,
			// rate limit, and penetration checks all skip them), so the body
			// scan does not apply to them either. Membership is checked per
			// client rather than skipping the scan whenever a whitelist is
			// configured, so body inspection still covers everyone else.
			clientIP := requestClientHost(r)
			if !whitelist.contains(clientIP) && scanRequestBody(r, maxBodyBytes, clientIP, conf.Passive) {
				// Mirror the engine's suspicious_activity rejection shape.
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(guardcore.SuspiciousBlockedMsg))
				return
			}
			next(w, r)
		})).ServeHTTP(w, r)
	}), nil
}

// invalidGuardConfigMiddleware responds 500 to every request when the guard
// was explicitly enabled but its config is unusable: a security layer the
// operator asked for must never silently degrade to absent.
func invalidGuardConfigMiddleware(err error) negroni.Handler {
	return negroni.HandlerFunc(func(w http.ResponseWriter, _ *http.Request, _ http.HandlerFunc) {
		http.Error(w, fmt.Sprintf(jsonErrFormat, err.Error()), http.StatusInternalServerError)
	})
}

// logOnBlock adapts the guard-core OnBlock hook to slog. The engine-provided
// payload carries only metadata (check name, reason, client IP, method, path);
// request secrets such as headers, query strings, or body content never reach
// this logger.
func logOnBlock(_ guardcore.Request, payload map[string]any) {
	logGuardBlockEvent(
		boolVal(payload["passive_mode"]),
		strVal(payload["check_name"]),
		strVal(payload["reason"]),
		strVal(payload["trigger_info"]),
		strVal(payload["client_ip"]),
		strVal(payload["method"]),
		strVal(payload["path"]),
	)
}

// logGuardBlockEvent emits one structured log line for a blocked (or, in
// passive preview mode, would-have-blocked) request.
func logGuardBlockEvent(passive bool, check, reason, trigger, clientIP, method, path string) {
	slog.Warn("guard request blocked",
		"passive", passive,
		"check", check,
		"reason", reason,
		"trigger", trigger,
		"client_ip", clientIP,
		"method", method,
		"path", path,
	)
}

func strVal(v any) string {
	s, _ := v.(string)
	return s
}

func boolVal(v any) bool {
	b, _ := v.(bool)
	return b
}

// trustedProxySet holds pre-parsed trusted proxy IPs and CIDRs.
type trustedProxySet struct {
	ips  map[string]bool
	nets []*net.IPNet
}

// newTrustedProxySet parses guard.trusted_proxies entries. Any entry that is
// neither an IP nor a CIDR is a config error so the guard fails closed at
// startup rather than mis-resolving clients at request time.
func newTrustedProxySet(entries []string) (*trustedProxySet, error) {
	if len(entries) == 0 {
		return nil, nil
	}
	set := &trustedProxySet{ips: make(map[string]bool, len(entries))}
	for i, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if ip := net.ParseIP(entry); ip != nil {
			set.ips[ip.String()] = true
			continue
		}
		_, cidr, err := net.ParseCIDR(entry)
		if err != nil {
			return nil, fmt.Errorf("trusted_proxies[%d]: %q is not an IP or CIDR", i, entry)
		}
		set.nets = append(set.nets, cidr)
	}
	return set, nil
}

// contains reports whether ip belongs to the trusted set.
func (t *trustedProxySet) contains(ip net.IP) bool {
	if t == nil || ip == nil {
		return false
	}
	if t.ips[ip.String()] {
		return true
	}
	for _, n := range t.nets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// applyTrustedProxy rewrites r.RemoteAddr to the real client IP when the
// direct peer is a trusted proxy and X-Forwarded-For is present. Resolution
// walks the hop list from right to left and picks the rightmost entry that is
// not itself a trusted proxy (standard rightmost-untrusted rule); when every
// hop is trusted the leftmost entry is used. When the peer is not trusted the
// forwarded header is attacker-controlled and ignored entirely, leaving
// RemoteAddr untouched. The rewritten address keeps a :port suffix
// (net.JoinHostPort) so downstream net.SplitHostPort keeps working.
func applyTrustedProxy(r *http.Request, trusted *trustedProxySet) {
	if trusted == nil {
		return
	}
	peer := net.ParseIP(requestClientHost(r))
	if !trusted.contains(peer) {
		return
	}
	// Header.Values joins every X-Forwarded-For line: some proxies append a
	// new header line instead of joining their hop into the existing value,
	// and taking only the first line would trust client-supplied data.
	xff := strings.Join(r.Header.Values("X-Forwarded-For"), ",")
	if strings.TrimSpace(xff) == "" {
		return
	}
	hops := strings.Split(xff, ",")
	for i := len(hops) - 1; i >= 0; i-- {
		hop := net.ParseIP(strings.TrimSpace(hops[i]))
		if hop == nil {
			// Malformed hop: the chain cannot be trusted, keep the peer.
			return
		}
		if trusted.contains(hop) {
			continue
		}
		r.RemoteAddr = net.JoinHostPort(hop.String(), "0")
		return
	}
	// All hops are trusted proxies: fall back to the leftmost entry.
	leftmost := net.ParseIP(strings.TrimSpace(hops[0]))
	if leftmost == nil {
		return
	}
	r.RemoteAddr = net.JoinHostPort(leftmost.String(), "0")
}

// requestClientHost extracts the bare host from RemoteAddr, tolerating
// addresses without a port.
func requestClientHost(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// pathExclusions holds normalized exclude_paths entries.
type pathExclusions struct {
	entries []string
}

// newPathExclusions drops empty entries and keeps the rest as-is.
func newPathExclusions(entries []string) *pathExclusions {
	kept := make([]string, 0, len(entries))
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		kept = append(kept, entry)
	}
	return &pathExclusions{entries: kept}
}

// matches reports whether path falls inside one of the exclusion subtrees,
// using the same subtree-or-equal semantics as the engine's exclude path
// matcher: an entry "/health" covers "/health" and anything under "/health/".
func (p *pathExclusions) matches(path string) bool {
	for _, entry := range p.entries {
		if entry == "/" || path == entry || strings.HasPrefix(path, entry+"/") {
			return true
		}
	}
	return false
}

// ipList is a pre-parsed whitelist entry set mirroring the engine's
// ipMatchesList semantics: entries are exact IPs or CIDRs, and matching
// accepts both the address and its IPv4-mapped form.
type ipList struct {
	ips  map[string]bool
	nets []netip.Prefix
}

// newIPList parses whitelist entries. The engine already validated them at
// startup, so entries that fail to parse here are skipped rather than fatal.
func newIPList(entries []string) *ipList {
	list := &ipList{ips: make(map[string]bool, len(entries))}
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if strings.Contains(entry, "/") {
			if prefix, err := netip.ParsePrefix(entry); err == nil {
				list.nets = append(list.nets, prefix)
			}
			continue
		}
		if addr, err := netip.ParseAddr(entry); err == nil {
			list.ips[addr.String()] = true
		}
	}
	return list
}

// contains reports whether ip belongs to the list.
func (l *ipList) contains(ip string) bool {
	if l == nil {
		return false
	}
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return false
	}
	if l.ips[addr.String()] {
		return true
	}
	for _, prefix := range l.nets {
		if prefix.Contains(addr.Unmap()) || prefix.Contains(addr) {
			return true
		}
	}
	return false
}

// scanRequestBody inspects up to maxBodyBytes of the request body with the
// engine's Detect, so body-only attack payloads get the same pattern source
// as URL, query, and header inspection. It reports whether the request must
// be rejected; in passive mode it only logs and lets the request through.
//
// The body is restored for downstream handlers: bytes already consumed are
// replayed from memory ahead of the untouched remainder of the stream, and
// the nethttp-guard shim then wraps and replays that restored reader.
func scanRequestBody(r *http.Request, maxBodyBytes int64, clientIP string, passive bool) bool {
	if r.Body == nil || maxBodyBytes <= 0 {
		return false
	}
	prefix, readErr := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes))
	r.Body = io.NopCloser(io.MultiReader(bytes.NewReader(prefix), r.Body))
	if readErr != nil {
		slog.Warn("guard body prescan read failed, skipping scan", "err", readErr)
		return false
	}
	if len(prefix) == 0 {
		return false
	}
	result := guardcore.Detect(string(prefix), clientIP, "body")
	if !result.IsThreat {
		return false
	}
	categories := make([]string, 0, len(result.Threats))
	seen := make(map[string]bool, len(result.Threats))
	for _, threat := range result.Threats {
		category, _ := threat["category"].(string)
		if category == "" || seen[category] {
			continue
		}
		seen[category] = true
		categories = append(categories, category)
	}
	trigger := strings.Join(categories, ",")
	logGuardBlockEvent(passive, "suspicious_activity",
		"suspicious body content", trigger, clientIP, r.Method, r.URL.Path)
	return !passive
}
