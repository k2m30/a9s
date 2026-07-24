package unit

// detail_enricher_contract_test.go — catalog-driven contract test for every
// registered on-demand detail enricher (resource.DetailEnricher), discovered
// dynamically via resource.AllResourceTypes() (plus each type's declared
// Children) rather than one hand-written test per enricher. Any FUTURE
// detail enricher gets this coverage automatically, with zero new test code.
//
// Frozen contract every enricher (core/aws/*_detail_enrichment.go,
// core/aws/detail_enrich_engine.go, core/aws/transfer_children.go) must
// honor, pinned here:
//   - garbage `clients any` (not a *DetailEnrichmentCtx) → non-nil error
//     containing "detail-enrichment context", resource unchanged, no panic
//   - nil typed (*DetailEnrichmentCtx)(nil) → same
//   - unexpected RawStruct type (with an otherwise-valid ctx) → non-nil
//     error, resource unchanged, no panic
//   - nil RawStruct (a disk-cache-seeded row: cache.Row carries only
//     ID/Name/Fields/Findings, no RawStruct, until the live refetch lands) →
//     unchanged resource, nil error, no panic — a silent, self-healing skip,
//     not a flashed error. Uniform across every enricher, including the
//     off-engine transfer_agreements (core/aws/transfer_children.go), which
//     carries the identical guard inline rather than through
//     core/aws/detail_enrich_engine.go (see nilRawStructWantErrSubstring
//     below for the one-line override this axis needs if a future enricher
//     legitimately diverges).
//   - registry count sanity: at least 9 enrichers registered (catches a
//     catalog rewire silently dropping registrations)
//
// A parallel refactor is migrating individual enrichers onto a shared
// generic engine (core/aws/detail_enrich_engine.go) — these guard behaviors
// are the frozen contract that refactor must preserve, so this test is
// written against the observable contract, not any specific enricher's
// internals.

import (
	"context"
	"reflect"
	"strings"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

// discoverDetailEnricherShortNames walks the catalog (every top-level type
// plus every declared child view) and returns the short names that have a
// registered DetailEnricher — the live, current set, not a hand-maintained
// list, so a newly added enricher (top-level or child) is picked up
// automatically.
func discoverDetailEnricherShortNames(t *testing.T) []string {
	t.Helper()
	seen := make(map[string]bool)
	var names []string
	add := func(shortName string) {
		if shortName == "" || seen[shortName] {
			return
		}
		if resource.GetDetailEnricher(shortName) == nil {
			return
		}
		seen[shortName] = true
		names = append(names, shortName)
	}
	for _, rt := range resource.AllResourceTypes() {
		add(rt.ShortName)
		for _, child := range rt.Children {
			add(child.ChildType)
		}
	}
	return names
}

// enricherCtxErrOverrides holds a per-shortName override for the substring
// expected in the garbage-clients/nil-ctx error message, for the rare
// enricher whose wording legitimately differs from the uniform
// "detail-enrichment context" guard. An empty map entry value means "any
// non-nil error, no substring check".
//
// Verified empirically: every currently registered enricher, including
// transfer_agreements (core/aws/transfer_children.go), uses the uniform
// fmt.Errorf("invalid detail-enrichment context") wording — this map starts
// empty. It exists so a future divergent enricher is a one-line table entry
// here, not a special case in the test logic below.
var enricherCtxErrOverrides = map[string]string{}

const uniformCtxErrSubstring = "detail-enrichment context"

func expectedCtxErrSubstring(shortName string) string {
	if override, ok := enricherCtxErrOverrides[shortName]; ok {
		return override
	}
	return uniformCtxErrSubstring
}

// nilRawStructWantErrSubstring holds a per-shortName override for the
// nil-RawStruct axis, mirroring enricherCtxErrOverrides above. Absent from
// this map means the uniform contract: nil error (a disk-cache-seeded row is
// silently deferred, not flashed as an error). Verified empirically: every
// currently registered enricher, including the off-engine
// transfer_agreements, honors this — this map starts empty so a future
// divergent enricher is a one-line table entry here, not a special case in
// the test logic below.
var nilRawStructWantErrSubstring = map[string]string{}

// runGuarded runs fn and turns any panic into a t.Fatal with a clear message
// instead of crashing the whole test binary — a panicking enricher is a
// contract violation this test must report, not a suite-ending crash.
func runGuarded(t *testing.T, fn func(t *testing.T)) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("enricher panicked: %v", r)
		}
	}()
	fn(t)
}

func TestDetailEnricherContract(t *testing.T) {
	shortNames := discoverDetailEnricherShortNames(t)

	if len(shortNames) < 9 {
		t.Fatalf("discovered only %d detail enrichers (%v), want at least 9 — "+
			"a catalog rewire may have silently dropped registrations", len(shortNames), shortNames)
	}

	for _, shortName := range shortNames {
		enricher := resource.GetDetailEnricher(shortName)

		t.Run(shortName+"/garbage_clients", func(t *testing.T) {
			runGuarded(t, func(t *testing.T) {
				res := resource.Resource{ID: "contract-test-id"}
				got, err := enricher(context.Background(), 42, res)
				if err == nil {
					t.Fatalf("%s: expected error for garbage clients arg, got nil", shortName)
				}
				wantSubstr := expectedCtxErrSubstring(shortName)
				if wantSubstr != "" && !strings.Contains(err.Error(), wantSubstr) {
					t.Errorf("%s: error = %q, want it to contain %q", shortName, err.Error(), wantSubstr)
				}
				if !reflect.DeepEqual(got, res) {
					t.Errorf("%s: resource changed on garbage-clients error; got %+v, want unchanged %+v", shortName, got, res)
				}
			})
		})

		t.Run(shortName+"/nil_typed_ctx", func(t *testing.T) {
			runGuarded(t, func(t *testing.T) {
				res := resource.Resource{ID: "contract-test-id"}
				got, err := enricher(context.Background(), (*awsclient.DetailEnrichmentCtx)(nil), res)
				if err == nil {
					t.Fatalf("%s: expected error for nil typed DetailEnrichmentCtx, got nil", shortName)
				}
				wantSubstr := expectedCtxErrSubstring(shortName)
				if wantSubstr != "" && !strings.Contains(err.Error(), wantSubstr) {
					t.Errorf("%s: error = %q, want it to contain %q", shortName, err.Error(), wantSubstr)
				}
				if !reflect.DeepEqual(got, res) {
					t.Errorf("%s: resource changed on nil-ctx error; got %+v, want unchanged %+v", shortName, got, res)
				}
			})
		})

		t.Run(shortName+"/unexpected_raw_struct", func(t *testing.T) {
			runGuarded(t, func(t *testing.T) {
				ctx := &awsclient.DetailEnrichmentCtx{
					Clients:    &awsclient.ServiceClients{},
					PolicyDocs: &awsclient.PolicyDocumentCache{},
					DetailDocs: &awsclient.DetailDocCache{},
				}
				res := resource.Resource{ID: "contract-test-id", RawStruct: struct{ X int }{1}}
				got, err := enricher(context.Background(), ctx, res)
				if err == nil {
					t.Fatalf("%s: expected error for unexpected RawStruct type, got nil", shortName)
				}
				if !reflect.DeepEqual(got, res) {
					t.Errorf("%s: resource changed on bad-RawStruct error; got %+v, want unchanged %+v", shortName, got, res)
				}
			})
		})

		t.Run(shortName+"/nil_raw_struct", func(t *testing.T) {
			runGuarded(t, func(t *testing.T) {
				ctx := &awsclient.DetailEnrichmentCtx{
					Clients:    &awsclient.ServiceClients{},
					PolicyDocs: &awsclient.PolicyDocumentCache{},
					DetailDocs: &awsclient.DetailDocCache{},
				}
				res := resource.Resource{ID: "contract-test-id", RawStruct: nil}
				got, err := enricher(context.Background(), ctx, res)

				if wantErrSubstr, override := nilRawStructWantErrSubstring[shortName]; override {
					if err == nil || !strings.Contains(err.Error(), wantErrSubstr) {
						t.Errorf("%s: error = %v, want it to contain %q", shortName, err, wantErrSubstr)
					}
				} else if err != nil {
					t.Errorf("%s: expected nil error for nil RawStruct (cache-seeded row), got %v", shortName, err)
				}
				if !reflect.DeepEqual(got, res) {
					t.Errorf("%s: resource changed on nil RawStruct; got %+v, want unchanged %+v", shortName, got, res)
				}
			})
		})

		// The two subtests below pin engine validation order (#261
		// boundary-sealing wave, item c): a nil RawStruct with a nil Clients
		// must still silently skip (a pre-connect open of a disk-cache-seeded
		// row has a non-nil *DetailEnrichmentCtx but a nil Clients — it must
		// hit the RawStruct-nil skip, not a Clients error), while a non-nil
		// RawStruct with a nil Clients must still error — Clients is required
		// once there is real work to do.
		t.Run(shortName+"/nil_raw_struct_nil_clients", func(t *testing.T) {
			runGuarded(t, func(t *testing.T) {
				ctx := &awsclient.DetailEnrichmentCtx{
					PolicyDocs: &awsclient.PolicyDocumentCache{},
					DetailDocs: &awsclient.DetailDocCache{},
				}
				res := resource.Resource{ID: "contract-test-id", RawStruct: nil}
				got, err := enricher(context.Background(), ctx, res)

				if err != nil {
					t.Errorf("%s: expected nil error for nil RawStruct with nil Clients (pre-connect, cache-seeded row) — the RawStruct-nil check must run before the Clients-nil check in EVERY enricher, engine-based or hand-rolled, got %v", shortName, err)
				}
				if !reflect.DeepEqual(got, res) {
					t.Errorf("%s: resource changed on nil-RawStruct/nil-Clients path; got %+v, want unchanged %+v", shortName, got, res)
				}
			})
		})

		t.Run(shortName+"/raw_struct_present_nil_clients", func(t *testing.T) {
			runGuarded(t, func(t *testing.T) {
				ctx := &awsclient.DetailEnrichmentCtx{
					PolicyDocs: &awsclient.PolicyDocumentCache{},
					DetailDocs: &awsclient.DetailDocCache{},
				}
				res := resource.Resource{ID: "contract-test-id", RawStruct: struct{ X int }{1}}
				got, err := enricher(context.Background(), ctx, res)

				if err == nil {
					t.Fatalf("%s: expected error for a non-nil RawStruct with nil Clients, got nil", shortName)
				}
				if !reflect.DeepEqual(got, res) {
					t.Errorf("%s: resource changed on RawStruct-present/nil-Clients error; got %+v, want unchanged %+v", shortName, got, res)
				}
			})
		})
	}
}
