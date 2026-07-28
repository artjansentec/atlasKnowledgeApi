package service

import (
	"reflect"
	"testing"
)

func TestIntersectProjectIDs(t *testing.T) {
	allowed := []string{"a", "b", "c"}

	got := intersectProjectIDs(allowed, nil)
	if !reflect.DeepEqual(got, allowed) {
		t.Fatalf("nil requested: got %v", got)
	}

	got = intersectProjectIDs(allowed, []string{"b", "x", "b", " a "})
	want := []string{"b", "a"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("filter: got %v want %v", got, want)
	}

	got = intersectProjectIDs(allowed, []string{"z"})
	if len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
}
