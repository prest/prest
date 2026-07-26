package postgres

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"reflect"
	"sort"
	"strings"

	"github.com/prest/prest/v2/internal/ident"
	"github.com/prest/prest/v2/internal/postgres/formatters"

	"github.com/pkg/errors"
)

// ReturningByRequest create interface for queries + returning
func (adapter *postgres) ReturningByRequest(r *http.Request) (returningSyntax string, err error) {
	queries := r.URL.Query()["_returning"]
	if len(queries) > 0 {
		cols := make([]string, 0, len(queries))
		for _, q := range queries {
			if q == "*" {
				cols = append(cols, "*")
				continue
			}
			if !ident.IsValid(q) {
				err = errors.Wrap(ErrInvalidIdentifier, "Returning")
				return
			}
			quoted, _ := ident.Quote(q)
			cols = append(cols, quoted)
		}
		returningSyntax = strings.Join(cols, ", ")
	}
	return
}

// sliceToJSONList converts a slice to a JSON list.
func sliceToJSONList(ifaceSlice interface{}) (returnValue string, err error) {
	v := reflect.ValueOf(ifaceSlice)

	if v.Kind() == reflect.Invalid {
		return "[]", ErrEmptyOrInvalidSlice
	}

	value := make([]string, 0)

	for i := 0; i < v.Len(); i++ {
		val := v.Index(i).Interface()
		switch val.(type) {
		case int, float64:
			newVal := fmt.Sprint(val)
			value = append(value, newVal)
		default:
			newVal := fmt.Sprintf(`"%s"`, val)
			value = append(value, newVal)
		}
	}
	returnValue = fmt.Sprintf(`[%v]`, strings.Join(value, ", "))
	return
}

// SetByRequest create a set clause for SQL
func (adapter *postgres) SetByRequest(r *http.Request, initialPlaceholderID int) (setSyntax string, values []interface{}, err error) {
	body := make(map[string]interface{})
	if err = json.NewDecoder(r.Body).Decode(&body); err != nil {
		return
	}
	defer r.Body.Close()

	if len(body) == 0 {
		err = ErrBodyEmpty
		return
	}
	fields := make([]string, 0)
	for key, value := range body {
		if !ident.IsValid(key) {
			err = errors.Wrap(ErrInvalidIdentifier, "Set")
			return
		}
		keys := strings.Split(key, ".")
		key = fmt.Sprintf(`"%s"`, strings.Join(keys, `"."`))
		fields = append(fields, fmt.Sprintf(`%s=$%d`, key, initialPlaceholderID))

		switch reflect.ValueOf(value).Kind() {
		case reflect.Interface:
			values = append(values, formatters.FormatArray(value))
		case reflect.Map:
			jsonData, err := json.Marshal(value)
			if err != nil {
				slog.Error("error details", "err", err)
			}
			values = append(values, string(jsonData))
		case reflect.Slice:
			value, err = sliceToJSONList(value)
			if err != nil {
				slog.Error("error details", "err", err)
			}
			values = append(values, value)
		default:
			values = append(values, value)
		}
		initialPlaceholderID++
	}
	setSyntax = strings.Join(fields, ", ")
	return
}

func closer(body io.Closer) {
	err := body.Close()
	if err != nil {
		slog.Error("error details", "err", err)
	}
}

// ParseBatchInsertRequest create insert SQL to batch request
func (adapter *postgres) ParseBatchInsertRequest(r *http.Request) (colsName string, placeholders string, values []interface{}, err error) {
	recordSet := make([]map[string]interface{}, 0)
	if err = json.NewDecoder(r.Body).Decode(&recordSet); err != nil {
		return
	}
	defer closer(r.Body)
	if len(recordSet) == 0 {
		err = ErrBodyEmpty
		return
	}
	recordKeys, err := adapter.tableKeys(recordSet[0])
	if err != nil {
		return
	}
	quotedKeys := make([]string, 0, len(recordKeys))
	for _, key := range recordKeys {
		quoted, qErr := ident.Quote(key)
		if qErr != nil {
			err = errors.Wrap(ErrInvalidIdentifier, "BatchInsert")
			return
		}
		quotedKeys = append(quotedKeys, quoted)
	}
	colsName = strings.Join(quotedKeys, ",")
	values, placeholders, err = adapter.operationValues(recordSet, recordKeys)
	return
}

func (adapter *postgres) operationValues(recordSet []map[string]interface{}, recordKeys []string) (values []interface{}, placeholders string, err error) {
	for i, record := range recordSet {
		initPH := len(values) + 1
		for _, key := range recordKeys {
			value := record[key]
			switch value.(type) {
			case []interface{}:
				values = append(values, formatters.FormatArray(value))
			default:
				values = append(values, value)
			}
		}
		pl := adapter.createPlaceholders(initPH, len(values))
		placeholders = fmt.Sprintf("%s,%s", placeholders, pl)
		if i == 0 {
			placeholders = pl
		}
	}
	return
}

func (adapter *postgres) tableKeys(json map[string]interface{}) (keys []string, err error) {
	for key := range json {
		if !ident.IsValid(key) {
			err = errors.Wrap(ErrInvalidIdentifier, "BatchInsert")
			return
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return
}

func (adapter *postgres) createPlaceholders(initial, lenValues int) (ret string) {
	for i := initial; i <= lenValues; i++ {
		if ret != "" {
			ret += ","
		}
		ret += fmt.Sprintf("$%d", i)
	}
	ret = fmt.Sprintf("(%s)", ret)
	return
}

// ParseInsertRequest create insert SQL
func (adapter *postgres) ParseInsertRequest(r *http.Request) (colsName string, colsValue string, values []interface{}, err error) {
	body := make(map[string]interface{})
	if err = json.NewDecoder(r.Body).Decode(&body); err != nil {
		return
	}
	defer closer(r.Body)

	if len(body) == 0 {
		err = ErrBodyEmpty
		return
	}

	fields := make([]string, 0)
	for key, value := range body {
		if !ident.IsValid(key) {
			err = errors.Wrap(ErrInvalidIdentifier, "Insert")
			return
		}
		fields = append(fields, fmt.Sprintf(`"%s"`, key))

		switch value.(type) {
		case []interface{}:
			values = append(values, formatters.FormatArray(value))
		default:
			values = append(values, value)
		}
	}

	colsName = strings.Join(fields, ", ")
	colsValue = adapter.createPlaceholders(1, len(values))
	return
}
