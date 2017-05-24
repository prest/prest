package template

import (
	"html/template"
)

// FuncRegistry registry func for templates
type FuncRegistry struct {
	TemplateData map[string]string
}

// RegistryAllFuncs for template
func (fr *FuncRegistry) RegistryAllFuncs() (funcs template.FuncMap) {
	funcs = template.FuncMap{
		"isset":          fr.isset,
		"defaultOrValue": fr.defaultOrValue,
	}
	return
}

func (fr *FuncRegistry) isset(key string) (ok bool) {
	_, ok = fr.TemplateData[key]
	return
}

func (fr *FuncRegistry) defaultOrValue(key, defaultValue string) (value string) {
	if ok := fr.isset(key); !ok {
		fr.TemplateData[key] = defaultValue
	}
	value = fr.TemplateData[key]
	return
}
