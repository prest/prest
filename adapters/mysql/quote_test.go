package mysql

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestQuoteLiteral checks that config names become one single-quoted MySQL
// literal, with quotes and backslashes escaped.
func TestQuoteLiteral(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"shop":           `'shop'`,
		"o'brien":        `'o''brien'`,
		`a\b`:            `'a\\b'`,
		`x') OR 1=1 -- `: `'x'') OR 1=1 -- '`,
		`\' OR '1'='1`:   `'\\'' OR ''1''=''1'`,
		"":               `''`,
	}
	for in, want := range cases {
		require.Equal(t, want, quoteLiteral(in), in)
	}
}
