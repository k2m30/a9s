package unit_test

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

var t562BenchTypes = []string{"ec2", "cf", "trail", "s3"} //nolint:gochecknoglobals // test-only list

func t562Bench(t *testing.T) map[string][]resource.Resource {
	t.Helper()
	clients := demo.NewServiceClients()
	byType, cache := buildVisibilityTypeCache(t)
	out := map[string][]resource.Resource{}
	for _, short := range t562BenchTypes {
		td := resource.FindResourceType(short)
		if td == nil {
			t.Fatalf("%s not registered", short)
		}
		out[short] = mergeWave2Findings(t, *td, byType[short], cache, clients)
	}
	return out
}

func t562Has(r resource.Resource, code domain.FindingCode) bool {
	for _, f := range r.Findings {
		if f.Code == code {
			return true
		}
	}
	return false
}

// The demo shows an instance reachable only over IPv6 with its exposure, and
// one on the same kind of address that is not exposed, so the route rule is
// visible holding and failing.
func TestT562_DemoIPv6OnlyInstancesShowTheRouteRule(t *testing.T) {
	td := resource.FindResourceType("ec2")
	rows := t562Bench(t)["ec2"]
	var exposed, quiet []resource.Resource
	for _, r := range rows {
		if r.Fields["public_ip"] != "" || r.Fields["ipv6_address"] == "" || r.Fields["state"] != "running" {
			continue
		}
		if t562Has(r, "ec2.internet-exposed") || t562Has(r, "ec2.internet-exposed-all") {
			exposed = append(exposed, r)
		} else {
			quiet = append(quiet, r)
		}
	}
	if len(exposed) == 0 {
		t.Fatal("no running demo instance without a public IPv4 address carries an internet-exposure finding over IPv6")
	}
	if len(quiet) == 0 {
		t.Fatal("no running demo instance with only an IPv6 address is left unexposed; the egress-only case is not shown")
	}
	for _, r := range exposed {
		top, _ := domain.TopFinding(r.Findings)
		if !strings.HasSuffix(top.Phrase, "reachable from the internet") {
			t.Errorf("%s top finding = %s %q, want the exposure", r.ID, top.Code, top.Phrase)
		}
		cell, ok := listStatusCellFor(t, *td, rows, r.ID)
		if !ok {
			t.Fatalf("no list row for %s", r.ID)
		}
		if got, _, _ := strings.Cut(cell, " (+"); got != top.Phrase {
			t.Errorf("%s Status cell = %q, want %q", r.ID, cell, top.Phrase)
		}
		if got, want := td.ResolveColor(r), w5ColorOfSeverity(top.Severity); got != want {
			t.Errorf("%s colour = %v, want %v (the colour of its top finding)", r.ID, got, want)
		}
	}
}

// The demo shows a distribution whose default behaviour redirects to HTTPS and
// whose ordered behaviour allows plain HTTP, named by its path.
func TestT562_DemoDistributionNamesTheBehaviourAllowingHTTP(t *testing.T) {
	for _, r := range t562Bench(t)["cf"] {
		for _, row := range r.AttentionDetails["cf.insecure-protocol"].Rows {
			if row.Label == "Viewer protocol policy" && strings.Contains(row.Value, "/") {
				return
			}
		}
	}
	t.Fatal("no demo distribution's \"Viewer protocol policy\" row names a cache-behaviour path")
}

// A trail's log bucket is public exactly when the bucket list calls it public,
// and the demo holds one public by ACL grant alone.
func TestT562_DemoTrailLogBucketVerdictMatchesTheBucketList(t *testing.T) {
	bench := t562Bench(t)
	buckets := map[string]resource.Resource{}
	for _, b := range bench["s3"] {
		buckets[b.ID] = b
	}
	compared, aclOnly := 0, 0
	for _, tr := range bench["trail"] {
		b, ok := buckets[tr.Fields["s3_bucket"]]
		if !ok {
			continue
		}
		compared++
		trailPublic, bucketPublic := t562Has(tr, "trail.log-bucket-public"), t562Has(b, "s3.public")
		if trailPublic != bucketPublic {
			t.Errorf("trail %s: log bucket public = %v, bucket %s s3.public = %v", tr.ID, trailPublic, b.ID, bucketPublic)
		}
		if !bucketPublic {
			continue
		}
		var byACL, byPolicy bool
		for _, row := range b.AttentionDetails["s3.public"].Rows {
			byACL = byACL || row.Label == "Access control list"
			byPolicy = byPolicy || row.Label == "Policy status"
		}
		if byACL && !byPolicy {
			aclOnly++
		}
	}
	if compared == 0 {
		t.Fatal("no demo trail delivers to a bucket on the demo bucket list")
	}
	if aclOnly == 0 {
		t.Error("no demo trail delivers to a bucket public by ACL grant alone")
	}
}

// Row colour and Status cell select the same finding, and a supporting row
// adds words: it neither restates its phrase nor prints a Go bool literal or
// SDK enum casing (a trailing parenthesised aside is stripped first).
func TestT562_BenchRowsSelectOneFindingAndAddWords(t *testing.T) {
	multi := 0
	for short, rows := range t562Bench(t) {
		td := resource.FindResourceType(short)
		for _, r := range rows {
			top, ok := domain.TopFinding(r.Findings)
			if !ok {
				continue
			}
			if len(r.Findings) > 1 {
				multi++
			}
			if got, want := td.ResolveColor(r), w5ColorOfSeverity(top.Severity); got != want {
				t.Errorf("%s/%s: colour %v, top finding %q is %v", short, r.ID, got, top.Code, want)
			}
			if got, _, _ := strings.Cut(domain.StatusPhrase(r.Findings), " (+"); got != top.Phrase {
				t.Errorf("%s/%s: status names %q, colour's top finding is %q", short, r.ID, got, top.Phrase)
			}
			for _, f := range r.Findings {
				phrase := w5Normalize(f.Phrase)
				for _, row := range r.AttentionDetails[f.Code].Rows {
					if w5Normalize(row.Label+" "+row.Value) == phrase || w5Normalize(row.Value) == phrase {
						t.Errorf("%s/%s %s: row %q=%q restates the phrase %q", short, r.ID, f.Code, row.Label, row.Value, f.Phrase)
					}
					v := strings.TrimSpace(row.Value)
					if i := strings.LastIndex(v, " ("); i > 0 && strings.HasSuffix(v, ")") {
						v = strings.TrimSpace(v[:i])
					}
					switch {
					case v == "true" || v == "false":
						t.Errorf("%s/%s %s row %q = %q: a Go bool literal", short, r.ID, f.Code, row.Label, row.Value)
					case v != "" && v != "*" && v != strings.ToLower(v) && v == strings.ToUpper(v) && !strings.ContainsAny(v, "0123456789"):
						t.Errorf("%s/%s %s row %q = %q: SDK enum casing", short, r.ID, f.Code, row.Label, row.Value)
					}
				}
			}
		}
	}
	if multi == 0 {
		t.Error("no bench row carries more than one finding; the selector check cannot fail")
	}
}
