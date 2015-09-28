package main

import (
	"testing"
)

func TestInsert(t *testing.T) {
	for _, test := range []struct {
		input    string
		addition string
		expected string
		err      bool
	}{
		{
			"",
			"",
			"",
			false,
		},
		{
			`*.c
`,
			"",
			`*.c
`,
			false,
		},
		{
			"",
			"executable",
			delimiterStart + `executable
` + delimiterEnd,
			false,
		},
		{
			`*.c
`,
			`executable
`,
			`*.c
` + delimiterStart + `executable
` + delimiterEnd,
			false,
		},
		{
			"*.c",
			"executable",
			`*.c
` + delimiterStart + `executable
` + delimiterEnd,
			false,
		},
		{
			`*.c
` + delimiterStart + `oldexecutable
` + delimiterEnd,
			"executable",
			`*.c
` + delimiterStart + `executable
` + delimiterEnd,
			false,
		},
	} {
		got, err := insert(test.input, test.addition)
		if (err != nil) != test.err {
			t.Fatalf("Expected error: %t, got error: %t, with input '%s' and addition '%s'", test.err, (err == nil), test.input, test.addition)
		}
		if got != test.expected {
			t.Fatalf("With input '%s' and addition '%s' expected '%s' got '%s'\n", test.input, test.addition, test.expected, got)
		}
	}
}
