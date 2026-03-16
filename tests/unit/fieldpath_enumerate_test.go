package unit_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/k2m30/a9s/internal/fieldpath"
)

// ---------------------------------------------------------------------------
// Test struct definitions for EnumeratePaths
// ---------------------------------------------------------------------------

type enumState struct {
	Name string `json:"name"`
	Code int32  `json:"code"`
}

type enumTag struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type enumSimple struct {
	Name  string      `json:"name"`
	Count int32       `json:"count"`
	State enumState   `json:"state"`
	Tags  []enumTag   `json:"tags"`
}

// Named string type (like AWS SDK's StateName)
type enumStateName string

type enumEdge struct {
	State      *enumState    `json:"state"`
	LaunchTime time.Time     `json:"launchTime"`
	Status     enumStateName `json:"status"`
	Name       *string       `json:"name"`
}

// ---------------------------------------------------------------------------
// T036 — EnumeratePaths on simple structs
// ---------------------------------------------------------------------------

func TestEnumeratePaths_ScalarStringField(t *testing.T) {
	paths := fieldpath.EnumeratePaths(reflect.TypeOf(enumSimple{}), "")

	if !containsPath(paths, "name") {
		t.Errorf("expected paths to contain %q, got %v", "name", paths)
	}
}

func TestEnumeratePaths_Int32Field(t *testing.T) {
	paths := fieldpath.EnumeratePaths(reflect.TypeOf(enumSimple{}), "")

	if !containsPath(paths, "count") {
		t.Errorf("expected paths to contain %q, got %v", "count", paths)
	}
}

func TestEnumeratePaths_NestedStruct(t *testing.T) {
	paths := fieldpath.EnumeratePaths(reflect.TypeOf(enumSimple{}), "")

	if !containsPath(paths, "state.name") {
		t.Errorf("expected paths to contain %q, got %v", "state.name", paths)
	}
	if !containsPath(paths, "state.code") {
		t.Errorf("expected paths to contain %q, got %v", "state.code", paths)
	}
}

func TestEnumeratePaths_SliceField(t *testing.T) {
	paths := fieldpath.EnumeratePaths(reflect.TypeOf(enumSimple{}), "")

	if !containsPath(paths, "tags[].key") {
		t.Errorf("expected paths to contain %q, got %v", "tags[].key", paths)
	}
	if !containsPath(paths, "tags[].value") {
		t.Errorf("expected paths to contain %q, got %v", "tags[].value", paths)
	}
}

// ---------------------------------------------------------------------------
// T037 — EnumeratePaths edge cases
// ---------------------------------------------------------------------------

func TestEnumeratePaths_PointerToStruct(t *testing.T) {
	paths := fieldpath.EnumeratePaths(reflect.TypeOf(enumEdge{}), "")

	// Pointer to struct should be dereferenced — no "*" in path
	if !containsPath(paths, "state.name") {
		t.Errorf("expected paths to contain %q (pointer dereferenced), got %v", "state.name", paths)
	}
	if !containsPath(paths, "state.code") {
		t.Errorf("expected paths to contain %q (pointer dereferenced), got %v", "state.code", paths)
	}
	for _, p := range paths {
		if containsSubstring(p, "*") {
			t.Errorf("path %q should not contain '*'", p)
		}
	}
}

func TestEnumeratePaths_TimeFieldIsLeaf(t *testing.T) {
	paths := fieldpath.EnumeratePaths(reflect.TypeOf(enumEdge{}), "")

	// time.Time should be treated as leaf — just "launchTime", not "launchTime.wall" etc.
	if !containsPath(paths, "launchTime") {
		t.Errorf("expected paths to contain %q, got %v", "launchTime", paths)
	}
	for _, p := range paths {
		if len(p) > len("launchTime") && p[:len("launchTime")+1] == "launchTime." {
			t.Errorf("time.Time should be leaf; found recursive path %q", p)
		}
	}
}

func TestEnumeratePaths_NamedStringTypeIsLeaf(t *testing.T) {
	paths := fieldpath.EnumeratePaths(reflect.TypeOf(enumEdge{}), "")

	if !containsPath(paths, "status") {
		t.Errorf("expected paths to contain %q, got %v", "status", paths)
	}
}

func TestEnumeratePaths_PointerToStringIsLeaf(t *testing.T) {
	paths := fieldpath.EnumeratePaths(reflect.TypeOf(enumEdge{}), "")

	if !containsPath(paths, "name") {
		t.Errorf("expected paths to contain %q, got %v", "name", paths)
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func containsPath(paths []string, target string) bool {
	for _, p := range paths {
		if p == target {
			return true
		}
	}
	return false
}
