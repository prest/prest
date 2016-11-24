package postgres

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/caarlos0/env"
	"github.com/jmoiron/sqlx"
	// Used pg drive on sqlx
	_ "github.com/lib/pq"

	"github.com/nuveo/prest/config"
)

// Conn connect on PostgreSQL
// Used sqlx
func Conn() (db *sqlx.DB) {
	cfg := config.Prest{}
	env.Parse(&cfg)

	db, err := sqlx.Connect("postgres", fmt.Sprintf("user=%s dbname=%s sslmode=disable", cfg.PGUser, cfg.PGDatabase))
	if err != nil {
		panic(fmt.Sprintf("Unable to connection to database: %v\n", err))
	}
	return
}

// WhereByRequest create interface for queries + where
func WhereByRequest(r *http.Request) (whereSyntax string) {
	u, _ := url.Parse(r.URL.String())
	where := []string{}
	for key, val := range u.Query() {
		where = append(where, fmt.Sprintf("%s='%s'", key, val[0]))
	}
	whereSyntax = strings.Join(where, " and ")
	return
}

// Query process queries
func Query(SQL string) (jsonData []byte, err error) {
	db := Conn()
	rows, _ := db.Queryx(SQL)
	defer rows.Close()

	columns, _ := rows.Columns()

	count := len(columns)
	tableData := make([]map[string]interface{}, 0)
	values := make([]interface{}, count)
	valuePtrs := make([]interface{}, count)
	for rows.Next() {
		for i := 0; i < count; i++ {
			valuePtrs[i] = &values[i]
		}
		rows.Scan(valuePtrs...)
		entry := make(map[string]interface{})
		for i, col := range columns {
			var v interface{}
			val := values[i]
			b, ok := val.([]byte)
			if ok {
				v = string(b)
			} else {
				v = val
			}
			entry[col] = v
		}
		tableData = append(tableData, entry)
	}
	jsonData, err = json.Marshal(tableData)

	return
}
