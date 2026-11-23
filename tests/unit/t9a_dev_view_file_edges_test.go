// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit

import (
	"bytes"
	"slices"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/k2m30/a9s/v3/core/config"
)

func t9aDevStartTwice(t *testing.T, dir string) {
	t.Helper()
	for start := 1; start <= 2; start++ {
		if err := config.EnsureViewsDir(dir); err != nil {
			t.Fatalf("start %d: EnsureViewsDir: %v", start, err)
		}
	}
}

func TestT9aDev_RenamedColumnKeepsItsPlaceAndTheOldTitleStaysGone(t *testing.T) {
	def := t9aDefaults(t, "ec2")
	root := t9aParseNode(t, config.GenerateViewYAML(def))
	t9aValue(t, root, "list").Content[2].Value = "Renamed"

	dir := t.TempDir()
	t9aWrite(t, dir, "ec2", t9aEncode(t, root, 2))
	want := t9aCopyView(def)
	want.List[1].Title = "Renamed"
	t9aRequireView(t, "as edited", t9aResolve(t, "ec2", dir), want)
	t9aDevStartTwice(t, dir)
	t9aRequireView(t, "after two starts", t9aResolve(t, "ec2", dir), want)
}

func TestT9aDev_DuplicateDetailFieldIsTheOperatorsSecondCopy(t *testing.T) {
	def := t9aDefaults(t, "ec2")
	root := t9aParseNode(t, config.GenerateViewYAML(def))
	t9aInsertFieldAfter(t, root, def.Detail[2], def.Detail[0])

	dir := t.TempDir()
	t9aWrite(t, dir, "ec2", t9aEncode(t, root, 2))
	want := t9aCopyView(def)
	want.Detail = slices.Insert(want.Detail, 3, def.Detail[0])
	t9aRequireView(t, "as edited", t9aResolve(t, "ec2", dir), want)
	t9aDevStartTwice(t, dir)
	t9aRequireView(t, "after two starts", t9aResolve(t, "ec2", dir), want)
}

func TestT9aDev_DeletedBlockFallsBackToTheDefaults(t *testing.T) {
	def := t9aDefaults(t, "ec2")
	root := t9aParseNode(t, config.GenerateViewYAML(def))
	i := t9aKeyIndex(root, "detail")
	root.Content = slices.Delete(root.Content, i, i+2)

	dir := t.TempDir()
	t9aWrite(t, dir, "ec2", t9aEncode(t, root, 2))
	t9aRequireView(t, "as edited", t9aResolve(t, "ec2", dir), def)
	t9aDevStartTwice(t, dir)
	t9aRequireView(t, "after two starts", t9aResolve(t, "ec2", dir), def)
}

func TestT9aDev_UnparseableFileIsLeftAlone(t *testing.T) {
	data := []byte("generated: 12\nlist: [not a mapping\n")
	dir := t.TempDir()
	t9aWrite(t, dir, "ec2", data)
	if err := config.EnsureViewsDir(dir); err != nil {
		t.Fatalf("EnsureViewsDir: %v", err)
	}
	if got := t9aRead(t, dir, "ec2"); !bytes.Equal(got, data) {
		t.Errorf("a file that does not parse was rewritten:\n%s", got)
	}
}

func TestT9aDev_NoDefaultViewRepeatsADetailField(t *testing.T) {
	for name, v := range config.DefaultConfig().Views {
		seen := map[string]bool{}
		for _, f := range v.Detail {
			if seen[f.ID()] {
				t.Errorf("%s declares detail field %s twice; the second copy has no hash and reads as the operator's", name, f.ID())
			}
			seen[f.ID()] = true
		}
	}
}

func TestT9aDev_StartLeavesAnUnchangedMergeByteForByte(t *testing.T) {
	def := t9aDefaults(t, "ec2")
	root := t9aParseNode(t, config.GenerateViewYAML(def))
	t9aSetWidth(t, root, def.List[0].Title, def.List[0].Width+4)
	dir := t.TempDir()
	t9aWrite(t, dir, "ec2", t9aEncode(t, root, 2))
	if err := config.EnsureViewsDir(dir); err != nil {
		t.Fatal(err)
	}
	first := t9aRead(t, dir, "ec2")
	if err := config.EnsureViewsDir(dir); err != nil {
		t.Fatal(err)
	}
	if again := t9aRead(t, dir, "ec2"); !bytes.Equal(again, first) {
		t.Errorf("second start rewrote a settled file\n got:\n%s\nwant:\n%s", again, first)
	}
	var n yaml.Node
	if err := yaml.Unmarshal(first, &n); err != nil {
		t.Fatalf("the merged file does not parse: %v", err)
	}
}
