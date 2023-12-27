package connection

import (
	"testing"

	"github.com/stretchr/testify/require"

	config "github.com/prest/prest/config"
)

func TestGet(t *testing.T) {
	t.Log("Open connection")

	p := NewPool(config.New())

	db, err := p.Get()
	require.NoError(t, err)
	require.NotNil(t, db)

	t.Log("Ping Pong")
	err = db.Ping()
	require.NoError(t, err)
}

func TestMustGet(t *testing.T) {
	t.Log("Open connection")

	p := NewPool(config.New())

	db := p.MustGet()
	require.NotNil(t, db)

	t.Log("Ping Pong")
	err := db.Ping()
	require.NoError(t, err)
}
