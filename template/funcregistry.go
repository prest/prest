package template

import (
	"text/template"
)

// FuncRegistry registry func for templates
type FuncRegistry struct {
	TemplateData map[string]string
}

// RegistryAllFuncs for template
func (fr *FuncRegistry) RegistryAllFuncs() (funcs template.FuncMap) {
	funcs = template.FuncMap{
		"isSet":          fr.isSet,
		"defaultOrValue": fr.defaultOrValue,
	}
	return
}

func (fr *FuncRegistry) isSet(key string) (ok bool) {
	_, ok = fr.TemplateData[key]
	return
}

func (fr *FuncRegistry) defaultOrValue(key, defaultValue string) (value string) {
	if ok := fr.isSet(key); !ok {
		fr.TemplateData[key] = defaultValue
	}
	value = fr.TemplateData[key]
	return
}
