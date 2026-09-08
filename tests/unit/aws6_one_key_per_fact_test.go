package unit_test

// aws6_one_key_per_fact_test.go — one Fields key per fact, and the standing
// gate that keeps it that way.
//
// A fact written under two spellings is a fact that can disagree with itself:
// one writer updates the key it knows and every reader of the other spelling
// keeps rendering the stale value. The rule pinned here is that a row never
// carries the same value under two keys that NAME THE SAME FACT — two
// spellings of one name, differing only in case, underscores, or a trailing
// id/name/arn suffix.
//
// The comparison is deliberately not "any two keys with equal values". Demo
// rows carry hundreds of coincidental equalities (a desired count that happens
// to equal a minimum, two booleans both false, an account id repeated as a
// recipient), and a gate that flags those would need a permanent allowlist
// large enough to hide the real thing. Same VALUE under two spellings of the
// same NAME is the shape that is always a bug.
//
// The sweep runs over the three places a Fields map is built: the fetcher, the
// StubCreator that stands in for a row a pivot named but never listed, and the
// on-demand detail enricher.

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// oneFactAllowlist names a (source, key-a, key-b) triple that is permitted to
// carry one value under two spellings, with the reason it cannot be one key.
// It is empty: no such pair has a reason today, and a new entry needs one
// written here before the gate accepts it.
var oneFactAllowlist = map[string]string{}

// sameFactKey normalizes a Fields key to the FACT it names: case and
// underscores dropped, and one trailing id/name/arn identifier suffix removed.
// "ImageId", "image_id" and "image" all name the image; "local_profile" and
// "local_profile_id" both name the local profile.
//
// Exactly one suffix is stripped. Stripping repeatedly would fold unrelated
// keys together ("domain_name" and "domain" are the same fact, but so would
// "role_name_arn" become "role", which no key spells).
func sameFactKey(k string) string {
	n := strings.ToLower(strings.ReplaceAll(k, "_", ""))
	for _, suffix := range []string{"id", "name", "arn"} {
		if strings.HasSuffix(n, suffix) && len(n) > len(suffix) {
			return strings.TrimSuffix(n, suffix)
		}
	}
	return n
}

// duplicateFactPairs returns every pair of keys in fields that names one fact
// and carries one non-empty value, as "keyA==keyB" strings sorted for a stable
// failure message.
func duplicateFactPairs(fields map[string]string) []string {
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var pairs []string
	for i := range keys {
		for j := i + 1; j < len(keys); j++ {
			a, b := keys[i], keys[j]
			if fields[a] == "" || fields[a] != fields[b] {
				continue
			}
			if sameFactKey(a) != sameFactKey(b) {
				continue
			}
			pairs = append(pairs, a+"=="+b)
		}
	}
	return pairs
}

// reportDuplicateFacts fails for every duplicate-fact pair in fields that the
// allowlist does not excuse.
func reportDuplicateFacts(t *testing.T, source string, fields map[string]string) {
	t.Helper()
	for _, pair := range duplicateFactPairs(fields) {
		key := source + " " + pair
		if reason, ok := oneFactAllowlist[key]; ok {
			t.Logf("allowlisted duplicate fact %s: %s", key, reason)
			continue
		}
		t.Errorf("%s carries one fact under two keys (%s) with value %q — "+
			"one spelling per fact, the fetcher's snake_case key; every reader of "+
			"the other spelling must be routed to it and the duplicate deleted",
			source, pair, fields[strings.SplitN(pair, "==", 2)[0]])
	}
}

// TestAMIStubCreatorWritesOneKeyPerFact pins the AMI stub: the id a pivot
// named is one fact, so it is written once.
//
// A stub stands in for a row nobody listed, and it is the row the detail and
// the console link read. Four keys for one id means four places a later real
// fetch has to overwrite in step, and the path-cased pair exists only because
// two readers each learned a different spelling.
func TestAMIStubCreatorWritesOneKeyPerFact(t *testing.T) {
	td := resource.FindResourceType("ami")
	if td == nil {
		t.Fatal("no ami resource type registered")
	}
	if td.StubCreator == nil {
		t.Fatal("ami has no StubCreator")
	}
	stub := td.StubCreator("ami-0a1b2c3d4e5f60001")
	reportDuplicateFacts(t, "ami StubCreator", stub.Fields)
}

// TestAMIStubIDReadableUnderTheFetcherKey pins the SURVIVING spelling. Deleting
// the duplicates is only correct if the key that stays is the one the fetcher
// writes, so a stub row and a fetched row answer the same lookup.
func TestAMIStubIDReadableUnderTheFetcherKey(t *testing.T) {
	td := resource.FindResourceType("ami")
	if td == nil {
		t.Fatal("no ami resource type registered")
	}
	const id = "ami-0a1b2c3d4e5f60001"
	stub := td.StubCreator(id)

	if got := stub.Fields["image_id"]; got != id {
		t.Errorf("stub Fields[\"image_id\"] = %q, want %q — the fetcher's key is the one that survives", got, id)
	}

	rows, ok := drainVisibilityFixtures(t, *td, demo.NewServiceClients())
	if !ok || len(rows) == 0 {
		t.Fatal("no demo ami rows")
	}
	for _, r := range rows {
		if r.Fields["image_id"] != r.ID {
			t.Errorf("fetched ami row %q: Fields[\"image_id\"] = %q, want the row id — "+
				"stub and fetched rows must answer the same key", r.ID, r.Fields["image_id"])
		}
	}
}

// TestNoDemoRowCarriesAPathCasedFieldsKey is the reader half of row 1. A
// path-cased reader ("ImageId", "InstanceId") only works because some writer
// puts the value there; once no writer does, the second spelling reads nothing
// and every reader has to use the fetcher's key.
//
// Pinning the absence of the key rather than the presence of the reader is what
// makes the rule hold for readers nobody has written yet: there is no second
// spelling to find.
func TestNoDemoRowCarriesAPathCasedFieldsKey(t *testing.T) {
	clients := demo.NewServiceClients()
	byType, _ := buildVisibilityTypeCache(t)

	check := func(source string, fields map[string]string) {
		keys := make([]string, 0, len(fields))
		for k := range fields {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if strings.ToLower(k) == k {
				continue
			}
			t.Errorf("%s writes Fields[%q] — a path-cased key is a second spelling of a fact "+
				"the fetcher already writes in snake_case; delete it and route its readers to the snake key",
				source, k)
		}
	}

	for _, td := range resource.AllResourceTypes() {
		for _, r := range byType[td.ShortName] {
			check(fmt.Sprintf("%s fetcher %s", td.ShortName, r.ID), r.Fields)
		}
		if td.StubCreator != nil {
			check(td.ShortName+" StubCreator", td.StubCreator("stub-probe-id").Fields)
		}
	}

	for shortName, rows := range demoRowsIncludingChildren(t, clients, byType) {
		enricher := resource.GetDetailEnricher(shortName)
		if enricher == nil {
			continue
		}
		for _, r := range rows {
			out, err := enricher(context.Background(), newDemoDetailCtx(clients), r)
			if err != nil {
				continue
			}
			check(shortName+" enricher "+r.ID, out.Fields)
		}
	}
}

// TestTransferAgreementProfileIsOneKey pins row 2 on the demo bench: the
// agreement detail enricher resolves each profile's AS2 id once and writes it
// once.
func TestTransferAgreementProfileIsOneKey(t *testing.T) {
	clients := demo.NewServiceClients()
	rows := demoTransferAgreements(t, clients)
	if len(rows) == 0 {
		t.Fatal("no demo transfer agreements")
	}

	enricher := resource.GetDetailEnricher("transfer_agreements")
	if enricher == nil {
		t.Fatal("no detail enricher registered for transfer_agreements")
	}

	sawResolved := false
	for _, r := range rows {
		out, err := enricher(context.Background(), newDemoDetailCtx(clients), r)
		if err != nil {
			t.Fatalf("transfer_agreements enricher on %q: %v", r.ID, err)
		}
		if out.Fields["local_profile"] != "" {
			sawResolved = true
		}
		reportDuplicateFacts(t, "transfer_agreements enricher "+r.ID, out.Fields)
	}
	if !sawResolved {
		t.Fatal("no demo agreement resolved a local profile — the fixture no longer exercises the enricher this row is about")
	}
}

// TestTransferAgreementProfileReadableAfterDedup pins which spelling survives:
// the detail must still show the AS2 id after the duplicate is deleted, so the
// key that stays is the one the rendered detail reads.
func TestTransferAgreementProfileReadableAfterDedup(t *testing.T) {
	clients := demo.NewServiceClients()
	rows := demoTransferAgreements(t, clients)
	enricher := resource.GetDetailEnricher("transfer_agreements")
	if enricher == nil {
		t.Fatal("no detail enricher registered for transfer_agreements")
	}

	found := false
	for _, r := range rows {
		out, err := enricher(context.Background(), newDemoDetailCtx(clients), r)
		if err != nil {
			t.Fatalf("transfer_agreements enricher on %q: %v", r.ID, err)
		}
		if out.Fields["local_profile"] == "" {
			continue
		}
		found = true
		want := out.Fields["local_profile"]

		c := newVisibilityDetailController(t)
		c.EnsureDetailState(out, "transfer_agreements")
		body := c.Snapshot().Body.Detail
		if body == nil {
			t.Fatalf("%s: no detail body", r.ID)
		}
		var got string
		for _, f := range body.Fields {
			if strings.EqualFold(f.Path, "local_profile") || strings.EqualFold(f.Key, "Local Profile") {
				got = f.Value
			}
		}
		if got != want {
			t.Errorf("%s: rendered local profile = %q, want %q — "+
				"the surviving key must be the one the detail reads", r.ID, got, want)
		}
	}
	if !found {
		t.Fatal("no demo agreement resolved a local profile")
	}
}

// TestNoRowCarriesOneFactTwice is the standing gate row 3 asks for: the demo
// account, walked the way the app walks it, carries no row with one fact under
// two keys — through the fetcher, the StubCreator, and the detail enricher.
//
// It replaces the hand-run sweep. A hand-run parser is only as current as the
// last time somebody remembered to run it, and it missed a form once.
func TestNoRowCarriesOneFactTwice(t *testing.T) {
	clients := demo.NewServiceClients()
	byType, _ := buildVisibilityTypeCache(t)

	t.Run("fetchers", func(t *testing.T) {
		for _, td := range resource.AllResourceTypes() {
			for _, r := range byType[td.ShortName] {
				reportDuplicateFacts(t, fmt.Sprintf("%s fetcher %s", td.ShortName, r.ID), r.Fields)
			}
		}
	})

	t.Run("stub_creators", func(t *testing.T) {
		for _, td := range resource.AllResourceTypes() {
			if td.StubCreator == nil {
				continue
			}
			stub := td.StubCreator("stub-probe-id")
			reportDuplicateFacts(t, td.ShortName+" StubCreator", stub.Fields)
		}
	})

	t.Run("detail_enrichers", func(t *testing.T) {
		for shortName, rows := range demoRowsIncludingChildren(t, clients, byType) {
			enricher := resource.GetDetailEnricher(shortName)
			if enricher == nil {
				continue
			}
			for _, r := range rows {
				out, err := enricher(context.Background(), newDemoDetailCtx(clients), r)
				if err != nil {
					// A refusal is this row's subject only when it produced
					// fields; an enricher that could not run has none to check.
					continue
				}
				reportDuplicateFacts(t, shortName+" enricher "+r.ID, out.Fields)
			}
		}
	})
}

// newDemoDetailCtx builds the enrichment context the app hands a detail
// enricher, backed by the demo account.
func newDemoDetailCtx(clients *awsclient.ServiceClients) *awsclient.DetailEnrichmentCtx {
	return &awsclient.DetailEnrichmentCtx{
		Clients:    clients,
		PolicyDocs: &awsclient.PolicyDocumentCache{},
		DetailDocs: &awsclient.DetailDocCache{},
	}
}

// demoTransferAgreements drains the agreements child view for every demo
// Transfer server, through ResolveChildContext so the parent context is built
// the way navigation builds it.
func demoTransferAgreements(t *testing.T, clients *awsclient.ServiceClients) []resource.Resource {
	t.Helper()
	parent := resource.FindResourceType("transfer")
	if parent == nil {
		t.Fatal("no transfer resource type registered")
	}
	servers, ok := drainVisibilityFixtures(t, *parent, clients)
	if !ok {
		t.Fatal("transfer has no fetcher")
	}
	var out []resource.Resource
	for _, child := range parent.Children {
		if child.ChildType != "transfer_agreements" {
			continue
		}
		ctd := resource.GetChildType(child.ChildType)
		if ctd == nil || ctd.ChildFetcher == nil {
			t.Fatal("transfer_agreements has no child fetcher")
		}
		for i := range servers {
			dr := domain.Resource(servers[i])
			pctx := resource.ResolveChildContext(child, &dr, nil)
			res, err := ctd.ChildFetcher(context.Background(), clients, pctx, "")
			if err != nil {
				continue
			}
			out = append(out, res.Resources...)
		}
	}
	return out
}

// demoRowsIncludingChildren returns the demo rows of every top-level type plus
// every child view reachable from one, keyed by short name. Child rows are
// where two of the three Fields writers live, so a sweep that stops at
// top-level types cannot see them.
func demoRowsIncludingChildren(
	t *testing.T,
	clients *awsclient.ServiceClients,
	byType map[string][]resource.Resource,
) map[string][]resource.Resource {
	t.Helper()
	out := make(map[string][]resource.Resource, len(byType))
	for k, v := range byType {
		out[k] = v
	}

	var walk func(td resource.ResourceTypeDef, rows []resource.Resource, parentCtx map[string]string, depth int)
	walk = func(td resource.ResourceTypeDef, rows []resource.Resource, parentCtx map[string]string, depth int) {
		if depth > 2 {
			return
		}
		for _, child := range td.Children {
			ctd := resource.GetChildType(child.ChildType)
			if ctd == nil || ctd.ChildFetcher == nil {
				continue
			}
			for i := range rows {
				dr := domain.Resource(rows[i])
				pctx := resource.ResolveChildContext(child, &dr, parentCtx)
				res, err := ctd.ChildFetcher(context.Background(), clients, pctx, "")
				if err != nil || len(res.Resources) == 0 {
					continue
				}
				out[child.ChildType] = append(out[child.ChildType], res.Resources...)
				walk(*ctd, res.Resources, pctx, depth+1)
			}
		}
	}

	for _, td := range resource.AllResourceTypes() {
		walk(td, byType[td.ShortName], nil, 0)
	}
	return out
}
