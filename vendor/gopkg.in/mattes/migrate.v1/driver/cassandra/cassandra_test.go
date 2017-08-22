package cassandra

import (
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/gocql/gocql"
	"gopkg.in/mattes/migrate.v1/file"
	"gopkg.in/mattes/migrate.v1/migrate/direction"
	pipep "gopkg.in/mattes/migrate.v1/pipe"
)

func TestMigrate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	var session *gocql.Session

	host := os.Getenv("CASSANDRA_PORT_9042_TCP_ADDR")
	port := os.Getenv("CASSANDRA_PORT_9042_TCP_PORT")
	driverURL := "cassandra://" + host + ":" + port + "/system"

	// prepare a clean test database
	u, err := url.Parse(driverURL)
	if err != nil {
		t.Fatal(err)
	}

	cluster := gocql.NewCluster(u.Host)
	cluster.Keyspace = u.Path[1:len(u.Path)]
	cluster.Consistency = gocql.All
	cluster.Timeout = 1 * time.Minute

	session, err = cluster.CreateSession()
	if err != nil {
		t.Fatal(err)
	}

	if err = session.Query(`DROP KEYSPACE IF EXISTS migrate;`).Exec(); err != nil {
		t.Fatal(err)
	}
	if err = session.Query(`CREATE KEYSPACE IF NOT EXISTS migrate WITH REPLICATION = {'class': 'SimpleStrategy', 'replication_factor': 1};`).Exec(); err != nil {
		t.Fatal(err)
	}
	cluster.Keyspace = "migrate"
	session, err = cluster.CreateSession()
	if err != nil {
		t.Fatal(err)
	}
	driverURL = "cassandra://" + host + ":" + port + "/migrate"

	d := &Driver{}
	if err := d.Initialize(driverURL); err != nil {
		t.Fatal(err)
	}

	files := []file.File{
		{
			Path:      "/foobar",
			FileName:  "001_foobar.up.sql",
			Version:   1,
			Name:      "foobar",
			Direction: direction.Up,
			Content: []byte(`
                CREATE TABLE yolo (
                    id varint primary key,
                    msg text
                );

            `),
		},
		{
			Path:      "/foobar",
			FileName:  "002_foobar.down.sql",
			Version:   1,
			Name:      "foobar",
			Direction: direction.Down,
			Content: []byte(`
                DROP TABLE yolo;
            `),
		},
		{
			Path:      "/foobar",
			FileName:  "002_foobar.up.sql",
			Version:   2,
			Name:      "foobar",
			Direction: direction.Up,
			Content: []byte(`
                CREATE TABLE error (
                    id THIS WILL CAUSE AN ERROR
                )
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

func TestInitializeReturnsErrorsForBadUrls(t *testing.T) {
	var session *gocql.Session

	host := os.Getenv("CASSANDRA_PORT_9042_TCP_ADDR")
	port := os.Getenv("CASSANDRA_PORT_9042_TCP_PORT")

	cluster := gocql.NewCluster(host)
	cluster.Consistency = gocql.All
	cluster.Timeout = 1 * time.Minute

	session, err := cluster.CreateSession()
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if err := session.Query(`CREATE KEYSPACE IF NOT EXISTS migrate WITH REPLICATION = {'class': 'SimpleStrategy', 'replication_factor': 1};`).Exec(); err != nil {
		t.Fatal(err)
	}

	d := &Driver{}
	invalidURL := "sdf://asdf://as?df?a"
	if err := d.Initialize(invalidURL); err == nil {
		t.Errorf("expected an error to be returned if url could not be parsed")
	}

	noKeyspace := "cassandra://" + host + ":" + port
	if err := d.Initialize(noKeyspace); err == nil {
		t.Errorf("expected an error to be returned if no keyspace provided")
	}
}
