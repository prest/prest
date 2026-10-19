package controllers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"

	"github.com/prest/prest/v2/internal/logsafe"

	"github.com/structy/log"
)

// errUnexpectedResponseShape is returned when a write response is neither a row
// object nor an array of row objects, so its columns cannot be filtered.
var errUnexpectedResponseShape = errors.New("unexpected write response shape")

// rejectUnwritableFields writes 400 and returns true when the request body sets
// a column outside the caller's write allowlist (GHSA-h25q-9rmj-646h). An empty
// allowlist (no fields configured) or "*" leaves the write unrestricted. batch
// selects the BatchInsert body (an array of records).
func (h *CRUDHandler) rejectUnwritableFields(w http.ResponseWriter, r *http.Request, database, schema, table string, batch bool) bool {
	// permissionRequest strips the query so _select/_join cannot change the allowlist.
	allowed, err := h.perms.FieldsPermissions(permissionRequest(r), database, schema, table, "write", currentUserName(r))
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return true
	}
	if len(allowed) == 0 || containsString(allowed, "*") {
		return false
	}
	keys, err := bodyKeys(r, batch)
	if err != nil {
		jsonError(w, fmt.Sprintf("could not parse request body: %v", err), http.StatusBadRequest)
		return true
	}
	for _, key := range keys {
		if !containsString(allowed, key) {
			jsonError(w, fmt.Sprintf("column %q is not writable on this table", key), http.StatusBadRequest)
			return true
		}
	}
	return false
}

// bodyKeys returns the sorted top-level keys of the JSON body (every record's
// for a batch) and restores r.Body so the query builder decodes the same bytes.
// It decodes only the first JSON value, exactly like the builders, so no key the
// builder sees can be skipped here.
func bodyKeys(r *http.Request, batch bool) ([]string, error) {
	buf, err := io.ReadAll(r.Body)
	_ = r.Body.Close()
	if err != nil {
		return nil, err
	}
	r.Body = io.NopCloser(bytes.NewReader(buf))

	var records []map[string]json.RawMessage
	dec := json.NewDecoder(bytes.NewReader(buf))
	if batch {
		err = dec.Decode(&records)
	} else {
		var record map[string]json.RawMessage
		err = dec.Decode(&record)
		records = append(records, record)
	}
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var keys []string
	for _, record := range records {
		for key := range record {
			if !seen[key] {
				seen[key] = true
				keys = append(keys, key)
			}
		}
	}
	sort.Strings(keys) // deterministic error message
	return keys, nil
}

// readableColumns returns the columns a write response may echo. all is true
// when nothing needs filtering (restrict off, or read fields "*"). A caller with
// no read grant gets an empty list: FieldsFor returns "*" for an op the rule does
// not grant, so the table-level read check must come first.
func (h *CRUDHandler) readableColumns(r *http.Request, database, schema, table string) (columns []string, all bool, err error) {
	user := currentUserName(r)
	if !h.perms.TablePermissions(database, schema, table, "read", user) {
		return nil, false, nil
	}
	fields, err := h.perms.FieldsPermissions(permissionRequest(r), database, schema, table, "read", user)
	if err != nil {
		return nil, false, err
	}
	if containsString(fields, "*") {
		return nil, true, nil
	}
	return fields, false, nil
}

// returningColumns is readableColumns for Update and Delete: only a RETURNING
// response carries row data, a bare rows_affected count needs no filtering.
func (h *CRUDHandler) returningColumns(r *http.Request, database, schema, table, returningSyntax string) (columns []string, all bool, err error) {
	if returningSyntax == "" {
		return nil, true, nil
	}
	return h.readableColumns(r, database, schema, table)
}

// writeFiltered writes body with status, first removing every column outside
// columns from the row object(s) it carries unless all is set. A body that
// cannot be filtered is never sent: the write already ran, so this answers 500
// rather than risk echoing an unreadable column.
func writeFiltered(w http.ResponseWriter, status int, body []byte, columns []string, all bool) {
	if !all {
		filtered, err := filterColumns(body, columns)
		if err != nil {
			log.Errorln(logsafe.Error(err))
			jsonError(w, "could not filter response columns", http.StatusInternalServerError)
			return
		}
		body = filtered
	}
	w.WriteHeader(status)
	//nolint
	w.Write(body)
}

// filterColumns keeps only the listed keys of a row object or an array of row
// objects, preserving key order and each value's exact bytes (numbers, nested
// JSON). Empty input and JSON null pass through; any other shape is an error.
func filterColumns(body []byte, columns []string) ([]byte, error) {
	if len(bytes.TrimSpace(body)) == 0 {
		return body, nil
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	switch tok {
	case nil:
		out.WriteString("null")
	case json.Delim('{'):
		if err = filterObject(dec, &out, columns); err != nil {
			return nil, err
		}
	case json.Delim('['):
		out.WriteByte('[')
		for i := 0; dec.More(); i++ {
			if tok, err = dec.Token(); err != nil {
				return nil, err
			}
			if tok != json.Delim('{') {
				return nil, errUnexpectedResponseShape
			}
			if i > 0 {
				out.WriteByte(',')
			}
			if err = filterObject(dec, &out, columns); err != nil {
				return nil, err
			}
		}
		if _, err = dec.Token(); err != nil { // closing ]
			return nil, err
		}
		out.WriteByte(']')
	default:
		return nil, errUnexpectedResponseShape
	}
	// Executors emit exactly one value; anything after it is not filtered, so
	// refuse it rather than pass it on.
	if _, err = dec.Token(); err != io.EOF {
		return nil, errUnexpectedResponseShape
	}
	return out.Bytes(), nil
}

// filterObject copies the members of the object whose opening brace dec has
// just read, keeping only the listed keys, and consumes its closing brace.
func filterObject(dec *json.Decoder, out *bytes.Buffer, columns []string) error {
	out.WriteByte('{')
	first := true
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		key, ok := tok.(string)
		if !ok {
			return errUnexpectedResponseShape
		}
		var value json.RawMessage
		if err = dec.Decode(&value); err != nil {
			return err
		}
		if !containsString(columns, key) {
			continue
		}
		encodedKey, err := json.Marshal(key)
		if err != nil {
			return err
		}
		if !first {
			out.WriteByte(',')
		}
		first = false
		out.Write(encodedKey)
		out.WriteByte(':')
		out.Write(value)
	}
	if _, err := dec.Token(); err != nil { // closing }
		return err
	}
	out.WriteByte('}')
	return nil
}
