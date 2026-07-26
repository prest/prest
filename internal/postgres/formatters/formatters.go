package formatters

import (
	"fmt"
	"strconv"
	"strings"
)

// FormatArray format slice to a postgres array format
// today support a slice of string, int and fmt.Stringer
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
	case fmt.Stringer:
		return FormatArray(value.String())
	}
	return ""
}
