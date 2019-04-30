package httpjson

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"reflect"

	"github.com/interstellar/testbot/log"
	"golang.org/x/xerrors"
)

// Read decodes a single JSON text from r into v.
// The only error it returns is ErrBadRequest
// (wrapped with the original error message as context).
func Read(r io.Reader, v interface{}) error {
	dec := json.NewDecoder(r)
	dec.UseNumber()
	err := dec.Decode(v)
	if err != nil {
		return xerrors.Errorf("bad request: %w", err)
	}
	return err
}

// Write sets the Content-Type header field to indicate
// JSON data, writes the header using status,
// then writes v to w.
// It logs any error encountered during the write.
func Write(ctx context.Context, w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)

	err := json.NewEncoder(w).Encode(Array(v))
	if err != nil {
		log.Error(ctx, err)
	}
}

// Array returns an empty JSON array if v is a nil slice,
// so that it renders as "[]" rather than "null".
// Otherwise, it returns v.
func Array(v interface{}) interface{} {
	if rv := reflect.ValueOf(v); rv.Kind() == reflect.Slice && rv.IsNil() {
		v = []struct{}{}
	}
	return v
}
