package formatters

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// FormatArray format slice to a postgres array format
// today support a slice of string, int, float64, json.Number and fmt.Stringer
func FormatArray(value interface{}) string {
	var aux string
	var check = func(aux string, value interface{}) (ret string) {
		if aux != "" {
			aux += ","
		}
		ret = aux + FormatArray(value)
		return
	}
	switch value := value.(type) {
	case []fmt.Stringer:
		for _, v := range value {
			aux = check(aux, v)
		}
		return "{" + aux + "}"
	case []interface{}:
		for _, v := range value {
			aux = check(aux, v)
		}
		return "{" + aux + "}"
	case []string:
		for _, v := range value {
			aux = check(aux, v)
		}
		return "{" + aux + "}"
	case []int:
		for _, v := range value {
			aux = check(aux, v)
		}
		return "{" + aux + "}"
	case string:
		aux := value
		aux = strings.Replace(aux, `\`, `\\`, -1)
		aux = strings.Replace(aux, `"`, `\"`, -1)
		return `"` + aux + `"`
	case int:
		return strconv.Itoa(value)
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64)
	case json.Number:
		// unquoted so Postgres reads it as a numeric element, digits intact
		return string(value)
	case fmt.Stringer:
		return FormatArray(value.String())
	}
	return ""
}
