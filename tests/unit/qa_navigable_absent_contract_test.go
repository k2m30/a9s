package unit

// qa_navigable_absent_contract_test.go — absent/nil AWS fields must not
// be marked navigable.
//
// This file is NOT a PR #273 regression pin. It surfaced during PR #273
// review as a fundamental bug in fieldpath.ExtractFieldList: when the
// AWS API returns nil for a navigable pointer, the "-" placeholder is
// still marked IsNavigable=true, producing a dead affordance on every
// registered navigable field across every resource type.
//
// Bug report: a DocDB cluster with KmsKeyId=nil rendered the KmsKeyId row
// as "-" but with the navigable style. Pressing Enter tries to navigate
// to a kms resource identified by "-" — a dead affordance.
//
// The bug lives in fieldpath.ExtractFieldList: when the AWS API value is
// absent, the function emits a FieldItem with Value="-" AND
// IsNavigable=true if the path is in the navigable map. The "navigable"
// annotation is unconditional on path match; it ignores whether a real
// value exists to navigate to.
//
// Contract (asserted below for every registered shortName × every
// registered NavigableField):
//
//   Given:
//     - paths: [nf.FieldPath]
//     - navigable: {nf.FieldPath: nf.TargetType}
//     - fields: empty
//     - obj: nil (AWS SDK response had nil pointer / empty slice)
//
//   Then ExtractFieldList must return a FieldItem whose Value is the
//   absent sentinel "-" AND IsNavigable is false.
//
// This test stays at the fieldpath layer (no DetailModel, no rendering,
// no RawStruct fabrication per type) because:
//   (a) The bug IS at the fieldpath layer — rendering just passes the
//       IsNavigable flag through.
//   (b) Constructing realistic-but-absent RawStruct values for every
//       registered nav path across ~50 AWS SDK types is not tractable
//       and would conflate the test with struct composition details.
//   (c) The assertion is on the returned FieldItem shape — a stable,
//       documented API of the fieldpath package.

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/fieldpath"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// TestNavigableAbsent_AbsentFields_NotNavigable iterates every registered
// resource type and every NavigableField registered for that type, calls
// ExtractFieldList with an absent value, and asserts IsNavigable is
// false on the resulting "-" FieldItem.
func TestNavigableAbsent_AbsentFields_NotNavigable(t *testing.T) {
	for _, td := range resource.AllResourceTypes() {
		for _, nf := range resource.GetNavigableFields(td.ShortName) {
			shortName := td.ShortName
			path := nf.FieldPath
			target := nf.TargetType
			t.Run(shortName+"."+path, func(t *testing.T) {
				items := fieldpath.ExtractFieldList(
					nil, // obj: simulates nil pointer from AWS SDK response
					nil, // fields: empty (fetcher populated nothing for this path)
					[]string{path},
					map[string]string{path: target},
				)
				if len(items) == 0 {
					t.Fatalf("ExtractFieldList returned no items for path %q", path)
				}
				// Find the scalar item for this path.
				var scalar *fieldpath.FieldItem
				for i := range items {
					if items[i].Path != path {
						continue
					}
					if items[i].IsHeader || items[i].IsSubField || items[i].IsSection {
						continue
					}
					scalar = &items[i]
					break
				}
				if scalar == nil {
					// Multi-line expansion: path produced a header + sub-fields.
					// In that case the header is not a navigable row either;
					// inspect the header + subs for any IsNavigable=true with
					// absent value.
					for i := range items {
						it := items[i]
						if !it.IsNavigable {
							continue
						}
						if it.Value != "-" && it.Value != "" {
							continue
						}
						t.Errorf("%s.%s: absent value rendered as navigable (dead affordance).\n"+
							"  TargetType: %s\n"+
							"  FieldItem:  Path=%q Key=%q Value=%q IsNavigable=%v IsSubField=%v IsHeader=%v\n"+
							"  Fix: in fieldpath.ExtractFieldList / buildFieldList, only set IsNavigable=true "+
							"when the resolved value is non-empty and not the absent sentinel \"-\".",
							shortName, path, target, it.Path, it.Key, it.Value, it.IsNavigable, it.IsSubField, it.IsHeader)
					}
					return
				}
				if scalar.Value != "-" {
					t.Fatalf("expected absent sentinel \"-\" for %s.%s, got Value=%q", shortName, path, scalar.Value)
				}
				if scalar.IsNavigable {
					t.Errorf("%s.%s: absent value \"-\" marked IsNavigable=true (dead affordance — Enter would navigate to nowhere).\n"+
						"  TargetType: %s\n"+
						"  Fix: in fieldpath.ExtractFieldList, only set IsNavigable=true when the resolved value "+
						"is non-empty and not the absent sentinel \"-\".",
						shortName, path, target)
				}
			})
		}
	}
}
