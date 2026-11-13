package middlewares

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
)

// The csv renderer (_renderer=csv) returns a row object or an array of row
// objects as CSV: a header record with the column names, then one record per
// row with its values in the same order, for spreadsheets and tools that load
// tabular files:
//
//	[{"name":"a","age":1},{"name":"b, c","age":null}]
//
// renders as
//
//	name,age
//	a,1
//	"b, c",
//
// Fields are quoted as RFC 4180 describes. Records end in \n, as PostgreSQL's
// COPY ... CSV writes them: ending them in \r\n would also turn every \n
// inside a quoted value into \r\n and change the data. See csvField for how
// each JSON value is written.
const (
	csvRenderer = "csv"
	// csvContentType is the media type RFC 4180 registers for CSV; the
	// charset tells clients the fields are UTF-8, as the JSON body is.
	csvContentType = "text/csv; charset=utf-8"
)

// renderCSV converts body to CSV. ok is false when body is not a row object or
// an array of row objects, or its rows share too few columns to fill a table;
// the caller then returns body unchanged.
func renderCSV(body []byte) (out []byte, ok bool) {
	t, err := parseTable(body)
	if err != nil {
		return nil, false
	}
	// A result with no columns (an empty array, or rows with no keys) has
	// nothing to name or to hold a value, so it is an empty file rather than
	// blank lines.
	if len(t.columns) == 0 {
		return []byte{}, true
	}
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if err := writeCSVRecord(w, &buf, t.columns); err != nil {
		return nil, false
	}
	record := make([]string, len(t.columns))
	for _, row := range t.rows {
		clear(record)
		for _, c := range row {
			field, err := csvField(c.value)
			if err != nil {
				return nil, false
			}
			record[c.column] = field
		}
		if err := writeCSVRecord(w, &buf, record); err != nil {
			return nil, false
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, false
	}
	return buf.Bytes(), true
}

// writeCSVRecord writes record to w, whose output goes to buf. A record with a
// single empty field would be a blank line, which CSV readers skip, so that
// row (a one-column result whose value is null or "") is written as "" to
// keep it.
func writeCSVRecord(w *csv.Writer, buf *bytes.Buffer, record []string) error {
	if len(record) == 1 && record[0] == "" {
		w.Flush()
		if err := w.Error(); err != nil {
			return err
		}
		buf.WriteString("\"\"\n")
		return nil
	}
	return w.Write(record)
}

// csvField returns the text of one value: a string without its JSON quotes
// and escapes, null as an empty field, and a number, boolean, object or array
// as its exact bytes in the body, so large numbers, decimals and nested JSON
// keep their precision and can be parsed back. The CSV writer quotes the
// field when it holds a comma, a quote or a line break.
func csvField(value json.RawMessage) (string, error) {
	switch {
	case len(value) == 0:
		return "", nil
	case value[0] == '"':
		var s string
		if err := json.Unmarshal(value, &s); err != nil {
			return "", err
		}
		return s, nil
	case bytes.Equal(value, []byte("null")):
		return "", nil
	default:
		return string(value), nil
	}
}
