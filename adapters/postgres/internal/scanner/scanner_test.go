package scanner

import (
	"bytes"
	"encoding/json"
	"errors"
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

func TestPrestScanQuery(t *testing.T) {
	type ComplexType struct {
		Name string `json:"name,omitempty"`
	}
	act := make([]ComplexType, 0)
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
		{"scan Query map", bytes.NewBuffer(byt), nil, &tmap, nil},
		{"scan Query struct", bytes.NewBuffer(byt), nil, &ComplexType{}, nil},
		{"scan Query slice", bytes.NewBuffer(byt), nil, &act, nil},
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
			err := s.scanQuery(reflect.ValueOf(tc.scInput), tc.scInput)
			if err != tc.scErr {
				t.Errorf("expected %v, but got %v", tc.scErr, err)
			}
			if string(s.Bytes()) != tc.stInput.String() {
				t.Errorf("expected %v, but got %v", tc.stInput.Bytes(), s.Bytes())
			}
		})
	}
}

func TestPrestScanNotQuery(t *testing.T) {
	type ComplexType struct {
		Name string `json:"name,omitempty"`
	}
	act := make([]ComplexType, 0)
	tac := ComplexType{Name: "test"}
	byt, err := json.Marshal(tac)
	if err != nil {
		t.Errorf("expected no errors but got %v", err)
	}
	var tmap map[string]interface{}
	errJSON := errors.New("json: cannot unmarshal array into Go value of type scanner.ComplexType")
	var testCases = []struct {
		name    string
		stInput *bytes.Buffer
		stErr   error
		scInput interface{}
		scErr   error
	}{
		{"scan Not Query map", bytes.NewBuffer(byt), nil, &tmap, nil},
		{"scan Not Query struct", bytes.NewBuffer(byt), nil, &ComplexType{}, nil},
		{"scan Not Query slice", bytes.NewBuffer(byt), errJSON, &act, errUnsupTyp},
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
			err := s.scanNotQuery(reflect.ValueOf(tc.scInput), tc.scInput)
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
	ta := ComplexType{Name: "test"}
	b, err := json.Marshal(ta)
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
		isQuery bool
	}{
		{"scan error", &bytes.Buffer{}, errors.New("test error"), tmap, errPtr, true},
		{"scan err length", bytes.NewBuffer(byts), nil, &ComplexType{}, errLength, true},
		{"scan not slice", bytes.NewBuffer(byt), nil, &ComplexType{}, nil, true},
		{"scan slice", bytes.NewBuffer(byt), nil, &act, nil, true},
		{"scan using map", bytes.NewBuffer(byt), nil, &tmap, nil, true},
		{"scan not query", bytes.NewBuffer(b), nil, &tmap, nil, false},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s := &PrestScanner{
				Buff:    tc.stInput,
				Error:   tc.stErr,
				IsQuery: tc.isQuery,
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
