package config

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/spf13/viper"
)

const (
	// EnvConfigJSON carries application config as one JSON document, so a
	// container can run without a mounted prest.toml.
	EnvConfigJSON = "PREST_CONFIG_JSON"
	// EnvConfigJSONB64 is the base64 form of EnvConfigJSON, for platforms that
	// mangle multi-line env values. It is read only when EnvConfigJSON is unset.
	EnvConfigJSONB64 = "PREST_CONFIG_JSON_B64"

	configJSONWarnSize = 256 * 1024  // 256 KiB
	configJSONMaxSize  = 1024 * 1024 // 1 MiB, about the Kubernetes Secret limit
)

// infraKeys are the listener, database and filesystem settings that stay in
// their own env vars. They are never taken from the JSON document, so a stale
// export cannot move the port or point pREST at another database.
var infraKeys = []string{
	"http",
	"https",
	"engine",
	"pg",
	"mysql",
	"databases",
	"queries.location",
}

// isInfraKey reports whether the dotted key is, or sits below, an infra key.
func isInfraKey(key string) bool {
	for _, infra := range infraKeys {
		if key == infra || strings.HasPrefix(key, infra+".") {
			return true
		}
	}
	return false
}

// loadConfigJSONFromEnv merges the application config in PREST_CONFIG_JSON,
// or PREST_CONFIG_JSON_B64, into v. Both unset or empty is a no-op. A value
// that cannot be decoded fails, because a misconfigured ACL or JWT section
// must not fall back to a weaker posture.
func loadConfigJSONFromEnv(v *viper.Viper) error {
	raw, source, err := configJSONFromEnv()
	if err != nil || raw == nil {
		return err
	}
	if err := applyConfigJSON(v, raw); err != nil {
		return fmt.Errorf("parse %s: %w", source, err)
	}
	return nil
}

// configJSONFromEnv returns the JSON document and the env var it came from.
// It returns nil when neither variable holds a value.
func configJSONFromEnv() ([]byte, string, error) {
	plain := strings.TrimSpace(os.Getenv(EnvConfigJSON))
	encoded := strings.TrimSpace(os.Getenv(EnvConfigJSONB64))
	switch {
	case plain != "":
		if encoded != "" {
			slog.Warn("config json: both env vars set, using the plain one",
				"used", EnvConfigJSON, "ignored", EnvConfigJSONB64)
		}
		return []byte(plain), EnvConfigJSON, nil
	case encoded != "":
		raw, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return nil, EnvConfigJSONB64, fmt.Errorf("decode %s: %w", EnvConfigJSONB64, err)
		}
		return raw, EnvConfigJSONB64, nil
	default:
		return nil, "", nil
	}
}

// applyConfigJSON merges a JSON object of application config into v.
//
// The document lands in the same layer as prest.toml: it overrides file
// values key by key, an array replaces the file's array whole, and PREST_*
// env vars and infra env vars still override it. Infra keys are dropped with
// a warning, and a null value counts as absent.
func applyConfigJSON(v *viper.Viper, raw []byte) error {
	if len(raw) > configJSONMaxSize {
		return fmt.Errorf("document is %d bytes, the limit is %d", len(raw), configJSONMaxSize)
	}
	if len(raw) > configJSONWarnSize {
		slog.Warn("config json: large document", "bytes", len(raw), "warn_bytes", configJSONWarnSize)
	}

	var doc any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return err
	}
	root, ok := doc.(map[string]any)
	if !ok {
		return errors.New("document must be a JSON object")
	}
	if err := filterAppConfig(v, "", root); err != nil {
		return err
	}
	if err := v.MergeConfigMap(root); err != nil {
		return err
	}
	slog.Debug("config json applied", "keys", slices.Sorted(maps.Keys(root)))
	return nil
}

// filterAppConfig walks one JSON object in place. It drops infra keys and
// nulls, and rejects a key that would silently not apply: a dotted key, which
// viper would not merge into the nested table, a value where pREST expects a
// table, or a table where it expects a value. Arrays are leaves; their
// elements are not walked.
func filterAppConfig(v *viper.Viper, prefix string, obj map[string]any) error {
	for name, value := range obj {
		key := strings.ToLower(name)
		if prefix != "" {
			key = prefix + "." + key
		}
		if strings.Contains(name, ".") {
			return fmt.Errorf("key %q: use nested objects, not dotted keys", key)
		}
		if isInfraKey(key) {
			slog.Warn("config json: ignoring infrastructure key (use env vars)", "key", key)
			delete(obj, name)
			continue
		}
		if value == nil {
			delete(obj, name)
			continue
		}
		current := v.Get(key)
		_, currentTable := current.(map[string]any)
		table, isTable := value.(map[string]any)
		switch {
		case isTable && current != nil && !currentTable:
			return fmt.Errorf("key %q must not be an object", key)
		case !isTable && currentTable:
			return fmt.Errorf("key %q must be an object", key)
		case isTable:
			if err := filterAppConfig(v, key, table); err != nil {
				return err
			}
		}
	}
	return nil
}
