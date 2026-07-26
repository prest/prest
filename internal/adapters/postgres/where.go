package postgres

import (
	"fmt"
	"net/http"
	"strings"
	"unicode"

	"github.com/prest/prest/v2/internal/ident"
	"github.com/prest/prest/v2/internal/postgres/formatters"

	"github.com/pkg/errors"
)

// WhereByRequest create interface for queries + where
func (adapter *postgres) WhereByRequest(r *http.Request, initialPlaceholderID int) (whereSyntax string, values []interface{}, err error) {
	whereKey := []string{}
	whereValues := []interface{}{}
	orClauses := []string{}

	pid := initialPlaceholderID
	for key, val := range r.URL.Query() {
		if !strings.HasPrefix(key, "_") {
			// keep the original key untouched to avoid invalid identifier errors
			rawKey := key
			for _, v := range val {
				var k string
				var vls []interface{}
				k, vls, err = adapter.whereKeyAndValue(rawKey, v, &pid)
				if err != nil {
					return
				}
				if k != "" {
					whereKey = append(whereKey, k)
					whereValues = append(whereValues, vls...)
				}
			}
		} else if key == "_or" {
			for _, v := range val {
				v = strings.TrimSpace(v)
				if v == "" {
					continue
				}
				parts := splitTopLevelOrGroup(v)
				for _, part := range parts {
					part = strings.TrimSpace(part)
					if part == "" {
						continue
					}
					// part is expected to be field=condition
					// we look for the first "="
					pos := strings.Index(part, "=")
					if pos <= 0 {
						continue
					}
					field := part[:pos]
					condition := part[pos+1:]

					var k string
					var vls []interface{}
					k, vls, err = adapter.whereKeyAndValue(field, condition, &pid)
					if err != nil {
						return
					}
					if k != "" {
						orClauses = append(orClauses, k)
						whereValues = append(whereValues, vls...)
					}
				}
			}
		}
	}

	if len(orClauses) > 0 {
		whereKey = append(whereKey, fmt.Sprintf("(%s)", strings.Join(orClauses, " OR ")))
	}

	for i := 0; i < len(whereKey); i++ {
		if whereSyntax == "" {
			whereSyntax += whereKey[i]
		} else {
			whereSyntax += " AND " + whereKey[i]
		}
	}

	values = append(values, whereValues...)
	return
}

// splitTopLevelOrGroup splits a string into a slice of strings, each representing a top-level OR group.
func splitTopLevelOrGroup(v string) []string {
	parts := make([]string, 0)
	var current strings.Builder

	inSingleQuote := false
	inDoubleQuote := false

	flush := func() {
		part := strings.TrimSpace(current.String())
		if part != "" {
			parts = append(parts, part)
		}
		current.Reset()
	}

	for i := 0; i < len(v); {
		ch := v[i]

		if inSingleQuote {
			current.WriteByte(ch)
			if ch == '\'' {
				if i+1 < len(v) && v[i+1] == '\'' {
					current.WriteByte(v[i+1])
					i += 2
					continue
				}
				inSingleQuote = false
			}
			i++
			continue
		}

		if inDoubleQuote {
			current.WriteByte(ch)
			if ch == '"' {
				if i+1 < len(v) && v[i+1] == '"' {
					current.WriteByte(v[i+1])
					i += 2
					continue
				}
				inDoubleQuote = false
			}
			i++
			continue
		}

		if isTopLevelLegacySeparator(v, i) {
			flush()
			i += 2
			continue
		}

		if ch == '\'' {
			inSingleQuote = true
			current.WriteByte(ch)
			i++
			continue
		}

		if ch == '"' {
			inDoubleQuote = true
			current.WriteByte(ch)
			i++
			continue
		}

		if isTopLevelOrSeparator(v, i) {
			flush()
			i += 2
			for i < len(v) && isWhitespace(v[i]) {
				i++
			}
			continue
		}

		current.WriteByte(ch)
		i++
	}

	flush()
	return parts
}

func isTopLevelOrSeparator(v string, i int) bool {
	if i+1 >= len(v) {
		return false
	}

	if !strings.EqualFold(v[i:i+2], "OR") {
		return false
	}

	if i > 0 && !isWhitespace(v[i-1]) {
		return false
	}

	if i+2 >= len(v) || !isWhitespace(v[i+2]) {
		return false
	}

	return true
}

func isTopLevelLegacySeparator(v string, i int) bool {
	return i+1 < len(v) && v[i] == '|' && v[i+1] == '|'
}

func isWhitespace(b byte) bool {
	return unicode.IsSpace(rune(b))
}

// whereKeyAndValue splits a string into a key and a value, and returns the key and the values.
func (adapter *postgres) whereKeyAndValue(rawKey, v string, pid *int) (key string, values []interface{}, err error) {
	var value, op string
	if v == "" {
		err = ErrInvalidOperator
		return
	}

	op = removeOperatorRegex.FindString(v)
	op = strings.Replace(op, ".", "", -1)
	if op == "" {
		op = "$eq"
	}
	value = removeOperatorRegex.ReplaceAllString(v, "")
	op, err = GetQueryOperator(op)
	if err != nil {
		return
	}

	keyInfo := strings.Split(rawKey, ":")

	if len(keyInfo) > 1 {
		switch keyInfo[1] {
		case "jsonb":
			jsonField := strings.Split(keyInfo[0], "->>")
			if len(jsonField) != 2 || !ident.IsValid(jsonField[0]) || !ident.IsValid(jsonField[1]) {
				err = errors.Wrapf(ErrInvalidIdentifier, "%v", jsonField)
				return
			}
			fields := strings.Split(jsonField[0], ".")
			jsonField[0] = fmt.Sprintf(`"%s"`, strings.Join(fields, `"."`))
			// escape single quotes in json attribute key
			safeAttr := strings.ReplaceAll(jsonField[1], "'", "''")
			jsonLeft := fmt.Sprintf(`%s->>'%s'`, jsonField[0], safeAttr)
			switch op {
			case "IN", "NOT IN":
				v := strings.Split(value, ",")
				keyParams := make([]string, len(v))
				for i := 0; i < len(v); i++ {
					values = append(values, v[i])
					keyParams[i] = fmt.Sprintf(`$%d`, *pid+i)
				}
				*pid += len(v)
				key = fmt.Sprintf(`%s %s (%s)`, jsonLeft, op, strings.Join(keyParams, ","))
			case "ANY", "SOME", "ALL":
				key = fmt.Sprintf(`%s = %s ($%d)`, jsonLeft, op, *pid)
				values = append(values, formatters.FormatArray(strings.Split(value, ",")))
				*pid++
			case "IS NULL", "IS NOT NULL", "IS TRUE", "IS NOT TRUE", "IS FALSE", "IS NOT FALSE":
				key = fmt.Sprintf(`%s %s`, jsonLeft, op)
			default: // "=", "!=", ">", ">=", "<", "<="
				key = fmt.Sprintf(`%s %s $%d`, jsonLeft, op, *pid)
				values = append(values, value)
				*pid++
			}
		case "tsquery":
			tsQueryField := strings.Split(keyInfo[0], "$")
			if !ident.IsValid(tsQueryField[0]) {
				err = errors.Wrapf(ErrInvalidIdentifier, "%s", tsQueryField[0])
				return
			}
			safeVal := strings.ReplaceAll(value, "'", "''")
			tsQuery := fmt.Sprintf(`%s @@ to_tsquery('%s')`, tsQueryField[0], safeVal)
			if len(tsQueryField) == 2 {
				if !ident.IsValid(tsQueryField[1]) {
					err = errors.Wrapf(ErrInvalidIdentifier, "%s", tsQueryField[1])
					return
				}
				safeCfg := strings.ReplaceAll(tsQueryField[1], "'", "''")
				tsQuery = fmt.Sprintf(`%s @@ to_tsquery('%s', '%s')`, tsQueryField[0], safeCfg, safeVal)
			}
			key = tsQuery
		case "vecdist":
			// pgvector distance-threshold filter:
			// embedding:vecdist=<metric>:<comparison>:<vector>:<threshold>
			key, values, err = buildVectorFilter(keyInfo[0], value, pid)
			return
		default:
			if !ident.IsValid(keyInfo[0]) {
				err = errors.Wrapf(ErrInvalidIdentifier, "%s", keyInfo[0])
				return
			}
			err = errors.Errorf("unknown type suffix: %s", keyInfo[1])
			return
		}
		return
	}

	if !ident.IsValid(rawKey) {
		err = errors.Wrapf(ErrInvalidIdentifier, "%s", rawKey)
		return
	}

	// always quote the field for SQL usage without mutating the original key
	fields := strings.Split(rawKey, ".")
	quotedKey := fmt.Sprintf(`"%s"`, strings.Join(fields, `"."`))

	switch op {
	case "IN", "NOT IN":
		v := strings.Split(value, ",")
		keyParams := make([]string, len(v))
		for i := 0; i < len(v); i++ {
			values = append(values, v[i])
			keyParams[i] = fmt.Sprintf(`$%d`, *pid+i)
		}
		*pid += len(v)
		key = fmt.Sprintf(`%s %s (%s)`, quotedKey, op, strings.Join(keyParams, ","))
	case "ANY", "SOME", "ALL":
		key = fmt.Sprintf(`%s = %s ($%d)`, quotedKey, op, *pid)
		values = append(values, formatters.FormatArray(strings.Split(value, ",")))
		*pid++
	case "IS NULL", "IS NOT NULL", "IS TRUE", "IS NOT TRUE", "IS FALSE", "IS NOT FALSE":
		key = fmt.Sprintf(`%s %s`, quotedKey, op)
	default: // "=", "!=", ">", ">=", "<", "<="
		key = fmt.Sprintf(`%s %s $%d`, quotedKey, op, *pid)
		values = append(values, value)
		*pid++
	}
	return
}
