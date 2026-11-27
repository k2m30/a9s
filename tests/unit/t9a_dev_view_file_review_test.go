// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit

import (
	"bytes"
	"os"
	"path/filepath"
	goruntime "runtime"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/k2m30/a9s/v3/core/config"
)

func t9aDevStale(def config.ViewDef) config.ViewDef {
	stale := t9aCopyView(def)
	stale.List = append(stale.List, t9aRetiredColumn())
	stale.Detail = def.Detail[:len(def.Detail)-1]
	return stale
}

func t9aDevEditedStaleFile(t *testing.T, def config.ViewDef) []byte {
	t.Helper()
	root := t9aParseNode(t, config.GenerateViewYAML(t9aDevStale(def)))
	t9aSetWidth(t, root, def.List[0].Title, def.List[0].Width+5)
	return t9aEncode(t, root, 2)
}

func TestT9aDev_RewriteReplacesTheFileWhole(t *testing.T) {
	def := t9aDefaults(t, "ec2")
	dir := t.TempDir()
	t9aWrite(t, dir, "ec2", t9aDevEditedStaleFile(t, def))
	before, err := os.Stat(filepath.Join(dir, "ec2.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	// On Windows a FileInfo from Stat reads its file identity from the path at
	// the first comparison, so it is compared once here, while the path still
	// names the original.
	_ = os.SameFile(before, before)
	if err = config.EnsureViewsDir(dir); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(filepath.Join(dir, "ec2.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if os.SameFile(before, after) {
		t.Error("ec2.yaml was rewritten in place; a reader can see it half-written")
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".yaml") {
			t.Errorf("leftover file %s", e.Name())
		}
	}
}

func TestT9aDev_FailedRewriteLeavesTheOriginal(t *testing.T) {
	if goruntime.GOOS == "windows" {
		t.Skip("a Windows directory is writable whatever its mode bits say; os.Chmod sets only the read-only attribute")
	}
	def := t9aDefaults(t, "ec2")
	dir := t.TempDir()
	if err := config.EnsureViewsDir(dir); err != nil {
		t.Fatal(err)
	}
	edited := t9aDevEditedStaleFile(t, def)
	t9aWrite(t, dir, "ec2", edited)
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	if err := config.EnsureViewsDir(dir); err == nil {
		t.Error("EnsureViewsDir = nil in a directory it cannot write a new file to")
	}
	if got := t9aRead(t, dir, "ec2"); !bytes.Equal(got, edited) {
		t.Errorf("the original was changed by a rewrite that could not complete:\n%s", got)
	}
}

func t9aDevMapping(pairs ...string) *yaml.Node {
	n := &yaml.Node{Kind: yaml.MappingNode}
	for i, p := range pairs {
		tag := "!!str"
		if i%2 == 1 && pairs[i-1] == "width" {
			tag = "!!int"
		}
		n.Content = append(n.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: tag, Value: p})
	}
	return n
}

func TestT9aDev_ValuesThatNeedQuotingSurviveMergeAndReload(t *testing.T) {
	def := t9aDefaults(t, "ec2")
	col := config.ListColumn{Title: "team # note", Path: "*star", Key: "a: b", Width: 9, SortKey: "yes"}
	field := config.DetailField{Key: "x # y, z", Label: "*l: [q]"}
	pathField := config.DetailField{Path: "p # q", Label: "yes"}

	root := t9aParseNode(t, config.GenerateViewYAML(t9aDevStale(def)))
	list := t9aValue(t, root, "list")
	list.Content = append(list.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: col.Title},
		t9aDevMapping("path", col.Path, "key", col.Key, "width", "9", "sort_key", col.SortKey))
	detail := t9aValue(t, root, "detail")
	detail.Content = append(detail.Content,
		t9aDevMapping("key", field.Key, "label", field.Label),
		t9aDevMapping("path", pathField.Path, "label", pathField.Label))

	dir := t.TempDir()
	t9aWrite(t, dir, "ec2", t9aEncode(t, root, 2))
	n := len(def.Detail)
	want := config.ViewDef{
		List:   append(slices.Clone(def.List), col),
		Detail: slices.Insert(slices.Clone(def.Detail), n-1, field, pathField),
	}
	t9aRequireView(t, "as edited", t9aResolve(t, "ec2", dir), want)
	t9aDevStartTwice(t, dir)
	t9aRequireView(t, "after two starts", t9aResolve(t, "ec2", dir), want)
	vd, err := config.ParseSingle(t9aRead(t, dir, "ec2"))
	if err != nil {
		t.Fatalf("rewritten file does not parse: %v\n%s", err, t9aRead(t, dir, "ec2"))
	}
	t9aRequireView(t, "on disk", *vd, want)
}

func TestT9aDev_NewerStampIsUsedAsWrittenAndNotRewritten(t *testing.T) {
	def := t9aDefaults(t, "ec2")
	stale := t9aDevStale(def)
	data := bytes.Replace(config.GenerateViewYAML(stale), []byte("generated: 12\n"), []byte("generated: 13\nfuture_key: kept\n"), 1)
	dir := t.TempDir()
	t9aWrite(t, dir, "ec2", data)
	if err := config.EnsureViewsDir(dir); err != nil {
		t.Fatal(err)
	}
	if got := t9aRead(t, dir, "ec2"); !bytes.Equal(got, data) {
		t.Errorf("a newer build's file was rewritten:\n%s", got)
	}
	t9aRequireView(t, "newer stamp", t9aResolve(t, "ec2", dir), stale)
}

func TestT9aDev_UnstampedFileWithHashesIsUsedAsWritten(t *testing.T) {
	def := t9aDefaults(t, "ec2")
	stale := t9aDevStale(def)
	data := bytes.Replace(config.GenerateViewYAML(stale), []byte("generated: 12\n"), nil, 1)
	dir := t.TempDir()
	t9aWrite(t, dir, "ec2", data)
	t9aRequireView(t, "unstamped", t9aResolve(t, "ec2", dir), stale)
	if err := config.EnsureViewsDir(dir); err != nil {
		t.Fatal(err)
	}
	if got := t9aRead(t, dir, "ec2"); !bytes.Equal(got, data) {
		t.Errorf("an unstamped file was rewritten:\n%s", got)
	}
}

func t9aDevEmptyBlock(t *testing.T, root *yaml.Node, block string) {
	t.Helper()
	kind := yaml.MappingNode
	if block == "detail" {
		kind = yaml.SequenceNode
	}
	root.Content[t9aKeyIndex(root, block)+1] = &yaml.Node{Kind: kind, Style: yaml.FlowStyle}
}

func TestT9aDev_ExplicitlyEmptyBlockRemovesEveryItem(t *testing.T) {
	def := t9aDefaults(t, "ec2")
	for _, block := range []string{"list", "detail"} {
		t.Run(block, func(t *testing.T) {
			root := t9aParseNode(t, config.GenerateViewYAML(def))
			t9aDevEmptyBlock(t, root, block)
			want := t9aCopyView(def)
			if block == "list" {
				want.List = []config.ListColumn{}
			} else {
				want.Detail = []config.DetailField{}
			}

			dir := t.TempDir()
			t9aWrite(t, dir, "ec2", t9aEncode(t, root, 2))
			t9aRequireView(t, "as edited", t9aResolve(t, "ec2", dir), want)
			t9aDevStartTwice(t, dir)
			t9aRequireView(t, "after two starts", t9aResolve(t, "ec2", dir), want)

			global, project := t.TempDir(), t.TempDir()
			t9aWrite(t, global, "ec2", t9aLegacyYAML(0, config.ViewDef{
				List:   []config.ListColumn{{Title: "Instance", Path: "InstanceId", Width: 21}},
				Detail: []config.DetailField{{Path: "InstanceId"}},
			}))
			t9aWrite(t, project, "ec2", t9aEncode(t, root, 2))
			got := t9aResolve(t, "ec2", global, project)
			if block == "list" && len(got.List) != 0 || block == "detail" && len(got.Detail) != 0 {
				t.Errorf("the overlay's empty %s block let the global one through:\n%s", block, t9aViewDiff(got, want))
			}
		})
	}
}

func TestT9aDev_SameBuildStartKeepsTheOperatorsComments(t *testing.T) {
	def := t9aDefaults(t, "ec2")
	root := t9aParseNode(t, config.GenerateViewYAML(def))
	t9aSetWidth(t, root, def.List[0].Title, def.List[0].Width+5)
	t9aValue(t, root, "list").Content[0].HeadComment = "# widened for long names"
	data := t9aEncode(t, root, 4)

	dir := t.TempDir()
	t9aWrite(t, dir, "ec2", data)
	t9aDevStartTwice(t, dir)
	if got := t9aRead(t, dir, "ec2"); !bytes.Equal(got, data) {
		t.Errorf("a start with the same build rewrote an edited file\n got:\n%s\nwant:\n%s", got, data)
	}
}

func TestT9aDev_BlockWithoutHashesStaysTheOperators(t *testing.T) {
	def := t9aDefaults(t, "ec2")
	stale := t9aDevStale(def)
	root := t9aParseNode(t, config.GenerateViewYAML(stale))
	i := t9aKeyIndex(root, "list_hashes")
	root.Content = slices.Delete(root.Content, i, i+2)

	dir := t.TempDir()
	t9aWrite(t, dir, "ec2", t9aEncode(t, root, 2))
	want := config.ViewDef{List: stale.List, Detail: def.Detail}
	t9aRequireView(t, "as edited", t9aResolve(t, "ec2", dir), want)
	t9aDevStartTwice(t, dir)
	t9aRequireView(t, "after two starts", t9aResolve(t, "ec2", dir), want)
	if bytes.Contains(t9aRead(t, dir, "ec2"), []byte("list_hashes")) {
		t.Errorf("a start gave the operator's list its hashes back:\n%s", t9aRead(t, dir, "ec2"))
	}
}
