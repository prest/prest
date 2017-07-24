package scanner

import (
	"bytes"
	"encoding/json"
	"errors"
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

func TestPrestScanner(t *testing.T) {
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
		{"scan error", &bytes.Buffer{}, errors.New("test error"), tmap, errPtr},
		{"scan not slice", bytes.NewBuffer(byt), nil, &ComplexType{}, nil},
		{"scan slice", bytes.NewBuffer(byt), nil, &act, nil},
		{"scan slice using map", bytes.NewBuffer(byt), nil, &tmap, nil},
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
