package logsafe

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestError_nil(t *testing.T) {
	t.Parallel()

	require.Nil(t, Error(nil))
}

func TestError_passwordKV(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "plain",
			input: `connect failed: user=u password=secret dbname=db host=localhost`,
			want:  `connect failed: user=u password=*** dbname=db host=localhost`,
		},
		{
			name:  "single-quoted",
			input: `connect failed: password='s3cret!' dbname=db`,
			want:  `connect failed: password=*** dbname=db`,
		},
		{
			name:  "double-quoted",
			input: `connect failed: password="s3cret!" dbname=db`,
			want:  `connect failed: password=*** dbname=db`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := errors.New(tt.input)
			redacted := Error(err)
			require.Equal(t, tt.want, redacted.Error())
		})
	}
}

func TestError_postgresURL(t *testing.T) {
	t.Parallel()

	err := errors.New(`parse "postgresql://admin:supersecret@db.example.com:5432/app": invalid port`)
	redacted := Error(err)
	require.Equal(t, `parse "postgres://admin:***@db.example.com:5432/app": invalid port`, redacted.Error())
}

func TestError_unchanged(t *testing.T) {
	t.Parallel()

	err := errors.New("connection refused")
	require.Same(t, err, Error(err))
}
