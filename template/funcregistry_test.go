package template

import (
	"strings"
	"testing"
)

func TestIsSet(t *testing.T) {
	data := make(map[string]interface{})
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
	data := make(map[string]interface{})
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

func TestInFormat(t *testing.T) {
	data := make(map[string]interface{})
	data["test"] = []string{"test1", "test2"}
	funcs := &FuncRegistry{TemplateData: data}
	query := funcs.inFormat("test")
	if query != "('test1', 'test2')" {
		t.Errorf("expected ('test1', 'test2'), but got %s", query)
	}
	data["test"] = "test1"
	funcs = &FuncRegistry{TemplateData: data}
	query = funcs.inFormat("test")
	if query != "('test1')" {
		t.Errorf("expected ('test1'), but got %s", query)
	}
}

func TestSplit(t *testing.T) {
	data := make(map[string]interface{})
	list3itens := "test1,test2,test3"
	data["list3itens"] = list3itens
	funcs := &FuncRegistry{TemplateData: data}
	query := funcs.split(list3itens, ",")
	s := strings.Split(list3itens, ",")
	if len(query) != 3 {
		t.Errorf("expected (3), but got %d", len(query))
	}
	if len(query) != len(s) {
		t.Errorf("expected (%d), but got %d", len(query), len(s))
	}
}

func TestRegistryAllFuncs(t *testing.T) {
	data := make(map[string]interface{})
	data["test"] = "testValue"
	funcs := &FuncRegistry{TemplateData: data}

	fmap := funcs.RegistryAllFuncs()
	_, ok := fmap["isSet"]
	if !ok {
		t.Error("func `isSet` is not registred")
	}
	_, ok = fmap["defaultOrValue"]
	if !ok {
		t.Error("func `defaultOrValue` is not registred")
	}
	_, ok = fmap["inFormat"]
	if !ok {
		t.Error("func `in` is not registred")
	}
	_, ok = fmap["split"]
	if !ok {
		t.Error("func `split` is not registred")
	}
}

func TestUnEscape(t *testing.T) {
	data := make(map[string]interface{})
	uri := "test1%20test2%20test3"
	data["test"] = uri
	funcs := &FuncRegistry{TemplateData: data}
	value := funcs.unEscape(uri)
	if value != "test1 test2 test3" {
		t.Errorf("expected 'test1 test2 test3', bug got %s", value)
	}
}
