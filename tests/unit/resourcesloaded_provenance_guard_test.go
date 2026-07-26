// resourcesloaded_provenance_guard_test.go — structural guard closing the
// "forgot to declare Provenance" class at the source, the same shape as
// event_genstamp_guard_test.go's production-side GenStamp guard but aimed at
// this package: every messages.ResourcesLoaded{...} composite literal
// anywhere under tests/unit/ must set an explicit Provenance field.
//
// The zero value, FetchProvenanceUnknown, silently means "this is not the
// type's canonical top-level population" to every real consumer
// (core/app/handle.go's handleResourcesLoadedEvent, core/runtime's
// observeResourcesLoadedRows) — so a test literal that never states
// Provenance is not neutral, it is a discard instruction the author never
// meant to write. #209 found ~200 such literals already in this tree,
// several of them silently asserting against a PRIOR test's leaked rows
// instead of their own (tui_root_test.go's newRootSizedModel isolation
// defect made the leak possible; this guard closes the second half — the
// literals that would still discard themselves even with isolation fixed).
//
// This guard requires the field's PRESENCE, not any particular value: a
// literal that genuinely means "discard me" (a deliberate zero-Provenance
// fail-safe pin, see resourcesloaded_provenance_test.go's
// TestObserveResourcesLoadedRows_UnknownProvenance_FailSafe_DoesNotReplaceCanonical)
// satisfies this guard by writing `Provenance: messages.FetchProvenanceUnknown`
// explicitly — stating the zero value on purpose is exactly the intent this
// guard exists to force, not a violation of it.
//
// Style follows event_registry_contract_test.go's go/parser scanning
// (ercScanMarkerReceivers, ercFindFuncDeclByName) and event_genstamp_guard_test.go's
// production-side mirror: declared-set-from-AST, not a hand-maintained list.
package unit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// rlpgRepoRoot returns the repo-relative root, following the same
// filepath.Join("..", "..") pattern ercMessagesDir/esgRepoRoot use.
func rlpgRepoRoot() string {
	return filepath.Join("..", "..")
}

// rlpgViolation is one messages.ResourcesLoaded{...} composite literal that
// does not set an explicit Provenance field.
type rlpgViolation struct {
	Pos token.Position
}

// rlpgWalkGoFiles walks every .go file under root (recursing into
// subdirectories, e.g. tests/unit/tuitest) and returns every violating
// messages.ResourcesLoaded{...} composite literal: one whose keyed elements
// do not include a "Provenance" key, regardless of what value a caller that
// DOES set it chooses.
func rlpgWalkGoFiles(t *testing.T, root string) []rlpgViolation {
	t.Helper()

	var violations []rlpgViolation
	fset := token.NewFileSet()

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		file, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			t.Fatalf("ParseFile(%s): %v", path, perr)
		}

		ast.Inspect(file, func(n ast.Node) bool {
			cl, ok := n.(*ast.CompositeLit)
			if !ok {
				return true
			}
			sel, ok := cl.Type.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkgIdent, ok := sel.X.(*ast.Ident)
			if !ok || pkgIdent.Name != "messages" || sel.Sel.Name != "ResourcesLoaded" {
				return true
			}
			for _, elt := range cl.Elts {
				if kv, ok := elt.(*ast.KeyValueExpr); ok {
					if ident, ok := kv.Key.(*ast.Ident); ok && ident.Name == "Provenance" {
						return true // stated — not a violation, whatever the value
					}
				}
			}
			violations = append(violations, rlpgViolation{Pos: fset.Position(cl.Pos())})
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("filepath.Walk(%s): %v", root, err)
	}
	return violations
}

// TestResourcesLoadedProvenanceGuard_EveryLiteralStatesProvenance is the
// structural guard: every messages.ResourcesLoaded{...} composite literal
// under tests/unit/ must declare Provenance explicitly, whatever value it
// declares. Zero exceptions today (the #209 migration closed every prior
// site) — this asserts an empty violation set, not a tolerated allowlist. A
// future test that constructs a bare literal fails this test by name, with
// the exact file:line, rather than silently discarding itself the moment it
// happens to target a canonical top-level list (core/app/handle.go's
// isTopLevelCanonicalList gate).
func TestResourcesLoadedProvenanceGuard_EveryLiteralStatesProvenance(t *testing.T) {
	violations := rlpgWalkGoFiles(t, filepath.Join(rlpgRepoRoot(), "tests", "unit"))

	sort.Slice(violations, func(i, j int) bool {
		return violations[i].Pos.String() < violations[j].Pos.String()
	})
	for _, v := range violations {
		t.Errorf("%s: messages.ResourcesLoaded{...} does not declare an explicit Provenance field", v.Pos)
	}
}

// TestResourcesLoadedProvenanceGuard_ScannerDetectsUnstampedSite_InIsolatedTempDir
// is the executable detection proof (mirrors
// TestProductionGenStampGuard_ScannerDetectsUnstampedSite_InIsolatedTempDir):
// it writes a throwaway package that imports a canary "messages" package
// declaring a ResourcesLoaded-shaped type, constructs it both with and
// without a Provenance field, points rlpgWalkGoFiles at that directory, and
// asserts only the unstamped construction is reported — proving the scanner
// actually flags a real violation (not vacuously passing on an empty or
// misconfigured directory) and does not false-positive on a stamped sibling.
func TestResourcesLoadedProvenanceGuard_ScannerDetectsUnstampedSite_InIsolatedTempDir(t *testing.T) {
	dir := t.TempDir()
	src := `package canary

import "messages"

func buildUnstamped() messages.ResourcesLoaded {
	return messages.ResourcesLoaded{ResourceType: "ec2"}
}

func buildStamped() messages.ResourcesLoaded {
	return messages.ResourcesLoaded{ResourceType: "ec2", Provenance: messages.FetchProvenanceCanonicalList}
}

func buildStampedUnknownOnPurpose() messages.ResourcesLoaded {
	return messages.ResourcesLoaded{ResourceType: "ec2", Provenance: messages.FetchProvenanceUnknown}
}
`
	if err := os.WriteFile(filepath.Join(dir, "canary.go"), []byte(src), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	violations := rlpgWalkGoFiles(t, dir)

	if len(violations) != 1 {
		t.Fatalf("rlpgWalkGoFiles found %d violations in the isolated temp dir, want exactly 1 (buildUnstamped's literal): %+v", len(violations), violations)
	}
}
