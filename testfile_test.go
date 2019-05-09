package testbot

import (
	"reflect"
	"strings"
	"testing"
)

const sampleTestfile = `
# this is a comment
gotest: go test ./...
gocompile: go install chain/... # this is an end-of-line comment
`

func TestParseCommands(t *testing.T) {
	want := map[string]string{
		"gotest":    "go test ./...",
		"gocompile": "go install chain/... # this is an end-of-line comment",
	}
	got, err := ParseTestfile(strings.NewReader(sampleTestfile))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ParseTestfile(%#q) = %v, want %v", sampleTestfile, got, want)
	}
}

func TestOkNameOk(t *testing.T) {
	cases := []string{
		"web",
		"a",
		"a123",
		"123",
		"name_with_underscore",
	}

	for _, test := range cases {
		if !okName([]byte(test)) {
			t.Errorf("okName(%q) = false, want true", test)
		}
	}
}

func TestOkNameBad(t *testing.T) {
	cases := []string{
		"",
		" ",
		"a.b",
		"a-",
	}

	for _, test := range cases {
		if okName([]byte(test)) {
			t.Errorf("okName(%q) = true, want false", test)
		}
	}
}
