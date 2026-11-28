// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit

// A generated view file records what was generated. GenerateViewYAML(v) is the
// file a build whose defaults were v writes, so a file generated from a view
// that differs from this build's defaults is the file an earlier build left on
// the disk. The resolved view merges that file with the running build per item:
// an item nobody edited follows the build, an item the operator changed, added
// or removed stays the way the operator left it.

import (
	"bytes"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/config"
)

const (
	t9aOperatorLabel = "Operator Label"
	t9aEarlierLabel  = "Earlier Label"
)

var (
	t9aRetiredField  = config.DetailField{Path: "RetiredDetailPath"}
	t9aOperatorField = config.DetailField{Path: "OperatorDetailPath"}
)

// t9aSparePath is a scalar field of a struct the type stores that no default
// column reads: a path an operator may add as a column, and one the load
// accepts, since a path no stored struct has is reported.
func t9aSparePath(name string) string {
	td := catalog.FindAny(name)
	if td == nil {
		panic("no catalog type " + name)
	}
	used := map[string]bool{}
	for _, c := range config.DefaultViewDef(name).List {
		used[c.Path] = true
	}
	for _, rt := range td.RowTypes {
		for rt.Kind() == reflect.Pointer {
			rt = rt.Elem()
		}
		if rt.Kind() != reflect.Struct {
			continue
		}
		for f := range rt.Fields() {
			if f.IsExported() && !used[f.Name] && td.CellPath(f.Name) {
				return f.Name
			}
			ft := f.Type
			for ft.Kind() == reflect.Pointer {
				ft = ft.Elem()
			}
			if !f.IsExported() || ft.Kind() != reflect.Struct {
				continue
			}
			for sf := range ft.Fields() {
				if p := f.Name + "." + sf.Name; !used[p] && td.CellPath(p) {
					return p
				}
			}
		}
	}
	panic(name + " stores no scalar field outside its default columns")
}

func t9aRetiredColumn(name string) config.ListColumn {
	return config.ListColumn{Title: "Retired Column", Path: t9aSparePath(name), Width: 14}
}

func t9aOperatorColumn(name string) config.ListColumn {
	return config.ListColumn{Title: "Operator Note", Path: t9aSparePath(name), Width: 16}
}

func t9aListBlock(cols []config.ListColumn) string {
	var b strings.Builder
	b.WriteString("list:\n")
	for _, c := range cols {
		fmt.Fprintf(&b, "  %q:\n", c.Title)
		if c.Path != "" {
			fmt.Fprintf(&b, "    path: %s\n", c.Path)
		}
		if c.Key != "" {
			fmt.Fprintf(&b, "    key: %s\n", c.Key)
		}
		fmt.Fprintf(&b, "    width: %d\n", c.Width)
		if c.SortKey != "" {
			fmt.Fprintf(&b, "    sort_key: %s\n", c.SortKey)
		}
	}
	return b.String()
}

func t9aDetailBlock(fields []config.DetailField) string {
	var b strings.Builder
	b.WriteString("detail:\n")
	for _, f := range fields {
		switch {
		case f.Key != "" && f.Label != "":
			fmt.Fprintf(&b, "  - { key: %s, label: %q }\n", f.Key, f.Label)
		case f.Key != "":
			fmt.Fprintf(&b, "  - { key: %s }\n", f.Key)
		case f.Label != "":
			fmt.Fprintf(&b, "  - { path: %s, label: %q }\n", f.Path, f.Label)
		default:
			fmt.Fprintf(&b, "  - %s\n", f.Path)
		}
	}
	return b.String()
}

// t9aLegacyYAML is a file with an optional stamp and no record of what was
// generated: an earlier build's file when stamped, an operator's own when not.
func t9aLegacyYAML(stamp int, v config.ViewDef) []byte {
	var b strings.Builder
	if stamp > 0 {
		fmt.Fprintf(&b, "generated: %d\n", stamp)
	}
	if len(v.List) > 0 {
		b.WriteString(t9aListBlock(v.List))
	}
	if len(v.Detail) > 0 {
		if len(v.List) > 0 {
			b.WriteString("\n")
		}
		b.WriteString(t9aDetailBlock(v.Detail))
	}
	return []byte(b.String())
}

func t9aWrite(t *testing.T, dir, name string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".yaml"), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func t9aRead(t *testing.T, dir, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, name+".yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func t9aResolve(t *testing.T, name string, dirs ...string) config.ViewDef {
	t.Helper()
	cfg, err := config.LoadFromDirs(dirs)
	if err != nil {
		t.Fatalf("LoadFromDirs: %v", err)
	}
	return config.GetViewDef(cfg, name)
}

func t9aDefaults(t *testing.T, name string) config.ViewDef {
	t.Helper()
	def := config.DefaultViewDef(name)
	if len(def.List) < 4 || len(def.Detail) < 4 {
		t.Fatalf("%s: defaults have %d columns and %d detail rows, the test needs four of each", name, len(def.List), len(def.Detail))
	}
	return def
}

func t9aCols(cols []config.ListColumn) string {
	s := make([]string, len(cols))
	for i, c := range cols {
		s[i] = fmt.Sprintf("%s{path=%s key=%s width=%d sort=%s}", c.Title, c.Path, c.Key, c.Width, c.SortKey)
	}
	return strings.Join(s, ", ")
}

func t9aFields(fields []config.DetailField) string {
	s := make([]string, len(fields))
	for i, f := range fields {
		s[i] = fmt.Sprintf("{path=%s key=%s label=%s}", f.Path, f.Key, f.Label)
	}
	return strings.Join(s, ", ")
}

func t9aViewDiff(got, want config.ViewDef) string {
	var b strings.Builder
	if g, w := t9aCols(got.List), t9aCols(want.List); g != w {
		fmt.Fprintf(&b, "list\n got: %s\nwant: %s\n", g, w)
	}
	if g, w := t9aFields(got.Detail), t9aFields(want.Detail); g != w {
		fmt.Fprintf(&b, "detail\n got: %s\nwant: %s\n", g, w)
	}
	return b.String()
}

func t9aRequireView(t *testing.T, what string, got, want config.ViewDef) {
	t.Helper()
	if d := t9aViewDiff(got, want); d != "" {
		t.Errorf("%s:\n%s", what, d)
	}
}

func t9aCopyView(v config.ViewDef) config.ViewDef {
	return config.ViewDef{List: slices.Clone(v.List), Detail: slices.Clone(v.Detail)}
}

// t9aEarlierDefaults are the ways a build can differ from the one that wrote
// a file: a detail path it gained, a column it removed, a width or a source it
// corrected.
func t9aEarlierDefaults(name string, def config.ViewDef) map[string]config.ViewDef {
	out := map[string]config.ViewDef{}

	detail := t9aCopyView(def)
	if len(def.Detail) > 0 {
		detail.Detail = def.Detail[:len(def.Detail)-1]
	} else {
		detail.Detail = []config.DetailField{t9aRetiredField}
	}
	out["build gained a detail path"] = detail

	removed := t9aCopyView(def)
	removed.List = append(removed.List, t9aRetiredColumn(name))
	out["build removed a column"] = removed

	if len(def.List) > 0 {
		width := t9aCopyView(def)
		width.List[0].Width += 7
		out["build changed a width"] = width
	}

	if i := slices.IndexFunc(def.List, func(c config.ListColumn) bool { return c.Path != "" }); i >= 0 {
		path := t9aCopyView(def)
		path.List[i].Path += "Before"
		out["build changed a path"] = path
	}
	return out
}

// TestT9a_UneditedGeneratedFileFollowsTheBuild changes the defaults of every
// view the build ships, list and detail, and requires the file an earlier
// build generated and nobody edited to resolve to this build's view.
func TestT9a_UneditedGeneratedFileFollowsTheBuild(t *testing.T) {
	names := slices.Sorted(maps.Keys(config.DefaultConfig().Views))
	if len(names) < 50 {
		t.Fatalf("only %d built-in views", len(names))
	}

	for _, name := range names {
		def := config.DefaultViewDef(name)
		changes := t9aEarlierDefaults(name, def)
		for _, change := range slices.Sorted(maps.Keys(changes)) {
			t.Run(name+"/"+change, func(t *testing.T) {
				dir := t.TempDir()
				t9aWrite(t, dir, name, config.GenerateViewYAML(changes[change]))
				t9aRequireView(t, name, t9aResolve(t, name, dir), def)
			})
		}
	}
}

// The operator's edits are made on the parsed YAML of the generated file, so
// whatever the generator records beside an item stays as written and only the
// edited item changes.

func t9aParseNode(t *testing.T, data []byte) *yaml.Node {
	t.Helper()
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parsing a generated file: %v\n%s", err, data)
	}
	if len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		t.Fatalf("a generated file is not one YAML mapping:\n%s", data)
	}
	return doc.Content[0]
}

func t9aEncode(t *testing.T, root *yaml.Node, indent int) []byte {
	t.Helper()
	var b bytes.Buffer
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(indent)
	if err := enc.Encode(root); err != nil {
		t.Fatal(err)
	}
	if err := enc.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func t9aKeyIndex(m *yaml.Node, key string) int {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return i
		}
	}
	return -1
}

func t9aValue(t *testing.T, m *yaml.Node, key string) *yaml.Node {
	t.Helper()
	i := t9aKeyIndex(m, key)
	if i < 0 {
		t.Fatalf("no %q in the YAML mapping", key)
	}
	return m.Content[i+1]
}

func t9aScalar(v string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Value: v}
}

func t9aColumnPair(c config.ListColumn) (*yaml.Node, *yaml.Node) {
	v := &yaml.Node{Kind: yaml.MappingNode}
	if c.Path != "" {
		v.Content = append(v.Content, t9aScalar("path"), t9aScalar(c.Path))
	}
	if c.Key != "" {
		v.Content = append(v.Content, t9aScalar("key"), t9aScalar(c.Key))
	}
	v.Content = append(v.Content, t9aScalar("width"), t9aScalar(strconv.Itoa(c.Width)))
	return t9aScalar(c.Title), v
}

func t9aSetWidth(t *testing.T, root *yaml.Node, title string, width int) {
	t.Helper()
	col := t9aValue(t, t9aValue(t, root, "list"), title)
	t9aValue(t, col, "width").Value = strconv.Itoa(width)
}

func t9aRemoveColumn(t *testing.T, root *yaml.Node, title string) {
	t.Helper()
	list := t9aValue(t, root, "list")
	i := t9aKeyIndex(list, title)
	if i < 0 {
		t.Fatalf("no column %q to remove", title)
	}
	list.Content = slices.Delete(list.Content, i, i+2)
}

func t9aInsertColumnAfter(t *testing.T, root *yaml.Node, after string, c config.ListColumn) {
	t.Helper()
	list := t9aValue(t, root, "list")
	i := t9aKeyIndex(list, after)
	if i < 0 {
		t.Fatalf("no column %q to insert after", after)
	}
	k, v := t9aColumnPair(c)
	list.Content = slices.Insert(list.Content, i+2, k, v)
}

func t9aReorderColumns(t *testing.T, root *yaml.Node, titles []string) {
	t.Helper()
	list := t9aValue(t, root, "list")
	var out []*yaml.Node
	for _, title := range titles {
		i := t9aKeyIndex(list, title)
		if i < 0 {
			t.Fatalf("no column %q to reorder", title)
		}
		out = append(out, list.Content[i], list.Content[i+1])
	}
	for i := 0; i+1 < len(list.Content); i += 2 {
		if !slices.Contains(titles, list.Content[i].Value) {
			out = append(out, list.Content[i], list.Content[i+1])
		}
	}
	list.Content = out
}

func t9aFieldID(f config.DetailField) string {
	if f.Path != "" {
		return f.Path
	}
	return "key:" + f.Key
}

func t9aNodeFieldID(n *yaml.Node) string {
	switch n.Kind {
	case yaml.ScalarNode:
		return n.Value
	case yaml.MappingNode:
		if i := t9aKeyIndex(n, "path"); i >= 0 && n.Content[i+1].Value != "" {
			return n.Content[i+1].Value
		}
		if i := t9aKeyIndex(n, "key"); i >= 0 {
			return "key:" + n.Content[i+1].Value
		}
	}
	return ""
}

func t9aFieldIndex(t *testing.T, detail *yaml.Node, f config.DetailField) int {
	t.Helper()
	i := slices.IndexFunc(detail.Content, func(n *yaml.Node) bool { return t9aNodeFieldID(n) == t9aFieldID(f) })
	if i < 0 {
		t.Fatalf("no detail field %s", t9aFieldID(f))
	}
	return i
}

func t9aFieldNode(f config.DetailField) *yaml.Node {
	if f.Key == "" && f.Label == "" {
		return t9aScalar(f.Path)
	}
	n := &yaml.Node{Kind: yaml.MappingNode, Style: yaml.FlowStyle}
	if f.Path != "" {
		n.Content = append(n.Content, t9aScalar("path"), t9aScalar(f.Path))
	}
	if f.Key != "" {
		n.Content = append(n.Content, t9aScalar("key"), t9aScalar(f.Key))
	}
	if f.Label != "" {
		n.Content = append(n.Content, t9aScalar("label"), t9aScalar(f.Label))
	}
	return n
}

func t9aRelabel(t *testing.T, root *yaml.Node, f config.DetailField, label string) {
	t.Helper()
	detail := t9aValue(t, root, "detail")
	i := t9aFieldIndex(t, detail, f)
	n := detail.Content[i]
	if n.Kind == yaml.ScalarNode {
		detail.Content[i] = t9aFieldNode(config.DetailField{Path: n.Value, Label: label})
		return
	}
	if j := t9aKeyIndex(n, "label"); j >= 0 {
		n.Content[j+1].Value = label
		return
	}
	n.Content = append(n.Content, t9aScalar("label"), t9aScalar(label))
}

func t9aRemoveField(t *testing.T, root *yaml.Node, f config.DetailField) {
	t.Helper()
	detail := t9aValue(t, root, "detail")
	i := t9aFieldIndex(t, detail, f)
	detail.Content = slices.Delete(detail.Content, i, i+1)
}

func t9aInsertFieldAfter(t *testing.T, root *yaml.Node, after, f config.DetailField) {
	t.Helper()
	detail := t9aValue(t, root, "detail")
	i := t9aFieldIndex(t, detail, after)
	detail.Content = slices.Insert(detail.Content, i+1, t9aFieldNode(f))
}

func t9aReorderFields(t *testing.T, root *yaml.Node, order []config.DetailField) {
	t.Helper()
	detail := t9aValue(t, root, "detail")
	out := make([]*yaml.Node, 0, len(detail.Content))
	for _, f := range order {
		out = append(out, detail.Content[t9aFieldIndex(t, detail, f)])
	}
	detail.Content = out
}

type t9aMerge struct {
	old  config.ViewDef
	edit func(t *testing.T, root *yaml.Node)
	want config.ViewDef
}

const (
	t9aChangedOne = "operator changed one item; the build added, removed and changed others"
	t9aRemoved    = "operator removed an item the build changed"
	t9aAdded      = "operator added an item; the build added another"
	t9aOwned      = "operator changed an item the build removed"
	t9aReordered  = "operator reordered; the build added an item"
)

var t9aMergeRows = []string{t9aChangedOne, t9aRemoved, t9aAdded, t9aOwned, t9aReordered} //nolint:gochecknoglobals // fixed row order

func t9aReversedAndInserted[T any](old []T, after func(T) bool, item T) []T {
	rev := slices.Clone(old)
	slices.Reverse(rev)
	return slices.Insert(rev, slices.IndexFunc(rev, after)+1, item)
}

func t9aListMerges(name string, def config.ViewDef) map[string]t9aMerge {
	L, n := def.List, len(def.List)
	other := slices.Clone(def.Detail)
	if len(other) > 1 {
		other = other[:len(other)-1]
	}
	with := func(list []config.ListColumn) config.ViewDef {
		return config.ViewDef{List: list, Detail: slices.Clone(other)}
	}
	out := map[string]t9aMerge{}

	if n >= 3 {
		old := slices.Clone(L[:n-1])
		ti := slices.IndexFunc(old[1:], func(c config.ListColumn) bool { return c.Path != "" }) + 1
		if ti == 0 {
			old[1].Width += 3
		} else {
			old[ti].Path += "Before"
		}
		old = append(old, t9aRetiredColumn(name))
		want := slices.Clone(L)
		want[0].Width += 5
		out[t9aChangedOne] = t9aMerge{with(old), func(t *testing.T, r *yaml.Node) { t9aSetWidth(t, r, L[0].Title, L[0].Width+5) },
			config.ViewDef{List: want, Detail: def.Detail}}
	}
	if n >= 2 {
		old := slices.Clone(L)
		old[1].Width += 7
		out[t9aRemoved] = t9aMerge{with(old), func(t *testing.T, r *yaml.Node) { t9aRemoveColumn(t, r, L[1].Title) },
			config.ViewDef{List: slices.Delete(slices.Clone(L), 1, 2), Detail: def.Detail}}

		out[t9aAdded] = t9aMerge{with(slices.Clone(L[:n-1])),
			func(t *testing.T, r *yaml.Node) { t9aInsertColumnAfter(t, r, L[0].Title, t9aOperatorColumn(name)) },
			config.ViewDef{List: slices.Insert(slices.Clone(L), 1, t9aOperatorColumn(name)), Detail: def.Detail}}
	}
	owned := t9aRetiredColumn(name)
	owned.Width += 9
	out[t9aOwned] = t9aMerge{with(append(slices.Clone(L), t9aRetiredColumn(name))),
		func(t *testing.T, r *yaml.Node) { t9aSetWidth(t, r, owned.Title, owned.Width) },
		config.ViewDef{List: append(slices.Clone(L), owned), Detail: def.Detail}}

	if n >= 4 {
		k := n - 2
		old := slices.Delete(slices.Clone(L), k, k+1)
		titles := make([]string, len(old))
		for i, c := range old {
			titles[len(old)-1-i] = c.Title
		}
		want := t9aReversedAndInserted(old, func(c config.ListColumn) bool { return c.Title == L[k-1].Title }, L[k])
		out[t9aReordered] = t9aMerge{with(old), func(t *testing.T, r *yaml.Node) { t9aReorderColumns(t, r, titles) },
			config.ViewDef{List: want, Detail: def.Detail}}
	}
	return out
}

func t9aDetailMerges(name string, def config.ViewDef) map[string]t9aMerge {
	D, m := def.Detail, len(def.Detail)
	ids := map[string]bool{}
	for _, f := range D {
		if ids[t9aFieldID(f)] {
			return nil
		}
		ids[t9aFieldID(f)] = true
	}
	with := func(detail []config.DetailField) config.ViewDef {
		return config.ViewDef{List: append(slices.Clone(def.List), t9aRetiredColumn(name)), Detail: detail}
	}
	out := map[string]t9aMerge{}

	if m >= 3 {
		old := slices.Clone(D[:m-1])
		old[1].Label = t9aEarlierLabel
		old = append(old, t9aRetiredField)
		want := slices.Clone(D)
		want[0].Label = t9aOperatorLabel
		out[t9aChangedOne] = t9aMerge{with(old), func(t *testing.T, r *yaml.Node) { t9aRelabel(t, r, D[0], t9aOperatorLabel) },
			config.ViewDef{List: def.List, Detail: want}}
	}
	if m >= 2 {
		old := slices.Clone(D)
		old[1].Label = t9aEarlierLabel
		out[t9aRemoved] = t9aMerge{with(old), func(t *testing.T, r *yaml.Node) { t9aRemoveField(t, r, D[1]) },
			config.ViewDef{List: def.List, Detail: slices.Delete(slices.Clone(D), 1, 2)}}

		out[t9aAdded] = t9aMerge{with(slices.Clone(D[:m-1])),
			func(t *testing.T, r *yaml.Node) { t9aInsertFieldAfter(t, r, D[0], t9aOperatorField) },
			config.ViewDef{List: def.List, Detail: slices.Insert(slices.Clone(D), 1, t9aOperatorField)}}
	}
	owned := t9aRetiredField
	owned.Label = t9aOperatorLabel
	out[t9aOwned] = t9aMerge{with(append(slices.Clone(D), t9aRetiredField)),
		func(t *testing.T, r *yaml.Node) { t9aRelabel(t, r, t9aRetiredField, t9aOperatorLabel) },
		config.ViewDef{List: def.List, Detail: append(slices.Clone(D), owned)}}

	if m >= 4 {
		k := m - 2
		old := slices.Delete(slices.Clone(D), k, k+1)
		order := slices.Clone(old)
		slices.Reverse(order)
		want := t9aReversedAndInserted(old, func(f config.DetailField) bool { return t9aFieldID(f) == t9aFieldID(D[k-1]) }, D[k])
		out[t9aReordered] = t9aMerge{with(old), func(t *testing.T, r *yaml.Node) { t9aReorderFields(t, r, order) },
			config.ViewDef{List: def.List, Detail: want}}
	}
	return out
}

func t9aMergeFor(block, name string, def config.ViewDef) map[string]t9aMerge {
	if block == "list" {
		return t9aListMerges(name, def)
	}
	return t9aDetailMerges(name, def)
}

func t9aEditedFile(t *testing.T, m t9aMerge) []byte {
	t.Helper()
	root := t9aParseNode(t, config.GenerateViewYAML(m.old))
	m.edit(t, root)
	return t9aEncode(t, root, 2)
}

func t9aCheckMerged(t *testing.T, when, dir string, want map[string]config.ViewDef, onDisk bool) {
	t.Helper()
	cfg, err := config.LoadFromDirs([]string{dir})
	if err != nil {
		t.Errorf("%s: LoadFromDirs: %v", when, err)
	}
	var bad []string
	for _, name := range slices.Sorted(maps.Keys(want)) {
		if d := t9aViewDiff(config.GetViewDef(cfg, name), want[name]); d != "" {
			bad = append(bad, fmt.Sprintf("%s resolved:\n%s", name, d))
			continue
		}
		if !onDisk {
			continue
		}
		vd, perr := config.ParseSingle(t9aRead(t, dir, name))
		if perr != nil {
			bad = append(bad, fmt.Sprintf("%s.yaml does not parse: %v", name, perr))
			continue
		}
		if d := t9aViewDiff(*vd, want[name]); d != "" {
			bad = append(bad, fmt.Sprintf("%s.yaml on disk:\n%s", name, d))
		}
	}
	if len(bad) > 0 {
		t.Errorf("%s: %d of %d views wrong; first:\n%s", when, len(bad), len(want), strings.Join(bad[:min(3, len(bad))], "\n"))
	}
}

// TestT9a_OperatorEditsMergeWithTheBuild: the merge is a property of every
// view, list and detail alike. It holds when the file is read, the overlay
// being one that no start writes, and a start that rewrites the file must
// not move the result, or each start would merge against the wrong base.
func TestT9a_OperatorEditsMergeWithTheBuild(t *testing.T) {
	names := slices.Sorted(maps.Keys(config.DefaultConfig().Views))
	if len(names) < 50 {
		t.Fatalf("only %d built-in views", len(names))
	}
	t9aDefaults(t, "ec2")

	for _, block := range []string{"list", "detail"} {
		for _, row := range t9aMergeRows {
			t.Run(block+"/"+row, func(t *testing.T) {
				dir := t.TempDir()
				want := map[string]config.ViewDef{}
				for _, name := range names {
					m, ok := t9aMergeFor(block, name, config.DefaultViewDef(name))[row]
					if !ok {
						continue
					}
					t9aWrite(t, dir, name, t9aEditedFile(t, m))
					want[name] = m.want
				}
				if _, ok := want["ec2"]; !ok {
					t.Fatal("ec2 carries no case for this row")
				}

				t9aCheckMerged(t, "file as edited", dir, want, false)
				for start := 1; start <= 3; start++ {
					if err := config.EnsureViewsDir(dir); err != nil {
						t.Fatalf("start %d: EnsureViewsDir: %v", start, err)
					}
					t9aCheckMerged(t, fmt.Sprintf("after start %d", start), dir, want, true)
				}
			})
		}
	}
}

// TestT9a_ReformatIsNotAnEdit: another indentation, comments, blank lines and
// the flow spelling of a detail path leave the parsed content unchanged, so the
// file follows the build.
func TestT9a_ReformatIsNotAnEdit(t *testing.T) {
	def := t9aDefaults(t, "ec2")
	old := t9aCopyView(def)
	old.List = append(old.List, t9aRetiredColumn("ec2"))
	old.Detail = def.Detail[:len(def.Detail)-1]

	root := t9aParseNode(t, config.GenerateViewYAML(old))
	root.Content[0].HeadComment = "# columns I looked at"
	detail := t9aValue(t, root, "detail")
	for i, n := range detail.Content {
		if n.Kind == yaml.ScalarNode {
			detail.Content[i] = &yaml.Node{Kind: yaml.MappingNode, Style: yaml.FlowStyle,
				Content: []*yaml.Node{t9aScalar("path"), t9aScalar(n.Value)}}
		}
	}
	detail.Content[0].HeadComment = "# rows"

	dir := t.TempDir()
	t9aWrite(t, dir, "ec2", t9aEncode(t, root, 4))
	t9aRequireView(t, "reformatted file", t9aResolve(t, "ec2", dir), def)
	if err := config.EnsureViewsDir(dir); err != nil {
		t.Fatalf("EnsureViewsDir: %v", err)
	}
	t9aRequireView(t, "reformatted file after a start", t9aResolve(t, "ec2", dir), def)
}

func t9aFreshFile(t *testing.T, name string) []byte {
	t.Helper()
	dir := t.TempDir()
	if err := config.EnsureViewsDir(dir); err != nil {
		t.Fatalf("EnsureViewsDir on an empty directory: %v", err)
	}
	return t9aRead(t, dir, name)
}

func t9aSideFiles(t *testing.T, dir, name string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		n := e.Name()
		if n != name+".yaml" && strings.HasPrefix(n, name+".") {
			out = append(out, n)
		}
	}
	return out
}

// TestT9a_EarlierBuildsFileIsReplaced: a stamped file with no record of what
// was generated cannot say what the operator edited, so a start replaces it by
// the file this build generates, edited or not, with no side file and no
// startup notice. A child type's file is replaced like a parent's.
func TestT9a_EarlierBuildsFileIsReplaced(t *testing.T) {
	ec2 := t9aDefaults(t, "ec2")
	editedDetail := t9aCopyView(ec2)
	editedDetail.Detail = append(slices.Clone(ec2.Detail[1:]), config.DetailField{Path: t9aOperatorField.Path, Label: t9aOperatorLabel})
	editedList := t9aCopyView(ec2)
	editedList.List[0].Width += 7
	editedList.List = append(editedList.List, t9aOperatorColumn("ec2"))

	child := config.DefaultViewDef("tg_health")
	if len(child.List) < 2 {
		t.Fatalf("tg_health has %d built-in columns, the case needs two", len(child.List))
	}
	editedChild := t9aCopyView(child)
	editedChild.List[0], editedChild.List[1] = editedChild.List[1], editedChild.List[0]
	editedChild.List[0].Title = "Check"

	for _, tc := range []struct {
		name, view string
		stamp      int
		file       config.ViewDef
		want       config.ViewDef
	}{
		{"untouched", "ec2", 11, ec2, ec2},
		{"edited detail", "ec2", 11, editedDetail, ec2},
		{"edited list", "ec2", 5, editedList, ec2},
		{"child type", "tg_health", 6, editedChild, child},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			t9aWrite(t, dir, tc.view, t9aLegacyYAML(tc.stamp, tc.file))

			if err := config.EnsureViewsDir(dir); err != nil {
				t.Errorf("EnsureViewsDir = %v, want nil: replacing such a file is not reported", err)
			}
			if fresh := t9aFreshFile(t, tc.view); !bytes.Equal(t9aRead(t, dir, tc.view), fresh) {
				t.Errorf("%s.yaml is not the freshly generated file\n got:\n%s\nwant:\n%s", tc.view, t9aRead(t, dir, tc.view), fresh)
			}
			if side := t9aSideFiles(t, dir, tc.view); len(side) != 0 {
				t.Errorf("side files next to %s.yaml = %v, want none", tc.view, side)
			}
			t9aRequireView(t, "replaced file", t9aResolve(t, tc.view, dir), tc.want)
		})
	}
}

// TestT9a_OperatorAuthoredFileIsNeverRewritten: a file with no generation
// stamp and no record of what was generated was written by hand. Starts leave
// it byte for byte and it is used exactly as written; no built-in column or
// detail row is merged into it.
func TestT9a_OperatorAuthoredFileIsNeverRewritten(t *testing.T) {
	view := config.ViewDef{
		List: []config.ListColumn{
			{Title: "Instance", Path: "InstanceId", Width: 21},
			{Title: "Type", Path: "InstanceType", Width: 12},
		},
		Detail: []config.DetailField{{Path: "InstanceId"}, {Path: "PrivateIpAddress", Label: "Private IP"}},
	}
	dir := t.TempDir()
	authored := t9aLegacyYAML(0, view)
	t9aWrite(t, dir, "ec2", authored)
	for start := 1; start <= 2; start++ {
		if err := config.EnsureViewsDir(dir); err != nil {
			t.Errorf("start %d: EnsureViewsDir: %v", start, err)
		}
		if got := t9aRead(t, dir, "ec2"); !bytes.Equal(got, authored) {
			t.Errorf("start %d rewrote the operator's file\n got:\n%s\nwant:\n%s", start, got, authored)
		}
		t9aRequireView(t, fmt.Sprintf("after start %d", start), t9aResolve(t, "ec2", dir), view)
	}
}

// TestT9a_RepositoryOverlayStillWins: the repository's .a9s/views overlays
// the global directory, and the merge applies to the overlay file as it does
// to the global one. Nothing writes to the overlay, so the merge happens when
// it is read.
func TestT9a_RepositoryOverlayStillWins(t *testing.T) {
	def := t9aDefaults(t, "ec2")
	operatorView := config.ViewDef{List: []config.ListColumn{
		{Title: "Instance", Path: "InstanceId", Width: 21},
		{Title: "Type", Path: "InstanceType", Width: 12},
	}}
	stale := t9aCopyView(def)
	stale.List = append(stale.List, t9aRetiredColumn("ec2"))
	stale.Detail = def.Detail[:len(def.Detail)-1]

	t.Run("untouched overlay over an operator's global file", func(t *testing.T) {
		global, project := t.TempDir(), t.TempDir()
		t9aWrite(t, global, "ec2", t9aLegacyYAML(0, operatorView))
		t9aWrite(t, project, "ec2", config.GenerateViewYAML(stale))
		t9aRequireView(t, "overlay", t9aResolve(t, "ec2", global, project), def)
	})

	t.Run("operator's overlay over an untouched global file", func(t *testing.T) {
		global, project := t.TempDir(), t.TempDir()
		t9aWrite(t, global, "ec2", config.GenerateViewYAML(stale))
		t9aWrite(t, project, "ec2", t9aLegacyYAML(0, operatorView))
		t9aRequireView(t, "overlay", t9aResolve(t, "ec2", global, project),
			config.ViewDef{List: operatorView.List, Detail: def.Detail})
	})

	t.Run("edited overlay merges with the build", func(t *testing.T) {
		m := t9aListMerges("ec2", def)[t9aChangedOne]
		global, project := t.TempDir(), t.TempDir()
		if err := config.EnsureViewsDir(global); err != nil {
			t.Fatalf("EnsureViewsDir: %v", err)
		}
		t9aWrite(t, project, "ec2", t9aEditedFile(t, m))
		t9aRequireView(t, "overlay", t9aResolve(t, "ec2", global, project), m.want)
	})
}
