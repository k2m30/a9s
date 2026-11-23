// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package config

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed views_reference.yaml
var viewsReferenceData []byte

type ItemHash struct {
	Key  string
	Hash string
}

// An empty block is left out: written as `list: {}` it would be an edit that
// removes every item.
func GenerateViewYAML(v ViewDef) []byte {
	if len(v.List) == 0 {
		v.List = nil
	}
	if len(v.Detail) == 0 {
		v.Detail = nil
	}
	return viewYAML(v, listHashes(v.List), detailHashes(v.Detail))
}

func listHashes(cols []ListColumn) []ItemHash {
	return itemHashes(cols, columnKey, columnHash)
}

func detailHashes(fields []DetailField) []ItemHash {
	return itemHashes(fields, DetailField.ID, fieldHash)
}

func viewYAML(v ViewDef, listBase, detailBase []ItemHash) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "generated: %d\n", GeneratedViewsVersion)
	writeHashes(&b, "list_hashes", listBase)
	writeHashes(&b, "detail_hashes", detailBase)
	switch {
	case v.List == nil:
	case len(v.List) == 0:
		b.WriteString("list: {}\n")
	default:
		b.WriteString("list:\n")
		for _, col := range v.List {
			writeColumn(&b, col)
		}
	}
	switch {
	case v.Detail == nil:
	case len(v.Detail) == 0:
		b.WriteString("detail: []\n")
	default:
		if v.List != nil {
			b.WriteString("\n")
		}
		b.WriteString("detail:\n")
		for _, df := range v.Detail {
			writeField(&b, df)
		}
	}
	return []byte(b.String())
}

func writeHashes(b *strings.Builder, name string, hashes []ItemHash) {
	if len(hashes) == 0 {
		return
	}
	fmt.Fprintf(b, "%s:\n", name)
	for _, h := range hashes {
		fmt.Fprintf(b, "  %s: %s\n", yamlScalar(h.Key), h.Hash)
	}
}

// An item whose key an earlier item already has is not recorded, so a merge
// reads it as one the operator added.
func itemHashes[T any](items []T, key, hash func(T) string) []ItemHash {
	if len(items) == 0 {
		return nil
	}
	out := make([]ItemHash, 0, len(items))
	seen := make(map[string]bool, len(items))
	for _, it := range items {
		if k := key(it); !seen[k] {
			seen[k] = true
			out = append(out, ItemHash{Key: k, Hash: hash(it)})
		}
	}
	return out
}

func columnKey(c ListColumn) string { return c.Title }

// columnHash and fieldHash hash an item as the generator renders it from its
// parsed content, so comments, indentation and another YAML spelling of the
// same value hash alike. The rendering omits a zero field, so a field the
// generator learns to write later leaves every existing hash unchanged.
func columnHash(c ListColumn) string {
	var b strings.Builder
	writeColumn(&b, c)
	return contentHash(b.String())
}

func fieldHash(f DetailField) string {
	var b strings.Builder
	writeField(&b, f)
	return contentHash(b.String())
}

func contentHash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:8])
}

func writeColumn(b *strings.Builder, col ListColumn) {
	fmt.Fprintf(b, "  %s:\n", yamlScalar(col.Title))
	if col.Path != "" {
		fmt.Fprintf(b, "    path: %s\n", yamlScalar(col.Path))
	}
	if col.Key != "" {
		fmt.Fprintf(b, "    key: %s\n", yamlScalar(col.Key))
	}
	fmt.Fprintf(b, "    width: %d\n", col.Width)
	if col.SortKey != "" {
		fmt.Fprintf(b, "    sort_key: %s\n", yamlScalar(col.SortKey))
	}
}

func writeField(b *strings.Builder, df DetailField) {
	switch {
	case df.Key != "" && df.Label != "":
		fmt.Fprintf(b, "  - { key: %s, label: %s }\n", yamlScalar(df.Key), yamlScalar(df.Label))
	case df.Key != "":
		fmt.Fprintf(b, "  - { key: %s }\n", yamlScalar(df.Key))
	case df.Path != "" && df.Label != "":
		fmt.Fprintf(b, "  - { path: %s, label: %s }\n", yamlScalar(df.Path), yamlScalar(df.Label))
	default:
		// Bare path-form (no label) stays as a simple string for readability.
		fmt.Fprintf(b, "  - %s\n", yamlScalar(df.Path))
	}
}

// A value with an indicator character or a line break is double-quoted: the plain
// form the encoder may pick for it is valid in a block but not inside the
// `{ … }` of a detail field.
func yamlScalar(s string) string {
	n := yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: s}
	if strings.ContainsAny(s, ",:#[]{}&*?|>!%@`\n") {
		n.Style = yaml.DoubleQuotedStyle
	}
	out, _ := yaml.Marshal(&n) //nolint:errcheck // a string scalar node always encodes
	return strings.TrimSuffix(string(out), "\n")
}

func (v *ViewDef) mergeWithBuild(def ViewDef) {
	v.List = mergeItems(v.List, v.ListHashes, def.List, columnKey, columnHash)
	v.Detail = mergeItems(v.Detail, v.DetailHashes, def.Detail, DetailField.ID, fieldHash)
}

// build is -1 when the operator owns the entry; file is -1 for an item only
// the build has.
type mergeEntry[T any] struct {
	v           T
	build, file int
}

// Against the recorded base, a file item is
// "same" when its hash matches, "changed" when it does not, "added" when its
// key has no hash, and a base key with no file item was removed:
//
//	same, in the build     the build's item
//	same, not in the build removed
//	changed                the operator's item, whole
//	removed                stays removed
//	added                  the operator's item
//	only in the build      added
//
// When the base items left in the file are in base order, the result takes
// the build's order and each item the operator owns follows the item it
// followed in the file. Otherwise it takes the file's order, and each item
// only the build has follows the nearest preceding build item present.
//
// A nil block is absent from the file and stays absent, so the directory
// below it or the defaults answer for it. A block with no recorded hashes has
// no base: it is the operator's as written.
func mergeItems[T any](file []T, base []ItemHash, build []T, key, hash func(T) string) []T {
	if file == nil || len(base) == 0 {
		return file
	}
	baseAt := make(map[string]int, len(base))
	for i, b := range base {
		if _, dup := baseAt[b.Key]; !dup {
			baseAt[b.Key] = i
		}
	}
	buildAt := make(map[string]int, len(build))
	for i, b := range build {
		if _, dup := buildAt[key(b)]; !dup {
			buildAt[key(b)] = i
		}
	}

	var kept []mergeEntry[T]
	inFile := make(map[string]bool, len(file))
	last, reordered := -1, false
	for _, f := range file {
		k := key(f)
		bi, inBase := baseAt[k]
		if inFile[k] || !inBase {
			inFile[k] = true
			kept = append(kept, mergeEntry[T]{f, -1, len(kept)})
			continue
		}
		inFile[k] = true
		reordered = reordered || bi < last
		last = bi
		j, inBuild := buildAt[k]
		switch {
		case hash(f) != base[bi].Hash && inBuild:
			kept = append(kept, mergeEntry[T]{f, j, len(kept)})
		case hash(f) != base[bi].Hash:
			kept = append(kept, mergeEntry[T]{f, -1, len(kept)})
		case inBuild:
			kept = append(kept, mergeEntry[T]{build[j], j, len(kept)})
		}
	}
	isNew := func(b T) bool {
		_, inBase := baseAt[key(b)]
		return !inBase && !inFile[key(b)]
	}

	var out []mergeEntry[T]
	if reordered {
		out = kept
		for j, b := range build {
			if isNew(b) {
				at := after(out, func(e mergeEntry[T]) int { return rankBelow(e.build, j) })
				out = slices.Insert(out, at, mergeEntry[T]{b, j, -1})
			}
		}
	} else {
		for j, b := range build {
			if isNew(b) {
				out = append(out, mergeEntry[T]{b, j, -1})
			} else if i := slices.IndexFunc(kept, func(e mergeEntry[T]) bool { return e.build == j }); i >= 0 {
				out = append(out, kept[i])
			}
		}
		for i, e := range kept {
			if e.build < 0 {
				out = slices.Insert(out, after(out, func(o mergeEntry[T]) int { return rankBelow(o.file, i) }), e)
			}
		}
	}
	merged := make([]T, len(out))
	for i, e := range out {
		merged[i] = e.v
	}
	return merged
}

func after[T any](out []mergeEntry[T], rank func(mergeEntry[T]) int) int {
	best, at := -1, 0
	for i, e := range out {
		if r := rank(e); r > best {
			best, at = r, i+1
		}
	}
	return at
}

func rankBelow(i, limit int) int {
	if i < limit {
		return i
	}
	return -1
}

// A block without hashes in the file stays the operator's and is written
// without them again.
func rebased(vd *ViewDef, def ViewDef) ViewDef {
	m := *vd
	m.mergeWithBuild(def)
	m.ListHashes, m.DetailHashes = nil, nil
	if m.List != nil && len(vd.ListHashes) > 0 {
		m.ListHashes = listHashes(def.List)
	}
	if m.Detail != nil && len(vd.DetailHashes) > 0 {
		m.DetailHashes = detailHashes(def.Detail)
	}
	return m
}

func sameView(a, b ViewDef) bool {
	return (a.List == nil) == (b.List == nil) && slices.Equal(a.List, b.List) &&
		(a.Detail == nil) == (b.Detail == nil) && slices.Equal(a.Detail, b.Detail) &&
		slices.Equal(a.ListHashes, b.ListHashes) && slices.Equal(a.DetailHashes, b.DetailHashes)
}

// A file whose merge and next base equal what it holds is left byte for byte,
// so an operator's comments survive every start of the same build. A file with
// no stamp was written by hand and is never rewritten. A file a
// newer build stamped is left as that build wrote it. A file an earlier build
// stamped carries no item hashes and is replaced, edited or not.
func EnsureViewsDir(dir string) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	cfg := DefaultConfig()
	keys := make([]string, 0, len(cfg.Views))
	for k := range cfg.Views {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, name := range keys {
		dest := filepath.Join(dir, name+".yaml")
		def := cfg.Views[name]
		data, readErr := os.ReadFile(dest) //nolint:gosec // dest is built from the caller's own config dir
		next := GenerateViewYAML(def)
		switch {
		case readErr != nil && !os.IsNotExist(readErr):
			return readErr
		case readErr == nil:
			vd, err := ParseSingle(data)
			if err != nil || vd.Generated == 0 || vd.Generated > GeneratedViewsVersion {
				continue
			}
			if vd.Generated == GeneratedViewsVersion {
				m := rebased(vd, def)
				if sameView(*vd, m) {
					continue
				}
				next = viewYAML(m, m.ListHashes, m.DetailHashes)
			}
		}
		if err := replaceFile(dest, next); err != nil {
			return err
		}
	}
	return nil
}

// The rename means a concurrent reader sees the old file or the new one, never
// a partial write, and a failed write leaves the original.
func replaceFile(dest string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(dest), "."+filepath.Base(dest)+".*.tmp")
	if err != nil {
		return err
	}
	_, writeErr := tmp.Write(data)
	if err := errors.Join(writeErr, tmp.Close(), os.Chmod(tmp.Name(), 0644)); err != nil { //nolint:gosec // view YAML files are non-sensitive, world-readable is acceptable
		return errors.Join(err, os.Remove(tmp.Name()))
	}
	if err := os.Rename(tmp.Name(), dest); err != nil {
		return errors.Join(err, os.Remove(tmp.Name()))
	}
	return nil
}

// EnsureViewsReference writes the embedded views_reference.yaml to configDir.
// Always overwrites — this is generated reference data, not user-editable.
// Upgrades deliver updated field listings automatically.
func EnsureViewsReference(configDir string) error {
	dest := filepath.Join(configDir, "views_reference.yaml")
	return os.WriteFile(dest, viewsReferenceData, 0644) //nolint:gosec // view YAML files are non-sensitive, world-readable is acceptable
}
