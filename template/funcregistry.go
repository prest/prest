package template

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"text/template"
)

// FuncRegistry registry func for templates
type FuncRegistry struct {
	TemplateData map[string]interface{}
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
