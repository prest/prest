package controllers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/prest/prest/v2/adapters"
	"github.com/prest/prest/v2/adapters/mockgen"
	pctx "github.com/prest/prest/v2/context"
	"github.com/stretchr/testify/require"
)

// scopedAdapter is a registry adapter pinned to schemas it allows, like a
// MySQL alias that serves only its own physical database as {schema}.
type scopedAdapter struct {
	*mockgen.MockAdapter
	allow bool
}

func (a scopedAdapter) AllowsSchema(string) bool { return a.allow }

var _ adapters.SchemaScoper = scopedAdapter{}

func newScopedAdapter(ctrl *gomock.Controller, allow bool) scopedAdapter {
	a := scopedAdapter{MockAdapter: mockgen.NewMockAdapter(ctrl), allow: allow}
	a.EXPECT().IsRegistered(gomock.Any()).Return(true).AnyTimes()
	a.EXPECT().GetDatabase().Return("shop").AnyTimes()
	return a
}

func withAdapter(req *http.Request, a adapters.Adapter) *http.Request {
	return req.WithContext(context.WithValue(req.Context(), pctx.AdapterKey, a))
}

func TestValidateSchema(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// No adapter on the request: nothing to scope.
	require.NoError(t, validateSchema(req, "other"))
	// A plain adapter (Postgres) does not scope schemas.
	require.NoError(t, validateSchema(withAdapter(req, mockgen.NewMockAdapter(ctrl)), "other"))
	require.NoError(t, validateSchema(withAdapter(req, newScopedAdapter(ctrl, true)), "shop"))
	require.ErrorIs(t, validateSchema(withAdapter(req, newScopedAdapter(ctrl, false)), "other"), errSchemaNotInDatabase)
}

// A MySQL alias must not reach another physical database through {schema}:
// every table route answers 404 before any SQL is built or run.
func TestHandlers_RejectSchemaOutsideAlias(t *testing.T) {
	t.Parallel()

	vars := map[string]string{"database": "shop", "schema": "billing", "table": "invoices"}
	cases := []struct {
		name string
		call func(*gomock.Controller, http.ResponseWriter, *http.Request)
	}{
		{"select", func(c *gomock.Controller, w http.ResponseWriter, r *http.Request) {
			NewCRUDHandler(Deps{}).Select(w, r)
		}},
		{"insert", func(c *gomock.Controller, w http.ResponseWriter, r *http.Request) {
			NewCRUDHandler(Deps{}).Insert(w, r)
		}},
		{"batch insert", func(c *gomock.Controller, w http.ResponseWriter, r *http.Request) {
			NewCRUDHandler(Deps{}).BatchInsert(w, r)
		}},
		{"update", func(c *gomock.Controller, w http.ResponseWriter, r *http.Request) {
			NewCRUDHandler(Deps{}).Update(w, r)
		}},
		{"delete", func(c *gomock.Controller, w http.ResponseWriter, r *http.Request) {
			NewCRUDHandler(Deps{}).Delete(w, r)
		}},
		{"table show", func(c *gomock.Controller, w http.ResponseWriter, r *http.Request) {
			NewTableHandler(nil, nil, false).Show(w, r)
		}},
		{"list tables by schema", func(c *gomock.Controller, w http.ResponseWriter, r *http.Request) {
			NewCatalogHandler(Deps{}).ListTablesByDatabaseAndSchema(w, r)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// The scoped adapter has no other expectations: any SQL call fails the test.
			req := withAdapter(crudRequest(http.MethodGet, "/shop/billing/invoices", vars), newScopedAdapter(ctrl, false))
			rec := httptest.NewRecorder()
			tc.call(ctrl, rec, req)

			require.Equal(t, http.StatusNotFound, rec.Code)
			require.Contains(t, rec.Body.String(), errSchemaNotInDatabase.Error())
		})
	}
}

// When the alias serves the requested schema the handler proceeds as usual.
func TestTableHandler_Show_AllowedSchemaProceeds(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	a := newScopedAdapter(ctrl, true)
	scanner := mockgen.NewMockScanner(ctrl)
	scanner.EXPECT().Err().Return(nil)
	scanner.EXPECT().Bytes().Return([]byte(`[{"column_name":"id"}]`))
	a.EXPECT().ShowTableCtx(gomock.Any(), "shop", "items").Return(scanner)

	req := withAdapter(crudRequest(http.MethodGet, "/shop/shop/items", map[string]string{
		"database": "shop", "schema": "shop", "table": "items",
	}), a)
	rec := httptest.NewRecorder()
	NewTableHandler(nil, nil, false).Show(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "column_name")
}

// MCP tools address databases through the registry, not request context; the
// alias's adapter still decides which schemas it serves.
func TestMCPHandler_RejectsSchemaOutsideAlias(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	registry := adapters.NewRegistry()
	require.NoError(t, registry.Register("shop", newScopedAdapter(ctrl, false)))
	h := NewMCPHandler(Deps{AdapterRegistry: registry, Dialect: pgDialect{}, DB: mockDatabaseRegistry(ctrl), PGDatabase: "shop"})
	req := httptest.NewRequest(http.MethodGet, "/_mcp", nil)

	_, err := h.selectTable(req, mcpSelectArgs{Database: "shop", Schema: "billing", Table: "invoices"})
	require.ErrorIs(t, err, errSchemaNotInDatabase)

	_, err = h.describeTable(req, mcpDescribeArgs{Database: "shop", Schema: "billing", Table: "invoices"})
	require.ErrorIs(t, err, errSchemaNotInDatabase)
}

func TestMCPHandler_ValidateToolTarget_AllowedSchema(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	registry := adapters.NewRegistry()
	require.NoError(t, registry.Register("shop", newScopedAdapter(ctrl, true)))
	h := NewMCPHandler(Deps{AdapterRegistry: registry, DB: mockDatabaseRegistry(ctrl), PGDatabase: "shop"})

	require.NoError(t, h.validateToolTarget("shop", "shop", "items"))
}

func TestRegistryAliases(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	require.Nil(t, registryAliases(nil, false, ""))
	require.Equal(t, []string{"only"}, registryAliases(nil, true, "only"))
	require.Nil(t, registryAliases(nil, true, ""))

	db := mockgen.NewMockDatabaseRegistry(ctrl)
	db.EXPECT().Aliases().Return([]string{"a", "b", "a"})
	require.Equal(t, []string{"a", "b"}, registryAliases(db, false, "a"))

	empty := mockgen.NewMockDatabaseRegistry(ctrl)
	empty.EXPECT().Aliases().Return(nil)
	require.Equal(t, []string{"fallback"}, registryAliases(empty, false, "fallback"))
}

func TestPhysicalName(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	require.Equal(t, "alias", physicalName(nil, "alias"))
	db := mockgen.NewMockDatabaseRegistry(ctrl)
	db.EXPECT().PhysicalName("alias").Return("real_db")
	require.Equal(t, "real_db", physicalName(db, "alias"))
}
