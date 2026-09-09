// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package config

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/logging"
)

//go:embed views_reference.yaml
var viewsReferenceData []byte

// GenerateViewYAML returns the YAML bytes for a single ViewDef, stamped with
// the build that generated it.
func GenerateViewYAML(v ViewDef) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "generated: %d\n", GeneratedViewsVersion)

	if len(v.List) > 0 {
		b.WriteString("list:\n")
		for _, col := range v.List {
			fmt.Fprintf(&b, "  %s:\n", yamlKey(col.Title))
			if col.Path != "" {
				fmt.Fprintf(&b, "    path: %s\n", col.Path)
			}
			if col.Key != "" {
				fmt.Fprintf(&b, "    key: %s\n", col.Key)
			}
			fmt.Fprintf(&b, "    width: %d\n", col.Width)
			if col.SortKey != "" {
				fmt.Fprintf(&b, "    sort_key: %s\n", col.SortKey)
			}
		}
	}

	if len(v.Detail) > 0 {
		if len(v.List) > 0 {
			b.WriteString("\n")
		}
		b.WriteString("detail:\n")
		for _, df := range v.Detail {
			switch {
			case df.Key != "" && df.Label != "":
				fmt.Fprintf(&b, "  - { key: %s, label: %q }\n", df.Key, df.Label)
			case df.Key != "":
				fmt.Fprintf(&b, "  - { key: %s }\n", df.Key)
			case df.Path != "" && df.Label != "":
				fmt.Fprintf(&b, "  - { path: %s, label: %q }\n", df.Path, df.Label)
			default:
				// Bare path-form (no label) stays as a simple string for readability.
				fmt.Fprintf(&b, "  - %s\n", df.Path)
			}
		}
	}

	return []byte(b.String())
}

// yamlKey quotes a YAML key that contains special characters.
func yamlKey(s string) string {
	if strings.ContainsAny(s, "#:{}[]&*?|>!%@`") {
		return fmt.Sprintf("%q", s)
	}
	return s
}

// EnsureViewsDir writes any missing built-in view YAML files to dir and brings
// files written by an older build up to this one's column set.
//
// A file already stamped with this build is left byte for byte alone: whatever
// the operator did to it is theirs.
//
// A file with an older stamp (or none, which is every file written before the
// stamp existed) is first read for evidence of an edit. One whose every column
// is still the source and width the build at its stamp generated has never been
// touched: it takes this build's column set and order wholesale, which is the
// only way a corrected default order reaches an installation that already
// exists. Reordering columns and changing nothing else leaves no such evidence,
// so a file edited only that way is reordered back.
//
// Any other file is the operator's. It keeps every column it has, in the order
// it has them, and gains the built-in columns whose titles it does not carry —
// each at the position it holds in the built-in set, so a column added in the
// middle does not land at the far right of the operator's table.
//
// A column it does carry keeps what it has, field by field, with one
// exception: on a column this build has corrected (viewColumnChanges), every
// field still holding what the build at the file's stamp generated takes this
// build's value. That field has never been edited, and leaving it behind leaves
// the operator reading the source, or the raw constant, the correction moved
// away from. A field they changed is theirs — and only that field, so widening
// a column never costs them the rest of a correction.
func EnsureViewsDir(dir string) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	if err := carryRenamedViewFiles(dir); err != nil {
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
		data, readErr := os.ReadFile(dest) //nolint:gosec // dest is built from the caller's own config dir
		switch {
		case readErr != nil && !os.IsNotExist(readErr):
			return readErr
		case readErr != nil:
			data = GenerateViewYAML(cfg.Views[name])
		default:
			merged, changed := mergeGeneratedColumns(name, data, cfg.Views[name])
			if !changed {
				continue
			}
			data = merged
		}
		if writeErr := os.WriteFile(dest, data, 0644); writeErr != nil { //nolint:gosec // view YAML files are non-sensitive, world-readable is acceptable
			return writeErr
		}
	}
	return nil
}

// viewColumnChange records one built-in column this build has corrected, and
// the whole column the build before Version generated for it. What it is
// corrected TO is deliberately not recorded: that is whatever the defaults now
// say, which is the one place a built-in column is defined.
//
// A corrected column keeps its title, so the missing-title rule below never
// sees it and an operator who had run a9s once kept the column the correction
// moved away from — a cell that looks like it works and is wrong. Matching on
// the whole column the previous build generated is what separates a column
// nobody has touched from one the operator set themselves: theirs differs in
// at least the field they changed, and is left exactly as it is.
type viewColumnChange struct {
	Version int        // the GeneratedViewsVersion that introduced the correction
	View    string     // the view file's name, e.g. "sns"
	Was     ListColumn // the column the build before Version generated, whole
}

// viewColumnChanges is the migration table, scanned in order. Add a row in the
// same change that alters ANY field of a built-in column — its path, its key,
// its width, whether it humanizes — and bump GeneratedViewsVersion with it.
var viewColumnChanges = []viewColumnChange{
	{Version: 2, View: "sns", Was: ListColumn{Title: "Topic Name", Path: "TopicArn", Width: 40}},
	{Version: 2, View: "sns-sub", Was: ListColumn{Title: "Confirmed", Path: "SubscriptionArn", Width: 22}},
	{Version: 3, View: "alarm", Was: ListColumn{Title: "Threshold", Path: "Threshold", Width: 12}},
	{Version: 3, View: "ecs-task", Was: ListColumn{Title: "Task ID", Path: "TaskArn", Width: 38}},
	{Version: 3, View: "logs", Was: ListColumn{Title: "Retention", Path: "RetentionInDays", Width: 10}},
	{Version: 3, View: "ng", Was: ListColumn{Title: "Node Group", Path: "NodegroupName", Width: 28}},
	{Version: 3, View: "secrets", Was: ListColumn{Title: "Last Accessed", Path: "LastAccessedDate", Width: 18}},
	{Version: 3, View: "secrets", Was: ListColumn{Title: "Last Changed", Path: "LastChangedDate", Width: 18}},
	{Version: 3, View: "sns-sub", Was: ListColumn{Title: "Subscription ARN", Path: "SubscriptionArn", Width: 60}},
	// The two columns that gained humanize: the cell showed a raw AWS constant.
	{Version: 4, View: "ecs-task", Was: ListColumn{Title: "Stop Code", Path: "StopCode", Width: 24}},
	{Version: 4, View: "nat", Was: ListColumn{Title: "Failure", Path: "FailureCode", Width: 22}},
	// Version 5 regenerates every file: the humanize flag stopped being a
	// column field at all and became the type's own declaration
	// (ResourceTypeDef.HumanizeFields), so the key a v4 file carries for it is
	// read by nothing. The Retention entry is renewed at this stamp because it
	// is the one column here whose older spelling still differs from what this
	// build generates, so it is what the carry has to deliver.
	{Version: 5, View: "logs", Was: ListColumn{Title: "Retention", Path: "RetentionInDays", Width: 10}},
	// Version 6 moves the column list itself onto the resource type, which
	// keyMoveVersion below records as a class. The Retention entry is renewed
	// at this stamp for the same reason it was renewed at 5: it is the one
	// column whose older spelling still differs from what this build generates
	// in more than the key, so the class rule cannot deliver it and only a row
	// naming that whole spelling can.
	{Version: 6, View: "logs", Was: ListColumn{Title: "Retention", Path: "RetentionInDays", Width: 10}},
	// Renewed again at 7 for the reason it was renewed at 5 and 6: a file
	// still carrying the pre-correction Retention column is a file this stamp
	// must reach, and an entry only carries while its Version is above the
	// stamp the file was written at.
	{Version: 7, View: "logs", Was: ListColumn{Title: "Retention", Path: "RetentionInDays", Width: 10}},
}

// keyMoveVersion is the stamp at which every built-in column took the key its
// resource type declares. Before it, a column in a view file carried whatever
// key the second per-type column list in core/config gave it — for most
// columns none at all, the cell resolving through the RawStruct path beside it
// — while the renderer merged the type's key onto that same column anyway, on
// every frame. A file written before the move therefore differs from this
// build's column in Key and in no other field, and that difference is this
// build's own doing rather than an edit.
//
// It is one rule and not the 325 viewColumnChanges rows the same migration
// would need column by column, because every one of those rows would say this
// same sentence about a different column.
//
// It matches whole columns rather than the key alone, so a column the operator
// touched at all — a width they set — keeps no key at this migration. That
// costs them nothing on screen: ResolveListColumnCascade merges the type's key
// onto every title both declare on every frame, so the cell reads the same
// field either way. The alternative, taking the key wherever the title
// matches, would overwrite a key an operator wrote themselves, which the file
// does not record as theirs.
const keyMoveVersion = 6

// keyMovedOnly reports whether onDisk is, whole, what the build before
// keyMoveVersion generated for this column: its title, path, width and sort
// key, and NO key where this build declares one.
//
// The empty key is the load-bearing half. The move only ever added keys — 325
// of them across 77 files, and not one column's key was replaced — so the
// previous generated form of a moved column is this build's column with the
// key taken off, and that is what "untouched" has to mean. Asking merely
// whether the key DIFFERS from this build's reads an operator's own key as the
// generator's output, and since generatedAsIs asks this same question of every
// column to decide whether the file was ever edited, one such key made the
// whole file look untouched and be replaced wholesale.
func keyMovedOnly(on, now ListColumn, stamp int) bool {
	return stamp < keyMoveVersion &&
		on.Key == "" && now.Key != "" &&
		on.Title == now.Title &&
		on.Path == now.Path &&
		on.Width == now.Width &&
		on.SortKey == now.SortKey
}

// statusKeyMoveVersion is the stamp at which the status column stopped being
// the one a TITLE said was the status column, or the one whose key was spelled
// "status" whatever the type calls its own key. The type declares it now
// (catalog.ResourceTypeDef.LifecycleKey), so a file written before this stamp
// carries columns the replaced rule treated as the status and this build does
// not — a renamed Status column above all, which is the one the renderer's
// title merge cannot reach either.
//
// A migration encodes the rule it replaced. movedStatusKey is that rule,
// written out once: it is what config.IsStatusColumn said before this build,
// and every column it recognises takes the key the type now declares.
const statusKeyMoveVersion = 7

// movedStatusKey returns the key a column the REPLACED status rule recognised
// takes on this type, and whether it moved at all.
func movedStatusKey(view string, col ListColumn, stamp int) (string, bool) {
	if stamp >= statusKeyMoveVersion {
		return "", false
	}
	// catalog.FindAny and not Find: a child type has a view file, an operator
	// and a lifecycle key of its own, and looking only among the parents
	// stamped every child file as migrated while migrating nothing.
	td := catalog.FindAny(view)
	if td == nil {
		return "", false
	}
	statusKey := td.StatusKey()
	if col.Key == statusKey {
		return "", false
	}
	// The replaced predicate, verbatim: key "status", key == the type's
	// lifecycle key, or a title of "status"/"state" whatever the key said.
	wasStatus := col.Key == "status" ||
		strings.EqualFold(col.Title, "status") ||
		strings.EqualFold(col.Title, "state")
	if !wasStatus {
		return "", false
	}
	return statusKey, true
}

// viewFileRenames records a type this build has RENAMED, old name to new.
// The generator wrote a file under the old name, the rename landed, and the
// file stayed behind naming nothing: docdb-snap became dbc-snap and
// rds-snap became dbi-snap, so an installation that ran an older build
// still has both on disk.
//
// Same shape and same bargain as viewColumnAdditions: the migration owns what
// the generator wrote, and a file it wrote is not the operator's mistake to be
// told about on every start. Add a row in the same change that renames a type
// whose views file this repo carries.
var viewFileRenames = map[string]string{ //nolint:gochecknoglobals // static migration table
	"docdb-snap": "dbc-snap",
	"rds-snap":   "dbi-snap",
}

// carryRenamedViewFiles moves a view file written under a name this build has
// renamed to the name in use, and retires it when that name already has a
// file — the one under the name in use is the one being edited, so it is
// never written over. A retired file keeps its contents beside its old name
// with a suffix the loader does not read, because deleting what an operator
// may have edited to say "this build renamed the type" is not the migration's
// call.
func carryRenamedViewFiles(dir string) error {
	for from, to := range viewFileRenames {
		old := filepath.Join(dir, from+".yaml")
		if _, err := os.Stat(old); err != nil {
			continue
		}
		dest := filepath.Join(dir, to+".yaml")
		if _, err := os.Stat(dest); err == nil {
			retired := old + ".retired"
			if err := os.Rename(old, retired); err != nil {
				return err
			}
			logging.L().Info("retired a view file for a renamed type",
				"file", from+".yaml", "renamed_to", to, "kept", filepath.Base(retired),
				"reason", to+".yaml already exists and is the one in use")
			continue
		}
		if err := os.Rename(old, dest); err != nil {
			return err
		}
		logging.L().Info("carried a view file to the type's new name",
			"file", from+".yaml", "renamed_to", to+".yaml")
	}
	return nil
}

// viewColumnAdditions names a column the built-in views GAINED, and the
// GeneratedViewsVersion that gained it. Add a row in the same change that adds
// a column to core/config/defaults_*.go, and bump GeneratedViewsVersion with
// it.
//
// It is what tells a migration the difference between a column a file has
// never been offered and one the operator deleted. Absence is the only record
// a view file keeps of that decision, so a migration that re-adds every
// built-in the file does not carry reverses it — and does it again on the next
// start, so deleting the column twice does not help either.
var viewColumnAdditions = []struct { //nolint:gochecknoglobals // static migration table
	Version int
	View    string
	Title   string
}{
	{Version: 5, View: "lambda", Title: "Handler"},
}

// introducedAfter reports whether the named column entered the built-in views
// after the stamp a file was written at, which is the only reason to insert a
// column the file does not carry.
func introducedAfter(view, title string, stamp int) bool {
	return slices.ContainsFunc(viewColumnAdditions, func(a struct {
		Version int
		View    string
		Title   string
	}) bool {
		return a.View == view && a.Title == title && a.Version > stamp
	})
}

// mergeGeneratedColumns returns the YAML for onDisk brought up to this build:
// def's columns whose titles are missing are added, and a column still carrying
// the source an older build generated for it takes this build's source and
// width. changed=false when there is nothing to do: onDisk already carries this
// build's stamp, or it does not parse, which is the operator's problem to see
// and fix rather than this function's to overwrite.
func mergeGeneratedColumns(name string, onDisk []byte, def ViewDef) ([]byte, bool) {
	vd, err := ParseSingle(onDisk)
	if err != nil || vd.Generated >= GeneratedViewsVersion {
		return nil, false
	}
	have := make(map[string]bool, len(vd.List))
	for _, c := range vd.List {
		have[c.Title] = true
	}
	want := make(map[string]ListColumn, len(def.List))
	for _, c := range def.List {
		want[c.Title] = c
	}

	// belongs reports whether a built-in column belongs in the migrated file:
	// one the file already carries, or one introduced after the stamp it was
	// written at. Both writers below ask, so neither can restore a column the
	// operator deleted.
	belongs := func(title string) bool {
		// A file with no stamp at all predates the table: the build that wrote
		// it never generated the current column vocabulary, so nothing missing
		// from it can be a column the operator deleted. Every built-in is new
		// to that file.
		return vd.Generated == 0 || have[title] || introducedAfter(name, title, vd.Generated)
	}

	// A file nobody has touched carries this build's column set, or the set the
	// build at its stamp generated, and nothing else. It has no order of its
	// own to protect, so it takes this build's — which is the only way a
	// corrected default order reaches an operator who has already run a9s once.
	// The trade is deliberate: reordering columns and changing nothing else is
	// indistinguishable from never having opened the file, and such a file is
	// reordered back.
	if generatedAsIs(name, vd.List, want, vd.Generated) {
		kept := make([]ListColumn, 0, len(def.List))
		for _, c := range def.List {
			if belongs(c.Title) {
				kept = append(kept, c)
			}
		}
		vd.List = kept
		return GenerateViewYAML(*vd), true
	}

	for i, c := range def.List {
		if have[c.Title] || !belongs(c.Title) {
			continue
		}
		vd.List = slices.Insert(vd.List, min(i, len(vd.List)), c)
	}
	// Carry this build's corrections, field by field. An entry names the whole
	// column an older build generated, so every field of it can be compared:
	// one still holding what that build wrote is one nobody has touched and
	// takes the current default's value, and one the operator changed is
	// theirs. Comparing whole columns instead would let a single edit freeze
	// every other field, and comparing only the fields a given correction
	// happens to change would leave the next kind of correction stranded —
	// which is how the humanize flag reached new installations alone.
	for i, on := range vd.List {
		if moved, ok := movedStatusKey(name, on, vd.Generated); ok {
			vd.List[i].Key = moved
		}
		now, ok := want[on.Title]
		if !ok {
			continue
		}
		if keyMovedOnly(on, now, vd.Generated) {
			vd.List[i].Key = now.Key
		}
		for _, ch := range viewColumnChanges {
			if ch.View != name || ch.Was.Title != on.Title || ch.Version <= vd.Generated {
				continue
			}
			vd.List[i] = carryGeneratedFields(vd.List[i], ch.Was, now)
		}
	}
	return GenerateViewYAML(*vd), true
}

// generatedAsIs reports whether every column onDisk is one a build generated
// and left alone: this build's own declaration of that column, or — for a
// column a later build corrected — exactly what the build at stamp wrote for
// it. A column the defaults do not declare at all is the operator's, and so is
// any other spelling of one they do.
func generatedAsIs(name string, onDisk []ListColumn, want map[string]ListColumn, stamp int) bool {
	for _, on := range onDisk {
		now, ok := want[on.Title]
		if !ok {
			return false
		}
		if on != now && !generatedByStamp(name, on, stamp) && !keyMovedOnly(on, now, stamp) {
			return false
		}
	}
	return true
}

// generatedByStamp reports whether on is, whole, what the build at stamp
// generated for a column this build has since corrected. Whole rather than
// field by field because its caller asks a different question: not "may this
// field be corrected" but "has this file been edited at all".
func generatedByStamp(name string, on ListColumn, stamp int) bool {
	for _, ch := range viewColumnChanges {
		if ch.View == name && ch.Version > stamp && ch.Was == on {
			return true
		}
	}
	return false
}

// carryGeneratedFields returns on with every field that still holds what the
// previous build generated (was) replaced by what this build declares (now).
// Title identifies the column and is never carried; it is what the two are
// matched on.
//
// ponytail: "untouched" is "equal to what the previous build wrote", which a
// bool cannot always express — an operator who sets a flag back to the value
// that build generated is indistinguishable from one who never opened the
// file, and the correction lands on them once. They set it again afterwards
// and the current stamp leaves the file alone from then on. Recording what the
// operator changed, rather than inferring it, is the upgrade path.
func carryGeneratedFields(on, was, now ListColumn) ListColumn {
	if on.Path == was.Path {
		on.Path = now.Path
	}
	if on.Key == was.Key {
		on.Key = now.Key
	}
	if on.Width == was.Width {
		on.Width = now.Width
	}
	if on.SortKey == was.SortKey {
		on.SortKey = now.SortKey
	}
	return on
}

// EnsureViewsReference writes the embedded views_reference.yaml to configDir.
// Always overwrites — this is generated reference data, not user-editable.
// Upgrades deliver updated field listings automatically.
func EnsureViewsReference(configDir string) error {
	dest := filepath.Join(configDir, "views_reference.yaml")
	return os.WriteFile(dest, viewsReferenceData, 0644) //nolint:gosec // view YAML files are non-sensitive, world-readable is acceptable
}
