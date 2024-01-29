package dbtime

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type testTime struct {
	Date Time `json:"date"`
}

func TestUnmarshalJSON(t *testing.T) {
	t.Parallel()

	var p testTime
	j := []byte(`{"date":"2017-05-10T11:00:00.000001"}`)

	err := json.Unmarshal(j, &p)
	require.NoError(t, err)

	require.Equal(t, 2017, p.Date.Year())
	require.Equal(t, time.May, p.Date.Month())
	require.Equal(t, 10, p.Date.Day())
	require.Equal(t, 11, p.Date.Hour())

	j = []byte(`{"date":"null"}`)
	err = json.Unmarshal(j, &p)
	require.NoError(t, err)
	require.True(t, p.Date.IsZero())

}

func TestMarshalJSON(t *testing.T) {
	t.Parallel()

	var p testTime
	var tAux time.Time
	var err error
	var j []byte

	layout := "2006-01-02T15:04:05.999999"
	str := "2017-05-10T11:00:00.000001"

	tAux, err = time.Parse(layout, str)
	require.NoError(t, err)

	p.Date = Time{tAux}

	j, err = json.Marshal(p)
	require.NoError(t, err)
	require.Equal(t, []byte(`{"date":"2017-05-10T11:00:00.000001"}`), j)
}
