// Package log implements a standard convention for structured logging.
// Log entries are formatted as K=V pairs.
// By default, output is written to stdout; this can be changed with SetOutput.
package log

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/xerrors"
)

// context key type
type key int

var (
	logWriterMu sync.Mutex // protects the following
	logWriter   io.Writer  = os.Stdout

	// context key for log line prefixes
	prefixKey key = 0
)

const (
	// pairDelims contains a list of characters that may be used as delimeters
	// between key-value pairs in a log entry. Keys and values will be quoted or
	// otherwise formatted to ensure that key-value extraction is unambiguous.
	//
	// The list of pair delimiters follows Splunk conventions, described here:
	// http://answers.splunk.com/answers/143368/default-delimiters-for-key-value-extraction.html
	pairDelims      = " ,;|&\t\n\r"
	illegalKeyChars = pairDelims + `="`
)

// Conventional key names for log entries
const (
	KeyCaller  = "at"      // location of caller
	KeyMessage = "message" // produced by Message
	KeyError   = "error"   // produced by Error

	keyLogError = "log-error" // for errors produced by the log package itself
)

// SetOutput sets the log output to w.
// If SetOutput hasn't been called,
// the default behavior is to write to stdout.
func SetOutput(w io.Writer) {
	logWriterMu.Lock()
	logWriter = w
	logWriterMu.Unlock()
}

func prefix(ctx context.Context) []byte {
	b, _ := ctx.Value(prefixKey).([]byte)
	return b
}

// Printkv prints a structured log entry to stdout. Log fields are
// specified as a variadic sequence of alternating keys and values.
//
// Duplicate keys will be preserved.
//
// Two fields are automatically added to the log entry: t=[time]
// and at=[file:line] indicating the location of the caller.
// Use SkipFunc to prevent helper functions from showing up in the
// at=[file:line] field.
func Printkv(ctx context.Context, keyvals ...interface{}) {
	Helper()
	// Invariant: len(keyvals) is always even.
	if len(keyvals)%2 != 0 {
		keyvals = append(keyvals, "", keyLogError, "odd number of log params")
	}

	// Prepend the log entry with auto-generated fields.
	callerPrefix := fmt.Sprintf("%s=%s ", KeyCaller, caller())

	var out string
	for i := 0; i < len(keyvals); i += 2 {
		k := keyvals[i]
		v := keyvals[i+1]
		out += " " + formatKey(k) + "=" + formatValue(v)
	}

	logWriterMu.Lock()
	logWriter.Write([]byte(callerPrefix))
	logWriter.Write(prefix(ctx))
	logWriter.Write([]byte(out)) // ignore errors
	logWriter.Write([]byte{'\n'})
	logWriterMu.Unlock()
}

// Fatalkv is equivalent to Printkv() followed by a call to os.Exit(1).
func Fatalkv(ctx context.Context, keyvals ...interface{}) {
	Helper()
	Printkv(ctx, keyvals...)
	os.Exit(1)
}

// Printf prints a log entry containing a message assigned to the
// "message" key. Arguments are handled as in fmt.Printf.
func Printf(ctx context.Context, format string, a ...interface{}) {
	Helper()
	Printkv(ctx, KeyMessage, fmt.Sprintf(format, a...))
}

// Error prints a log entry containing an error message assigned to the
// "error" key.
// Optionally, an error message prefix can be included. Prefix arguments are
// handled as in fmt.Print.
func Error(ctx context.Context, err error, a ...interface{}) {
	Helper()
	if len(a) > 0 {
		err = xerrors.Errorf("%s: %s", fmt.Sprint(a...), err)
	}
	Printkv(ctx, KeyError, err)
}

// formatKey ensures that the stringified key is valid for use in a
// Splunk-style K=V format. It stubs out delimeter and quoter characters in
// the key string with hyphens.
func formatKey(k interface{}) string {
	s := fmt.Sprint(k)
	if s == "" {
		return "?"
	}

	for _, c := range illegalKeyChars {
		s = strings.Replace(s, string(c), "-", -1)
	}

	return s
}

// formatValue ensures that the stringified value is valid for use in a
// Splunk-style K=V format. It quotes the string value if delimeter or quoter
// characters are present in the value string.
func formatValue(v interface{}) string {
	s := fmt.Sprint(v)
	if strings.ContainsAny(s, pairDelims) {
		return strconv.Quote(s)
	}
	return s
}
