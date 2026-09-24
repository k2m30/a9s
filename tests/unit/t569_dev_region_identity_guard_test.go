package unit_test

// A resource's identity is its type, the Region it was read in and its ID.
// These guards read the production source so that the next payload, message
// or fold that names a resource without its Region, and the next checker that
// answers for another Region outside the one owner, fails here.

import (
	"go/ast"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

var (
	t569DevTypeFields = []string{"ResourceType", "TargetType", "ChildType", "SourceType"}
	t569DevIDFields   = []string{"ResourceID", "SourceResourceID", "SourceID", "ID", "TargetID", "Resource", "SourceResource", "ParentContext", "RelatedIDs", "Filter", "FetchFilter"}
	// t569DevRegionFromElsewhere are the identity structs whose Region is held
	// by something else: a detail's is its screen's ScreenContext, and a
	// rendered related block is a projection of the DetailRelatedRow Enter
	// navigates from.
	t569DevRegionFromElsewhere = []string{"DetailState", "RelatedBlock"}
)

func t569DevLitName(cl *ast.CompositeLit) string {
	switch x := cl.Type.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		return x.Sel.Name
	}
	return ""
}

func t569DevCallName(call *ast.CallExpr) string {
	switch f := call.Fun.(type) {
	case *ast.Ident:
		return f.Name
	case *ast.SelectorExpr:
		return f.Sel.Name
	}
	return ""
}

// TestT569Dev_EveryResourceIdentityCarriesItsRegion: a payload, message,
// intent or event that names a resource type and an ID names the Region too,
// and every literal of one that sets the ID sets the Region.
func TestT569Dev_EveryResourceIdentityCarriesItsRegion(t *testing.T) {
	fset, files := t569ParseProd(t, "core", "internal", "cmd")
	identity := map[string]bool{}
	for path, f := range files {
		switch filepath.Dir(path) {
		case "core/runtime", "core/runtime/messages", "core/app":
		default:
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			ts, ok := n.(*ast.TypeSpec)
			if !ok {
				return true
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				return true
			}
			var typed, named, region bool
			for _, field := range st.Fields.List {
				for _, name := range field.Names {
					typed = typed || slices.Contains(t569DevTypeFields, name.Name)
					named = named || slices.Contains(t569DevIDFields, name.Name)
					region = region || name.Name == "Region"
				}
			}
			if !typed || !named || slices.Contains(t569DevRegionFromElsewhere, ts.Name.Name) {
				return true
			}
			if !region {
				t.Errorf("%s: %s names a resource type and ID with no Region", fset.Position(ts.Pos()), ts.Name.Name)
			}
			identity[ts.Name.Name] = true
			return true
		})
	}
	if len(identity) < 20 {
		t.Fatalf("found %d identity structs; the scan no longer reaches the runtime payloads", len(identity))
	}
	for _, f := range files {
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil || (strings.HasPrefix(fn.Name.Name, "All") && strings.HasSuffix(fn.Name.Name, "Samples")) {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				cl, ok := n.(*ast.CompositeLit)
				if !ok || !identity[t569DevLitName(cl)] {
					return true
				}
				var ids []string
				region := false
				for _, e := range cl.Elts {
					kv, ok := e.(*ast.KeyValueExpr)
					if !ok {
						continue
					}
					key, _ := kv.Key.(*ast.Ident)
					if key == nil {
						continue
					}
					if slices.Contains(t569DevIDFields, key.Name) {
						ids = append(ids, key.Name)
					}
					region = region || key.Name == "Region"
				}
				if len(ids) > 0 && !region {
					t.Errorf("%s: %s{%s} names a resource without the Region it was read in", fset.Position(cl.Pos()), t569DevLitName(cl), strings.Join(ids, ", "))
				}
				return true
			})
		}
	}
}

// t569DevSessionRegionReaders are the functions that read the session's own
// Region because what they build is the session's: its header and identity,
// its connect, the Region picker, and the owners that turn a screen's Region
// into the session's when it names none.
var t569DevSessionRegionReaders = []string{
	"core/app/bootstrap.go:BootstrapLive",
	"core/app/navigate.go:applyNavResult",
	"core/app/detail_body.go:resolveNavIDs",
	"core/app/detail_state.go:localRegion",
	"core/app/detail_state.go:readRegionLocked",
	"core/app/snapshot.go:snapshot",
	"core/app/snapshot.go:consoleTargetFromRelatedRow",
	"core/app/snapshot.go:buildIdentityBody",
	"internal/tui/app.go:Init",
	"internal/tui/app_stack.go:updateActiveRS",
	"internal/tui/runtime_adapter_navigate.go:handleNavigate",
	"internal/tui/app_view.go:View",
}

// TestT569Dev_ScreensReadTheirOwnRegion: inside the controller and the TUI the
// session's Region is read only where the session's is what is meant. A
// screen reads its own Region through the controller (topRegionLocked,
// TopRegion), which falls back to the session's itself.
func TestT569Dev_ScreensReadTheirOwnRegion(t *testing.T) {
	fset, files := t569ParseProd(t, "core/app", "internal/tui")
	seen := map[string]bool{}
	for path, f := range files {
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			where := path + ":" + fn.Name.Name
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok || len(call.Args) != 0 {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "Region" {
					return true
				}
				recv, ok := sel.X.(*ast.SelectorExpr)
				if !ok || recv.Sel.Name != "core" {
					return true
				}
				seen[where] = true
				if !slices.Contains(t569DevSessionRegionReaders, where) {
					t.Errorf("%s: %s reads the session's Region; a screen's own is topRegionLocked/TopRegion", fset.Position(call.Pos()), where)
				}
				return true
			})
		}
	}
	for _, where := range t569DevSessionRegionReaders {
		if !seen[where] {
			t.Errorf("%s is allowed to read the session's Region but no longer does; drop it from the list", where)
		}
	}
}

func t569DevAWSFuncs(t *testing.T) (map[string]*ast.FuncDecl, map[string]string) {
	t.Helper()
	_, files := t569ParseProd(t, "core/aws")
	funcs, fileOf := map[string]*ast.FuncDecl{}, map[string]string{}
	for path, f := range files {
		if filepath.Dir(path) != "core/aws" {
			continue
		}
		for _, decl := range f.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Body != nil && fn.Recv == nil {
				funcs[fn.Name.Name], fileOf[fn.Name.Name] = fn, filepath.Base(path)
			}
		}
	}
	return funcs, fileOf
}

func t569DevCalls(fn *ast.FuncDecl) []*ast.CallExpr {
	var calls []*ast.CallExpr
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			calls = append(calls, call)
		}
		return true
	})
	return calls
}

// t569DevReaches is the set of functions that call, directly or through
// other core/aws functions, one of targets; a call to one of skip is not
// followed.
func t569DevReaches(funcs map[string]*ast.FuncDecl, targets, skip []string) map[string]bool {
	reach := map[string]bool{}
	for _, name := range targets {
		reach[name] = true
	}
	for changed := true; changed; {
		changed = false
		for name, fn := range funcs {
			if reach[name] {
				continue
			}
			for _, call := range t569DevCalls(fn) {
				callee := t569DevCallName(call)
				if reach[callee] && !slices.Contains(skip, callee) {
					reach[name], changed = true, true
					break
				}
			}
		}
	}
	return reach
}

// TestT569Dev_OtherRegionAnswersGoThroughTheOwner: a checker reads another
// Region only through related_shared.go's readers (related_fetch.go's list
// reads and ref_ids.go's reference resolution decide the same thing for a
// list and a reference) and answers for it only through regionalAnswer, the
// one place a result is marked with the Region it was read in; readOf, which
// keeps no Region, never unwraps such an answer.
func TestT569Dev_OtherRegionAnswersGoThroughTheOwner(t *testing.T) {
	funcs, fileOf := t569DevAWSFuncs(t)
	owners := []string{"related_shared.go", "related_fetch.go", "ref_ids.go"}
	private := []string{"relatedListIn", "refContextIn", "clientsIn", "inRegion", "elsewhere"}
	readers := []string{"readListIn", "refReadsIn", "kmsReads"}
	answers := []string{"regionalAnswer", "answerIn"}
	for name, fn := range funcs {
		var calls []string
		for _, call := range t569DevCalls(fn) {
			calls = append(calls, t569DevCallName(call))
		}
		for _, callee := range calls {
			if slices.Contains(private, callee) && !slices.Contains(owners, fileOf[name]) {
				t.Errorf("%s:%s calls %s; another Region is read through related_shared.go's readers", fileOf[name], name, callee)
			}
		}
		producer := false
		if res := fn.Type.Results; res != nil && len(res.List) == 1 {
			if m, ok := res.List[0].Type.(*ast.MapType); ok {
				if v, ok := m.Value.(*ast.Ident); ok && v.Name == "relatedRead" {
					producer = true
				}
			}
		}
		reads := slices.ContainsFunc(calls, func(c string) bool { return slices.Contains(readers, c) })
		answered := slices.ContainsFunc(calls, func(c string) bool { return slices.Contains(answers, c) })
		if reads && !producer && !answered {
			t.Errorf("%s:%s reads Regions but answers without regionalAnswer, so its row drills into none of them", fileOf[name], name)
		}
	}
	regional := t569DevReaches(funcs, append([]string{"relatedRefsByRegion"}, answers...), nil)
	for name, fn := range funcs {
		for _, call := range t569DevCalls(fn) {
			if t569DevCallName(call) != "readOf" || len(call.Args) != 1 {
				continue
			}
			if inner, ok := call.Args[0].(*ast.CallExpr); ok && regional[t569DevCallName(inner)] {
				t.Errorf("%s:%s unwraps %s with readOf, which drops the Region it was read in", fileOf[name], name, t569DevCallName(inner))
			}
		}
	}
	if !regional["checkCfACM"] || !regional["kmsRelated"] {
		t.Fatal("the scan no longer reaches the cross-Region checkers")
	}
}

// TestT569Dev_TargetPrefetchIsForSessionReadersOnly: NeedsTargetCache
// prefetches the session's list of the target and fails the row when that
// list cannot be read, so it belongs only on a pivot that counts in the
// session's Region. An alarm pivot counts where its source's metrics are
// published (MetricsRegion); any other pivot that lists a target in a Region
// it computes (readListIn) counts there.
func TestT569Dev_TargetPrefetchIsForSessionReadersOnly(t *testing.T) {
	checked := 0
	for _, td := range resource.AllResourceTypes() {
		for _, def := range td.Related {
			if def.TargetType != "alarm" || !def.NeedsTargetCache {
				continue
			}
			checked++
			if spec, ok := awsclient.AlarmMatchSpecFor(td.ShortName); ok && spec.MetricsRegion != nil {
				t.Errorf("%s -> alarm prefetches the session's alarms but counts where %s publishes its metrics", td.ShortName, td.ShortName)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no alarm pivot prefetches its target; the scan no longer reaches the catalog")
	}

	funcs, _ := t569DevAWSFuncs(t)
	elsewhere := t569DevReaches(funcs, []string{"readListIn"}, []string{"alarmIDsByDimension"})
	fset, files := t569ParseProd(t, "core/aws")
	for path, f := range files {
		if !strings.HasPrefix(filepath.Base(path), "catalog_") {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			cl, ok := n.(*ast.CompositeLit)
			if !ok {
				return true
			}
			var checker string
			prefetch := false
			for _, e := range cl.Elts {
				kv, ok := e.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				key, _ := kv.Key.(*ast.Ident)
				if key == nil {
					continue
				}
				switch key.Name {
				case "Checker":
					if id, ok := kv.Value.(*ast.Ident); ok {
						checker = id.Name
					}
				case "NeedsTargetCache":
					if v, ok := kv.Value.(*ast.Ident); ok && v.Name == "true" {
						prefetch = true
					}
				}
			}
			if prefetch && elsewhere[checker] {
				t.Errorf("%s: %s lists its target in a Region it computes, but NeedsTargetCache prefetches the session's", fset.Position(cl.Pos()), checker)
			}
			return true
		})
	}
}
