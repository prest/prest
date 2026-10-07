package mysql

import (
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"
)

func scanJSONArray(rows *sql.Rows) ([]byte, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	types, err := rows.ColumnTypes()
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0)
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := make(map[string]any, len(cols))
		for i, name := range cols {
			typeName := ""
			if i < len(types) && types[i] != nil {
				typeName = types[i].DatabaseTypeName()
			}
			row[name] = normalizeScanned(vals[i], typeName)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return json.Marshal(out)
}

func scanJSONObject(rows *sql.Rows) ([]byte, error) {
	raw, err := scanJSONArray(rows)
	if err != nil {
		return nil, err
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err != nil {
		return nil, err
	}
	if len(arr) == 0 {
		return []byte("null"), nil
	}
	return arr[0], nil
}

func normalizeScanned(v any, typeName string) any {
	if v == nil {
		return nil
	}
	kind := strings.ToUpper(typeName)
	switch t := v.(type) {
	case time.Time:
		return formatTime(t, kind)
	case int64, int32, int, int16, int8, uint64, uint32, uint16, uint8, float64, float32:
		return t
	case json.Number:
		return t
	case []byte:
		return normalizeBytes(t, kind)
	case string:
		if isDecimal(kind) {
			return t
		}
		if isJSONType(kind) && json.Valid([]byte(t)) {
			return json.RawMessage(t)
		}
		return t
	default:
		return t
	}
}

func normalizeBytes(b []byte, kind string) any {
	if b == nil {
		return nil
	}
	if isDecimal(kind) {
		return string(b)
	}
	if isJSONType(kind) {
		if json.Valid(b) {
			return json.RawMessage(append([]byte(nil), b...))
		}
		return string(b)
	}
	if isBinary(kind) {
		// Postgres bytea JSON form; raw bytes are not valid UTF-8 text.
		return `\x` + hex.EncodeToString(b)
	}
	if isInteger(kind) && isIntegerText(b) {
		// Binary protocol (prepare mode) returns BIGINT UNSIGNED as text.
		return json.Number(string(b))
	}
	// Column type is unavailable (sqlmock). Objects and arrays are embedded;
	// numbers and other text, including DECIMAL, stay strings.
	trim := strings.TrimSpace(string(b))
	if trim != "" && (trim[0] == '{' || trim[0] == '[') && json.Valid(b) {
		return json.RawMessage(append([]byte(nil), b...))
	}
	return string(b)
}

// formatTime renders values MySQL accepts back on write: no 'Z' suffix,
// DATE without a time part. TIMESTAMP is stored in UTC.
func formatTime(t time.Time, kind string) string {
	switch kind {
	case "DATE":
		return t.Format("2006-01-02")
	case "DATETIME":
		return t.Format("2006-01-02T15:04:05.999999")
	case "TIMESTAMP":
		return t.UTC().Format("2006-01-02T15:04:05.999999-07:00")
	default:
		return t.Format(time.RFC3339Nano)
	}
}

func isBinary(kind string) bool {
	switch kind {
	case "BLOB", "BINARY", "VARBINARY", "BIT", "GEOMETRY":
		return true
	default:
		return false
	}
}

func isInteger(kind string) bool {
	switch strings.TrimPrefix(kind, "UNSIGNED ") {
	case "TINYINT", "SMALLINT", "MEDIUMINT", "INT", "BIGINT":
		return true
	default:
		return false
	}
}

func isIntegerText(b []byte) bool {
	if len(b) > 0 && b[0] == '-' {
		b = b[1:]
	}
	if len(b) == 0 {
		return false
	}
	for _, c := range b {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func isDecimal(kind string) bool {
	switch kind {
	case "DECIMAL", "NUMERIC", "NEWDECIMAL":
		return true
	default:
		return false
	}
}

func isJSONType(kind string) bool {
	return kind == "JSON"
}
