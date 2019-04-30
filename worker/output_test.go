package worker

import "testing"

func TestLooksLikeError(t *testing.T) {
	cases := []struct {
		line string
		want bool
	}{
		{"path/to/file.ext:123: any text here", true},
		{"path/to/file.ext:123x: bad line number", false},
		{"path/to/file.ext: 123: space before number", false},
		{"path/to/file.ext:123 no second colon", false},
		{"text with spaces:123: rest of line", false},
		{"path/to/file.ext:123: warning: any text here", false},
		{"ERROR: path/to/file.ts[4, 3]: message here", true},
	}

	for _, test := range cases {
		got := looksLikeError(test.line)
		if got != test.want {
			t.Errorf("looksLikeError(%q) = %v, want %v", test.line, got, test.want)
		}
	}
}
