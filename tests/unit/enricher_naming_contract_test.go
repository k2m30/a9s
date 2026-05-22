package unit

// enricher_naming_contract_test.go — Pins the disambiguated names introduced in
// issue #276. Detail enrichment and Wave 2 issue enrichment are two distinct
// subsystems and must have disjoint, unambiguous type names.
//
// If this file ever needs to change its names back to bare "Enricher", revisit
// issue #276 first — reintroducing the ambiguity is a regression.

import (
	"context"
	"reflect"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// TestNamingContract_DetailEnricher_IsFunc verifies the detail enricher
// contract lives in internal/resource as a function type with the
// (ctx, clients, Resource) -> (Resource, error) signature.
func TestNamingContract_DetailEnricher_IsFunc(t *testing.T) {
	var fn resource.DetailEnricher = func(_ context.Context, _ any, r resource.Resource) (resource.Resource, error) {
		return r, nil
	}
	rt := reflect.TypeOf(fn)
	if rt.Kind() != reflect.Func {
		t.Fatalf("resource.DetailEnricher must be a func type, got %v", rt.Kind())
	}
	if rt.NumIn() != 3 {
		t.Fatalf("resource.DetailEnricher must take 3 arguments, got %d", rt.NumIn())
	}
	if rt.NumOut() != 2 {
		t.Fatalf("resource.DetailEnricher must return 2 values, got %d", rt.NumOut())
	}
}

// TestNamingContract_IssueEnricher_IsStructWithFnAndPriority pins the Wave 2
// issue enricher as a struct carrying an IssueEnricherFunc plus Priority.
func TestNamingContract_IssueEnricher_IsStructWithFnAndPriority(t *testing.T) {
	rt := reflect.TypeOf(awsclient.IssueEnricher{})
	if rt.Kind() != reflect.Struct {
		t.Fatalf("awsclient.IssueEnricher must be a struct type, got %v", rt.Kind())
	}
	if _, ok := rt.FieldByName("Fn"); !ok {
		t.Fatal("awsclient.IssueEnricher must expose an Fn field")
	}
	if _, ok := rt.FieldByName("Priority"); !ok {
		t.Fatal("awsclient.IssueEnricher must expose a Priority field")
	}
}

// TestNamingContract_Wave2Accessors_Shape pins the post-AS-795n Wave 2
// accessor API on awsclient. Wave2EnricherFor returns the enricher value
// plus a found bool; AllWave2 returns ordered Wave2Entry pairs.
func TestNamingContract_Wave2Accessors_Shape(t *testing.T) {
	// Wave2EnricherFor must be (string) (IssueEnricher, bool).
	lookupT := reflect.TypeOf(awsclient.Wave2EnricherFor)
	if lookupT.Kind() != reflect.Func {
		t.Fatalf("awsclient.Wave2EnricherFor must be a func, got %v", lookupT.Kind())
	}
	if lookupT.NumIn() != 1 || lookupT.In(0).Kind() != reflect.String {
		t.Fatalf("Wave2EnricherFor must take a single string, got %v", lookupT)
	}
	if lookupT.NumOut() != 2 ||
		lookupT.Out(0) != reflect.TypeOf(awsclient.IssueEnricher{}) ||
		lookupT.Out(1).Kind() != reflect.Bool {
		t.Fatalf("Wave2EnricherFor must return (IssueEnricher, bool), got %v", lookupT)
	}

	// AllWave2 must be () []Wave2Entry; Wave2Entry must expose ShortName + Enricher.
	allT := reflect.TypeOf(awsclient.AllWave2)
	if allT.Kind() != reflect.Func || allT.NumIn() != 0 || allT.NumOut() != 1 {
		t.Fatalf("awsclient.AllWave2 must be func() []Wave2Entry, got %v", allT)
	}
	if allT.Out(0).Kind() != reflect.Slice {
		t.Fatalf("AllWave2 must return a slice, got %v", allT.Out(0))
	}
	entryT := allT.Out(0).Elem()
	if entryT.Kind() != reflect.Struct {
		t.Fatalf("Wave2Entry must be a struct, got %v", entryT.Kind())
	}
	if _, ok := entryT.FieldByName("ShortName"); !ok {
		t.Error("Wave2Entry must expose a ShortName field")
	}
	if _, ok := entryT.FieldByName("Enricher"); !ok {
		t.Error("Wave2Entry must expose an Enricher field")
	}
}

// TestNamingContract_InFetcherWave2Sentinel_ReturnsEmptyResult pins the
// in-fetcher Wave 2 sentinel used for resource types whose Wave 2 work is
// performed by the fetcher itself (e.g. EKS DescribeCluster, EKS Node Group
// DescribeNodegroup, CloudTrail GetTrailStatus). The sentinel Fn is wired
// into IssueEnricher{} so TestAttentionSignalsDoc sees a non-nil Wave2
// without scheduling a redundant background enrichment pass.
// Renamed from NoOpIssueEnricher in AS-731 to make the in-fetcher
// contract explicit and to satisfy the zero-hits grep on `NoOpIssueEnricher`.
func TestNamingContract_InFetcherWave2Sentinel_ReturnsEmptyResult(t *testing.T) {
	res, err := awsclient.InFetcherWave2Sentinel(context.Background(), nil, nil, nil)
	if err != nil {
		t.Fatalf("InFetcherWave2Sentinel must not error, got %v", err)
	}
	if res.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0", res.IssueCount)
	}
	if res.Truncated {
		t.Error("InFetcherWave2Sentinel must not be truncated")
	}
	if len(res.Findings) != 0 {
		t.Errorf("Findings must be empty, got %d entries", len(res.Findings))
	}
}
