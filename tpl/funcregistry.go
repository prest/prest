package tpl

import (
	"text/template"
)

// TemplateFuncRegistry registry all template func
type TemplateFuncRegistry struct {
	TplData map[string]string
}

// AllFuncs return
func (tfr *TemplateFuncRegistry) AllFuncs() (funcs template.FuncMap) {
	funcs = template.FuncMap{
		"isset":          tfr.isset,
		"valueOrDefault": tfr.valueOrDefault,
	}
	return
}

func (tfr *TemplateFuncRegistry) isset(key string) (ok bool) {
	_, ok = tfr.TplData[key]
	return
}

func (tfr *TemplateFuncRegistry) valueOrDefault(key, defaultValue string) (value interface{}) {
	if tfr.isset(key) {
		value = tfr.TplData[key]
		return
	}
	tfr.TplData[key] = defaultValue
	value = defaultValue
	return
}
