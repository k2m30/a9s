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

// TestNamingContract_IssueEnricherRegistry_IsMap pins the Wave 2 registry name
// and shape. The registry key is a resource short name and the value is an
// IssueEnricher with Fn + Priority metadata.
func TestNamingContract_IssueEnricherRegistry_IsMap(t *testing.T) {
	rt := reflect.TypeOf(awsclient.IssueEnricherRegistry)
	if rt.Kind() != reflect.Map {
		t.Fatalf("awsclient.IssueEnricherRegistry must be a map, got %v", rt.Kind())
	}
	if rt.Key().Kind() != reflect.String {
		t.Fatalf("IssueEnricherRegistry key must be string, got %v", rt.Key().Kind())
	}
	if rt.Elem() != reflect.TypeOf(awsclient.IssueEnricher{}) {
		t.Fatalf("IssueEnricherRegistry value must be awsclient.IssueEnricher, got %v", rt.Elem())
	}
}

// TestNamingContract_NoOpIssueEnricher_ReturnsEmptyResult pins the no-op
// Wave 2 issue enricher used for resource types with no Wave 2 coverage.
func TestNamingContract_NoOpIssueEnricher_ReturnsEmptyResult(t *testing.T) {
	res, err := awsclient.NoOpIssueEnricher(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("NoOpIssueEnricher must not error, got %v", err)
	}
	if res.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0", res.IssueCount)
	}
	if res.Truncated {
		t.Error("NoOpIssueEnricher must not be truncated")
	}
	if len(res.Findings) != 0 {
		t.Errorf("Findings must be empty, got %d entries", len(res.Findings))
	}
}
