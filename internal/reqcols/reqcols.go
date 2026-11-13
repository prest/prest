// Package reqcols lists the column names a read request references outside
// _select, so they can be checked against the field ACL (GHSA-48r9-g6f9-x3gm).
// It reads the same positions as the SQL builders (WhereByRequest, _or,
// CountByRequest, groupByParts, OrderByRequest); drift tests in each adapter
// keep the two aligned.
package reqcols

import (
	"net/url"
	"strconv"
	"strings"
	"unicode"

	"github.com/prest/prest/v2/internal/ident"
)

// Extract returns every column named by WHERE keys, _or, _count, _groupby
// (group columns, HAVING aggregate field, allowlisted function arguments) and
// _order, as written: bare ("ssn") or qualified ("employee_secret.ssn").
// Duplicates are removed. An entry the builders would reject as an invalid
// identifier is skipped: the builder still rejects the request with its own
// 400, so Extract never changes which requests fail or how.
func Extract(q url.Values) []string {
	var cols []string
	seen := map[string]bool{}
	add := func(col string) {
		col = strings.TrimSpace(col)
		if col == "" || seen[col] || !ident.IsValid(col) {
			return
		}
		seen[col] = true
		cols = append(cols, col)
	}

	for key := range q {
		if !strings.HasPrefix(key, "_") {
			add(whereColumn(key))
		}
	}
	for _, v := range q["_or"] {
		for _, part := range SplitTopLevelOrGroup(strings.TrimSpace(v)) {
			if pos := strings.Index(part, "="); pos > 0 {
				add(whereColumn(part[:pos]))
			}
		}
	}
	for _, field := range strings.Split(q.Get("_count"), ",") {
		if strings.TrimSpace(field) != "*" {
			add(field)
		}
	}
	for _, col := range groupByColumns(q.Get("_groupby")) {
		add(col)
	}
	for _, field := range strings.Split(q.Get("_order"), ",") {
		add(strings.TrimPrefix(field, "-"))
	}
	return cols
}

// whereColumn is the column of a filter key: "col", "t.col", "col:tsquery",
// "col:vecdist" or "col->>attr:jsonb", as whereKeyAndValue reads it.
func whereColumn(key string) string {
	base, suffix, _ := strings.Cut(key, ":")
	if suffix == "jsonb" {
		base, _, _ = strings.Cut(base, "->>")
	}
	return base
}

// groupByColumns mirrors groupByParts: the group columns, the HAVING
// aggregate's field, and the column arguments of allowlisted functions.
func groupByColumns(groupQuery string) []string {
	if groupQuery == "" {
		return nil
	}
	if strings.Contains(groupQuery, "->>having") {
		before, _, _ := strings.Cut(groupQuery, "->>having")
		cols := strings.Split(before, ",")
		if params := strings.Split(groupQuery, ":"); len(params) == 5 && params[2] != "*" {
			cols = append(cols, params[2])
		}
		return cols
	}
	var cols []string
	for _, field := range strings.Split(groupQuery, ",") {
		field = strings.TrimSpace(field)
		if strings.Contains(field, "(") && strings.Contains(field, ")") {
			cols = append(cols, funcArgColumns(field)...)
			continue
		}
		cols = append(cols, field)
	}
	return cols
}

// funcArgColumns returns the column arguments of a function expression such
// as lower(ssn), date_trunc('day', created_at) or extract(year FROM ts):
// quoted literals and numbers are dropped, and for extract only the part
// after FROM names a column.
func funcArgColumns(expr string) []string {
	open, closing := strings.Index(expr, "("), strings.LastIndex(expr, ")")
	if open < 0 || closing <= open {
		return nil
	}
	args := stripQuoted(expr[open+1 : closing])
	tokens := strings.FieldsFunc(args, func(r rune) bool { return r == ',' || unicode.IsSpace(r) })
	if strings.EqualFold(strings.TrimSpace(expr[:open]), "extract") {
		for i, tok := range tokens {
			if strings.EqualFold(tok, "from") {
				tokens = tokens[i+1:]
				break
			}
		}
	}
	var cols []string
	for _, tok := range tokens {
		if _, err := strconv.ParseFloat(tok, 64); err == nil {
			continue
		}
		cols = append(cols, tok)
	}
	return cols
}

// stripQuoted blanks out single-quoted literals so their words are not read
// as columns.
func stripQuoted(s string) string {
	var b strings.Builder
	in := false
	for _, r := range s {
		if r == '\'' {
			in = !in
			b.WriteRune(' ')
			continue
		}
		if in {
			b.WriteRune(' ')
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// SplitTopLevelOrGroup splits an _or value on top-level " OR " and "||"
// separators, leaving separators inside single- or double-quoted values alone.
// It is the same algorithm as the adapters' splitTopLevelOrGroup.
func SplitTopLevelOrGroup(v string) []string {
	parts := make([]string, 0)
	var current strings.Builder
	inSingle, inDouble := false, false

	flush := func() {
		if part := strings.TrimSpace(current.String()); part != "" {
			parts = append(parts, part)
		}
		current.Reset()
	}

	for i := 0; i < len(v); {
		ch := v[i]
		switch {
		case inSingle || inDouble:
			quote := byte('\'')
			if inDouble {
				quote = '"'
			}
			current.WriteByte(ch)
			if ch == quote {
				if i+1 < len(v) && v[i+1] == quote {
					current.WriteByte(v[i+1])
					i += 2
					continue
				}
				inSingle, inDouble = false, false
			}
			i++
		case i+1 < len(v) && ch == '|' && v[i+1] == '|':
			flush()
			i += 2
		case ch == '\'':
			inSingle = true
			current.WriteByte(ch)
			i++
		case ch == '"':
			inDouble = true
			current.WriteByte(ch)
			i++
		case isOrSeparator(v, i):
			flush()
			i += 2
			for i < len(v) && unicode.IsSpace(rune(v[i])) {
				i++
			}
		default:
			current.WriteByte(ch)
			i++
		}
	}
	flush()
	return parts
}

func isOrSeparator(v string, i int) bool {
	return i+2 < len(v) &&
		strings.EqualFold(v[i:i+2], "OR") &&
		i > 0 && unicode.IsSpace(rune(v[i-1])) &&
		unicode.IsSpace(rune(v[i+2]))
}
