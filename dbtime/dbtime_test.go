package dbtime

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"
)

type testTime struct {
	Date Time `json:"date"`
}

func TestUnmarshalJSON(t *testing.T) {
	var p testTime
	j := []byte(`{"date":"2017-05-10T11:00:00.000001"}`)

	err := json.Unmarshal(j, &p)
	if err != nil {
		t.Fatal(err.Error())
	}

	if p.Date.Time.Unix() != 1494414000 || p.Date.UnixNano() != 1494414000000001000 {
		t.Fatal(`Error, incorrect date/time value.`)
	}

	j = []byte(`{"date":"null"}`)
	err = json.Unmarshal(j, &p)
	if err != nil {
		t.Fatal(err.Error())
	}

	if !p.Date.IsZero() {
		t.Fatal(`Error, p.Date should be zero`)
	}
}

func TestMarshalJSON(t *testing.T) {
	var p testTime
	var tAux time.Time
	var err error
	var j []byte

	layout := "2006-01-02T15:04:05.999999"
	str := "2017-05-10T11:00:00.000001"

	tAux, err = time.Parse(layout, str)
	if err != nil {
		t.Fatal(err.Error())
	}
	p.Date = Time{tAux}

	j, err = json.Marshal(p)
	if err != nil {
		t.Fatal(err.Error())
	}

	if !bytes.Equal(j, []byte(`{"date":"2017-05-10T11:00:00.000001"}`)) {
		t.Fatal("Error, the date returned is not the same as the date entered.")
	}
}
