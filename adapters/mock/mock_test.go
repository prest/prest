package mock

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"sync"
	"testing"

	"github.com/prest/prest/v2/adapters"
	"github.com/prest/prest/v2/adapters/scanner"
	"github.com/prest/prest/v2/config"
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
		{"validate", fields{&sync.RWMutex{}, t, []Item{{}}}},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

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
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

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
	tables := []config.TablesConf{
		config.TablesConf{
			Name:        "testpermission",
			Permissions: []string{"read"},
			Fields:      []string{"id", "name", "email"},
		},
		{
			Name:        "read_table",
			Permissions: []string{"read"},
			Fields:      []string{"id", "name", "email"},
		},
		{
			Name:        "write_table",
			Permissions: []string{"write"},
			Fields:      []string{"id", "name", "email"},
		},
		{
			Name:        "delete_table",
			Permissions: []string{"delete"},
			Fields:      []string{"id", "name", "email"},
		},
		{
			Name:        "read_write_table",
			Permissions: []string{"read", "write"},
			Fields:      []string{"id", "name", "email"},
		},
		{
			Name:        "read_delete_table",
			Permissions: []string{"read", "delete"},
			Fields:      []string{"id", "name", "email"},
		},
		{
			Name:        "write_delete_table",
			Permissions: []string{"write", "delete"},
			Fields:      []string{"id", "name", "email"},
		},
		{
			Name:        "read_write_delete_table",
			Permissions: []string{"read", "write", "delete"},
			Fields:      []string{"id", "name", "email"},
		},

		{
			Name:        "no_user_read_table",
			Permissions: []string{"read"},
			Fields:      []string{"id", "name", "email"},
		},
		{
			Name:        "no_user_write_table",
			Permissions: []string{"write"},
			Fields:      []string{"id", "name", "email"},
		},
		{
			Name:        "no_user_delete_table",
			Permissions: []string{"delete"},
			Fields:      []string{"id", "name", "email"},
		},
		{
			Name:        "no_user_read_write_table",
			Permissions: []string{"read", "write"},
			Fields:      []string{"id", "name", "email"},
		},
		{
			Name:        "no_user_read_delete_table",
			Permissions: []string{"read", "delete"},
			Fields:      []string{"id", "name", "email"},
		},
		{
			Name:        "no_user_write_delete_table",
			Permissions: []string{"write", "delete"},
			Fields:      []string{"id", "name", "email"},
		},
		{
			Name:        "no_user_read_write_delete_table",
			Permissions: []string{"read", "write", "delete"},
			Fields:      []string{"id", "name", "email"},
		},
	}
	users := []config.UsersConf{
		config.UsersConf{
			Name: "foo_read",
			Tables: []config.TablesConf{
				{Name: "read_table", Permissions: []string{"read"}, Fields: []string{"id", "name", "email"}},
				{Name: "write_table", Permissions: []string{"read"}, Fields: []string{"id", "name", "email"}},
				{Name: "delete_table", Permissions: []string{"read"}, Fields: []string{"id", "name", "email"}},
				{Name: "read_write_table", Permissions: []string{"read"}, Fields: []string{"id", "name", "email"}},
				{Name: "read_delete_table", Permissions: []string{"read"}, Fields: []string{"id", "name", "email"}},
				{Name: "write_delete_table", Permissions: []string{"read"}, Fields: []string{"id", "name", "email"}},
				{Name: "read_write_delete_table", Permissions: []string{"read"}, Fields: []string{"id", "name", "email"}},
			},
		},
		config.UsersConf{
			Name: "foo_write",
			Tables: []config.TablesConf{
				{Name: "read_table", Permissions: []string{"write"}, Fields: []string{"id", "name", "email"}},
				{Name: "write_table", Permissions: []string{"write"}, Fields: []string{"id", "name", "email"}},
				{Name: "delete_table", Permissions: []string{"write"}, Fields: []string{"id", "name", "email"}},
				{Name: "read_write_table", Permissions: []string{"write"}, Fields: []string{"id", "name", "email"}},
				{Name: "read_delete_table", Permissions: []string{"write"}, Fields: []string{"id", "name", "email"}},
				{Name: "write_delete_table", Permissions: []string{"write"}, Fields: []string{"id", "name", "email"}},
				{Name: "read_write_delete_table", Permissions: []string{"write"}, Fields: []string{"id", "name", "email"}},
			},
		},
		config.UsersConf{
			Name: "foo_delete",
			Tables: []config.TablesConf{
				{Name: "read_table", Permissions: []string{"delete"}, Fields: []string{"id", "name", "email"}},
				{Name: "write_table", Permissions: []string{"delete"}, Fields: []string{"id", "name", "email"}},
				{Name: "delete_table", Permissions: []string{"delete"}, Fields: []string{"id", "name", "email"}},
				{Name: "read_write_table", Permissions: []string{"delete"}, Fields: []string{"id", "name", "email"}},
				{Name: "read_delete_table", Permissions: []string{"delete"}, Fields: []string{"id", "name", "email"}},
				{Name: "write_delete_table", Permissions: []string{"delete"}, Fields: []string{"id", "name", "email"}},
				{Name: "read_write_delete_table", Permissions: []string{"delete"}, Fields: []string{"id", "name", "email"}},
			},
		},
		config.UsersConf{
			Name: "foo_read_write",
			Tables: []config.TablesConf{
				{Name: "read_table", Permissions: []string{"read", "write"}, Fields: []string{"id", "name", "email"}},
				{Name: "write_table", Permissions: []string{"read", "write"}, Fields: []string{"id", "name", "email"}},
				{Name: "delete_table", Permissions: []string{"read", "write"}, Fields: []string{"id", "name", "email"}},
				{Name: "read_write_table", Permissions: []string{"read", "write"}, Fields: []string{"id", "name", "email"}},
				{Name: "read_delete_table", Permissions: []string{"read", "write"}, Fields: []string{"id", "name", "email"}},
				{Name: "write_delete_table", Permissions: []string{"read", "write"}, Fields: []string{"id", "name", "email"}},
				{Name: "read_write_delete_table", Permissions: []string{"read", "write"}, Fields: []string{"id", "name", "email"}},
			},
		},
		config.UsersConf{
			Name: "foo_read_delete",
			Tables: []config.TablesConf{
				{Name: "read_table", Permissions: []string{"read", "delete"}, Fields: []string{"id", "name", "email"}},
				{Name: "write_table", Permissions: []string{"read", "delete"}, Fields: []string{"id", "name", "email"}},
				{Name: "delete_table", Permissions: []string{"read", "delete"}, Fields: []string{"id", "name", "email"}},
				{Name: "read_write_table", Permissions: []string{"read", "delete"}, Fields: []string{"id", "name", "email"}},
				{Name: "read_delete_table", Permissions: []string{"read", "delete"}, Fields: []string{"id", "name", "email"}},
				{Name: "write_delete_table", Permissions: []string{"read", "delete"}, Fields: []string{"id", "name", "email"}},
				{Name: "read_write_delete_table", Permissions: []string{"read", "delete"}, Fields: []string{"id", "name", "email"}},
			},
		},
		config.UsersConf{
			Name: "foo_write_delete",
			Tables: []config.TablesConf{
				{Name: "read_table", Permissions: []string{"write", "delete"}, Fields: []string{"id", "name", "email"}},
				{Name: "write_table", Permissions: []string{"write", "delete"}, Fields: []string{"id", "name", "email"}},
				{Name: "delete_table", Permissions: []string{"write", "delete"}, Fields: []string{"id", "name", "email"}},
				{Name: "read_write_table", Permissions: []string{"write", "delete"}, Fields: []string{"id", "name", "email"}},
				{Name: "read_delete_table", Permissions: []string{"write", "delete"}, Fields: []string{"id", "name", "email"}},
				{Name: "write_delete_table", Permissions: []string{"write", "delete"}, Fields: []string{"id", "name", "email"}},
				{Name: "read_write_delete_table", Permissions: []string{"write", "delete"}, Fields: []string{"id", "name", "email"}},
			},
		},
		config.UsersConf{
			Name: "foo_read_write_delete",
			Tables: []config.TablesConf{
				{Name: "read_table", Permissions: []string{"read", "write", "delete"}, Fields: []string{"id", "name", "email"}},
				{Name: "write_table", Permissions: []string{"read", "write", "delete"}, Fields: []string{"id", "name", "email"}},
				{Name: "delete_table", Permissions: []string{"read", "write", "delete"}, Fields: []string{"id", "name", "email"}},
				{Name: "read_write_table", Permissions: []string{"read", "write", "delete"}, Fields: []string{"id", "name", "email"}},
				{Name: "read_delete_table", Permissions: []string{"read", "write", "delete"}, Fields: []string{"id", "name", "email"}},
				{Name: "write_delete_table", Permissions: []string{"read", "write", "delete"}, Fields: []string{"id", "name", "email"}},
				{Name: "read_write_delete_table", Permissions: []string{"read", "write", "delete"}, Fields: []string{"id", "name", "email"}},
			},
		},
	}

	tests := []struct {
		name     string
		table    string
		op       string
		restrict bool
		userName string
		wantOk   bool
	}{

		{"no restrict", "", "", false, "", true},
		{"has permission", "testpermission", "read", true, "", true},
		{"do not have permission", "testpermission", "delete", true, "", false},

		{"empty username with restrict", "read_table", "read", true, "", true},
		{"invalid operation", "read_table", "invalid_op", true, "foo_read", false},

		{"non-existent table", "non_existent", "read", true, "foo_read", false},
		{"non-existent table", "non_existent", "write", true, "foo_read", false},
		{"non-existent table", "non_existent", "delete", true, "foo_read", false},

		{"non-existent user", "read_table", "read", true, "non_existent", true},
		{"non-existent user", "read_table", "write", true, "non_existent", false},
		{"non-existent user", "read_table", "delete", true, "non_existent", false},

		{"try foo_read read read_table", "read_table", "read", true, "foo_read", true},
		{"try foo_read read write_table", "write_table", "read", true, "foo_read", true},
		{"try foo_read read delete_table", "delete_table", "read", true, "foo_read", true},
		{"try foo_read read read_write_table", "read_write_table", "read", true, "foo_read", true},
		{"try foo_read read read_delete_table", "read_delete_table", "read", true, "foo_read", true},
		{"try foo_read read write_delete_table", "write_delete_table", "read", true, "foo_read", true},
		{"try foo_read read read_write_delete_table", "read_write_delete_table", "read", true, "foo_read", true},
		{"try foo_read read no_user_read_table", "no_user_read_table", "read", true, "foo_read", true},
		{"try foo_read read no_user_write_table without permission", "no_user_write_table", "read", true, "foo_read", false},
		{"try foo_read read no_user_delete_table without permission", "no_user_delete_table", "read", true, "foo_read", false},
		{"try foo_read read no_user_read_write_table", "no_user_read_write_table", "read", true, "foo_read", true},
		{"try foo_read read no_user_read_delete_table", "no_user_read_delete_table", "read", true, "foo_read", true},
		{"try foo_read read no_user_write_delete_table without permission", "no_user_write_delete_table", "read", true, "foo_read", false},
		{"try foo_read read no_user_read_write_delete_table", "no_user_read_write_delete_table", "read", true, "foo_read", true},
		{"try foo_read write read_table without permission", "read_table", "write", true, "foo_read", false},
		{"try foo_read write write_table without permission", "write_table", "write", true, "foo_read", false},
		{"try foo_read write delete_table without permission", "delete_table", "write", true, "foo_read", false},
		{"try foo_read write read_write_table without permission", "read_write_table", "write", true, "foo_read", false},
		{"try foo_read write read_delete_table without permission", "read_delete_table", "write", true, "foo_read", false},
		{"try foo_read write write_delete_table without permission", "write_delete_table", "write", true, "foo_read", false},
		{"try foo_read write read_write_delete_table without permission", "read_write_delete_table", "write", true, "foo_read", false},
		{"try foo_read write no_user_read_table without permission", "no_user_read_table", "write", true, "foo_read", false},
		{"try foo_read write no_user_write_table", "no_user_write_table", "write", true, "foo_read", true},
		{"try foo_read write no_user_delete_table without permission", "no_user_delete_table", "write", true, "foo_read", false},
		{"try foo_read write no_user_read_write_table", "no_user_read_write_table", "write", true, "foo_read", true},
		{"try foo_read write no_user_read_delete_table without permission", "no_user_read_delete_table", "write", true, "foo_read", false},
		{"try foo_read write no_user_write_delete_table", "no_user_write_delete_table", "write", true, "foo_read", true},
		{"try foo_read write no_user_read_write_delete_table", "no_user_read_write_delete_table", "write", true, "foo_read", true},
		{"try foo_read delete read_table without permission", "read_table", "delete", true, "foo_read", false},
		{"try foo_read delete write_table without permission", "write_table", "delete", true, "foo_read", false},
		{"try foo_read delete delete_table without permission", "delete_table", "delete", true, "foo_read", false},
		{"try foo_read delete read_write_table without permission", "read_write_table", "delete", true, "foo_read", false},
		{"try foo_read delete read_delete_table without permission", "read_delete_table", "delete", true, "foo_read", false},
		{"try foo_read delete write_delete_table without permission", "write_delete_table", "delete", true, "foo_read", false},
		{"try foo_read delete read_write_delete_table without permission", "read_write_delete_table", "delete", true, "foo_read", false},
		{"try foo_read delete no_user_read_table without permission", "no_user_read_table", "delete", true, "foo_read", false},
		{"try foo_read delete no_user_write_table without permission", "no_user_write_table", "delete", true, "foo_read", false},
		{"try foo_read delete no_user_delete_table", "no_user_delete_table", "delete", true, "foo_read", true},
		{"try foo_read delete no_user_read_write_table without permission", "no_user_read_write_table", "delete", true, "foo_read", false},
		{"try foo_read delete no_user_read_delete_table", "no_user_read_delete_table", "delete", true, "foo_read", true},
		{"try foo_read delete no_user_write_delete_table", "no_user_write_delete_table", "delete", true, "foo_read", true},
		{"try foo_read delete no_user_read_write_delete_table", "no_user_read_write_delete_table", "delete", true, "foo_read", true},
		{"try foo_write read read_table without permission", "read_table", "read", true, "foo_write", false},
		{"try foo_write read write_table without permission", "write_table", "read", true, "foo_write", false},
		{"try foo_write read delete_table without permission", "delete_table", "read", true, "foo_write", false},
		{"try foo_write read read_write_table without permission", "read_write_table", "read", true, "foo_write", false},
		{"try foo_write read read_delete_table without permission", "read_delete_table", "read", true, "foo_write", false},
		{"try foo_write read write_delete_table without permission", "write_delete_table", "read", true, "foo_write", false},
		{"try foo_write read read_write_delete_table without permission", "read_write_delete_table", "read", true, "foo_write", false},
		{"try foo_write read no_user_read_table", "no_user_read_table", "read", true, "foo_write", true},
		{"try foo_write read no_user_write_table without permission", "no_user_write_table", "read", true, "foo_write", false},
		{"try foo_write read no_user_delete_table without permission", "no_user_delete_table", "read", true, "foo_write", false},
		{"try foo_write read no_user_read_write_table", "no_user_read_write_table", "read", true, "foo_write", true},
		{"try foo_write read no_user_read_delete_table", "no_user_read_delete_table", "read", true, "foo_write", true},
		{"try foo_write read no_user_write_delete_table without permission", "no_user_write_delete_table", "read", true, "foo_write", false},
		{"try foo_write read no_user_read_write_delete_table", "no_user_read_write_delete_table", "read", true, "foo_write", true},
		{"try foo_write write read_table", "read_table", "write", true, "foo_write", true},
		{"try foo_write write write_table", "write_table", "write", true, "foo_write", true},
		{"try foo_write write delete_table", "delete_table", "write", true, "foo_write", true},
		{"try foo_write write read_write_table", "read_write_table", "write", true, "foo_write", true},
		{"try foo_write write read_delete_table", "read_delete_table", "write", true, "foo_write", true},
		{"try foo_write write write_delete_table", "write_delete_table", "write", true, "foo_write", true},
		{"try foo_write write read_write_delete_table", "read_write_delete_table", "write", true, "foo_write", true},
		{"try foo_write write no_user_read_table without permission", "no_user_read_table", "write", true, "foo_write", false},
		{"try foo_write write no_user_write_table", "no_user_write_table", "write", true, "foo_write", true},
		{"try foo_write write no_user_delete_table without permission", "no_user_delete_table", "write", true, "foo_write", false},
		{"try foo_write write no_user_read_write_table", "no_user_read_write_table", "write", true, "foo_write", true},
		{"try foo_write write no_user_read_delete_table without permission", "no_user_read_delete_table", "write", true, "foo_write", false},
		{"try foo_write write no_user_write_delete_table", "no_user_write_delete_table", "write", true, "foo_write", true},
		{"try foo_write write no_user_read_write_delete_table", "no_user_read_write_delete_table", "write", true, "foo_write", true},
		{"try foo_write delete read_table without permission", "read_table", "delete", true, "foo_write", false},
		{"try foo_write delete write_table without permission", "write_table", "delete", true, "foo_write", false},
		{"try foo_write delete delete_table without permission", "delete_table", "delete", true, "foo_write", false},
		{"try foo_write delete read_write_table without permission", "read_write_table", "delete", true, "foo_write", false},
		{"try foo_write delete read_delete_table without permission", "read_delete_table", "delete", true, "foo_write", false},
		{"try foo_write delete write_delete_table without permission", "write_delete_table", "delete", true, "foo_write", false},
		{"try foo_write delete read_write_delete_table without permission", "read_write_delete_table", "delete", true, "foo_write", false},
		{"try foo_write delete no_user_read_table without permission", "no_user_read_table", "delete", true, "foo_write", false},
		{"try foo_write delete no_user_write_table without permission", "no_user_write_table", "delete", true, "foo_write", false},
		{"try foo_write delete no_user_delete_table", "no_user_delete_table", "delete", true, "foo_write", true},
		{"try foo_write delete no_user_read_write_table without permission", "no_user_read_write_table", "delete", true, "foo_write", false},
		{"try foo_write delete no_user_read_delete_table", "no_user_read_delete_table", "delete", true, "foo_write", true},
		{"try foo_write delete no_user_write_delete_table", "no_user_write_delete_table", "delete", true, "foo_write", true},
		{"try foo_write delete no_user_read_write_delete_table", "no_user_read_write_delete_table", "delete", true, "foo_write", true},
		{"try foo_delete read read_table without permission", "read_table", "read", true, "foo_delete", false},
		{"try foo_delete read write_table without permission", "write_table", "read", true, "foo_delete", false},
		{"try foo_delete read delete_table without permission", "delete_table", "read", true, "foo_delete", false},
		{"try foo_delete read read_write_table without permission", "read_write_table", "read", true, "foo_delete", false},
		{"try foo_delete read read_delete_table without permission", "read_delete_table", "read", true, "foo_delete", false},
		{"try foo_delete read write_delete_table without permission", "write_delete_table", "read", true, "foo_delete", false},
		{"try foo_delete read read_write_delete_table without permission", "read_write_delete_table", "read", true, "foo_delete", false},
		{"try foo_delete read no_user_read_table", "no_user_read_table", "read", true, "foo_delete", true},
		{"try foo_delete read no_user_write_table without permission", "no_user_write_table", "read", true, "foo_delete", false},
		{"try foo_delete read no_user_delete_table without permission", "no_user_delete_table", "read", true, "foo_delete", false},
		{"try foo_delete read no_user_read_write_table", "no_user_read_write_table", "read", true, "foo_delete", true},
		{"try foo_delete read no_user_read_delete_table", "no_user_read_delete_table", "read", true, "foo_delete", true},
		{"try foo_delete read no_user_write_delete_table without permission", "no_user_write_delete_table", "read", true, "foo_delete", false},
		{"try foo_delete read no_user_read_write_delete_table", "no_user_read_write_delete_table", "read", true, "foo_delete", true},
		{"try foo_delete write read_table without permission", "read_table", "write", true, "foo_delete", false},
		{"try foo_delete write write_table without permission", "write_table", "write", true, "foo_delete", false},
		{"try foo_delete write delete_table without permission", "delete_table", "write", true, "foo_delete", false},
		{"try foo_delete write read_write_table without permission", "read_write_table", "write", true, "foo_delete", false},
		{"try foo_delete write read_delete_table without permission", "read_delete_table", "write", true, "foo_delete", false},
		{"try foo_delete write write_delete_table without permission", "write_delete_table", "write", true, "foo_delete", false},
		{"try foo_delete write read_write_delete_table without permission", "read_write_delete_table", "write", true, "foo_delete", false},
		{"try foo_delete write no_user_read_table without permission", "no_user_read_table", "write", true, "foo_delete", false},
		{"try foo_delete write no_user_write_table", "no_user_write_table", "write", true, "foo_delete", true},
		{"try foo_delete write no_user_delete_table without permission", "no_user_delete_table", "write", true, "foo_delete", false},
		{"try foo_delete write no_user_read_write_table", "no_user_read_write_table", "write", true, "foo_delete", true},
		{"try foo_delete write no_user_read_delete_table without permission", "no_user_read_delete_table", "write", true, "foo_delete", false},
		{"try foo_delete write no_user_write_delete_table", "no_user_write_delete_table", "write", true, "foo_delete", true},
		{"try foo_delete write no_user_read_write_delete_table", "no_user_read_write_delete_table", "write", true, "foo_delete", true},
		{"try foo_delete delete read_table", "read_table", "delete", true, "foo_delete", true},
		{"try foo_delete delete write_table", "write_table", "delete", true, "foo_delete", true},
		{"try foo_delete delete delete_table", "delete_table", "delete", true, "foo_delete", true},
		{"try foo_delete delete read_write_table", "read_write_table", "delete", true, "foo_delete", true},
		{"try foo_delete delete read_delete_table", "read_delete_table", "delete", true, "foo_delete", true},
		{"try foo_delete delete write_delete_table", "write_delete_table", "delete", true, "foo_delete", true},
		{"try foo_delete delete read_write_delete_table", "read_write_delete_table", "delete", true, "foo_delete", true},
		{"try foo_delete delete no_user_read_table without permission", "no_user_read_table", "delete", true, "foo_delete", false},
		{"try foo_delete delete no_user_write_table without permission", "no_user_write_table", "delete", true, "foo_delete", false},
		{"try foo_delete delete no_user_delete_table", "no_user_delete_table", "delete", true, "foo_delete", true},
		{"try foo_delete delete no_user_read_write_table without permission", "no_user_read_write_table", "delete", true, "foo_delete", false},
		{"try foo_delete delete no_user_read_delete_table", "no_user_read_delete_table", "delete", true, "foo_delete", true},
		{"try foo_delete delete no_user_write_delete_table", "no_user_write_delete_table", "delete", true, "foo_delete", true},
		{"try foo_delete delete no_user_read_write_delete_table", "no_user_read_write_delete_table", "delete", true, "foo_delete", true},
		{"try foo_read_write read read_table", "read_table", "read", true, "foo_read_write", true},
		{"try foo_read_write read write_table", "write_table", "read", true, "foo_read_write", true},
		{"try foo_read_write read delete_table", "delete_table", "read", true, "foo_read_write", true},
		{"try foo_read_write read read_write_table", "read_write_table", "read", true, "foo_read_write", true},
		{"try foo_read_write read read_delete_table", "read_delete_table", "read", true, "foo_read_write", true},
		{"try foo_read_write read write_delete_table", "write_delete_table", "read", true, "foo_read_write", true},
		{"try foo_read_write read read_write_delete_table", "read_write_delete_table", "read", true, "foo_read_write", true},
		{"try foo_read_write read no_user_read_table", "no_user_read_table", "read", true, "foo_read_write", true},
		{"try foo_read_write read no_user_write_table without permission", "no_user_write_table", "read", true, "foo_read_write", false},
		{"try foo_read_write read no_user_delete_table without permission", "no_user_delete_table", "read", true, "foo_read_write", false},
		{"try foo_read_write read no_user_read_write_table", "no_user_read_write_table", "read", true, "foo_read_write", true},
		{"try foo_read_write read no_user_read_delete_table", "no_user_read_delete_table", "read", true, "foo_read_write", true},
		{"try foo_read_write read no_user_write_delete_table without permission", "no_user_write_delete_table", "read", true, "foo_read_write", false},
		{"try foo_read_write read no_user_read_write_delete_table", "no_user_read_write_delete_table", "read", true, "foo_read_write", true},
		{"try foo_read_write write read_table", "read_table", "write", true, "foo_read_write", true},
		{"try foo_read_write write write_table", "write_table", "write", true, "foo_read_write", true},
		{"try foo_read_write write delete_table", "delete_table", "write", true, "foo_read_write", true},
		{"try foo_read_write write read_write_table", "read_write_table", "write", true, "foo_read_write", true},
		{"try foo_read_write write read_delete_table", "read_delete_table", "write", true, "foo_read_write", true},
		{"try foo_read_write write write_delete_table", "write_delete_table", "write", true, "foo_read_write", true},
		{"try foo_read_write write read_write_delete_table", "read_write_delete_table", "write", true, "foo_read_write", true},
		{"try foo_read_write write no_user_read_table without permission", "no_user_read_table", "write", true, "foo_read_write", false},
		{"try foo_read_write write no_user_write_table", "no_user_write_table", "write", true, "foo_read_write", true},
		{"try foo_read_write write no_user_delete_table without permission", "no_user_delete_table", "write", true, "foo_read_write", false},
		{"try foo_read_write write no_user_read_write_table", "no_user_read_write_table", "write", true, "foo_read_write", true},
		{"try foo_read_write write no_user_read_delete_table without permission", "no_user_read_delete_table", "write", true, "foo_read_write", false},
		{"try foo_read_write write no_user_write_delete_table", "no_user_write_delete_table", "write", true, "foo_read_write", true},
		{"try foo_read_write write no_user_read_write_delete_table", "no_user_read_write_delete_table", "write", true, "foo_read_write", true},
		{"try foo_read_write delete read_table without permission", "read_table", "delete", true, "foo_read_write", false},
		{"try foo_read_write delete write_table without permission", "write_table", "delete", true, "foo_read_write", false},
		{"try foo_read_write delete delete_table without permission", "delete_table", "delete", true, "foo_read_write", false},
		{"try foo_read_write delete read_write_table without permission", "read_write_table", "delete", true, "foo_read_write", false},
		{"try foo_read_write delete read_delete_table without permission", "read_delete_table", "delete", true, "foo_read_write", false},
		{"try foo_read_write delete write_delete_table without permission", "write_delete_table", "delete", true, "foo_read_write", false},
		{"try foo_read_write delete read_write_delete_table without permission", "read_write_delete_table", "delete", true, "foo_read_write", false},
		{"try foo_read_write delete no_user_read_table without permission", "no_user_read_table", "delete", true, "foo_read_write", false},
		{"try foo_read_write delete no_user_write_table without permission", "no_user_write_table", "delete", true, "foo_read_write", false},
		{"try foo_read_write delete no_user_delete_table", "no_user_delete_table", "delete", true, "foo_read_write", true},
		{"try foo_read_write delete no_user_read_write_table without permission", "no_user_read_write_table", "delete", true, "foo_read_write", false},
		{"try foo_read_write delete no_user_read_delete_table", "no_user_read_delete_table", "delete", true, "foo_read_write", true},
		{"try foo_read_write delete no_user_write_delete_table", "no_user_write_delete_table", "delete", true, "foo_read_write", true},
		{"try foo_read_write delete no_user_read_write_delete_table", "no_user_read_write_delete_table", "delete", true, "foo_read_write", true},
		{"try foo_read_delete read read_table", "read_table", "read", true, "foo_read_delete", true},
		{"try foo_read_delete read write_table", "write_table", "read", true, "foo_read_delete", true},
		{"try foo_read_delete read delete_table", "delete_table", "read", true, "foo_read_delete", true},
		{"try foo_read_delete read read_write_table", "read_write_table", "read", true, "foo_read_delete", true},
		{"try foo_read_delete read read_delete_table", "read_delete_table", "read", true, "foo_read_delete", true},
		{"try foo_read_delete read write_delete_table", "write_delete_table", "read", true, "foo_read_delete", true},
		{"try foo_read_delete read read_write_delete_table", "read_write_delete_table", "read", true, "foo_read_delete", true},
		{"try foo_read_delete read no_user_read_table", "no_user_read_table", "read", true, "foo_read_delete", true},
		{"try foo_read_delete read no_user_write_table without permission", "no_user_write_table", "read", true, "foo_read_delete", false},
		{"try foo_read_delete read no_user_delete_table without permission", "no_user_delete_table", "read", true, "foo_read_delete", false},
		{"try foo_read_delete read no_user_read_write_table", "no_user_read_write_table", "read", true, "foo_read_delete", true},
		{"try foo_read_delete read no_user_read_delete_table", "no_user_read_delete_table", "read", true, "foo_read_delete", true},
		{"try foo_read_delete read no_user_write_delete_table without permission", "no_user_write_delete_table", "read", true, "foo_read_delete", false},
		{"try foo_read_delete read no_user_read_write_delete_table", "no_user_read_write_delete_table", "read", true, "foo_read_delete", true},
		{"try foo_read_delete write read_table without permission", "read_table", "write", true, "foo_read_delete", false},
		{"try foo_read_delete write write_table without permission", "write_table", "write", true, "foo_read_delete", false},
		{"try foo_read_delete write delete_table without permission", "delete_table", "write", true, "foo_read_delete", false},
		{"try foo_read_delete write read_write_table without permission", "read_write_table", "write", true, "foo_read_delete", false},
		{"try foo_read_delete write read_delete_table without permission", "read_delete_table", "write", true, "foo_read_delete", false},
		{"try foo_read_delete write write_delete_table without permission", "write_delete_table", "write", true, "foo_read_delete", false},
		{"try foo_read_delete write read_write_delete_table without permission", "read_write_delete_table", "write", true, "foo_read_delete", false},
		{"try foo_read_delete write no_user_read_table without permission", "no_user_read_table", "write", true, "foo_read_delete", false},
		{"try foo_read_delete write no_user_write_table", "no_user_write_table", "write", true, "foo_read_delete", true},
		{"try foo_read_delete write no_user_delete_table without permission", "no_user_delete_table", "write", true, "foo_read_delete", false},
		{"try foo_read_delete write no_user_read_write_table", "no_user_read_write_table", "write", true, "foo_read_delete", true},
		{"try foo_read_delete write no_user_read_delete_table without permission", "no_user_read_delete_table", "write", true, "foo_read_delete", false},
		{"try foo_read_delete write no_user_write_delete_table", "no_user_write_delete_table", "write", true, "foo_read_delete", true},
		{"try foo_read_delete write no_user_read_write_delete_table", "no_user_read_write_delete_table", "write", true, "foo_read_delete", true},
		{"try foo_read_delete delete read_table", "read_table", "delete", true, "foo_read_delete", true},
		{"try foo_read_delete delete write_table", "write_table", "delete", true, "foo_read_delete", true},
		{"try foo_read_delete delete delete_table", "delete_table", "delete", true, "foo_read_delete", true},
		{"try foo_read_delete delete read_write_table", "read_write_table", "delete", true, "foo_read_delete", true},
		{"try foo_read_delete delete read_delete_table", "read_delete_table", "delete", true, "foo_read_delete", true},
		{"try foo_read_delete delete write_delete_table", "write_delete_table", "delete", true, "foo_read_delete", true},
		{"try foo_read_delete delete read_write_delete_table", "read_write_delete_table", "delete", true, "foo_read_delete", true},
		{"try foo_read_delete delete no_user_read_table without permission", "no_user_read_table", "delete", true, "foo_read_delete", false},
		{"try foo_read_delete delete no_user_write_table without permission", "no_user_write_table", "delete", true, "foo_read_delete", false},
		{"try foo_read_delete delete no_user_delete_table", "no_user_delete_table", "delete", true, "foo_read_delete", true},
		{"try foo_read_delete delete no_user_read_write_table without permission", "no_user_read_write_table", "delete", true, "foo_read_delete", false},
		{"try foo_read_delete delete no_user_read_delete_table", "no_user_read_delete_table", "delete", true, "foo_read_delete", true},
		{"try foo_read_delete delete no_user_write_delete_table", "no_user_write_delete_table", "delete", true, "foo_read_delete", true},
		{"try foo_read_delete delete no_user_read_write_delete_table", "no_user_read_write_delete_table", "delete", true, "foo_read_delete", true},
		{"try foo_write_delete read read_table without permission", "read_table", "read", true, "foo_write_delete", false},
		{"try foo_write_delete read write_table without permission", "write_table", "read", true, "foo_write_delete", false},
		{"try foo_write_delete read delete_table without permission", "delete_table", "read", true, "foo_write_delete", false},
		{"try foo_write_delete read read_write_table without permission", "read_write_table", "read", true, "foo_write_delete", false},
		{"try foo_write_delete read read_delete_table without permission", "read_delete_table", "read", true, "foo_write_delete", false},
		{"try foo_write_delete read write_delete_table without permission", "write_delete_table", "read", true, "foo_write_delete", false},
		{"try foo_write_delete read read_write_delete_table without permission", "read_write_delete_table", "read", true, "foo_write_delete", false},
		{"try foo_write_delete read no_user_read_table", "no_user_read_table", "read", true, "foo_write_delete", true},
		{"try foo_write_delete read no_user_write_table without permission", "no_user_write_table", "read", true, "foo_write_delete", false},
		{"try foo_write_delete read no_user_delete_table without permission", "no_user_delete_table", "read", true, "foo_write_delete", false},
		{"try foo_write_delete read no_user_read_write_table", "no_user_read_write_table", "read", true, "foo_write_delete", true},
		{"try foo_write_delete read no_user_read_delete_table", "no_user_read_delete_table", "read", true, "foo_write_delete", true},
		{"try foo_write_delete read no_user_write_delete_table without permission", "no_user_write_delete_table", "read", true, "foo_write_delete", false},
		{"try foo_write_delete read no_user_read_write_delete_table", "no_user_read_write_delete_table", "read", true, "foo_write_delete", true},
		{"try foo_write_delete write read_table", "read_table", "write", true, "foo_write_delete", true},
		{"try foo_write_delete write write_table", "write_table", "write", true, "foo_write_delete", true},
		{"try foo_write_delete write delete_table", "delete_table", "write", true, "foo_write_delete", true},
		{"try foo_write_delete write read_write_table", "read_write_table", "write", true, "foo_write_delete", true},
		{"try foo_write_delete write read_delete_table", "read_delete_table", "write", true, "foo_write_delete", true},
		{"try foo_write_delete write write_delete_table", "write_delete_table", "write", true, "foo_write_delete", true},
		{"try foo_write_delete write read_write_delete_table", "read_write_delete_table", "write", true, "foo_write_delete", true},
		{"try foo_write_delete write no_user_read_table without permission", "no_user_read_table", "write", true, "foo_write_delete", false},
		{"try foo_write_delete write no_user_write_table", "no_user_write_table", "write", true, "foo_write_delete", true},
		{"try foo_write_delete write no_user_delete_table without permission", "no_user_delete_table", "write", true, "foo_write_delete", false},
		{"try foo_write_delete write no_user_read_write_table", "no_user_read_write_table", "write", true, "foo_write_delete", true},
		{"try foo_write_delete write no_user_read_delete_table without permission", "no_user_read_delete_table", "write", true, "foo_write_delete", false},
		{"try foo_write_delete write no_user_write_delete_table", "no_user_write_delete_table", "write", true, "foo_write_delete", true},
		{"try foo_write_delete write no_user_read_write_delete_table", "no_user_read_write_delete_table", "write", true, "foo_write_delete", true},
		{"try foo_write_delete delete read_table", "read_table", "delete", true, "foo_write_delete", true},
		{"try foo_write_delete delete write_table", "write_table", "delete", true, "foo_write_delete", true},
		{"try foo_write_delete delete delete_table", "delete_table", "delete", true, "foo_write_delete", true},
		{"try foo_write_delete delete read_write_table", "read_write_table", "delete", true, "foo_write_delete", true},
		{"try foo_write_delete delete read_delete_table", "read_delete_table", "delete", true, "foo_write_delete", true},
		{"try foo_write_delete delete write_delete_table", "write_delete_table", "delete", true, "foo_write_delete", true},
		{"try foo_write_delete delete read_write_delete_table", "read_write_delete_table", "delete", true, "foo_write_delete", true},
		{"try foo_write_delete delete no_user_read_table without permission", "no_user_read_table", "delete", true, "foo_write_delete", false},
		{"try foo_write_delete delete no_user_write_table without permission", "no_user_write_table", "delete", true, "foo_write_delete", false},
		{"try foo_write_delete delete no_user_delete_table", "no_user_delete_table", "delete", true, "foo_write_delete", true},
		{"try foo_write_delete delete no_user_read_write_table without permission", "no_user_read_write_table", "delete", true, "foo_write_delete", false},
		{"try foo_write_delete delete no_user_read_delete_table", "no_user_read_delete_table", "delete", true, "foo_write_delete", true},
		{"try foo_write_delete delete no_user_write_delete_table", "no_user_write_delete_table", "delete", true, "foo_write_delete", true},
		{"try foo_write_delete delete no_user_read_write_delete_table", "no_user_read_write_delete_table", "delete", true, "foo_write_delete", true},
		{"try foo_read_write_delete read read_table", "read_table", "read", true, "foo_read_write_delete", true},
		{"try foo_read_write_delete read write_table", "write_table", "read", true, "foo_read_write_delete", true},
		{"try foo_read_write_delete read delete_table", "delete_table", "read", true, "foo_read_write_delete", true},
		{"try foo_read_write_delete read read_write_table", "read_write_table", "read", true, "foo_read_write_delete", true},
		{"try foo_read_write_delete read read_delete_table", "read_delete_table", "read", true, "foo_read_write_delete", true},
		{"try foo_read_write_delete read write_delete_table", "write_delete_table", "read", true, "foo_read_write_delete", true},
		{"try foo_read_write_delete read read_write_delete_table", "read_write_delete_table", "read", true, "foo_read_write_delete", true},
		{"try foo_read_write_delete read no_user_read_table", "no_user_read_table", "read", true, "foo_read_write_delete", true},
		{"try foo_read_write_delete read no_user_write_table without permission", "no_user_write_table", "read", true, "foo_read_write_delete", false},
		{"try foo_read_write_delete read no_user_delete_table without permission", "no_user_delete_table", "read", true, "foo_read_write_delete", false},
		{"try foo_read_write_delete read no_user_read_write_table", "no_user_read_write_table", "read", true, "foo_read_write_delete", true},
		{"try foo_read_write_delete read no_user_read_delete_table", "no_user_read_delete_table", "read", true, "foo_read_write_delete", true},
		{"try foo_read_write_delete read no_user_write_delete_table without permission", "no_user_write_delete_table", "read", true, "foo_read_write_delete", false},
		{"try foo_read_write_delete read no_user_read_write_delete_table", "no_user_read_write_delete_table", "read", true, "foo_read_write_delete", true},
		{"try foo_read_write_delete write read_table", "read_table", "write", true, "foo_read_write_delete", true},
		{"try foo_read_write_delete write write_table", "write_table", "write", true, "foo_read_write_delete", true},
		{"try foo_read_write_delete write delete_table", "delete_table", "write", true, "foo_read_write_delete", true},
		{"try foo_read_write_delete write read_write_table", "read_write_table", "write", true, "foo_read_write_delete", true},
		{"try foo_read_write_delete write read_delete_table", "read_delete_table", "write", true, "foo_read_write_delete", true},
		{"try foo_read_write_delete write write_delete_table", "write_delete_table", "write", true, "foo_read_write_delete", true},
		{"try foo_read_write_delete write read_write_delete_table", "read_write_delete_table", "write", true, "foo_read_write_delete", true},
		{"try foo_read_write_delete write no_user_read_table without permission", "no_user_read_table", "write", true, "foo_read_write_delete", false},
		{"try foo_read_write_delete write no_user_write_table", "no_user_write_table", "write", true, "foo_read_write_delete", true},
		{"try foo_read_write_delete write no_user_delete_table without permission", "no_user_delete_table", "write", true, "foo_read_write_delete", false},
		{"try foo_read_write_delete write no_user_read_write_table", "no_user_read_write_table", "write", true, "foo_read_write_delete", true},
		{"try foo_read_write_delete write no_user_read_delete_table without permission", "no_user_read_delete_table", "write", true, "foo_read_write_delete", false},
		{"try foo_read_write_delete write no_user_write_delete_table", "no_user_write_delete_table", "write", true, "foo_read_write_delete", true},
		{"try foo_read_write_delete write no_user_read_write_delete_table", "no_user_read_write_delete_table", "write", true, "foo_read_write_delete", true},
		{"try foo_read_write_delete delete read_table", "read_table", "delete", true, "foo_read_write_delete", true},
		{"try foo_read_write_delete delete write_table", "write_table", "delete", true, "foo_read_write_delete", true},
		{"try foo_read_write_delete delete delete_table", "delete_table", "delete", true, "foo_read_write_delete", true},
		{"try foo_read_write_delete delete read_write_table", "read_write_table", "delete", true, "foo_read_write_delete", true},
		{"try foo_read_write_delete delete read_delete_table", "read_delete_table", "delete", true, "foo_read_write_delete", true},
		{"try foo_read_write_delete delete write_delete_table", "write_delete_table", "delete", true, "foo_read_write_delete", true},
		{"try foo_read_write_delete delete read_write_delete_table", "read_write_delete_table", "delete", true, "foo_read_write_delete", true},
		{"try foo_read_write_delete delete no_user_read_table without permission", "no_user_read_table", "delete", true, "foo_read_write_delete", false},
		{"try foo_read_write_delete delete no_user_write_table without permission", "no_user_write_table", "delete", true, "foo_read_write_delete", false},
		{"try foo_read_write_delete delete no_user_delete_table", "no_user_delete_table", "delete", true, "foo_read_write_delete", true},
		{"try foo_read_write_delete delete no_user_read_write_table without permission", "no_user_read_write_table", "delete", true, "foo_read_write_delete", false},
		{"try foo_read_write_delete delete no_user_read_delete_table", "no_user_read_delete_table", "delete", true, "foo_read_write_delete", true},
		{"try foo_read_write_delete delete no_user_write_delete_table", "no_user_write_delete_table", "delete", true, "foo_read_write_delete", true},
		{"try foo_read_write_delete delete no_user_read_write_delete_table", "no_user_read_write_delete_table", "delete", true, "foo_read_write_delete", true},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m := &Mock{
				mtx: &sync.RWMutex{},
				t:   t,
				AccessConf: config.AccessConf{
					Restrict: tt.restrict,
					Tables:   tables,
					Users:    users,
				},
			}
			if gotOk := m.TablePermissions("", "", tt.table, tt.op, tt.userName); gotOk != tt.wantOk {
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
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

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
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

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
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

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
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

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
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

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
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

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
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

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
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

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
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

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

func TestMock_GetTransaction(t *testing.T) {
	tests := []struct {
		t       *testing.T
		name    string
		wantTx  *sql.Tx
		wantErr bool
	}{
		{
			name:    "transaction not nil",
			wantErr: false,
			wantTx:  &sql.Tx{},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m := New(t)
			gotTx, err := m.GetTransaction()
			errMactches := (err != nil) != tt.wantErr
			if errMactches {
				t.Errorf("Mock.GetTransaction() error = %v, wantErr %v", err, tt.wantErr)
			}
			if gotTx == nil {
				t.Error("expected not nil, got nil")
			}
			// should not panic on commit
			if err := gotTx.Commit(); err != nil {
				t.Errorf("expected nil, got %v", err)
			}
		})
	}
}

func TestMockEmptyMethods(t *testing.T) {
	mock := Mock{
		mtx:   &sync.RWMutex{},
		t:     t,
		Items: []Item{{}},
	}

	var err error

	// GetScript
	_, err = mock.GetScript("WRITE", "folder", "select")
	if err != nil {
		t.Errorf("expected empty return, got: %s", err)
	}

	// ParseScript
	_, _, err = mock.ParseScript("path/to/script", map[string]interface{}{"q": []string{"test"}})
	if err != nil {
		t.Errorf("expected empty return, got: %s", err)
	}

	// ExecuteScripts
	sc := mock.ExecuteScripts("READ", "", nil)
	if sc != nil {
		t.Errorf("expected empty return, got: %s", err)
	}

	// WhereByRequest
	_, _, err = mock.WhereByRequest(&http.Request{}, 1)
	if err != nil {
		t.Errorf("expected empty return, got: %s", err)
	}

	// ReturningByRequest
	_, err = mock.ReturningByRequest(&http.Request{})
	if err != nil {
		t.Errorf("expected empty return, got: %s", err)
	}

	// OrderByRequest
	_, err = mock.OrderByRequest(&http.Request{})
	if err != nil {
		t.Errorf("expected empty return, got: %s", err)
	}

	// PaginateIfPossible
	_, err = mock.PaginateIfPossible(&http.Request{})
	if err != nil {
		t.Errorf("expected empty return, got: %s", err)
	}

	// GetTransaction
	_, err = mock.GetTransaction()
	if err != nil {
		t.Errorf("expected empty return, got: %s", err)
	}

	// FieldsPermissions
	fields, err := mock.FieldsPermissions(&http.Request{}, "", "public", "test", "select", "")
	if err != nil {
		t.Errorf("expected empty return, got: %s", err)
	}
	if len(fields) > 1 {
		t.Errorf("expected one field, got: %d", len(fields))
	}

	// SelectFields
	_, err = mock.SelectFields(fields)
	if err != nil {
		t.Errorf("expected empty return, got: %s", err)
	}

	// CountByRequest
	_, err = mock.CountByRequest(&http.Request{})
	if err != nil {
		t.Errorf("expected empty return, got: %s", err)
	}

	// JoinByRequest
	_, err = mock.JoinByRequest(&http.Request{})
	if err != nil {
		t.Errorf("expected empty return, got: %s", err)
	}

	// GroupByClause
	groupBySQL := mock.GroupByClause(&http.Request{})
	if groupBySQL != "" {
		t.Errorf("expected empty return, got: %s", groupBySQL)
	}

	// ParseInsertRequest
	_, _, _, err = mock.ParseInsertRequest(&http.Request{})
	if err != nil {
		t.Errorf("expected empty return, got: %s", err)
	}

	// InsertWithTransaction
	sc = mock.InsertWithTransaction(&sql.Tx{}, "select 1", "test1", "test2")
	if sc == nil {
		t.Errorf("expected empty return, got: %x", sc)
	}

	// DeleteWithTransaction
	mock.Items = []Item{{}}
	sc = mock.DeleteWithTransaction(&sql.Tx{}, "select 1", "test1", "test2")
	if sc == nil {
		t.Errorf("expected empty return, got: %x", sc)
	}

	// SetByRequest
	_, _, err = mock.SetByRequest(&http.Request{}, 1)
	if err != nil {
		t.Errorf("expected empty return, got: %s", err)
	}

	// UpdateWithTransaction
	mock.Items = []Item{{}}
	sc = mock.UpdateWithTransaction(&sql.Tx{}, "select 1", "test1", "test2")
	if sc == nil {
		t.Errorf("expected empty return, got: %x", sc)
	}

	// DistinctClause
	_, err = mock.DistinctClause(&http.Request{})
	if err != nil {
		t.Errorf("expected empty return, got: %s", err)
	}

	// SetDatabase
	mock.SetDatabase("prest")

	// SelectSQL
	s := mock.SelectSQL("select 1", "prest", "public", "test1")
	if s != "" {
		t.Errorf("expected empty return, got: %s", s)
	}

	// InsertSQL
	s = mock.InsertSQL("prest", "public", "test1", "test0,test2,test3", "")
	if s != "" {
		t.Errorf("expected empty return, got: %s", s)
	}

	// DeleteSQL
	s = mock.DeleteSQL("prest", "public", "test1")
	if s != "" {
		t.Errorf("expected empty return, got: %s", s)
	}

	// UpdateSQL
	s = mock.UpdateSQL("prest", "public", "test1", "")
	if s != "" {
		t.Errorf("expected empty return, got: %s", s)
	}

	// DatabaseWhere
	s = mock.DatabaseWhere("")
	if s != "" {
		t.Errorf("expected empty return, got: %s", s)
	}

	// DatabaseOrderBy
	s = mock.DatabaseOrderBy("DESC", false)
	if s != "" {
		t.Errorf("expected empty return, got: %s", s)
	}

	// SchemaOrderBy
	s = mock.SchemaOrderBy("ASC", true)
	if s != "" {
		t.Errorf("expected empty return, got: %s", s)
	}

	// TableClause
	s = mock.TableClause()
	if s != "" {
		t.Errorf("expected empty return, got: %s", s)
	}

	// TableWhere
	s = mock.TableWhere("")
	if s != "" {
		t.Errorf("expected empty return, got: %s", s)
	}

	// TableOrderBy
	s = mock.TableOrderBy("")
	if s != "" {
		t.Errorf("expected empty return, got: %s", s)
	}

	// SchemaTablesClause
	s = mock.SchemaTablesClause()
	if s != "" {
		t.Errorf("expected empty return, got: %s", s)
	}

	// SchemaTablesWhere
	s = mock.SchemaTablesWhere("")
	if s != "" {
		t.Errorf("expected empty return, got: %s", s)
	}

	// SchemaTablesOrderBy
	s = mock.SchemaTablesOrderBy("")
	if s != "" {
		t.Errorf("expected empty return, got: %s", s)
	}

	// ParseBatchInsertRequest
	_, _, _, err = mock.ParseBatchInsertRequest(&http.Request{})
	if err != nil {
		t.Errorf("expected empty return, got: %x", sc)
	}

	// BatchInsertCopy
	mock.Items = []Item{{}}
	sc = mock.BatchInsertCopy("prest", "public", "test1", []string{"key1", "key2", "key3"}, "val1", "val2", "val3")
	if sc == nil {
		t.Errorf("expected empty return, got: %x", sc)
	}
}

func TestMock_QueryCtx(t *testing.T) {
	tests := []struct {
		name    string
		item    Item
		isQuery bool
	}{
		{
			"query ctx with body",
			Item{
				Body: []byte(`[{"test":"test"}]`),
			},
			true,
		},
		{
			"query ctx with error",
			Item{
				Error: errors.New("test error"),
			},
			true,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m := &Mock{
				mtx: &sync.RWMutex{},
				t:   t,
			}
			m.AddItem(tt.item.Body, tt.item.Error, tt.item.IsCount)
			ctx := context.Background()
			gotSc := m.QueryCtx(ctx, "SELECT * FROM test", "param1")
			if gotSc == nil {
				t.Errorf("QueryCtx() returned nil, want Scanner")
			}
		})
	}
}

func TestMock_QueryCountCtx(t *testing.T) {
	tests := []struct {
		name string
		item Item
	}{
		{
			"query count ctx with body",
			Item{
				Body: []byte(`[{"count":10}]`),
			},
		},
		{
			"query count ctx with error",
			Item{
				Error: errors.New("count error"),
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m := &Mock{
				mtx: &sync.RWMutex{},
				t:   t,
			}
			m.AddItem(tt.item.Body, tt.item.Error, false)
			ctx := context.Background()
			gotSc := m.QueryCountCtx(ctx, "SELECT COUNT(*) FROM test")
			if gotSc == nil {
				t.Errorf("QueryCountCtx() returned nil, want Scanner")
			}
		})
	}
}

func TestMock_InsertCtx(t *testing.T) {
	tests := []struct {
		name string
		item Item
	}{
		{
			"insert ctx with success",
			Item{
				Body: []byte(`{"id":1}`),
			},
		},
		{
			"insert ctx with error",
			Item{
				Error: errors.New("insert failed"),
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m := &Mock{
				mtx: &sync.RWMutex{},
				t:   t,
			}
			m.AddItem(tt.item.Body, tt.item.Error, false)
			ctx := context.Background()
			gotSc := m.InsertCtx(ctx, "INSERT INTO test VALUES ($1)", "value1")
			if gotSc == nil {
				t.Errorf("InsertCtx() returned nil, want Scanner")
			}
		})
	}
}

func TestMock_DeleteCtx(t *testing.T) {
	tests := []struct {
		name string
		item Item
	}{
		{
			"delete ctx with success",
			Item{
				Body: []byte(`{"affected":1}`),
			},
		},
		{
			"delete ctx with error",
			Item{
				Error: errors.New("delete failed"),
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m := &Mock{
				mtx: &sync.RWMutex{},
				t:   t,
			}
			m.AddItem(tt.item.Body, tt.item.Error, false)
			ctx := context.Background()
			gotSc := m.DeleteCtx(ctx, "DELETE FROM test WHERE id=$1", 1)
			if gotSc == nil {
				t.Errorf("DeleteCtx() returned nil, want Scanner")
			}
		})
	}
}

func TestMock_UpdateCtx(t *testing.T) {
	tests := []struct {
		name string
		item Item
	}{
		{
			"update ctx with success",
			Item{
				Body: []byte(`{"affected":1}`),
			},
		},
		{
			"update ctx with error",
			Item{
				Error: errors.New("update failed"),
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m := &Mock{
				mtx: &sync.RWMutex{},
				t:   t,
			}
			m.AddItem(tt.item.Body, tt.item.Error, false)
			ctx := context.Background()
			gotSc := m.UpdateCtx(ctx, "UPDATE test SET name=$1 WHERE id=$2", "newname", 1)
			if gotSc == nil {
				t.Errorf("UpdateCtx() returned nil, want Scanner")
			}
		})
	}
}

func TestMock_BatchInsertValuesCtx(t *testing.T) {
	tests := []struct {
		name string
		item Item
	}{
		{
			"batch insert values ctx with success",
			Item{
				Body: []byte(`[{"id":1},{"id":2}]`),
			},
		},
		{
			"batch insert values ctx with error",
			Item{
				Error: errors.New("batch insert failed"),
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m := &Mock{
				mtx: &sync.RWMutex{},
				t:   t,
			}
			m.AddItem(tt.item.Body, tt.item.Error, false)
			ctx := context.Background()
			gotSc := m.BatchInsertValuesCtx(ctx, "INSERT INTO test VALUES ($1), ($2)", "val1", "val2")
			if gotSc == nil {
				t.Errorf("BatchInsertValuesCtx() returned nil, want Scanner")
			}
		})
	}
}

func TestMock_BatchInsertCopyCtx(t *testing.T) {
	tests := []struct {
		name string
		item Item
	}{
		{
			"batch insert copy ctx with success",
			Item{
				Body: []byte(`{"rows":2}`),
			},
		},
		{
			"batch insert copy ctx with error",
			Item{
				Error: errors.New("copy failed"),
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m := &Mock{
				mtx: &sync.RWMutex{},
				t:   t,
			}
			m.AddItem(tt.item.Body, tt.item.Error, false)
			ctx := context.Background()
			gotSc := m.BatchInsertCopyCtx(ctx, "prest", "public", "test", []string{"col1", "col2"}, "val1", "val2")
			if gotSc == nil {
				t.Errorf("BatchInsertCopyCtx() returned nil, want Scanner")
			}
		})
	}
}

func TestMock_GetTransactionCtx(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"get transaction ctx success", false},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m := New(t)
			ctx := context.Background()
			gotTx, err := m.GetTransactionCtx(ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("Mock.GetTransactionCtx() error = %v, wantErr %v", err, tt.wantErr)
			}
			if gotTx == nil && !tt.wantErr {
				t.Error("expected not nil, got nil")
			}
			if gotTx != nil {
				_ = gotTx.Commit()
			}
		})
	}
}

func TestMock_ExecuteScriptsCtx(t *testing.T) {
	tests := []struct {
		name   string
		method string
		sql    string
		values []interface{}
	}{
		{"execute scripts ctx READ", "READ", "SELECT * FROM test", []interface{}{}},
		{"execute scripts ctx WRITE", "WRITE", "INSERT INTO test VALUES ($1)", []interface{}{"val1"}},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m := &Mock{
				mtx: &sync.RWMutex{},
				t:   t,
			}
			ctx := context.Background()
			gotSc := m.ExecuteScriptsCtx(ctx, tt.method, tt.sql, tt.values)
			if gotSc != nil {
				t.Errorf("ExecuteScriptsCtx() expected nil, got %v", gotSc)
			}
		})
	}
}

func TestMock_SetDatabase(t *testing.T) {
	tests := []struct {
		name   string
		dbName string
	}{
		{"set database prest", "prest"},
		{"set database testdb", "testdb"},
		{"set database empty", ""},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m := &Mock{
				mtx: &sync.RWMutex{},
				t:   t,
			}
			m.SetDatabase(tt.dbName)
			// SetDatabase is a no-op stub, just verify it doesn't panic
		})
	}
}

func TestMock_GetDatabase(t *testing.T) {
	m := &Mock{
		mtx: &sync.RWMutex{},
		t:   t,
	}
	db := m.GetDatabase()
	if db != "" {
		t.Errorf("GetDatabase() expected empty string, got %q", db)
	}
}

func TestMock_IsRegistered(t *testing.T) {
	tests := []struct {
		name  string
		alias string
		want  bool
	}{
		{"is registered prest", "prest", true},
		{"is registered unknown", "unknown", true},
		{"is registered empty", "", true},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m := &Mock{
				mtx: &sync.RWMutex{},
				t:   t,
			}
			if got := m.IsRegistered(tt.alias); got != tt.want {
				t.Errorf("IsRegistered(%q) = %v, want %v", tt.alias, got, tt.want)
			}
		})
	}
}

func TestMock_PhysicalName(t *testing.T) {
	tests := []struct {
		name  string
		alias string
		want  string
	}{
		{"physical name prest", "prest", "prest"},
		{"physical name testdb", "testdb", "testdb"},
		{"physical name empty", "", ""},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m := &Mock{
				mtx: &sync.RWMutex{},
				t:   t,
			}
			if got := m.PhysicalName(tt.alias); got != tt.want {
				t.Errorf("PhysicalName(%q) = %q, want %q", tt.alias, got, tt.want)
			}
		})
	}
}

func TestMock_Ping(t *testing.T) {
	m := &Mock{
		mtx: &sync.RWMutex{},
		t:   t,
	}
	ctx := context.Background()
	err := m.Ping(ctx)
	if err != nil {
		t.Errorf("Ping() expected nil, got %v", err)
	}
}

func TestMock_PingAll(t *testing.T) {
	m := &Mock{
		mtx: &sync.RWMutex{},
		t:   t,
	}
	ctx := context.Background()
	err := m.PingAll(ctx)
	if err != nil {
		t.Errorf("PingAll() expected nil, got %v", err)
	}
}

func TestMock_ShowTable(t *testing.T) {
	tests := []struct {
		name   string
		schema string
		table  string
	}{
		{
			"show table public users",
			"public",
			"users",
		},
		{
			"show table nonexistent",
			"public",
			"nonexistent",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m := &Mock{
				mtx: &sync.RWMutex{},
				t:   t,
			}
			// ShowTable is a stub that returns nil
			gotSc := m.ShowTable(tt.schema, tt.table)
			if gotSc != nil {
				t.Errorf("ShowTable() expected nil, got %v", gotSc)
			}
		})
	}
}

func TestMock_ShowTableCtx(t *testing.T) {
	tests := []struct {
		name   string
		schema string
		table  string
	}{
		{
			"show table ctx public users",
			"public",
			"users",
		},
		{
			"show table ctx nonexistent",
			"public",
			"nonexistent",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m := &Mock{
				mtx: &sync.RWMutex{},
				t:   t,
			}
			ctx := context.Background()
			// ShowTableCtx is a stub that returns nil
			gotSc := m.ShowTableCtx(ctx, tt.schema, tt.table)
			if gotSc != nil {
				t.Errorf("ShowTableCtx() expected nil, got %v", gotSc)
			}
		})
	}
}

func TestMock_ShowColumnsCtx(t *testing.T) {
	t.Parallel()

	m := &Mock{
		mtx: &sync.RWMutex{},
		t:   t,
	}
	gotSc := m.ShowColumnsCtx(context.Background())
	if gotSc != nil {
		t.Errorf("ShowColumnsCtx() expected nil, got %v", gotSc)
	}
}

func TestMock_New(t *testing.T) {
	m := New(t)
	if m == nil {
		t.Error("New() returned nil, want *Mock")
	}
	if m.mtx == nil {
		t.Error("New() mutex is nil")
	}
	if m.t == nil {
		t.Error("New() testing.T is nil")
	}
	// Verify driver is registered
	drivers := sql.Drivers()
	found := false
	for _, d := range drivers {
		if d == "mock" {
			found = true
			break
		}
	}
	if !found {
		t.Error("mock driver not registered after New()")
	}
}

func TestMock_Open(t *testing.T) {
	tests := []struct {
		name    string
		dsn     string
		wantErr bool
	}{
		{"open prest connection", "prest", false},
		{"open unknown connection", "unknown", true},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			m := New(t)
			gotConn, err := m.Open(tt.dsn)
			if (err != nil) != tt.wantErr {
				t.Errorf("Open(%q) error = %v, wantErr %v", tt.dsn, err, tt.wantErr)
			}
			if !tt.wantErr && gotConn == nil {
				t.Error("Open() returned nil connection")
			}
		})
	}
}

// Additional integration-style tests for better coverage

func TestMock_ConcurrentAddItem(t *testing.T) {
	m := New(t)
	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func(idx int) {
			body := []byte(`[{"id":` + fmt.Sprintf("%d", idx) + `}]`)
			m.AddItem(body, nil, false)
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	if len(m.Items) < 10 {
		t.Errorf("expected at least 10 items, got %d", len(m.Items))
	}
}

func TestMock_PermissionsEdgeCases(t *testing.T) {
	m := New(t)
	m.AccessConf = config.AccessConf{
		Restrict: true,
		Tables: []config.TablesConf{
			{
				Name:        "restricted_table",
				Permissions: []string{"read"},
			},
		},
		Users: []config.UsersConf{
			{
				Name: "readonly_user",
				Tables: []config.TablesConf{
					{
						Name:        "restricted_table",
						Permissions: []string{"read"},
					},
				},
			},
		},
	}

	tests := []struct {
		name     string
		table    string
		op       string
		userName string
		want     bool
	}{
		{"restricted table, allowed op, allowed user", "restricted_table", "read", "readonly_user", true},
		{"restricted table, disallowed op, allowed user", "restricted_table", "write", "readonly_user", false},
		{"restricted table, allowed op, no user", "restricted_table", "read", "", true},
		{"restricted table, disallowed op, no user", "restricted_table", "write", "", false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := m.TablePermissions("", "", tt.table, tt.op, tt.userName)
			if got != tt.want {
				t.Errorf("TablePermissions() = %v, want %v", got, tt.want)
			}
		})
	}
}
