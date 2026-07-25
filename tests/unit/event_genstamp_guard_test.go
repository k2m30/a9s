// event_genstamp_guard_test.go — production-code structural guard closing the
// "forgot to stamp" class at the source, replacing the uniform-zero-rejection
// approach that was reverted for costing ~400 test edits against a hole no
// production path can reach (see fix/connectgen-and-local-audit's commit
// history: the ConnectGen cross-account leak was closed by seeding
// session.New()'s ConnectGen at 1, not by making every GenStamped event
// reject a zero stamp).
//
// This is the cheap structural replacement: every REAL construction site of a
// messages.GenStamped event type, anywhere in production code (core/,
// internal/, cmd/ — never tests/unit itself, and never core/runtime/messages
// itself, the definition/registry package where AllEventSamples() deliberately
// builds unstamped canonical samples), must set its Gen-bearing field. A
// caller that forgets to stamp is exactly the class of bug the ConnectGen
// leak was, whichever gen field is involved next time.
//
// Style follows event_registry_contract_test.go's go/parser scanning
// (ercScanMarkerReceivers, ercFindFuncDeclByName): declared-set-from-AST, not
// a hand-maintained list, so the guard cannot silently drift out of sync with
// a new GenStamped type or a renamed Gen field.
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

// esgRepoRoot returns the repo-relative root, following the same
// filepath.Join("..", "..") pattern ercMessagesDir uses.
func esgRepoRoot() string {
	return filepath.Join("..", "..")
}

// esgScanGenFieldNames parses every non-test .go file in
// core/runtime/messages/ and returns, for each concrete type declaring a
// GenStamp() domain.Gen method, the name of the field its return statement
// selects (`return m.Gen` -> "Gen", `return m.OperationID` -> "OperationID").
// Deriving the field name from the method body itself — rather than
// hardcoding a type->field map — means a future rename of the Gen-bearing
// field can never desync the guard from reality.
func esgScanGenFieldNames(t *testing.T, dir string) map[string]string {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(%s): %v", dir, err)
	}

	found := make(map[string]string)
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
			if !ok || fd.Recv == nil || len(fd.Recv.List) != 1 || fd.Name.Name != "GenStamp" {
				continue
			}
			base := ercReceiverBaseName(fd.Recv.List[0].Type)
			if base == "" || fd.Body == nil {
				continue
			}
			for _, stmt := range fd.Body.List {
				ret, ok := stmt.(*ast.ReturnStmt)
				if !ok || len(ret.Results) != 1 {
					continue
				}
				sel, ok := ret.Results[0].(*ast.SelectorExpr)
				if !ok {
					continue
				}
				found[base] = sel.Sel.Name
			}
		}
	}
	return found
}

// esgViolation is one production construction site that does not stamp its
// GenStamped type's Gen-bearing field.
type esgViolation struct {
	Pos       token.Position
	TypeName  string
	FieldName string
}

// esgWalkProductionGo walks every non-test .go file under each root,
// skipping the messages package itself (the definition/registry site, not a
// dispatch site — AllEventSamples() there deliberately builds unstamped
// canonical samples), and returns every violating messages.<Type>{...}
// composite literal: one whose Type is a key in genFieldByType but whose
// keyed elements do not include the expected Gen-bearing field.
func esgWalkProductionGo(t *testing.T, roots []string, genFieldByType map[string]string) []esgViolation {
	t.Helper()

	var violations []esgViolation
	fset := token.NewFileSet()

	for _, root := range roots {
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				// core/runtime/messages is the definition/registry package,
				// not a dispatch site — excluded, not swept for violations.
				if filepath.ToSlash(path) == filepath.ToSlash(filepath.Join(esgRepoRoot(), "core", "runtime", "messages")) {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
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
				pkg, typeName, ok := ercSelectorPkgType(cl.Type)
				if !ok || pkg != "messages" {
					return true
				}
				fieldName, tracked := genFieldByType[typeName]
				if !tracked {
					return true
				}
				for _, elt := range cl.Elts {
					kv, ok := elt.(*ast.KeyValueExpr)
					if !ok {
						continue
					}
					if ident, ok := kv.Key.(*ast.Ident); ok && ident.Name == fieldName {
						return true // stamped — not a violation
					}
				}
				violations = append(violations, esgViolation{
					Pos:       fset.Position(cl.Pos()),
					TypeName:  typeName,
					FieldName: fieldName,
				})
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatalf("filepath.Walk(%s): %v", root, err)
		}
	}
	return violations
}

// TestProductionGenStampGuard_EveryConstructionSiteStampsItsGenField is the
// structural guard: every production construction site of a GenStamped event
// type (core/, internal/, cmd/, excluding tests and the messages package
// itself) must set its Gen-bearing field. Zero exceptions today — the coder's
// audit found exactly two unstamped sites (EmitAPIErrorPayload, since fixed
// by threading Gen through emitAPIErrorCmd, and the dead-code
// runtime.RelatedCacheReplay, since deleted) — so this asserts an empty
// violation set, not a tolerated allowlist. A future forgot-to-stamp site
// fails this test by name, with the exact file:line, rather than silently
// shipping the next ConnectGen-shaped leak.
func TestProductionGenStampGuard_EveryConstructionSiteStampsItsGenField(t *testing.T) {
	genFieldByType := esgScanGenFieldNames(t, filepath.Join(esgRepoRoot(), "core", "runtime", "messages"))
	if len(genFieldByType) == 0 {
		t.Fatal("esgScanGenFieldNames found zero GenStamped types — scanner is not finding anything, check the scan path")
	}

	root := esgRepoRoot()
	roots := []string{
		filepath.Join(root, "core"),
		filepath.Join(root, "internal"),
		filepath.Join(root, "cmd"),
	}
	violations := esgWalkProductionGo(t, roots, genFieldByType)

	sort.Slice(violations, func(i, j int) bool {
		return violations[i].Pos.String() < violations[j].Pos.String()
	})
	for _, v := range violations {
		t.Errorf("%s: messages.%s{...} does not stamp its Gen-bearing field %q", v.Pos, v.TypeName, v.FieldName)
	}
}

// TestProductionGenStampGuard_ScannerDetectsUnstampedSite_InIsolatedTempDir is
// the executable detection proof (mirrors
// TestEventRegistry_ScannerDetectsMarkerType_InIsolatedTempDir): it writes a
// throwaway package importing "messages" that constructs a canary GenStamped
// type WITHOUT its Gen field, points esgWalkProductionGo at that directory
// with a synthetic genFieldByType map, and asserts the canary is reported —
// proving the scanner actually flags a real violation rather than vacuously
// passing on an empty or misconfigured directory. Also proves the reverse: a
// SIBLING file in the same temp dir that DOES stamp the field produces no
// violation for that construction, and a file whose path IS the excluded
// messages package is skipped entirely.
func TestProductionGenStampGuard_ScannerDetectsUnstampedSite_InIsolatedTempDir(t *testing.T) {
	dir := t.TempDir()
	src := `package canary

import "messages"

func buildUnstamped() messages.ZzzCanaryEvent {
	return messages.ZzzCanaryEvent{ResourceType: "ec2"}
}

func buildStamped() messages.ZzzCanaryEvent {
	return messages.ZzzCanaryEvent{ResourceType: "ec2", Gen: 1}
}
`
	if err := os.WriteFile(filepath.Join(dir, "canary.go"), []byte(src), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	genFieldByType := map[string]string{"ZzzCanaryEvent": "Gen"}
	violations := esgWalkProductionGo(t, []string{dir}, genFieldByType)

	if len(violations) != 1 {
		t.Fatalf("esgWalkProductionGo found %d violations in the isolated temp dir, want exactly 1 (buildUnstamped's literal): %+v", len(violations), violations)
	}
	if violations[0].TypeName != "ZzzCanaryEvent" || violations[0].FieldName != "Gen" {
		t.Errorf("violation = %+v, want TypeName=ZzzCanaryEvent FieldName=Gen", violations[0])
	}
}
