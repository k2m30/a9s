package unit_test

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type t9bTextLanes struct {
	coreYAML, coreJSON []string
	tuiYAML, tuiJSON   []string
}

func t9bLanes(t *testing.T, d *t9bTUI, row t9bRow) t9bTextLanes {
	t.Helper()
	var l t9bTextLanes
	l.coreYAML, l.coreJSON = t9bCoreText(t, row.res, row.typ)
	l.tuiYAML = d.textOf(row.res, row.typ, "y", t9bScreenWidth(l.coreYAML), len(l.coreYAML)+12)
	l.tuiJSON = d.textOf(row.res, row.typ, "J", t9bScreenWidth(l.coreJSON), len(l.coreJSON)+12)
	return l
}

func t9bScreenWidth(lines []string) int {
	w := 120
	for _, l := range lines {
		if n := len([]rune(l)) + 8; n > w {
			w = n
		}
	}
	return w
}

func t9bFindRow(t *testing.T, rows []t9bRow, typ, needle string) t9bRow {
	t.Helper()
	for _, r := range rows {
		if r.typ == typ && (strings.Contains(r.res.ID, needle) || strings.Contains(r.res.Fields["arn"], needle)) {
			return r
		}
	}
	t.Fatalf("no demo %s row matches %q", typ, needle)
	return t9bRow{}
}

func t9bUnquote(lines []string) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = strings.ReplaceAll(l, `"`, "")
	}
	return out
}

// A KMS key's KeyMetadata.Enabled is a plain bool and an ECS service's
// DesiredCount and RunningCount are plain int32 (kms/types KeyMetadata,
// ecs/types Service): AWS always states them, so a disabled key and a service
// scaled to zero carry false and 0 as facts, and the YAML screen shows them on
// both lanes. Quoting of the scalar is not part of the fact.
func TestT9b_Row5_YAMLKeepsFalseAndZero(t *testing.T) {
	rows := t9bDemoRows(t)
	d := newT9bTUI(t)

	disabled := t9bLanes(t, d, t9bFindRow(t, rows, "kms", "c3d4e5f6-7890-12ab-cdef-333333333333"))
	scaledToZero := t9bLanes(t, d, t9bFindRow(t, rows, "ecs-svc", "acme-svc-inactive"))
	running := t9bLanes(t, d, t9bFindRow(t, rows, "ecs-svc", "/api-gateway"))

	for lane, lines := range map[string][]string{"core": disabled.coreYAML, "tui": disabled.tuiYAML} {
		if !t9bHasLine(t9bUnquote(lines), "Enabled: false") {
			t.Errorf("kms disabled key, %s YAML: no line \"Enabled: false\":\n%s", lane, strings.Join(lines, "\n"))
		}
	}
	for lane, lines := range map[string][]string{"core": scaledToZero.coreYAML, "tui": scaledToZero.tuiYAML} {
		for _, want := range []string{"DesiredCount: 0", "RunningCount: 0"} {
			if !t9bHasLine(t9bUnquote(lines), want) {
				t.Errorf("ecs-svc acme-svc-inactive, %s YAML: no line %q:\n%s", lane, want, strings.Join(lines, "\n"))
			}
		}
	}
	for lane, lines := range map[string][]string{"core": running.coreYAML, "tui": running.tuiYAML} {
		for _, want := range []string{"DesiredCount: 4", "RunningCount: 4"} {
			if !t9bHasLine(t9bUnquote(lines), want) {
				t.Errorf("ecs-svc api-gateway, %s YAML: no line %q:\n%s", lane, want, strings.Join(lines, "\n"))
			}
		}
		if t9bHasLine(t9bUnquote(lines), "DesiredCount: 0") {
			t.Errorf("ecs-svc api-gateway, %s YAML shows DesiredCount: 0 for a service that wants 4 tasks", lane)
		}
	}
}

// The TUI and the web client show one text screen per resource: the lines
// core builds. YAML and JSON describe the same struct, so they name the same
// fields; only a nil or empty value may be left out of one of them.
func TestT9b_Row6_TextScreensSameInBothLanes(t *testing.T) {
	rows := t9bDemoRows(t)
	d := newT9bTUI(t)

	var laneDiffs, keyDiffs []string
	for _, row := range rows {
		l := t9bLanes(t, d, row)
		id := row.typ + " " + row.res.ID
		if got, want := strings.Join(l.tuiYAML, "\n"), strings.Join(l.coreYAML, "\n"); got != want {
			laneDiffs = append(laneDiffs, fmt.Sprintf("%s YAML: tui %d lines, core %d lines; first difference: %s", id, len(l.tuiYAML), len(l.coreYAML), t9bFirstDiff(l.tuiYAML, l.coreYAML)))
		}
		if got, want := strings.Join(l.tuiJSON, "\n"), strings.Join(l.coreJSON, "\n"); got != want {
			laneDiffs = append(laneDiffs, fmt.Sprintf("%s JSON: tui %d lines, core %d lines; first difference: %s", id, len(l.tuiJSON), len(l.coreJSON), t9bFirstDiff(l.tuiJSON, l.coreJSON)))
		}

		var yv, jv any
		if err := yaml.Unmarshal([]byte(strings.Join(l.coreYAML, "\n")), &yv); err != nil {
			t.Errorf("%s: core YAML does not parse: %v", id, err)
			continue
		}
		if err := json.Unmarshal([]byte(strings.Join(l.coreJSON, "\n")), &jv); err != nil {
			if row.res.RawStruct != nil || len(row.res.Fields) > 0 {
				t.Errorf("%s: core JSON does not parse: %v", id, err)
			}
			continue
		}
		var diffs []string
		t9bDiffKeys("", jv, yv, &diffs)
		for _, df := range diffs {
			keyDiffs = append(keyDiffs, id+": "+df)
		}
	}
	t9bReport(t, "rows whose TUI text screen differs from the lines core built", laneDiffs)
	t9bReport(t, "fields present in one of YAML and JSON but not the other", keyDiffs)
}

func t9bReport(t *testing.T, what string, items []string) {
	t.Helper()
	if len(items) == 0 {
		return
	}
	sort.Strings(items)
	shown := items
	if len(shown) > 40 {
		shown = shown[:40]
	}
	t.Errorf("%d %s:\n  %s", len(items), what, strings.Join(shown, "\n  "))
}

func t9bFirstDiff(a, b []string) string {
	for i := 0; i < len(a) || i < len(b); i++ {
		var x, y string
		if i < len(a) {
			x = a[i]
		}
		if i < len(b) {
			y = b[i]
		}
		if x != y {
			return fmt.Sprintf("line %d tui=%q core=%q", i+1, x, y)
		}
	}
	return "none"
}

func t9bEmpty(v any) bool {
	switch x := v.(type) {
	case nil:
		return true
	case string:
		return x == ""
	case map[string]any:
		for _, e := range x {
			if !t9bEmpty(e) {
				return false
			}
		}
		return true
	case []any:
		for _, e := range x {
			if !t9bEmpty(e) {
				return false
			}
		}
		return true
	}
	return false
}

// t9bDiffKeys walks the JSON and YAML documents of one resource together and
// records each non-empty field one names and the other does not. Where the two
// give one field different shapes (a tag list YAML shows as a map, a JSON
// document held in a string), only the field's presence is compared.
func t9bDiffKeys(path string, j, y any, diffs *[]string) {
	jm, jok := j.(map[string]any)
	ym, yok := y.(map[string]any)
	if jok && yok {
		for k, jv := range jm {
			if t9bEmpty(jv) {
				continue
			}
			yv, ok := ym[k]
			if !ok || t9bEmpty(yv) {
				*diffs = append(*diffs, fmt.Sprintf("%s.%s=%v only in JSON", path, k, jv))
				continue
			}
			t9bDiffKeys(path+"."+k, jv, yv, diffs)
		}
		for k, yv := range ym {
			if t9bEmpty(yv) {
				continue
			}
			if jv, ok := jm[k]; !ok || t9bEmpty(jv) {
				*diffs = append(*diffs, fmt.Sprintf("%s.%s only in YAML", path, k))
			}
		}
		return
	}
	js, jok := j.([]any)
	ys, yok := y.([]any)
	if jok && yok && len(js) == len(ys) {
		for i := range js {
			t9bDiffKeys(fmt.Sprintf("%s[%d]", path, i), js[i], ys[i], diffs)
		}
	}
}

func t9bRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate the test file")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

// A resource's YAML and JSON text has one builder each, in core; every
// renderer shows the lines they build. In the view layers (core/app, core/web
// and the renderers under internal/) a function that marshals and reads a
// resource's RawStruct or Fields (or converts one through ToSafeValue) is such
// a builder, so core holds at most one per format and internal/ holds none.
func TestT9b_Guard_OneBuilderPerTextFormat(t *testing.T) {
	root := t9bRepoRoot(t)
	builders := map[string][]string{}
	for _, dir := range []string{filepath.Join("core", "app"), filepath.Join("core", "web"), "internal"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(p string, de fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if de.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
				return nil
			}
			src, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			f, err := parser.ParseFile(token.NewFileSet(), p, src, 0)
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(root, p) //nolint:errcheck // p is under root
			for _, decl := range f.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				formats, readsResource := t9bMarshalScan(fn.Body)
				if !readsResource {
					continue
				}
				for format := range formats {
					builders[format] = append(builders[format], rel+":"+fn.Name.Name)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("scan %s: %v", dir, err)
		}
	}
	for _, format := range []string{"yaml", "json"} {
		sites := builders[format]
		sort.Strings(sites)
		var outsideCore []string
		for _, s := range sites {
			if !strings.HasPrefix(s, "core"+string(filepath.Separator)) {
				outsideCore = append(outsideCore, s)
			}
		}
		if len(sites) > 1 || len(outsideCore) > 0 {
			t.Errorf("%s text of a resource is built in %d functions, want one in core:\n  %s", format, len(sites), strings.Join(sites, "\n  "))
		}
	}
}

func t9bMarshalScan(body *ast.BlockStmt) (map[string]bool, bool) {
	formats := map[string]bool{}
	reads := false
	ast.Inspect(body, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		switch sel.Sel.Name {
		case "RawStruct", "ToSafeValue", "Fields":
			reads = true
		case "Marshal", "MarshalIndent", "NewEncoder":
			if pkg, ok := sel.X.(*ast.Ident); ok && (pkg.Name == "yaml" || pkg.Name == "json") {
				formats[pkg.Name] = true
			}
		}
		return true
	})
	return formats, reads
}
