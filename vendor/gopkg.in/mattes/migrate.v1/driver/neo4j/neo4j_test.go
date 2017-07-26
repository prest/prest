package neo4j

import (
	"os"
	"testing"

	"github.com/jmcvetta/neoism"
	"gopkg.in/mattes/migrate.v1/file"
	"gopkg.in/mattes/migrate.v1/migrate/direction"
	pipep "gopkg.in/mattes/migrate.v1/pipe"
)

// TestMigrate runs some additional tests on Migrate().
// Basic testing is already done in migrate/migrate_test.go
func TestMigrate(t *testing.T) {
	t.Skip("TODO: fix test: neo4j_test.go:26: Get http://neo4j:test@/db/data/: http: no Host in request URL")

	host := os.Getenv("NEO4J_PORT_7474_TCP_ADDR")
	port := os.Getenv("NEO4J_PORT_7474_TCP_PORT")

	driverUrl := "http://neo4j:test@" + host + ":" + port + "/db/data"

	// prepare clean database
	db, err := neoism.Connect(driverUrl)
	if err != nil {
		t.Fatal(err)
	}

	cq := neoism.CypherQuery{
		Statement: `DROP INDEX ON :Yolo(name)`,
	}

	// If an error dropping the index then ignore it
	db.Cypher(&cq)

	driverUrl = "neo4j://neo4j:test@" + host + ":" + port + "/db/data"

	d := &Driver{}
	if err := d.Initialize(driverUrl); err != nil {
		t.Fatal(err)
	}

	files := []file.File{
		{
			Path:      "/foobar",
			FileName:  "001_foobar.up.cql",
			Version:   1,
			Name:      "foobar",
			Direction: direction.Up,
			Content: []byte(`
        CREATE INDEX ON :Yolo(name)
      `),
		},
		{
			Path:      "/foobar",
			FileName:  "001_foobar.down.cql",
			Version:   1,
			Name:      "foobar",
			Direction: direction.Down,
			Content: []byte(`
        DROP INDEX ON :Yolo(name)
      `),
		},
		{
			Path:      "/foobar",
			FileName:  "002_foobar.up.cql",
			Version:   2,
			Name:      "foobar",
			Direction: direction.Up,
			Content: []byte(`
        CREATE INDEX :Yolo(name) THIS WILL CAUSE AN ERROR
      `),
		},
	}

	pipe := pipep.New()
	go d.Migrate(files[0], pipe)
	errs := pipep.ReadErrors(pipe)
	if len(errs) > 0 {
		t.Fatal(errs)
	}

	pipe = pipep.New()
	go d.Migrate(files[1], pipe)
	errs = pipep.ReadErrors(pipe)
	if len(errs) > 0 {
		t.Fatal(errs)
	}

	pipe = pipep.New()
	go d.Migrate(files[2], pipe)
	errs = pipep.ReadErrors(pipe)
	if len(errs) == 0 {
		t.Error("Expected test case to fail")
	}

	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
}
