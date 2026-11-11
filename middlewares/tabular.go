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

// cell is one value of a row: the index of its column and the value's bytes
// exactly as they appear in the response body (numbers, nested JSON,
// whitespace).
type cell struct {
	column int
	value  json.RawMessage
}

// table is a response body split into column names and rows. A row holds only
// the cells it has; a column it does not have is written as null.
type table struct {
	columns []string
	rows    [][]cell
	index   map[columnKey]int
}

// renderTable reshapes body for the rows renderer, or with byColumn for the
// columns renderer. A body that is not a row object or an array of row
// objects, or whose rows share too few columns to fill a table, is returned
// unchanged.
func renderTable(body []byte, byColumn bool) []byte {
	t, err := parseTable(body)
	if err != nil {
		return body
	}
	out, err := t.encode(byColumn)
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
		rows:    [][]cell{},
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
	// Both layouts write a value, null if missing, for every column of every
	// row. Rows that each bring keys of their own would make that grow with
	// the square of the body, so a table with more cells than the body has
	// bytes is left as it is. Rows that share their columns, as query results
	// do, always fit: every value takes several bytes of the body.
	if len(t.columns)*len(t.rows) > len(body) {
		return nil, errNotTabular
	}
	return t, nil
}

// readRow appends the object whose opening brace dec has just read as a row,
// adding any column not seen before, and consumes its closing brace.
func (t *table) readRow(dec *json.Decoder) error {
	row := make([]cell, 0, len(t.columns))
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
		row = append(row, cell{column: i, value: value})
	}
	if _, err := dec.Token(); err != nil { // closing }
		return err
	}
	t.rows = append(t.rows, row)
	return nil
}

// encode writes the table as {"metadata": [...], "rows": [...]}, or with
// byColumn as {"metadata": [...], "columns": [...]} holding one array of
// values per column. Values are copied from the body, not re-encoded.
func (t *table) encode(byColumn bool) ([]byte, error) {
	var names bytes.Buffer
	enc := json.NewEncoder(&names)
	// Keep <, > and & in column names as they are, as the values and the
	// default JSON renderer do.
	enc.SetEscapeHTML(false)
	if err := enc.Encode(t.columns); err != nil {
		return nil, err
	}

	var out bytes.Buffer
	out.WriteString(`{"metadata":`)
	out.Write(bytes.TrimSuffix(names.Bytes(), []byte("\n")))
	if byColumn {
		out.WriteString(`,"columns":`)
		t.writeColumns(&out)
	} else {
		out.WriteString(`,"rows":`)
		t.writeRows(&out)
	}
	out.WriteByte('}')
	return out.Bytes(), nil
}

// writeRows writes one array per row with a value for every column.
func (t *table) writeRows(out *bytes.Buffer) {
	values := make([]json.RawMessage, len(t.columns))
	out.WriteByte('[')
	for r, row := range t.rows {
		clear(values)
		for _, c := range row {
			values[c.column] = c.value
		}
		if r > 0 {
			out.WriteByte(',')
		}
		out.WriteByte('[')
		for i, value := range values {
			if i > 0 {
				out.WriteByte(',')
			}
			writeValue(out, value)
		}
		out.WriteByte(']')
	}
	out.WriteByte(']')
}

// writeColumns writes one array per column with a value for every row.
func (t *table) writeColumns(out *bytes.Buffer) {
	// The cells of each column in row order, with the row index in place of
	// the column index.
	columns := make([][]cell, len(t.columns))
	for r, row := range t.rows {
		for _, c := range row {
			columns[c.column] = append(columns[c.column], cell{column: r, value: c.value})
		}
	}
	out.WriteByte('[')
	for i, cells := range columns {
		if i > 0 {
			out.WriteByte(',')
		}
		out.WriteByte('[')
		for r := range t.rows {
			if r > 0 {
				out.WriteByte(',')
			}
			if len(cells) > 0 && cells[0].column == r {
				writeValue(out, cells[0].value)
				cells = cells[1:]
			} else {
				writeValue(out, nil)
			}
		}
		out.WriteByte(']')
	}
	out.WriteByte(']')
}

// writeValue writes value, or null for a column the row does not have.
func writeValue(out *bytes.Buffer, value json.RawMessage) {
	if value == nil {
		out.WriteString("null")
		return
	}
	out.Write(value)
}
