package ident

import (
	"testing"
)

func TestQuote(t *testing.T) {
	tests := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{"table", `"table"`, false},
		{"schema.table", `"schema"."table"`, false},
		{"a.b.c", `"a"."b"."c"`, false},
		{"_foo123", `"_foo123"`, false},
		{"", "", true},
		{".foo", "", true},
		{"foo.", "", true},
		{"foo..bar", "", true},
		{"foo-bar", "", true}, // hyphen not allowed in IsValid
		{"foo.bar.baz", `"foo"."bar"."baz"`, false},
		{"123abc", "", true}, // cannot start with digit
		{"foo.bar.", "", true},
		{"foo..bar", "", true},
		{"foo.bar..baz", "", true},
		// SQL injection attempts
		{`table;DROP TABLE users;--`, "", true},
		{`schema."table"`, "", true}, // embedded quotes not allowed in IsValid
		{`foo.bar;SELECT * FROM users`, "", true},
		{`foo.bar--comment`, "", true},
		{`foo.bar/*comment*/`, "", true},
		{`foo.bar' OR '1'='1`, "", true},
		{`foo.bar;DELETE FROM baz`, "", true},
		{`foo.bar;`, "", true},
		{`foo.bar;DROP TABLE baz;--`, "", true},
		{`foo.bar;--`, "", true},
		{`foo.bar/*hack*/`, "", true},
		{`foo.bar'`, "", true},
		// Additional valid cases
		{"A", `"A"`, false},
		{"_A", `"_A"`, false},
		{"a123", `"a123"`, false},
		{"foo_bar", `"foo_bar"`, false},
		{"foo123.bar456", `"foo123"."bar456"`, false},
		{"foo_bar.baz_qux", `"foo_bar"."baz_qux"`, false},
		{"foo_bar123.baz_qux456", `"foo_bar123"."baz_qux456"`, false},
		{"foo.bar.baz.qux", `"foo"."bar"."baz"."qux"`, false},
		// Edge cases for length
		{"a23456789012345678901234567890123456789012345678901234567890123", `"a23456789012345678901234567890123456789012345678901234567890123"`, false}, // 63 chars
		{"a234567890123456789012345678901234567890123456789012345678901234", "", true},                                                                  // 64 chars, invalid
		{"foo.a23456789012345678901234567890123456789012345678901234567890123", `"foo"."a23456789012345678901234567890123456789012345678901234567890123"`, false},
		{"foo.a234567890123456789012345678901234567890123456789012345678901234", "", true},
		// Invalid: segment too long
		{"foo.a234567890123456789012345678901234567890123456789012345678901234", "", true},
		// Invalid: empty segment in middle
		{"foo..bar", "", true},
		{"foo...bar", "", true},
		// Invalid: segment with only digits
		{"123", "", true},
		{"foo.123", "", true},
		// Invalid: segment with special characters
		{"foo@bar", "", true},
		{"foo#bar", "", true},
		{"foo$bar", "", true},
		{"foo/bar", "", true},
		{"foo*bar", "", true},
		{"foo;bar", "", true},
		{"foo bar", "", true},
		{"foo'bar", "", true},
		{"foo\"bar", "", true},
		// Valid: underscores and digits
		{"foo_bar_123", `"foo_bar_123"`, false},
		{"foo_bar_123.baz_qux_456", `"foo_bar_123"."baz_qux_456"`, false},
		// Invalid: starts with digit
		{"1foo", "", true},
		{"1foo.bar", "", true},
		// Invalid: segment with hyphen
		{"foo-bar", "", true},
		{"foo.bar-baz", "", true},
		// Valid: leading underscore
		{"_foo", `"_foo"`, false},
		{"_foo._bar", `"_foo"."_bar"`, false},
		// Invalid: segment with only underscore
		{"_", `"_"`, false},
		{"foo._", `"foo"."_"`, false},
		// Invalid: segment with only dot
		{".", "", true},
		// Invalid: segment with only quotes
		{`"foo"`, "", true},
		// Valid: mixed case
		{"FooBar", `"FooBar"`, false},
		{"fooBar.BazQux", `"fooBar"."BazQux"`, false},
	}

	for _, tt := range tests {
		got, err := Quote(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("Quote(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("Quote(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestIsSafeSegment(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"table", true},
		{"table_name", true},
		{"table-name", true},
		{"Table123", true},
		{"_underscore", true},
		{"123table", true},
		{"", false},
		{"a_very_long_identifier_name_that_exceeds_sixty_three_characters_0123456789", false},
		{"table.name", false},     // dot not allowed
		{"table'name", false},     // quote not allowed
		{"table name", false},     // space not allowed
		{"table$name", false},     // dollar not allowed
		{"table@name", false},     // at symbol not allowed
		{"table#name", false},     // hash not allowed
		{"table/name", false},     // slash not allowed
		{"table*", false},         // asterisk not allowed
		{"table;", false},         // semicolon not allowed
		{"table--comment", true},  // double hyphen allowed
		{"-leadinghyphen", true},  // leading hyphen allowed
		{"trailinghyphen-", true}, // trailing hyphen allowed
	}

	for _, tt := range tests {
		got := IsSafeSegment(tt.input)
		if got != tt.want {
			t.Errorf("IsSafeSegment(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestSplitAndValidateCSV(t *testing.T) {
	tests := []struct {
		input   string
		want    []string
		wantErr bool
	}{
		{"", nil, false},
		{"table", []string{"table"}, false},
		{"table1,table2", []string{"table1", "table2"}, false},
		{"schema.table,foo.bar", []string{"schema.table", "foo.bar"}, false},
		{"_foo123,_bar456", []string{"_foo123", "_bar456"}, false},
		{"table,", nil, true},                   // trailing comma, empty identifier
		{",table", nil, true},                   // leading comma, empty identifier
		{"foo..bar", nil, true},                 // invalid identifier
		{"foo-bar,table", nil, true},            // hyphen not allowed in IsValid
		{"123abc,table", nil, true},             // cannot start with digit
		{"foo.bar.baz,", nil, true},             // trailing comma
		{"foo,bar;DROP TABLE users", nil, true}, // SQL injection attempt
		{"foo,bar--comment", nil, true},         // double hyphen not allowed in IsValid
		{"foo,bar/*comment*/", nil, true},
		{"foo,bar' OR '1'='1", nil, true},
	}

	for _, tt := range tests {
		got, err := SplitAndValidateCSV(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("SplitAndValidateCSV(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if !tt.wantErr && !equalStringSlices(got, tt.want) {
			t.Errorf("SplitAndValidateCSV(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

// Helper for comparing slices
func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
