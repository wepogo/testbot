package farmer

import (
	"reflect"
	"testing"
)

func TestFillParents(t *testing.T) {
	input := []string{"/a/b/c", "/d"}
	want := []string{"/", "/a", "/a/b", "/a/b/c", "/d"}
	got := fillParents(input)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("fillParents(%v) = %v, want %v", input, got, want)
	}
}

func TestFillParentsRel(t *testing.T) {
	input := []string{"a/b/c", "d"}
	want := []string{".", "a", "a/b", "a/b/c", "d"}
	got := fillParents(input)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("fillParents(%v) = %v, want %v", input, got, want)
	}
}
