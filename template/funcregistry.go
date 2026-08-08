package template

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"text/template"

	"github.com/prest/prest/v2/internal/ident"
)

// FuncRegistry registry func for templates
type FuncRegistry struct {
	TemplateData map[string]interface{}
	Args         []interface{}
	next         int
}

// RegistryAllFuncs for template
func (fr *FuncRegistry) RegistryAllFuncs() (funcs template.FuncMap) {
	funcs = template.FuncMap{
		"isSet":          fr.isSet,
		"defaultOrValue": fr.defaultOrValue,
		"inFormat":       fr.inFormat,
		"unEscape":       fr.unEscape,
		"split":          fr.split,
		"limitOffset":    fr.limitOffset,
		// secure SQL helpers
		"sqlVal":  fr.sqlVal,
		"sqlList": fr.sqlList,
		"ident":   fr.ident,
	}
	return
}

func (fr *FuncRegistry) isSet(key string) (ok bool) {
	_, ok = fr.TemplateData[key]
	return
}

func (fr *FuncRegistry) defaultOrValue(key, defaultValue string) (value interface{}) {
	if ok := fr.isSet(key); !ok {
		fr.TemplateData[key] = defaultValue
	}
	value = fr.TemplateData[key]
	return
}

func (fr *FuncRegistry) inFormat(key string) (query string) {
	items, ok := fr.TemplateData[key].([]string)
	if !ok {
		query = fmt.Sprintf("('%v')", fr.TemplateData[key])
		return
	}
	query = fmt.Sprintf("('%s')", strings.Join(items, "', '"))
	return
}

func (fr *FuncRegistry) unEscape(key string) (value string) {
	value, _ = url.QueryUnescape(key)
	return
}

func (fr *FuncRegistry) split(orig, sep string) (values []string) {
	values = strings.Split(orig, sep)
	return
}

// LimitOffset create and format limit query (offset, SQL ANSI)
func LimitOffset(pageNumberStr, pageSizeStr string) (paginatedQuery string, err error) {
	pageNumber, err := strconv.Atoi(pageNumberStr)
	if err != nil {
		return
	}
	pageSize, err := strconv.Atoi(pageSizeStr)
	if err != nil {
		return
	}
	if pageNumber-1 < 0 {
		pageNumber = 1
	}
	paginatedQuery = fmt.Sprintf("LIMIT %d OFFSET(%d - 1) * %d", pageSize, pageNumber, pageSize)
	return
}

func (fr *FuncRegistry) limitOffset(pageNumber, pageSize string) (value string) {
	value, err := LimitOffset(pageNumber, pageSize)
	if err != nil {
		value = ""
	}
	return
}

// Reserved TemplateData keys holding the caller's unscreened values, published by
// the controller. Kept in sync with controllers.rawParamKey / rawHeaderKey.
const (
	rawParamKey  = "_param"
	rawHeaderKey = "_header"
)

// headerKeyPrefix addresses a header from the binding helpers, e.g.
// {{sqlVal "header.X-Application"}}, mirroring the `header` map templates read
// for inline interpolation.
const headerKeyPrefix = "header."

// boundValue resolves the value to bind for key.
//
// The controller screens values for SQL syntax before a template sees them,
// because an interpolated value becomes part of the SQL text. A bound value never
// does — the driver sends it separately — so binding the screened version would
// discard the caller's data for no safety gain: searching for a phrase containing
// a SQL keyword would bind an empty string (issue #1030). The raw maps are
// therefore consulted first, falling back to TemplateData for keys the template
// author set itself (defaultOrValue, split, and so on).
//
// Credential headers are already blanked in the raw map by the controller, so
// binding cannot be used to smuggle one into a query.
func (fr *FuncRegistry) boundValue(key string) interface{} {
	if name, ok := strings.CutPrefix(key, headerKeyPrefix); ok {
		if headers, ok := fr.TemplateData[rawHeaderKey].(map[string]interface{}); ok {
			if v, ok := headers[name]; ok {
				return v
			}
		}
		if headers, ok := fr.TemplateData["header"].(map[string]interface{}); ok {
			return headers[name]
		}
		return nil
	}
	if params, ok := fr.TemplateData[rawParamKey].(map[string]interface{}); ok {
		if v, ok := params[key]; ok {
			return v
		}
	}
	return fr.TemplateData[key]
}

// sqlVal returns a positional placeholder for a single value and stores it in Args
func (fr *FuncRegistry) sqlVal(key string) string {
	fr.Args = append(fr.Args, fr.boundValue(key))
	fr.next++
	return fmt.Sprintf("$%d", fr.next)
}

// sqlList returns a parenthesized, comma-separated list of placeholders for a slice value
func (fr *FuncRegistry) sqlList(key string) string {
	if s, ok := fr.boundValue(key).([]string); ok {
		ph := make([]string, len(s))
		for i := range s {
			fr.Args = append(fr.Args, s[i])
			fr.next++
			ph[i] = fmt.Sprintf("$%d", fr.next)
		}
		return fmt.Sprintf("(%s)", strings.Join(ph, ","))
	}
	fr.Args = append(fr.Args, fr.boundValue(key))
	fr.next++
	return fmt.Sprintf("($%d)", fr.next)
}

// ident validates and safely quotes an identifier (optionally dotted path).
//
// Resolves the raw value like the binding helpers: an identifier cannot be bound,
// but ident.Quote validates it against a strict charset and quotes it, which is a
// stronger guarantee than the keyword screen — a table legitimately named `order`
// is quoted rather than blanked. An invalid identifier returns an error, which
// aborts template rendering.
func (fr *FuncRegistry) ident(key string) (string, error) {
	s, _ := fr.boundValue(key).(string)
	return ident.Quote(s)
}
