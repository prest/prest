package mysql

import (
	"testing"

	"github.com/prest/prest/v2/adapters"
	"github.com/stretchr/testify/require"
)

var _ adapters.Dialect = (*Adapter)(nil)

func TestQuoteIdentifier(t *testing.T) {
	a := &Adapter{}
	cases := []struct {
		in, want string
		wantErr  bool
	}{
		{in: "items", want: "`items`"},
		{in: "shop.items", want: "`shop`.`items`"},
		{in: "my-db", want: "`my-db`"},
		{in: "a;b", wantErr: true},
		{in: "", wantErr: true},
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
	require.Equal(t, "?", (&Adapter{}).Placeholder(3))
}
