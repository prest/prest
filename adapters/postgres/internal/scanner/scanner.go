package scanner

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
)

var (
	errPtr      = errors.New("item to input data is not a pointer")
	errUnsupTyp = errors.New("item to input data has an unupported type")
	errLength   = errors.New("rows returned is not 1")
	supType     = map[reflect.Kind]bool{
		reflect.Slice:  true,
		reflect.Struct: true,
		reflect.Map:    true,
	}
)

func validateType(i interface{}) (ref reflect.Value, err error) {
	ref = reflect.ValueOf(i)
	if ref.Kind() != reflect.Ptr {
		err = errPtr
		return
	}
	if _, ok := supType[ref.Elem().Kind()]; !ok {
		err = errUnsupTyp
		return
	}
	return
}

// PrestScanner is a default implementation of postgres.Scanner
type PrestScanner struct {
	Buff  *bytes.Buffer
	Error error
}

// Scan put prest response into a struct or map
func (p *PrestScanner) Scan(i interface{}) (err error) {
	var ref reflect.Value
	if ref, err = validateType(i); err != nil {
		return
	}
	decoder := json.NewDecoder(p.Buff)
	if ref.Elem().Kind() == reflect.Slice {
		err = decoder.Decode(&i)
		return
	}
	ret := make([]map[string]interface{}, 0)
	if err = decoder.Decode(&ret); err != nil {
		return
	}
	if len(ret) != 1 {
		err = errLength
		return
	}
	var byt []byte
	byt, err = json.Marshal(ret[0])
	if err != nil {
		return
	}
	err = json.Unmarshal(byt, &i)
	return
}

// Bytes return prest response in bytes
func (p *PrestScanner) Bytes() (byt []byte) {
	byt = p.Buff.Bytes()
	return
}

// Err return prest response error
func (p *PrestScanner) Err() (err error) {
	err = p.Error
	return
}
