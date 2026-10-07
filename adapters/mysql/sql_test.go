package mysql

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBindValueJSONAsString(t *testing.T) {
	// Objects and arrays bind as strings so interpolateParams does not send
	// them as _binary literals, which JSON columns reject (error 3144).
	got, err := bindValue(map[string]interface{}{"kind": "apparel"})
	require.NoError(t, err)
	require.Equal(t, `{"kind":"apparel"}`, got)

	got, err = bindValue([]interface{}{1, "a"})
	require.NoError(t, err)
	require.Equal(t, `[1,"a"]`, got)

	got, err = bindValue([2]int{1, 2})
	require.NoError(t, err)
	require.Equal(t, `[1,2]`, got)

	got, err = bindValue([]byte("raw"))
	require.NoError(t, err)
	require.Equal(t, []byte("raw"), got)

	got, err = bindValue(nil)
	require.NoError(t, err)
	require.Nil(t, got)

	_, err = bindValue(map[string]interface{}{"bad": make(chan int)})
	require.Error(t, err)
}
