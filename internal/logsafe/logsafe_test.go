package logsafe

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestError_nil(t *testing.T) {
	require.Nil(t, Error(nil))
}

func TestError_passwordKV(t *testing.T) {
	err := errors.New(`connect failed: user=u password=secret dbname=db host=localhost`)
	redacted := Error(err)
	require.Equal(t, `connect failed: user=u password=*** dbname=db host=localhost`, redacted.Error())
}

func TestError_postgresURL(t *testing.T) {
	err := errors.New(`parse "postgresql://admin:supersecret@db.example.com:5432/app": invalid port`)
	redacted := Error(err)
	require.Equal(t, `parse "postgres://admin:***@db.example.com:5432/app": invalid port`, redacted.Error())
}

func TestError_unchanged(t *testing.T) {
	err := errors.New("connection refused")
	require.Same(t, err, Error(err))
}
