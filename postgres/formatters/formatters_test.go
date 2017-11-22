package formatters

import (
	"fmt"
	"testing"
)

type str struct{}

func (s str) String() string {
	return "test"
}

func TestFormatArray(t *testing.T) {
	testCases := []struct {
		name string
		in   interface{}
		ret  string
	}{
		{"array string", []string{"value 1", "value 2", "value 3"}, `{"value 1","value 2","value 3"}`},
		{"array int", []int{10, 20, 30}, `{10,20,30}`},
		{"empty array", []int{}, `{}`},
		{"stringer array", []fmt.Stringer{str{}, str{}, str{}}, `{"test","test","test"}`},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ret := FormatArray(tc.in)
			if ret != tc.ret {
				t.Errorf("expected %v, but got %v", tc.ret, ret)
			}
		})
	}
}
