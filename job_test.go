package testbot

import "testing"

func TestParseJob(t *testing.T) {
	cases := []struct {
		s string
		j Job
	}{
		{"91ac/meta", Job{"91ac", "/", "meta"}},
		{"91ac/core/gotest", Job{"91ac", "/core", "gotest"}},
		{"91ac/cmd/ledgerd/gotest", Job{"91ac", "/cmd/ledgerd", "gotest"}},
	}

	for _, test := range cases {
		j, err := ParseJob(test.s)
		if err != nil {
			t.Errorf("ParseJob(%q) err = %v, want nil", test.s, err)
		}
		if j != test.j {
			t.Errorf("ParseJob(%q) = %+v, want %+v", test.s, j, test.j)
		}
	}
}

func TestParseJobBad(t *testing.T) {
	cases := []string{
		"91ac",
		"91ac/",
		"91ac/meta/",
		"foo/meta",
	}

	for _, test := range cases {
		_, err := ParseJob(test)
		if err == nil {
			t.Errorf("ParseJob(%q) err = nil, want error", test)
		}
	}
}
