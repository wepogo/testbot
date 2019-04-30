package farmer

import "testing"

func TestAbbrevMiddle(t *testing.T) {
	cases := []struct {
		s string
		n int
		w string
	}{
		{"", 0, ""},
		{"x", 0, ""},
		{"x", 1, "x"},
		{"xx", 1, "."},
		{"xx", 2, "xx"},
		{"xxx", 2, ".."},
		{"xxx", 3, "xxx"},
		{"xxxx", 3, "..."},
		{"xxxx", 4, "xxxx"},
		{"xxxxx", 4, "x..."},
		{"xxxxx", 5, "xxxxx"},
		{"xxxxxx", 5, "x...x"},
	}

	for _, test := range cases {
		g := abbrevMiddle(test.s, test.n)
		if g != test.w {
			t.Errorf("abbrevMiddle(%q, %d) = %q, want %q", test.s, test.n, g, test.w)
		}
	}
}
