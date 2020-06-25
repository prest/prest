package mock

import (
	"bytes"
	"errors"
	"reflect"
	"sync"
	"testing"

	"github.com/prest/prest/adapters"
	"github.com/prest/prest/adapters/scanner"
	"github.com/prest/prest/config"
)

func TestMock_validate(t *testing.T) {
	type fields struct {
		mtx   *sync.RWMutex
		t     *testing.T
		Items []Item
	}
	tests := []struct {
		name   string
		fields fields
	}{
		{"validate", fields{&sync.RWMutex{}, t, []Item{Item{}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Mock{
				mtx:   tt.fields.mtx,
				t:     tt.fields.t,
				Items: tt.fields.Items,
			}
			m.validate()
		})
	}
}

func TestMock_perform(t *testing.T) {
	tests := []struct {
		name    string
		item    Item
		isQuery bool
		wantSc  adapters.Scanner
	}{
		{
			"perform body",
			Item{
				Body: []byte(`[{"test":"test"}]`),
			},
			true,
			&scanner.PrestScanner{
				Buff:    bytes.NewBuffer([]byte(`[{"test":"test"}]`)),
				IsQuery: true,
			},
		},
		{
			"perform err",
			Item{
				Error: errors.New("test error"),
			},
			false,
			&scanner.PrestScanner{
				Error: errors.New("test error"),
				Buff:  &bytes.Buffer{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Mock{
				mtx: &sync.RWMutex{},
				t:   t,
			}
			m.AddItem(tt.item.Body, tt.item.Error, tt.item.IsCount)
			if gotSc := m.perform(tt.isQuery); !reflect.DeepEqual(gotSc, tt.wantSc) {
				t.Errorf("Mock.perform() = %v, want %v", gotSc, tt.wantSc)
			}
		})
	}
}

func TestMock_TablePermissions(t *testing.T) {
	config.Load()
	config.PrestConf.AccessConf.Tables = append(config.PrestConf.AccessConf.Tables,
		config.TablesConf{
			Name:        "testpermission",
			Permissions: []string{"read", "write"},
			Fields:      []string{"*"},
		})
	tests := []struct {
		name     string
		table    string
		op       string
		restrict bool
		wantOk   bool
	}{
		{"no restrict", "", "", false, true},
		{"has permission", "testpermission", "read", true, true},
		{"do not have permission", "testpermission", "delete", true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.PrestConf.AccessConf.Restrict = tt.restrict
			m := &Mock{
				mtx: &sync.RWMutex{},
				t:   t,
			}
			if gotOk := m.TablePermissions(tt.table, tt.op); gotOk != tt.wantOk {
				t.Errorf("Mock.TablePermissions() = %v, want %v", gotOk, tt.wantOk)
			}
		})
	}
}

func TestMock_DatabaseClause(t *testing.T) {
	tests := []struct {
		name   string
		item   Item
		wantOk bool
	}{
		{"is count", Item{IsCount: true}, true},
		{"is not count", Item{IsCount: false}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Mock{
				mtx: &sync.RWMutex{},
				t:   t,
			}
			m.AddItem(tt.item.Body, tt.item.Error, tt.item.IsCount)
			if _, gotOk := m.DatabaseClause(nil); gotOk != tt.wantOk {
				t.Errorf("Mock.TablePermissions() = %v, want %v", gotOk, tt.wantOk)
			}
		})
	}
}

func TestMock_SchemaClause(t *testing.T) {
	tests := []struct {
		name   string
		item   Item
		wantOk bool
	}{
		{"is count", Item{IsCount: true}, true},
		{"is not count", Item{IsCount: false}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Mock{
				mtx: &sync.RWMutex{},
				t:   t,
			}
			m.AddItem(tt.item.Body, tt.item.Error, tt.item.IsCount)
			if _, gotOk := m.SchemaClause(nil); gotOk != tt.wantOk {
				t.Errorf("Mock.TablePermissions() = %v, want %v", gotOk, tt.wantOk)
			}
		})
	}
}

func TestMock_AddItem(t *testing.T) {
	type args struct {
		body    []byte
		err     error
		isCount bool
	}
	tests := []struct {
		name string
		args args
		len  int
	}{
		{"add item", args{body: []byte(`[]`), err: nil, isCount: false}, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Mock{
				mtx: &sync.RWMutex{},
				t:   t,
			}
			m.AddItem(tt.args.body, tt.args.err, tt.args.isCount)
			if len(m.Items) != tt.len {
				t.Errorf("expected %v, but got: %v", tt.len, len(m.Items))
			}
		})
	}
}

func TestMock_Insert(t *testing.T) {
	tests := []struct {
		name   string
		item   Item
		wantSc adapters.Scanner
	}{
		{
			"insert body",
			Item{
				Body: []byte(`[{"test":"test"}]`),
			},
			&scanner.PrestScanner{
				Buff: bytes.NewBuffer([]byte(`[{"test":"test"}]`)),
			},
		},
		{
			"insert err",
			Item{
				Error: errors.New("test error"),
			},
			&scanner.PrestScanner{
				Error: errors.New("test error"),
				Buff:  &bytes.Buffer{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Mock{
				mtx: &sync.RWMutex{},
				t:   t,
			}
			m.AddItem(tt.item.Body, tt.item.Error, tt.item.IsCount)
			if gotSc := m.Insert(""); !reflect.DeepEqual(gotSc, tt.wantSc) {
				t.Errorf("Mock.Insert() = %v, want %v", gotSc, tt.wantSc)
			}
		})
	}
}

func TestMock_BatchInsertValues(t *testing.T) {
	tests := []struct {
		name   string
		item   Item
		wantSc adapters.Scanner
	}{
		{
			"batch insert body",
			Item{
				Body: []byte(`[{"test":"test"}]`),
			},
			&scanner.PrestScanner{
				Buff:    bytes.NewBuffer([]byte(`[{"test":"test"}]`)),
				IsQuery: true,
			},
		},
		{
			"batch insert err",
			Item{
				Error: errors.New("test error"),
			},
			&scanner.PrestScanner{
				Error:   errors.New("test error"),
				Buff:    &bytes.Buffer{},
				IsQuery: true,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Mock{
				mtx: &sync.RWMutex{},
				t:   t,
			}
			m.AddItem(tt.item.Body, tt.item.Error, tt.item.IsCount)
			if gotSc := m.BatchInsertValues(""); !reflect.DeepEqual(gotSc, tt.wantSc) {
				t.Errorf("Mock.BatchInsert() = %v, want %v", gotSc, tt.wantSc)
			}
		})
	}
}

func TestMock_Delete(t *testing.T) {
	tests := []struct {
		name   string
		item   Item
		wantSc adapters.Scanner
	}{
		{
			"delete body",
			Item{
				Body: []byte(`[{"test":"test"}]`),
			},
			&scanner.PrestScanner{
				Buff: bytes.NewBuffer([]byte(`[{"test":"test"}]`)),
			},
		},
		{
			"delete err",
			Item{
				Error: errors.New("test error"),
			},
			&scanner.PrestScanner{
				Error: errors.New("test error"),
				Buff:  &bytes.Buffer{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Mock{
				mtx: &sync.RWMutex{},
				t:   t,
			}
			m.AddItem(tt.item.Body, tt.item.Error, tt.item.IsCount)
			if gotSc := m.Delete(""); !reflect.DeepEqual(gotSc, tt.wantSc) {
				t.Errorf("Mock.Delete() = %v, want %v", gotSc, tt.wantSc)
			}
		})
	}
}

func TestMock_Update(t *testing.T) {
	tests := []struct {
		name   string
		item   Item
		wantSc adapters.Scanner
	}{
		{
			"update body",
			Item{
				Body: []byte(`[{"test":"test"}]`),
			},
			&scanner.PrestScanner{
				Buff: bytes.NewBuffer([]byte(`[{"test":"test"}]`)),
			},
		},
		{
			"update err",
			Item{
				Error: errors.New("test error"),
			},
			&scanner.PrestScanner{
				Error: errors.New("test error"),
				Buff:  &bytes.Buffer{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Mock{
				mtx: &sync.RWMutex{},
				t:   t,
			}
			m.AddItem(tt.item.Body, tt.item.Error, tt.item.IsCount)
			if gotSc := m.Update(""); !reflect.DeepEqual(gotSc, tt.wantSc) {
				t.Errorf("Mock.Update() = %v, want %v", gotSc, tt.wantSc)
			}
		})
	}
}

func TestMock_QueryCount(t *testing.T) {
	tests := []struct {
		name   string
		item   Item
		wantSc adapters.Scanner
	}{
		{
			"count body",
			Item{
				Body: []byte(`[{"test":"test"}]`),
			},
			&scanner.PrestScanner{
				Buff: bytes.NewBuffer([]byte(`[{"test":"test"}]`)),
			},
		},
		{
			"count err",
			Item{
				Error: errors.New("test error"),
			},
			&scanner.PrestScanner{
				Error: errors.New("test error"),
				Buff:  &bytes.Buffer{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Mock{
				mtx: &sync.RWMutex{},
				t:   t,
			}
			m.AddItem(tt.item.Body, tt.item.Error, tt.item.IsCount)
			if gotSc := m.QueryCount(""); !reflect.DeepEqual(gotSc, tt.wantSc) {
				t.Errorf("Mock.QueryCount() = %v, want %v", gotSc, tt.wantSc)
			}
		})
	}
}

func TestMock_Query(t *testing.T) {
	tests := []struct {
		name   string
		item   Item
		wantSc adapters.Scanner
	}{
		{
			"query body",
			Item{
				Body: []byte(`[{"test":"test"}]`),
			},
			&scanner.PrestScanner{
				Buff:    bytes.NewBuffer([]byte(`[{"test":"test"}]`)),
				IsQuery: true,
			},
		},
		{
			"query err",
			Item{
				Error: errors.New("test error"),
			},
			&scanner.PrestScanner{
				Error:   errors.New("test error"),
				Buff:    &bytes.Buffer{},
				IsQuery: true,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Mock{
				mtx: &sync.RWMutex{},
				t:   t,
			}
			m.AddItem(tt.item.Body, tt.item.Error, tt.item.IsCount)
			if gotSc := m.Query(""); !reflect.DeepEqual(gotSc, tt.wantSc) {
				t.Errorf("Mock.Query() = %v, want %v", gotSc, tt.wantSc)
			}
		})
	}
}
