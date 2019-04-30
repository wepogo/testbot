package testbot

import (
	"bufio"
	"bytes"
	"io"
)

type SyntaxError string

func (e SyntaxError) Error() string {
	return string(e)
}

func ParseTestfile(r io.Reader) (entries map[string]string, err error) {
	m := make(map[string]string)
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		l := bytes.TrimSpace(sc.Bytes())
		if len(l) == 0 || l[0] == '#' {
			continue
		}
		if i := bytes.IndexByte(l, ':'); i >= 0 && okName(l[:i]) {
			m[string(l[:i])] = string(bytes.TrimSpace(l[i+1:]))
		} else {
			return nil, SyntaxError("bad line: " + sc.Text())
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return m, nil
}

func okName(name []byte) bool {
	for _, c := range name {
		if '0' <= c && c <= '9' || 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || c == '_' {
			continue
		}
		return false
	}
	return len(name) > 0
}
