package scanner

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"testing"
)

func TestValidateType(t *testing.T) {
	var tmap map[string]interface{}
	var tint int
	var testCases = []struct {
		name  string
		input interface{}
		err   error
	}{
		{"is not a pointer", 1, errPtr},
		{"is not a valid type", &tint, errUnsupTyp},
		{"is a valid type", &tmap, nil},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := validateType(tc.input)
			if err != tc.err {
				t.Errorf("expected %v, but got %v", tc.err, err)
			}
		})
	}
}

func TestPrestScanGET(t *testing.T) {
	type ComplexType struct {
		Name string `json:"name,omitempty"`
	}
	tac := []ComplexType{ComplexType{Name: "test"}}
	byt, err := json.Marshal(tac)
	if err != nil {
		t.Errorf("expected no errors but got %v", err)
	}
	var tmap map[string]interface{}
	var testCases = []struct {
		name    string
		stInput *bytes.Buffer
		stErr   error
		scInput interface{}
		scErr   error
	}{
		{"scan GET", bytes.NewBuffer(byt), nil, &tmap, nil},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s := &PrestScanner{
				Buff:  tc.stInput,
				Error: tc.stErr,
			}
			if s.Err() != tc.stErr {
				t.Errorf("expected %v, but got %v", tc.stErr, s.Err())
			}
			err := s.scanGET(reflect.ValueOf(tc.scInput), tc.scInput)
			if err != tc.scErr {
				t.Errorf("expected %v, but got %v", tc.scErr, err)
			}
			if string(s.Bytes()) != tc.stInput.String() {
				t.Errorf("expected %v, but got %v", tc.stInput.Bytes(), s.Bytes())
			}
		})
	}
}

func TestPrestScan(t *testing.T) {
	type ComplexType struct {
		Name string `json:"name,omitempty"`
	}
	act := make([]ComplexType, 0)
	tac := []ComplexType{ComplexType{Name: "test"}}
	byt, err := json.Marshal(tac)
	if err != nil {
		t.Errorf("expected no errors but got %v", err)
	}
	tacs := []ComplexType{
		ComplexType{Name: "test"},
		ComplexType{Name: "Test"},
	}
	byts, err := json.Marshal(tacs)
	if err != nil {
		t.Errorf("expected no errors but got %v", err)
	}
	var tmap map[string]interface{}
	var testCases = []struct {
		name    string
		stInput *bytes.Buffer
		stErr   error
		scInput interface{}
		scErr   error
		method  string
	}{
		{"scan error", &bytes.Buffer{}, errors.New("test error"), tmap, errPtr, http.MethodGet},
		{"scan err length", bytes.NewBuffer(byts), nil, &ComplexType{}, errLength, http.MethodGet},
		{"scan err method", bytes.NewBuffer(byt), nil, &ComplexType{}, errMethod, http.MethodHead},
		{"scan not slice", bytes.NewBuffer(byt), nil, &ComplexType{}, nil, http.MethodGet},
		{"scan slice", bytes.NewBuffer(byt), nil, &act, nil, http.MethodGet},
		{"scan using map", bytes.NewBuffer(byt), nil, &tmap, nil, http.MethodGet},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s := &PrestScanner{
				Buff:   tc.stInput,
				Error:  tc.stErr,
				Method: tc.method,
			}
			if s.Err() != tc.stErr {
				t.Errorf("expected %v, but got %v", tc.stErr, s.Err())
			}
			err := s.Scan(tc.scInput)
			if err != tc.scErr {
				t.Errorf("expected %v, but got %v", tc.scErr, err)
			}
			if string(s.Bytes()) != tc.stInput.String() {
				t.Errorf("expected %v, but got %v", tc.stInput.Bytes(), s.Bytes())
			}
		})
	}
}
