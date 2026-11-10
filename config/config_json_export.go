package config

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/viper"
)

// ExportConfigJSON converts the TOML config file at path into a document for
// PREST_CONFIG_JSON. An empty path resolves the way Load does: PREST_CONF,
// else ./prest.toml. Only the keys the file sets are exported, so defaults,
// environment variables and an existing PREST_CONFIG_JSON are not baked in.
// Infra keys are left out unless includeInfra is set. The output can hold
// secrets such as jwt.key and must be stored like one.
func ExportConfigJSON(path string, pretty, includeInfra bool) ([]byte, error) {
	if path == "" {
		path = getPrestConfFile(os.Getenv("PREST_CONF"))
	}
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("toml")
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	doc := v.AllSettings()
	if !includeInfra {
		stripInfraKeys("", doc)
	}
	if pretty {
		return json.MarshalIndent(doc, "", "  ")
	}
	return json.Marshal(doc)
}

// stripInfraKeys removes infra keys from a nested settings map in place, and
// drops any table left empty by the removal.
func stripInfraKeys(prefix string, obj map[string]any) {
	for name, value := range obj {
		key := name
		if prefix != "" {
			key = prefix + "." + name
		}
		if isInfraKey(key) {
			delete(obj, name)
			continue
		}
		if table, ok := value.(map[string]any); ok {
			stripInfraKeys(key, table)
			if len(table) == 0 {
				delete(obj, name)
			}
		}
	}
}
