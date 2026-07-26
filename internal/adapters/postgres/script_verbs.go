package postgres

import "fmt"

var scriptVerbSuffixes = map[string]string{
	"GET":    ".read.sql",
	"POST":   ".write.sql",
	"PATCH":  ".update.sql",
	"PUT":    ".update.sql",
	"DELETE": ".delete.sql",
}

var scriptSuffixColumns = map[string]string{
	".read.sql":   "read_sql",
	".write.sql":  "write_sql",
	".update.sql": "update_sql",
	".delete.sql": "delete_sql",
}

func scriptVerbColumn(verb string) (string, error) {
	suffix, ok := scriptVerbSuffixes[verb]
	if !ok {
		return "", fmt.Errorf("invalid http method %s", verb)
	}
	col, ok := scriptSuffixColumns[suffix]
	if !ok {
		return "", fmt.Errorf("invalid http method %s", verb)
	}
	return col, nil
}

func scriptSuffixForColumn(col string) string {
	for suffix, column := range scriptSuffixColumns {
		if column == col {
			return suffix
		}
	}
	return ""
}
