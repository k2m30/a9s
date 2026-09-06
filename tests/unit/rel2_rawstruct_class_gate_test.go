package unit_test

// rel2_rawstruct_class_gate_test.go — row 7's class gate: a checker that reads
// its source row's RawStruct, handed a row that carries none, must answer "?".
//
// The on-disk cache carries no RawStruct by design (core/cache/cache.go), and
// the detail operation runs the checkers against the restored row before
// enrichment refills it. A checker that needs the source struct and has none
// never scanned anything, so it knows nothing — and a confident "0" is the one
// answer the operator has no way to doubt.
//
// The gate is scoped to the checkers that actually read RawStruct, derived here
// from the compiled function names rather than taken from a list, because a
// checker that scans the target cache by a Fields value the cache preserves
// answers honestly on a warm row and is none of this rule's business.

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// rel2RawStructReaders returns the names of every function in core/aws that
// reads the SOURCE row's RawStruct — either as res.RawStruct in its own body,
// or by handing res to a function that does. A function that asserts a row of
// the TARGET list is deliberately not a reader: that row came from the cache
// the checker was given, and its shape is the truncation rule's business.
func rel2RawStructReaders(t *testing.T) map[string]bool {
	t.Helper()
	files, err := filepath.Glob("../../core/aws/*.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("no core/aws files found: %v", err)
	}

	fset := token.NewFileSet()
	// bodies maps a function name to its source text; calls maps it to the
	// names it calls, so a read one hop away still counts.
	bodies := map[string]string{}
	calls := map[string][]string{}

	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		src, err := os.ReadFile(path) //nolint:gosec // a glob of this repo's own source
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		file, err := parser.ParseFile(fset, path, src, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil || fn.Recv != nil {
				continue
			}
			start := fset.Position(fn.Body.Pos()).Offset
			end := fset.Position(fn.Body.End()).Offset
			bodies[fn.Name.Name] = string(src[start:end])
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				id, ok := call.Fun.(*ast.Ident)
				if !ok {
					return true
				}
				// Only a call that hands the helper the SOURCE row passes the
				// read along. A helper reading a target list row's struct is a
				// different question and none of this rule's business.
				for _, arg := range call.Args {
					if argID, ok := arg.(*ast.Ident); ok && argID.Name == "res" {
						calls[fn.Name.Name] = append(calls[fn.Name.Name], id.Name)
						break
					}
				}
				return true
			})
		}
	}

	direct := map[string]bool{}
	for name, body := range bodies {
		if strings.Contains(body, "res.RawStruct") {
			direct[name] = true
		}
	}
	// One fixed point: a function that calls a direct reader reads it too.
	for changed := true; changed; {
		changed = false
		for name, callees := range calls {
			if direct[name] {
				continue
			}
			for _, callee := range callees {
				if direct[callee] {
					direct[name] = true
					changed = true
					break
				}
			}
		}
	}
	return direct
}

// rel2CheckerFuncName returns the compiled name of a checker, e.g.
// "checkAlarmASG", so it can be matched against the source scan.
func rel2CheckerFuncName(c resource.RelatedChecker) string {
	full := runtime.FuncForPC(reflect.ValueOf(c).Pointer()).Name()
	if i := strings.LastIndex(full, "."); i >= 0 {
		full = full[i+1:]
	}
	return strings.TrimSuffix(full, "-fm")
}

// TestRel2WarmRowNeverGetsAConfidentZero is row 7's class gate. Every checker
// that reads its source row's RawStruct is handed a real demo row of its own
// type with RawStruct stripped — the exact shape a disk-cache replay produces —
// against the full demo target cache. A resolved zero is the failure: the
// checker never read the row, so it cannot know there are none. Resolving a
// real count from a Fields value the cache preserved is fine, and so is "?".
func TestRel2WarmRowNeverGetsAConfidentZero(t *testing.T) {
	readers := rel2RawStructReaders(t)
	if len(readers) < 50 {
		t.Fatalf("only %d RawStruct-reading functions found; the source scan is broken", len(readers))
	}

	clients := demo.NewServiceClients()
	cache := resource.ResourceCache{}
	rowsFor := map[string][]resource.Resource{}
	load := func(shortName string) []resource.Resource {
		if rows, done := rowsFor[shortName]; done {
			return rows
		}
		var rows []resource.Resource
		if td := resource.FindResourceType(shortName); td != nil && td.Fetcher != nil {
			// A partial page is still a usable list; several demo fetchers
			// deliberately fail one id.
			out, _ := td.Fetcher(context.Background(), clients, "") //nolint:errcheck // partial results are the point
			rows = out.Resources
		}
		rowsFor[shortName] = rows
		cache[shortName] = resource.ResourceCacheEntry{Resources: rows}
		return rows
	}

	var offenders []string
	for _, td := range resource.AllResourceTypes() {
		defs := resource.GetRelated(td.ShortName)
		if len(defs) == 0 {
			continue
		}
		sourceRows := load(td.ShortName)
		if len(sourceRows) == 0 {
			continue
		}
		for _, def := range defs {
			if def.Checker == nil || !readers[rel2CheckerFuncName(def.Checker)] {
				continue
			}
			if def.TargetType != "" {
				load(def.TargetType)
			}
			// Every source row, because the offending branch may be reachable
			// from only some of them.
			flagged := false
			for _, row := range sourceRows {
				if flagged {
					break
				}
				warm := row
				warm.RawStruct = nil
				warmResult := def.Checker(context.Background(), clients, warm, cache)
				if warmResult.State() != domain.RelatedResolved || warmResult.Count() != 0 {
					continue
				}
				// The control: the same checker on the same row with its struct
				// present. If that also answers zero, the zero is a fact about
				// the row and reading it changed nothing. Only a zero that the
				// unread row invented is a defect.
				readResult := def.Checker(context.Background(), clients, row, cache)
				if readResult.State() == domain.RelatedResolved && readResult.Count() == 0 {
					continue
				}
				offenders = append(offenders, td.ShortName+"→"+def.TargetType+
					" ("+rel2CheckerFuncName(def.Checker)+", row "+row.ID+
					": read → "+readResult.State().String()+" "+strconv.Itoa(readResult.Count())+
					", unread → resolved 0)")
				flagged = true
			}
		}
	}

	if len(offenders) > 0 {
		sort.Strings(offenders)
		t.Errorf("%d checker(s) answer a confident zero for a source row they never read. "+
			"A disk-cache replay carries no RawStruct, so this is what the panel shows on a warm "+
			"start. Guard the failure branch on res.RawStruct == nil and answer Unknown:\n  %s",
			len(offenders), strings.Join(offenders, "\n  "))
	}
}
