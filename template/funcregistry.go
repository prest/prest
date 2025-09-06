package template

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"text/template"
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
		"sqlVal":         fr.sqlVal,
		"sqlList":        fr.sqlList,
		"ident":          fr.ident,
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

// sqlVal returns a positional placeholder for a single value and stores it in Args
func (fr *FuncRegistry) sqlVal(key string) string {
	v := fr.TemplateData[key]
	fr.Args = append(fr.Args, v)
	fr.next++
	return fmt.Sprintf("$%d", fr.next)
}

// sqlList returns a parenthesized, comma-separated list of placeholders for a slice value
func (fr *FuncRegistry) sqlList(key string) string {
	if s, ok := fr.TemplateData[key].([]string); ok {
		ph := make([]string, len(s))
		for i := range s {
			fr.Args = append(fr.Args, s[i])
			fr.next++
			ph[i] = fmt.Sprintf("$%d", fr.next)
		}
		return fmt.Sprintf("(%s)", strings.Join(ph, ","))
	}
	fr.Args = append(fr.Args, fr.TemplateData[key])
	fr.next++
	return fmt.Sprintf("($%d)", fr.next)
}

var identRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)*$`)

// ident validates and safely quotes an identifier (optionally dotted path)
func (fr *FuncRegistry) ident(key string) (string, error) {
	s, _ := fr.TemplateData[key].(string)
	if !identRe.MatchString(s) {
		return "", fmt.Errorf("invalid identifier: %s", s)
	}
	parts := strings.Split(s, ".")
	for i := range parts {
		parts[i] = `"` + parts[i] + `"`
	}
	return strings.Join(parts, "."), nil
}
