package template

import (
	"fmt"
	"strings"
	"testing"
)

func TestIsSet(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

	data := make(map[string]interface{})
	uri := "test1%20test2%20test3"
	data["test"] = uri
	funcs := &FuncRegistry{TemplateData: data}
	value := funcs.unEscape(uri)
	if value != "test1 test2 test3" {
		t.Errorf("expected 'test1 test2 test3', bug got %s", value)
	}
}

func TestLimitOffset(t *testing.T) {
	t.Parallel()

	data := make(map[string]interface{})
	pageNumber := 1
	pageSize := 10
	data["_page"] = pageNumber
	data["_page_size"] = pageSize
	funcs := &FuncRegistry{TemplateData: data}
	value := funcs.limitOffset(fmt.Sprint(pageNumber), fmt.Sprint(pageSize))
	expected := fmt.Sprintf("LIMIT %d OFFSET(%d - 1) * %d", pageSize, pageNumber, pageSize)
	if value != expected {
		t.Errorf("expected '%s', bug got %s", expected, value)
	}

	value = funcs.limitOffset("0", fmt.Sprint(pageSize))
	if value != expected {
		t.Errorf("expected '%s', bug got %s", expected, value)
	}

	value = funcs.limitOffset("a", fmt.Sprint(pageSize))
	if value != "" {
		t.Errorf("expected '%s', bug got %s", "", value)
	}

	value = funcs.limitOffset(fmt.Sprint(pageNumber), "a")
	if value != "" {
		t.Errorf("expected '%s', bug got %s", "", value)
	}
}

// The controller screens values for SQL syntax before they reach a template,
// because an interpolated value lands in the SQL text. A bound value does not:
// the driver sends it out of band, so composition is impossible by construction.
// sqlVal must therefore bind the caller's real value, not the screened one —
// otherwise searching for "compra do mes" binds an empty string (issue #1030).
func TestSQLVal_BindsRawValue(t *testing.T) {
	t.Parallel()

	funcs := &FuncRegistry{
		TemplateData: map[string]interface{}{
			"phrase":  "", // screened to empty by the controller
			"_param":  map[string]interface{}{"phrase": "compra do mes"},
			"another": "kept",
		},
	}

	if got := funcs.sqlVal("phrase"); got != "$1" {
		t.Errorf("expected placeholder $1, got %s", got)
	}
	if got := funcs.sqlVal("another"); got != "$2" {
		t.Errorf("expected placeholder $2, got %s", got)
	}

	if len(funcs.Args) != 2 {
		t.Fatalf("expected 2 bound args, got %d", len(funcs.Args))
	}
	if funcs.Args[0] != "compra do mes" {
		t.Errorf("expected the raw value to be bound, got %v", funcs.Args[0])
	}
	// A key with no raw entry falls back to TemplateData, so template-author
	// supplied keys keep working.
	if funcs.Args[1] != "kept" {
		t.Errorf("expected fallback to TemplateData, got %v", funcs.Args[1])
	}
}

func TestSQLVal_BindsRawHeaderValue(t *testing.T) {
	t.Parallel()

	funcs := &FuncRegistry{
		TemplateData: map[string]interface{}{
			"_header": map[string]interface{}{"X-Application": "prest do brasil"},
		},
	}

	if got := funcs.sqlVal("header.X-Application"); got != "$1" {
		t.Errorf("expected placeholder $1, got %s", got)
	}
	if len(funcs.Args) != 1 || funcs.Args[0] != "prest do brasil" {
		t.Errorf("expected the raw header value to be bound, got %v", funcs.Args)
	}
}

func TestSQLList_BindsRawValues(t *testing.T) {
	t.Parallel()

	funcs := &FuncRegistry{
		TemplateData: map[string]interface{}{
			"tags":   []string{"", ""},
			"_param": map[string]interface{}{"tags": []string{"a or b", "plain"}},
		},
	}

	if got := funcs.sqlList("tags"); got != "($1,$2)" {
		t.Errorf("expected ($1,$2), got %s", got)
	}
	if len(funcs.Args) != 2 || funcs.Args[0] != "a or b" || funcs.Args[1] != "plain" {
		t.Errorf("expected the raw values to be bound in order, got %v", funcs.Args)
	}
}

func TestSQLList_NonSliceBindsSingleValue(t *testing.T) {
	t.Parallel()

	funcs := &FuncRegistry{
		TemplateData: map[string]interface{}{
			"_param": map[string]interface{}{"tag": "solo"},
		},
	}

	if got := funcs.sqlList("tag"); got != "($1)" {
		t.Errorf("expected ($1), got %s", got)
	}
	if len(funcs.Args) != 1 || funcs.Args[0] != "solo" {
		t.Errorf("expected the raw value to be bound, got %v", funcs.Args)
	}
}

func TestIdent_QuotesAndRejects(t *testing.T) {
	t.Parallel()

	funcs := &FuncRegistry{
		TemplateData: map[string]interface{}{
			"good": "public.users",
			"bad":  `users"; DROP TABLE x; --`,
		},
	}

	got, err := funcs.ident("good")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != `"public"."users"` {
		t.Errorf(`expected "public"."users", got %s`, got)
	}

	// An identifier cannot be bound, so an invalid one must abort rendering.
	if _, err := funcs.ident("bad"); err == nil {
		t.Error("expected an error for an unsafe identifier")
	}
}
