package config

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/spf13/viper"
)

func setJWTDefaults(v *viper.Viper) {
	v.SetDefault("jwt.default", false)
	v.SetDefault("jwt.algo", "HS256")
	v.SetDefault("jwt.wellknownurl", "")
	v.SetDefault("jwt.jwks", "")
	v.SetDefault("jwt.whitelist", []string{`^\/auth$`})
}

func ensureJWTConfig(cfg *Prest) {
	if min := hmacMinKeyBytes(cfg.JWTAlgo); min > 0 && cfg.JWTKey != "" && len(cfg.JWTKey) < min {
		slog.Error("jwt.key too short for HMAC algorithm",
			"algo", cfg.JWTAlgo, "got", len(cfg.JWTKey), "want", min, "err", ErrJWTKeyTooShort)
		// Treat an undersized HMAC key as unusable verification material so
		// go-jose/v4 cannot reject (or worse, surprise) at request time.
		cfg.JWTKey = ""
	}
	if cfg.AuthEnabled && cfg.JWTKey == "" {
		slog.Error("auth disabled: jwt.key is empty", "err", ErrAuthEnabledNoJWTKey)
		cfg.AuthEnabled = false
	}
	if !cfg.EnableDefaultJWT || cfg.Debug {
		return
	}
	if cfg.JWTKey != "" || cfg.JWTJWKS != "" || cfg.JWTWellKnownURL != "" {
		return
	}
	slog.Error(
		"default JWT middleware disabled: no verification material",
		"err", ErrJWTDefaultEnabledNoKey)
	cfg.EnableDefaultJWT = false
}

// hmacMinKeyBytes returns the RFC 7518 minimum HMAC key size for algo, or 0
// when algo is not HMAC (RS*/ES*/PS*/EdDSA) and jwt.key is not used as a MAC key.
func hmacMinKeyBytes(algo string) int {
	switch strings.ToUpper(algo) {
	case "HS384":
		return 48
	case "HS512":
		return 64
	case "HS256", "":
		// Empty matches viper default jwt.algo = HS256.
		return 32
	default:
		return 0
	}
}

// ErrJWTDefaultEnabledNoKey is returned when the default JWT middleware is
// enabled but no verification material (HMAC key, JWKS or .well-known URL) was
// provided. This guards against accidentally serving requests with an empty
// HMAC key, which would let any client forge bearer tokens. See GHSA-fj7v-859r-2fm4.
var ErrJWTDefaultEnabledNoKey = errors.New(
	"jwt.default is enabled but no verification material was provided " +
		"(set jwt.key, jwt.jwks or jwt.wellknownurl, or disable jwt.default)")

// ErrAuthEnabledNoJWTKey is returned when basic auth is enabled but jwt.key
// is empty. AuthMiddleware uses the same []byte(JWTKey) to verify HS256
// tokens, so an empty key opens the same auth-bypass as the default JWT
// middleware. See GHSA-fj7v-859r-2fm4.
var ErrAuthEnabledNoJWTKey = errors.New(
	"auth.enabled is true but jwt.key is empty (required to verify HS256 tokens)")

// ErrJWTKeyTooShort is returned when jwt.key is shorter than the RFC 7518
// minimum for the configured HMAC algorithm (HS256: 32 bytes, HS384: 48,
// HS512: 64). go-jose/v4 rejects undersized HMAC keys at sign/verify time.
var ErrJWTKeyTooShort = errors.New(
	"jwt.key is shorter than the minimum required for the configured HMAC algorithm")

// fetchJWKS tries to get the JWKS from the URL in the config
// redactURL returns a log-safe "scheme://host/path" form of raw, dropping
// userinfo, query, and fragment which may carry credentials or tokens. It
// returns "" when raw cannot be parsed, so no unsanitized value is ever logged.
func redactURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return (&url.URL{Scheme: u.Scheme, Host: u.Host, Path: u.Path}).String()
}

func fetchJWKS(cfg *Prest) {
	if cfg.JWTWellKnownURL == "" {
		slog.Debug("no JWT WellKnown url found, skipping")
		return
	}
	if cfg.JWTJWKS != "" {
		slog.Debug("JWKS already set, skipping")
		return
	}

	// Call provider to obtain .well-known config
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	r, err := client.Get(cfg.JWTWellKnownURL)
	if err != nil {
		slog.Error("Cannot get .well-known configuration", "url", cfg.JWTWellKnownURL, "err", err)
		return
	}
	defer r.Body.Close()

	var wellKnown map[string]interface{}
	err = json.NewDecoder(r.Body).Decode(&wellKnown)
	if err != nil {
		slog.Error("Failed to decode JSON", "err", err)
		return
	}

	//Retrieve the JWKS from the endpoint
	uri, ok := wellKnown["jwks_uri"].(string)
	if !ok {
		slog.Error("Unable to convert .WellKnown configuration of jwks_uri to a string")
		return
	}

	jwksResp, err := client.Get(uri)
	if err != nil {
		slog.Error("Failed to fetch JWK", "err", err)
		return
	}
	defer jwksResp.Body.Close()

	if jwksResp.StatusCode < 200 || jwksResp.StatusCode >= 300 {
		slog.Error("JWKS endpoint returned non-success status", "status", jwksResp.StatusCode, "url", redactURL(uri))
		return
	}

	// Cap the JWKS body to guard against oversized or hostile responses.
	const maxJWKSBytes = 1 << 20 // 1 MiB
	jwksBody, err := io.ReadAll(io.LimitReader(jwksResp.Body, maxJWKSBytes+1))
	if err != nil {
		slog.Error("Failed to read JWKS response body", "err", err)
		return
	}
	if len(jwksBody) > maxJWKSBytes {
		slog.Error("JWKS response body exceeds size limit", "limit", maxJWKSBytes)
		return
	}

	JWKSet, err := jwk.Parse(jwksBody)
	if err != nil {
		slog.Error("Failed to parse JWK", "err", err)
		return
	}

	//Convert set to json string
	jwkSetJSON, err := json.Marshal(JWKSet)
	if err != nil {
		slog.Error("Failed to marshal JWKSet to JSON", "err", err)
		return
	}

	cfg.JWTJWKS = string(jwkSetJSON)
}
