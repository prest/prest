package crate_test

import (
	"database/sql"
	"fmt"
	_ "github.com/herenow/go-crate"
	"log"
)

func ExampleCrateDrive_Open() {
	_, err := sql.Open("crate", "http://localhost:4200")

	if err != nil {
		log.Fatal(err)
	}
}

func ExampleCrateDriver_Query() {
	db, err := sql.Open("crate", "http://localhost:4200")

	if err != nil {
		log.Fatal(err)
	}

	rows, err := db.Query("SELECT name FROM sys.cluster")

	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	for rows.Next() {
		var name string

		if err := rows.Scan(&name); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%s\n", name)
	}

	if err := rows.Err(); err != nil {
		log.Fatal(err)
	}
}
