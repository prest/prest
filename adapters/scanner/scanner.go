package scanner

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"

	"github.com/structy/log"
)

var (
	errPtr      = errors.New("item to input data is not a pointer")
	errUnsupTyp = errors.New("item to input data has an unsupported type")
	errLength   = errors.New("rows returned is not 1")
	supType     = map[reflect.Kind]bool{
		reflect.Slice:  true,
		reflect.Struct: true,
		reflect.Map:    true,
	}
)

// Scanner interface to enable map pREST result to a struct
type Scanner interface {
	// Scan copies the columns from the matched row into the value provided.
	// returns the number of columns copied into the interface (multiple copies due to Scan)
	Scan(interface{}) (int, error)

	// Bytes returns the bytes representation of the value
	Bytes() []byte

	// Err returns the error, if any, that was encountered during iteration.
	Err() error
}

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

// PrestScanner is a default implementation of adapter.Scanner
type PrestScanner struct {
	Buff    *bytes.Buffer
	Error   error
	IsQuery bool
}

// Scan put prest response into a struct or map
func (p *PrestScanner) Scan(i interface{}) (l int, err error) {
	var ref reflect.Value
	log.Debugln("database return:", p.Buff.String())
	if ref, err = validateType(i); err != nil {
		return
	}
	if p.IsQuery {
		l, err = p.scanQuery(ref, i)
		return
	}
	l, err = p.scanNotQuery(ref, i)
	return
}

func (p *PrestScanner) scanQuery(ref reflect.Value, i interface{}) (l int, err error) {
	decoder := json.NewDecoder(p.Buff)
	if ref.Elem().Kind() == reflect.Slice {
		err = decoder.Decode(&i)
		l = ref.Elem().Len()
		return
	}
	ret := make([]map[string]interface{}, 0)
	if err = decoder.Decode(&ret); err != nil {
		return
	}
	l = len(ret)
	if len(ret) == 0 {
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

func (p *PrestScanner) scanNotQuery(ref reflect.Value, i interface{}) (l int, err error) {
	const notQueryReturnLen = 1
	l = notQueryReturnLen
	if ref.Elem().Kind() == reflect.Slice {
		err = errUnsupTyp
		return
	}
	err = json.NewDecoder(p.Buff).Decode(&i)
	return
}

// Bytes return prest response in bytes
func (p *PrestScanner) Bytes() (byt []byte) {
	if p.Buff != nil {
		byt = p.Buff.Bytes()
	}
	return
}

// Err return prest response error
func (p *PrestScanner) Err() (err error) {
	err = p.Error
	return
}
