package template

import (
	"testing"
)

func TestIsSet(t *testing.T) {
	data := make(map[string]string)
	data["test"] = "testValue"
	funcs := &FuncRegistry{TemplateData: data}
	ok := funcs.isSet("test")
	if !ok {
		t.Error("expected true but got false")
	}
	ok = funcs.isSet("testFalse")
	if ok {
		t.Error("expected false but got true")
	}
}

func TestDefaultOrValue(t *testing.T) {
	data := make(map[string]string)
	data["test"] = "testValue"
	funcs := &FuncRegistry{TemplateData: data}
	value := funcs.defaultOrValue("test", "testDefault")
	if value != "testValue" {
		t.Errorf("expected 'testValue' but got %s", value)
	}
	value = funcs.defaultOrValue("testDefaultValue", "testDefault")
	if value != "testDefault" {
		t.Errorf("expected 'testDefault' but got %s", value)
	}
}

func TestRegistryAllFuncs(t *testing.T) {
	data := make(map[string]string)
	data["test"] = "testValue"
	funcs := &FuncRegistry{TemplateData: data}

	fmap := funcs.RegistryAllFuncs()
	_, ok := fmap["isSet"]
	if !ok {
		t.Error("func isSet is not registred")
	}
	_, ok = fmap["defaultOrValue"]
	if !ok {
		t.Error("func defaultOrValue is not registred")
	}
}
