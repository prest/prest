package middlewares

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

// The rows and columns renderers (_renderer=rows, _renderer=columns) send the
// column names once instead of repeating them in every row object, which is
// the shape chart and data-grid libraries consume directly:
//
//	[{"name":"a","age":1},{"name":"b","age":2}]
//
// renders as
//
//	rows:    {"metadata":["name","age"],"rows":[["a",1],["b",2]]}
//	columns: {"metadata":["name","age"],"columns":[["a","b"],[1,2]]}
const (
	rowsRenderer    = "rows"
	columnsRenderer = "columns"
)

var errNotTabular = errors.New("response is not a row object or an array of row objects")

// columnKey identifies a column by its name and by how many times that name
// has already appeared in the same row, so a row with a repeated key (two
// aggregates both named "sum") keeps each value in a column of its own.
type columnKey struct {
	name       string
	occurrence int
}

// table is a response body split into column names and per-row values. Each
// value keeps its exact bytes (numbers, nested JSON); a nil value stands for a
// column the row does not have and is encoded as null.
type table struct {
	columns []string
	rows    [][]json.RawMessage
	index   map[columnKey]int
}

// renderTable reshapes body for the rows or columns renderer. A body that is
// not a row object or an array of row objects is returned unchanged.
func renderTable(body []byte, format string) []byte {
	t, err := parseTable(body)
	if err != nil {
		return body
	}
	out, err := t.encode(format == columnsRenderer)
	if err != nil {
		return body
	}
	return out
}

// parseTable reads a row object or an array of row objects. Columns are listed
// in the order they first appear in body, which is the column order of the
// query with json_agg and jsonb's own key order with jsonb_agg; a column
// missing from some rows is null in those rows.
func parseTable(body []byte) (*table, error) {
	t := &table{
		columns: []string{},
		rows:    [][]json.RawMessage{},
		index:   map[columnKey]int{},
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch tok {
	case json.Delim('{'):
		if err = t.readRow(dec); err != nil {
			return nil, err
		}
	case json.Delim('['):
		for dec.More() {
			if tok, err = dec.Token(); err != nil {
				return nil, err
			}
			if tok != json.Delim('{') {
				return nil, errNotTabular
			}
			if err = t.readRow(dec); err != nil {
				return nil, err
			}
		}
		if _, err = dec.Token(); err != nil { // closing ]
			return nil, err
		}
	default:
		return nil, errNotTabular
	}
	if _, err = dec.Token(); err != io.EOF {
		return nil, errNotTabular
	}
	return t, nil
}

// readRow appends the object whose opening brace dec has just read as a row,
// adding any column not seen before, and consumes its closing brace.
func (t *table) readRow(dec *json.Decoder) error {
	row := make([]json.RawMessage, len(t.columns))
	occurrences := map[string]int{}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		name, ok := tok.(string)
		if !ok {
			return errNotTabular
		}
		var value json.RawMessage
		if err = dec.Decode(&value); err != nil {
			return err
		}
		key := columnKey{name: name, occurrence: occurrences[name]}
		occurrences[name]++
		i, ok := t.index[key]
		if !ok {
			i = len(t.columns)
			t.index[key] = i
			t.columns = append(t.columns, name)
		}
		if i >= len(row) {
			row = append(row, make([]json.RawMessage, i+1-len(row))...)
		}
		row[i] = value
	}
	if _, err := dec.Token(); err != nil { // closing }
		return err
	}
	t.rows = append(t.rows, row)
	return nil
}

// encode writes the table as {"metadata": [...], "rows": [...]}, or with
// byColumn as {"metadata": [...], "columns": [...]} holding one array of
// values per column.
func (t *table) encode(byColumn bool) ([]byte, error) {
	width := len(t.columns)
	for i, row := range t.rows {
		if len(row) < width {
			t.rows[i] = append(row, make([]json.RawMessage, width-len(row))...)
		}
	}

	var payload any
	if byColumn {
		columns := make([][]json.RawMessage, width)
		for c := range columns {
			columns[c] = make([]json.RawMessage, len(t.rows))
			for r, row := range t.rows {
				columns[c][r] = row[c]
			}
		}
		payload = struct {
			Metadata []string            `json:"metadata"`
			Columns  [][]json.RawMessage `json:"columns"`
		}{t.columns, columns}
	} else {
		payload = struct {
			Metadata []string            `json:"metadata"`
			Rows     [][]json.RawMessage `json:"rows"`
		}{t.columns, t.rows}
	}

	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	// Keep string values byte-for-byte as the database returned them, as the
	// default JSON renderer does, instead of escaping <, > and &.
	enc.SetEscapeHTML(false)
	if err := enc.Encode(payload); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(out.Bytes(), []byte("\n")), nil
}
