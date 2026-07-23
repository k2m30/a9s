package unit

// event_registry_contract_test.go — exhaustiveness gate for the
// core/runtime/messages Event/Cmd registries.
//
// AllEventSamples() (core/runtime/messages/event.go) and AllCmdSamples()
// (core/runtime/messages/cmd.go) each claim to carry exactly one sample per
// concrete type implementing the Event/Cmd marker interfaces. This test
// verifies that claim by source-scanning the package for every isEvent()/
// isCmd() method declaration (a go/parser walk, not reflection over the
// registries alone — reflection can only see what the registries already
// claim to contain, never a type the registries forgot) and asserting the
// declared set and the sampled set are identical, in both directions, with
// no duplicate entries hiding a real gap behind a coincidentally-matching
// count.
//
// v2 note: this file restores ONLY the exhaustiveness gate. It deliberately
// does NOT restore attempt-1's routing-classification tables (which message
// gets routed where) — v2's routing is a single Controller fold, a
// structurally different shape a classification table would misrepresent.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// ercMessagesDir returns the repo-relative path to core/runtime/messages,
// following the same filepath.Join("..", "..") repo-root pattern
// testhelpers_forbidden_test.go uses to locate tests/unit.
func ercMessagesDir() string {
	root := filepath.Join("..", "..")
	return filepath.Join(root, "core", "runtime", "messages")
}

// ercScanMarkerReceivers parses every non-test .go file in dir and returns
// the set of receiver base type names for every method declaration named
// methodName (e.g. "isEvent" or "isCmd"). isEvent/isCmd are declared only as
// interface method specs on messages.Event/messages.Cmd (no receiver, not an
// *ast.FuncDecl at all) and as concrete-type method implementations (which
// do have a receiver) — so filtering on Recv != nil naturally selects only
// the concrete implementations, exactly the set AllEventSamples/
// AllCmdSamples are supposed to cover.
func ercScanMarkerReceivers(t *testing.T, dir, methodName string) map[string]bool {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(%s): %v", dir, err)
	}

	found := make(map[string]bool)
	fset := token.NewFileSet()
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(dir, name)
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", path, err)
		}
		for _, decl := range file.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Recv == nil || len(fd.Recv.List) != 1 || fd.Name.Name != methodName {
				continue
			}
			if base := ercReceiverBaseName(fd.Recv.List[0].Type); base != "" {
				found[base] = true
			}
		}
	}
	return found
}

// ercReceiverBaseName returns the bare type name of a receiver expression,
// unwrapping a pointer receiver's leading *.
func ercReceiverBaseName(expr ast.Expr) string {
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	if ident, ok := expr.(*ast.Ident); ok {
		return ident.Name
	}
	return ""
}

// ercSampleTypeCounts returns, for a registry slice of interface values
// (messages.Event or messages.Cmd), how many times each concrete dynamic
// type name appears — reflect.TypeOf on an interface value yields the
// dynamic (concrete) type, exactly the name ercScanMarkerReceivers collects.
func ercSampleTypeCounts[T any](samples []T) map[string]int {
	counts := make(map[string]int, len(samples))
	for _, s := range samples {
		counts[reflect.TypeOf(s).Name()]++
	}
	return counts
}

// ercAssertExhaustive asserts declared and sampled name the exact same set
// of concrete types, and that no sampled type appears more than once (a
// duplicate could otherwise mask a missing type behind a matching count).
func ercAssertExhaustive(t *testing.T, label string, declared map[string]bool, sampled map[string]int) {
	t.Helper()

	var missingSample []string
	for name := range declared {
		if _, ok := sampled[name]; !ok {
			missingSample = append(missingSample, name)
		}
	}
	sort.Strings(missingSample)
	if len(missingSample) > 0 {
		t.Errorf("%s: declared but missing a sample: %v", label, missingSample)
	}

	var extraSample []string
	for name := range sampled {
		if !declared[name] {
			extraSample = append(extraSample, name)
		}
	}
	sort.Strings(extraSample)
	if len(extraSample) > 0 {
		t.Errorf("%s: sampled but not declared: %v", label, extraSample)
	}

	var duplicates []string
	for name, count := range sampled {
		if count > 1 {
			duplicates = append(duplicates, name)
		}
	}
	sort.Strings(duplicates)
	if len(duplicates) > 0 {
		t.Errorf("%s: duplicate sample types: %v", label, duplicates)
	}
}

// TestEventRegistry_Events_ExhaustiveEnumeration is the exhaustiveness gate
// for AllEventSamples(): every concrete type declaring isEvent() in
// core/runtime/messages must have exactly one sample there, and every
// sampled type must be a real isEvent() declaration.
func TestEventRegistry_Events_ExhaustiveEnumeration(t *testing.T) {
	declared := ercScanMarkerReceivers(t, ercMessagesDir(), "isEvent")
	sampled := ercSampleTypeCounts(messages.AllEventSamples())
	ercAssertExhaustive(t, "Event", declared, sampled)
}

// TestEventRegistry_Cmds_ExhaustiveEnumeration is the exhaustiveness gate
// for AllCmdSamples(): every concrete type declaring isCmd() in
// core/runtime/messages must have exactly one sample there, and every
// sampled type must be a real isCmd() declaration.
func TestEventRegistry_Cmds_ExhaustiveEnumeration(t *testing.T) {
	declared := ercScanMarkerReceivers(t, ercMessagesDir(), "isCmd")
	sampled := ercSampleTypeCounts(messages.AllCmdSamples())
	ercAssertExhaustive(t, "Cmd", declared, sampled)
}

// TestEventRegistry_ScannerDetectsMarkerType_InIsolatedTempDir is the
// executable detection proof: it writes a throwaway package messages file
// containing a canary type with an isEvent() method into a fresh t.TempDir(),
// points ercScanMarkerReceivers (the exact same helper the gate tests above
// use) at that directory, and asserts the canary is found — proving the
// scanner actually parses and matches declarations rather than vacuously
// passing on an empty or misconfigured directory.
func TestEventRegistry_ScannerDetectsMarkerType_InIsolatedTempDir(t *testing.T) {
	dir := t.TempDir()
	src := `package messages

type zzzCanaryEvent struct{}

func (zzzCanaryEvent) isEvent() {}
`
	if err := os.WriteFile(filepath.Join(dir, "canary.go"), []byte(src), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	found := ercScanMarkerReceivers(t, dir, "isEvent")
	if !found["zzzCanaryEvent"] {
		t.Errorf("scanner did not detect canary type zzzCanaryEvent in isolated temp dir; found=%v", found)
	}
}
