// Structural guard: every
// messages.ResourcesLoaded{...} composite literal anywhere under tests/ (every
// test tree, walked recursively from rlpgTestsRoot) must set an explicit
// Provenance field.
//
// The zero value, FetchProvenanceUnknown, means "this is not the type's
// canonical top-level population" to every real consumer (core/app/handle.go's
// handleResourcesLoadedEvent, which owns both the screen write and the store
// write) — so a test literal that never states Provenance is not neutral: it is
// a discard instruction the author never meant to write.
//
// Scanning tests/ itself rather than a list of named trees covers any tree
// added under it with no code change here. Non-Go trees are walked but never
// parsed: rlpgWalkGoFiles filters by the ".go" suffix first.
//
// This guard requires the field's PRESENCE, not any particular value: a
// literal that genuinely means "discard me" (see
// resourcesloaded_provenance_test.go's
// TestObserveResourcesLoadedRows_UnknownProvenance_FailSafe_DoesNotReplaceCanonical)
// writes `Provenance: messages.FetchProvenanceUnknown` explicitly.
package unit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// rlpgRepoRoot returns the repo-relative root, following the same
// filepath.Join("..", "..") pattern ercMessagesDir/esgRepoRoot use.
func rlpgRepoRoot() string {
	return filepath.Join("..", "..")
}

// rlpgTestsRoot is the scan root for the guard: the repo's tests/ directory
// itself, so one filepath.Walk sweeps every test tree under it.
func rlpgTestsRoot() string {
	return filepath.Join(rlpgRepoRoot(), "tests")
}

// rlpgMessagesImportPath is the fully-qualified import path of the runtime
// messages package whose ResourcesLoaded composite literals this guard
// scans for. Resolving against this path (not a hardcoded local identifier)
// is what lets the scanner follow an import alias or a dot-import to the
// same underlying package.
const rlpgMessagesImportPath = "github.com/k2m30/a9s/v3/core/runtime/messages"

// rlpgViolation is one messages.ResourcesLoaded{...} composite literal that
// does not set an explicit Provenance field.
type rlpgViolation struct {
	Pos token.Position
}

// rlpgResolveMessagesAlias inspects file's import declarations and returns
// the local identifier this file binds rlpgMessagesImportPath to: the
// package's own name ("messages") when the import has no explicit alias,
// the explicit alias otherwise (e.g. `msg "…/messages"` -> "msg"), or "."
// for a dot import. ok is false when the file does not import the package
// at all, in which case no ResourcesLoaded literal in it can possibly
// reference it. Matching the resolved alias — rather than the literal
// string "messages" — is what keeps `msg.ResourcesLoaded{...}` and a
// dot-imported bare `ResourcesLoaded{...}` from slipping past this guard.
func rlpgResolveMessagesAlias(file *ast.File) (alias string, ok bool) {
	for _, imp := range file.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil || path != rlpgMessagesImportPath {
			continue
		}
		if imp.Name == nil {
			return "messages", true
		}
		return imp.Name.Name, true
	}
	return "", false
}

// rlpgWalkGoFiles walks every .go file under root (recursing into
// subdirectories, e.g. tests/unit/tuitest) and returns every violating
// ResourcesLoaded{...} composite literal of the rlpgMessagesImportPath
// package — however that package is imported in the given file (its own
// name, an alias, or a dot-import) — whose keyed elements do not include a
// "Provenance" key, regardless of what value a caller that DOES set it
// chooses.
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

		alias, imported := rlpgResolveMessagesAlias(file)
		if !imported {
			return nil
		}

		ast.Inspect(file, func(n ast.Node) bool {
			cl, ok := n.(*ast.CompositeLit)
			if !ok {
				return true
			}

			switch alias {
			case ".":
				ident, ok := cl.Type.(*ast.Ident)
				if !ok || ident.Name != "ResourcesLoaded" {
					return true
				}
			default:
				sel, ok := cl.Type.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				pkgIdent, ok := sel.X.(*ast.Ident)
				if !ok || pkgIdent.Name != alias || sel.Sel.Name != "ResourcesLoaded" {
					return true
				}
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
// anywhere under tests/ must declare Provenance explicitly, whatever value it
// declares. The violation set must be empty; a bare literal fails this test
// with its exact file:line rather than silently discarding itself when it
// targets a canonical top-level list (core/app/handle.go's
// isTopLevelCanonicalList gate).
func TestResourcesLoadedProvenanceGuard_EveryLiteralStatesProvenance(t *testing.T) {
	violations := rlpgWalkGoFiles(t, rlpgTestsRoot())

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
// it writes a throwaway package that imports the real messages package path
// unaliased, constructs a ResourcesLoaded-shaped type both with and without
// a Provenance field, points rlpgWalkGoFiles at that directory, and asserts
// only the unstamped construction is reported — proving the scanner
// actually flags a real violation (not vacuously passing on an empty or
// misconfigured directory) and does not false-positive on a stamped sibling.
func TestResourcesLoadedProvenanceGuard_ScannerDetectsUnstampedSite_InIsolatedTempDir(t *testing.T) {
	dir := t.TempDir()
	src := `package canary

import "github.com/k2m30/a9s/v3/core/runtime/messages"

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

// TestResourcesLoadedProvenanceGuard_ScannerDetectsUnstampedSite_ThroughImportAlias
// proves the scanner resolves the real import path rather than matching the
// literal local package name "messages": an aliased import
// (`rl "github.com/k2m30/a9s/v3/core/runtime/messages"`) referencing the
// type as `rl.ResourcesLoaded{...}` must still be caught;
// rlpgResolveMessagesAlias resolves the alias, so a scanner matching
// pkgIdent.Name == "messages" literally would let this site slip through
// silently.
func TestResourcesLoadedProvenanceGuard_ScannerDetectsUnstampedSite_ThroughImportAlias(t *testing.T) {
	dir := t.TempDir()
	src := `package canary

import rl "github.com/k2m30/a9s/v3/core/runtime/messages"

func buildUnstampedAliased() rl.ResourcesLoaded {
	return rl.ResourcesLoaded{ResourceType: "ec2"}
}

func buildStampedAliased() rl.ResourcesLoaded {
	return rl.ResourcesLoaded{ResourceType: "ec2", Provenance: rl.FetchProvenanceCanonicalList}
}
`
	if err := os.WriteFile(filepath.Join(dir, "canary_alias.go"), []byte(src), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	violations := rlpgWalkGoFiles(t, dir)

	if len(violations) != 1 {
		t.Fatalf("rlpgWalkGoFiles found %d violations for an aliased import, want exactly 1 (buildUnstampedAliased's literal): %+v", len(violations), violations)
	}
}

// TestResourcesLoadedProvenanceGuard_ScannerDetectsUnstampedSite_ThroughDotImport
// is the same proof for a dot-import (`. "…/messages"`), where the type is
// referenced bare as `ResourcesLoaded{...}` with no package qualifier at
// all — an *ast.Ident composite-literal type rather than the
// *ast.SelectorExpr every non-dot import produces.
func TestResourcesLoadedProvenanceGuard_ScannerDetectsUnstampedSite_ThroughDotImport(t *testing.T) {
	dir := t.TempDir()
	src := `package canary

import . "github.com/k2m30/a9s/v3/core/runtime/messages"

func buildUnstampedDotImported() ResourcesLoaded {
	return ResourcesLoaded{ResourceType: "ec2"}
}

func buildStampedDotImported() ResourcesLoaded {
	return ResourcesLoaded{ResourceType: "ec2", Provenance: FetchProvenanceCanonicalList}
}
`
	if err := os.WriteFile(filepath.Join(dir, "canary_dot.go"), []byte(src), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	violations := rlpgWalkGoFiles(t, dir)

	if len(violations) != 1 {
		t.Fatalf("rlpgWalkGoFiles found %d violations for a dot-import, want exactly 1 (buildUnstampedDotImported's literal): %+v", len(violations), violations)
	}
}

// TestResourcesLoadedProvenanceGuard_DiscoversViolationsAcrossSiblingTestTrees
// builds three sibling directories under one temp root — "unit" (stamped,
// clean), "integration" (unstamped) and "somefuturetree" (unstamped, a name no
// production code names) — and checks that rlpgWalkGoFiles pointed at the
// shared parent finds both violations by recursion alone, with no
// per-tree-name special case.
func TestResourcesLoadedProvenanceGuard_DiscoversViolationsAcrossSiblingTestTrees(t *testing.T) {
	root := t.TempDir()
	trees := map[string]string{
		"unit": `package unit

import "github.com/k2m30/a9s/v3/core/runtime/messages"

func buildStamped() messages.ResourcesLoaded {
	return messages.ResourcesLoaded{ResourceType: "ec2", Provenance: messages.FetchProvenanceCanonicalList}
}
`,
		"integration": `package integration

import "github.com/k2m30/a9s/v3/core/runtime/messages"

func buildUnstampedIntegration() messages.ResourcesLoaded {
	return messages.ResourcesLoaded{ResourceType: "alarm"}
}
`,
		"somefuturetree": `package somefuturetree

import "github.com/k2m30/a9s/v3/core/runtime/messages"

func buildUnstampedFutureTree() messages.ResourcesLoaded {
	return messages.ResourcesLoaded{ResourceType: "vpc"}
}
`,
	}
	for dirName, src := range trees {
		treeDir := filepath.Join(root, dirName)
		if err := os.MkdirAll(treeDir, 0o755); err != nil {
			t.Fatalf("MkdirAll(%s): %v", treeDir, err)
		}
		if err := os.WriteFile(filepath.Join(treeDir, "canary.go"), []byte(src), 0o600); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}

	violations := rlpgWalkGoFiles(t, root)

	if len(violations) != 2 {
		t.Fatalf("rlpgWalkGoFiles(%s) found %d violations, want exactly 2 (integration's and somefuturetree's unstamped literals — unit's is clean): %+v", root, len(violations), violations)
	}
	for _, v := range violations {
		if strings.Contains(v.Pos.String(), string(filepath.Separator)+"unit"+string(filepath.Separator)) {
			t.Errorf("violation %s reported inside the stamped \"unit\" tree; want it confined to the unstamped siblings", v.Pos)
		}
	}
}
