// resourcesloaded_provenance_guard_test.go — structural guard closing the
// "forgot to declare Provenance" class at the source, the same shape as
// event_genstamp_guard_test.go's production-side GenStamp guard but aimed at
// this package: every messages.ResourcesLoaded{...} composite literal
// anywhere under tests/ (every test tree — tests/unit, tests/integration,
// and any tree added later; see rlpgTestsRoot) must set an explicit
// Provenance field.
//
// The zero value, FetchProvenanceUnknown, silently means "this is not the
// type's canonical top-level population" to every real consumer
// (core/app/handle.go's handleResourcesLoadedEvent, which owns both the
// screen write and the store write) — so a test literal that never states
// Provenance is not neutral, it is a discard instruction the author never
// meant to write. #209 found ~200 such literals already in tests/unit,
// several of them silently asserting against a PRIOR test's leaked rows
// instead of their own (tui_root_test.go's newRootSizedModel isolation
// defect made the leak possible; this guard closes the second half — the
// literals that would still discard themselves even with isolation fixed).
//
// The #209 migration and this guard both originally stopped at tests/unit —
// tests/integration kept building unstamped ResourcesLoaded literals and
// nothing caught it until a `make ready-to-push` run went red on a filtered
// list that rendered "Loading..." forever (fix/resourcesloaded-provenance).
// A guard whose scope is narrower than the problem just moves the blind
// spot to whichever directory it doesn't cover, so this guard now scans
// rlpgTestsRoot() — tests/ itself, walked recursively — rather than
// listing tests/unit and tests/integration by name: any test tree nested
// under tests/ today or added tomorrow (tests/stories, tests/e2e's Go
// helpers, a future tests/web) is covered with no code change here.
// Non-Go trees under tests/ (tests/e2e's Playwright/node_modules, tests/
// testdata's fixtures) are walked but never parsed — rlpgWalkGoFiles
// already filters by the ".go" suffix before calling parser.ParseFile — so
// widening the root to all of tests/ costs nothing beyond a directory
// listing over content that was always going to be skipped.
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
	"strconv"
	"strings"
	"testing"
)

// rlpgRepoRoot returns the repo-relative root, following the same
// filepath.Join("..", "..") pattern ercMessagesDir/esgRepoRoot use.
func rlpgRepoRoot() string {
	return filepath.Join("..", "..")
}

// rlpgTestsRoot is the scan root for the production guard test below: the
// repo's tests/ directory itself, not any single tree under it. Discovering
// every test tree this way — one filepath.Walk from the shared parent —
// means a future tests/<newtree> is swept automatically; naming tests/unit
// and tests/integration explicitly here would only repeat the mistake this
// guard exists to close (see the package doc comment above).
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
// anywhere under tests/ (every tree the repo has today, and every tree it
// grows tomorrow — rlpgTestsRoot) must declare Provenance explicitly,
// whatever value it declares. Zero exceptions today (the #209 migration
// plus the fix/resourcesloaded-provenance follow-up closed every prior
// site, in tests/unit and tests/integration alike) — this asserts an empty
// violation set, not a tolerated allowlist. A future test that constructs a
// bare literal fails this test by name, with the exact file:line, rather
// than silently discarding itself the moment it happens to target a
// canonical top-level list (core/app/handle.go's isTopLevelCanonicalList
// gate) — regardless of which test tree it was written in.
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
// type as `rl.ResourcesLoaded{...}` must still be caught. Before
// rlpgResolveMessagesAlias existed, the scanner matched pkgIdent.Name ==
// "messages" literally and this exact site would have slipped through
// silently — the gap CodeRabbit flagged.
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
// is the canary for the actual defect this branch found: the #209 migration
// and the original version of this guard both scanned only tests/unit/,
// leaving tests/integration (and any other sibling tree) free to build
// unstamped ResourcesLoaded literals with nothing to catch it — exactly what
// happened to ec2_nav_chain_spec008_test.go. This test proves the fix is
// general rather than "add tests/integration to a list": it builds three
// sibling directories under one temp root — "unit" (stamped, clean),
// "integration" (unstamped, a violation), and "somefuturetree" (unstamped, a
// violation) — a name no production code names literally anywhere. Pointing
// rlpgWalkGoFiles at the shared parent must find both non-"unit" violations
// by recursion alone, with no per-tree-name special case, so a tree added
// after this test is written is covered the same way "somefuturetree" is
// covered here.
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
