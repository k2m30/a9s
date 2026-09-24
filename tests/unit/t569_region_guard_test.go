package unit_test

// One place decides which Region a detail and a related read run in: the
// detail operation is begun with the Region its row was listed in, and no
// checker picks a Region's clients for itself. These guards read the
// production source so that the next detail entry point and the next
// cross-Region pivot fail here rather than read the session's Region.

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func t569ParseProd(t *testing.T, roots ...string) (*token.FileSet, map[string]*ast.File) {
	t.Helper()
	fset := token.NewFileSet()
	files := map[string]*ast.File{}
	for _, root := range roots {
		err := filepath.WalkDir(filepath.Join("../..", root), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			f, perr := parser.ParseFile(fset, path, nil, 0)
			if perr != nil {
				return perr
			}
			rel, _ := filepath.Rel("../..", path) //nolint:errcheck // path is under ../..
			files[filepath.ToSlash(rel)] = f
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
	return fset, files
}

func t569Source(fset *token.FileSet, n ast.Node) string {
	var b bytes.Buffer
	_ = printer.Fprint(&b, fset, n) //nolint:errcheck // printing an in-memory AST node does not fail
	return b.String()
}

var t569RegionWord = regexp.MustCompile(`(?i)region`)

// TestT569Guard_DetailOperationCarriesTheRowsRegion: every call that begins a
// detail operation hands it the Region of the row it opens, and no detail
// operation is assembled anywhere but where it is begun.
func TestT569Guard_DetailOperationCarriesTheRowsRegion(t *testing.T) {
	fset, files := t569ParseProd(t, "core", "internal", "cmd")
	calls := 0
	var bad []string
	for path, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.CallExpr:
				sel, ok := x.Fun.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "BeginDetailOperation" {
					return true
				}
				calls++
				carries := false
				for _, arg := range x.Args {
					if t569RegionWord.MatchString(t569Source(fset, arg)) {
						carries = true
					}
				}
				if !carries {
					bad = append(bad, fset.Position(x.Pos()).String()+": "+t569Source(fset, x)+" begins a detail operation without the row's Region")
				}
			case *ast.CompositeLit:
				name := ""
				switch typ := x.Type.(type) {
				case *ast.Ident:
					name = typ.Name
				case *ast.SelectorExpr:
					name = typ.Sel.Name
				}
				if name == "DetailOperation" && path != "core/runtime/detail_op.go" {
					bad = append(bad, fset.Position(x.Pos()).String()+": a DetailOperation assembled outside BeginDetailOperation")
				}
			}
			return true
		})
	}
	if calls == 0 {
		t.Fatal("no BeginDetailOperation call found; the scan no longer reaches the detail entry points")
	}
	for _, b := range bad {
		t.Error(b)
	}
}

// TestT569Guard_NoCheckerPicksARegionClient: a checker reads another Region
// through the owner of cross-Region reads (related_shared.go) or through a
// reference's Region (ref_ids.go), never by calling InRegion itself.
func TestT569Guard_NoCheckerPicksARegionClient(t *testing.T) {
	fset, files := t569ParseProd(t, "core/aws")
	found := 0
	var bad []string
	for path, f := range files {
		base := filepath.Base(path)
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			checkerSide := (strings.Contains(base, "_related") && base != "related_shared.go") || strings.HasPrefix(fn.Name.Name, "check")
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "InRegion" {
					return true
				}
				found++
				if checkerSide {
					bad = append(bad, fset.Position(call.Pos()).String()+": "+fn.Name.Name+" picks a Region client with "+t569Source(fset, call))
				}
				return true
			})
		}
	}
	if found == 0 {
		t.Fatal("no InRegion call found in core/aws; the scan no longer reaches the Region owner")
	}
	for _, b := range bad {
		t.Error(b)
	}
}
