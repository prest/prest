package postgres

import (
	"testing"

	"github.com/prest/prest/v2/adapters"
	"github.com/stretchr/testify/require"
)

var _ adapters.Dialect = (*postgres)(nil)

func TestQuoteIdentifier(t *testing.T) {
	a := &postgres{}
	cases := []struct {
		in, want string
		wantErr  bool
	}{
		{in: "items", want: `"items"`},
		{in: "public.prest_users", want: `"public"."prest_users"`},
		{in: "my-db", want: `"my-db"`},
		{in: "a;b", wantErr: true},
		{in: `a"b`, wantErr: true},
		{in: "", wantErr: true},
		{in: "a..b", wantErr: true},
	}
	for _, tc := range cases {
		got, err := a.QuoteIdentifier(tc.in)
		if tc.wantErr {
			require.Error(t, err, tc.in)
			continue
		}
		require.NoError(t, err, tc.in)
		require.Equal(t, tc.want, got)
	}
}

func TestPlaceholder(t *testing.T) {
	require.Equal(t, "$3", (&postgres{}).Placeholder(3))
}
