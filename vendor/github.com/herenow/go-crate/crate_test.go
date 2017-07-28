// NOTE: this tests were written posteriorly
package crate

import "testing"
import "database/sql"

//import "fmt"

func connect() (*sql.DB, error) {
	return sql.Open("crate", "http://127.0.0.1:4200/")
}

func TestConnect(t *testing.T) {
	_, err := connect()

	if err != nil {
		t.Fatalf("Error connecting: %s", err.Error())
	}
}

func TestQuery(t *testing.T) {
	db, _ := connect()

	rows, err := db.Query("select count(*) from sys.cluster limit ?", 1)

	if err != nil {
		t.Fatalf("Error on db.Query: %s", err.Error())
	}

	cols, _ := rows.Columns()
	n := len(cols)

	if n != 1 {
		t.Error(
			"rows.Columns expected 1, but got,",
			n,
			cols,
		)
	}

	rows, err = db.Query("select column_name from information_schema.columns")

	for rows.Next() {
		var column string

		if err = rows.Scan(&column); err != nil {
			t.Error(err)
		}
	}
}

func TestExec(t *testing.T) {
	db, _ := connect()

	_, err := db.Exec("create table go_crate (id int, str string)")

	if err != nil {
		t.Error(err)
	}

	_, err = db.Exec("drop table go_crate")

	if err != nil {
		t.Error(err)
	}
}

func TestQueryRowBigInt(t *testing.T) {
	db, _ := connect()

	row := db.QueryRow("select 655300500 from sys.cluster ")

	var test int
	err := row.Scan(&test)

	if err != nil {
		t.Fatalf("Error on QueryRow.Scan", err)
	}

	if test != 655300500 {
		t.Error("Expected 1 on test, but got", test)
	}
}

func TestPreparedStmtExec(t *testing.T) {
	db, _ := connect()

	stmt, err := db.Prepare("create table go_crate (id int, str string)")
	stmt2, err2 := db.Prepare("drop table go_crate")

	if err != nil || err2 != nil {
		t.Fatalf("Error on db.Prepared()", err, err2)
	}

	_, err = stmt.Exec()

	if err != nil {
		t.Error(err)
	}

	_, err = stmt2.Exec()

	if err != nil {
		t.Error(err)
	}
}
