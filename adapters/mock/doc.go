// Package mock provides a lightweight pREST adapter for tests.
//
// The adapter lets tests exercise controllers and middleware without a live
// PostgreSQL server. Create it with New(t), assign it to config.PrestConf.Adapter,
// and enqueue the scanner results that each adapter operation should return:
//
//	m := mock.New(t)
//	config.PrestConf.Adapter = m
//	m.AddItem([]byte(`[{"id":1,"name":"Ada"}]`), nil, false)
//
// Results are consumed in FIFO order. Methods such as Query, QueryCtx, Insert,
// Update, Delete, BatchInsertValues, and BatchInsertCopy pop one Item from the
// queue and return it as an adapters.Scanner. Item.Body becomes the scanner
// buffer, Item.Error becomes the scanner error, and Item.IsCount controls the
// count flag used by clause helpers such as DatabaseClause and SchemaClause.
//
// Add one Item per expected adapter operation. If a test calls an operation
// without a queued item, the mock fails the test immediately, making missing
// expectations visible.
package mock
