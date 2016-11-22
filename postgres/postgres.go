package postgres

import (
	"fmt"

	"github.com/jackc/pgx"
	"github.com/nuveo/prest/config"
)

// Conn connect on PostgreSQL
func Conn() (conn *pgx.Conn) {
	conn, err := pgx.Connect(config.PrestPg())
	if err != nil {
		panic(fmt.Sprintf("Unable to connection to database: %v\n", err))
	}
	return
}
