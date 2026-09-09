// qa_one_type_registry_test.go — one registry for resource types, and no
// production read of a test-only one.
//
// A test-only override map is safe while only tests touch it. It stops being
// safe the moment a production read path reaches it, because the production
// reader runs on whatever goroutine its caller is on while the writer runs on
// a test goroutine, and nothing in a test-only registry is guarded: the write
// is a plain map assignment and the read a plain map access. The two are then
// a data race in every test binary that does both, which is every one of them
// once a shared read path is involved.
//
// The rule is the registry, not the lock: types live in the catalog, test
// registration goes through the catalog under the same guard the catalog's own
// readers use, and no package keeps a parallel map beside it.
package unit_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/session"
)

// raceProbeChildName is the type this pin registers while the readers run.
const raceProbeChildName = "qa-race-probe-child"

// TestTypeLookupIsSafeBesideATestRegistration runs the two production readers
// the acceptance trace names — awsclient.Wave2EnricherFor directly, and
// runtime.Core.HasIssueEnricher, which is what reaches it from the cache-writer
// goroutine — against a concurrent stream of test registrations.
//
// Meaningful under -race only; without it the read and the write simply
// interleave and nothing reports. `make test-race` is where this pin speaks,
// and it is the same shape as the failure acceptance reproduced 8 times out of
// 8: SetChildTypeForTest writing the map while HasIssueEnricher reads it.
func TestTypeLookupIsSafeBesideATestRegistration(t *testing.T) {
	core := runtime.New(session.New(), nil)

	const rounds = 200
	var wg sync.WaitGroup
	wg.Add(3)

	go func() {
		defer wg.Done()
		for i := 0; i < rounds; i++ {
			resource.SetChildTypeForTest(resource.ResourceTypeDef{
				Name:      "QA Race Probe Child",
				ShortName: raceProbeChildName,
				Category:  "COMPUTE",
			})
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < rounds; i++ {
			awsclient.Wave2EnricherFor("ec2")
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < rounds; i++ {
			core.HasIssueEnricher("ec2")
		}
	}()

	wg.Wait()
	resource.CleanupChildTypeForTest(raceProbeChildName)
}

// productionRoots are the trees whose non-test files are production code. A
// file named *_test.go is excluded wherever it sits; a helper file that merely
// declares a test seam is production and stays in scope, because declaring the
// seam is not what makes it unsafe — reading it from a production path is.
var productionRoots = []string{"../../core", "../../internal", "../../cmd"}

// TestCoreResourceKeepsNoParallelTypeRegistry holds the ARCH rule directly:
// the catalog is the registry, and core/resource keeps no map of its own
// beside it. A second registry is a second answer to "what types exist", and
// the readers that consult one and not the other were the bug this rule
// closes — a child type's declaration that half the getters could not see.
func TestCoreResourceKeepsNoParallelTypeRegistry(t *testing.T) {
	fset := token.NewFileSet()
	pkg, err := parser.ParseDir(fset, "../../core/resource", func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatalf("parsing core/resource: %v", err)
	}

	var found []string
	for _, p := range pkg {
		for path, file := range p.Files {
			for _, decl := range file.Decls {
				gd, ok := decl.(*ast.GenDecl)
				if !ok || gd.Tok != token.VAR {
					continue
				}
				for _, spec := range gd.Specs {
					vs, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}
					for _, name := range vs.Names {
						if name.Name != "childTypes" {
							continue
						}
						found = append(found, fset.Position(name.Pos()).String()+" "+path)
					}
				}
			}
		}
	}

	if len(found) > 0 {
		sort.Strings(found)
		t.Errorf("core/resource still declares the parallel child-type registry "+
			"childTypes at:\n  %s\nEvery production reader that resolves a type name "+
			"reaches this map through resource.TypeDef, and its only writers are the "+
			"test seams, so an unguarded map write on a test goroutine races an "+
			"unguarded read on whichever goroutine the production caller is on. The "+
			"types belong in the catalog, with test registration going through the "+
			"catalog under the same guard its own readers use.",
			strings.Join(found, "\n  "))
	}
}

// TestNoProductionCodeCallsATestSeam is the standing half of the rule: a test
// seam may be declared in a production package (that is where the symbol it
// overrides lives) but never called from one. A production call site is the
// point at which the seam's unguarded state joins a production code path,
// which is the shape of the defect this batch is closing.
func TestNoProductionCodeCallsATestSeam(t *testing.T) {
	var offenders []string

	for _, root := range productionRoots {
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			fset := token.NewFileSet()
			file, perr := parser.ParseFile(fset, path, nil, 0)
			if perr != nil {
				return perr
			}
			// Only call expressions count, so declaring SetChildTypeForTest is
			// fine and calling it is not. A seam delegating to another seam is
			// still the seam, so calls made from inside a *ForTest function are
			// not production call sites.
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || strings.HasSuffix(fn.Name.Name, "ForTest") {
					continue
				}
				ast.Inspect(fn, func(n ast.Node) bool {
					call, ok := n.(*ast.CallExpr)
					if !ok {
						return true
					}
					var name string
					switch callee := call.Fun.(type) {
					case *ast.Ident:
						name = callee.Name
					case *ast.SelectorExpr:
						name = callee.Sel.Name
					default:
						return true
					}
					if strings.HasSuffix(name, "ForTest") {
						offenders = append(offenders, fset.Position(call.Pos()).String()+
							": "+fn.Name.Name+" calls "+name)
					}
					return true
				})
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walking %s: %v", root, err)
		}
	}

	if len(offenders) > 0 {
		sort.Strings(offenders)
		t.Errorf("%d production call site(s) reach a test-only seam. The seam's state "+
			"is written from test goroutines and guarded by nothing, so a production "+
			"reader of it races every test that sets it:\n  %s",
			len(offenders), strings.Join(offenders, "\n  "))
	}
}
