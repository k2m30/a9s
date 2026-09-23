package unit_test

import (
	"context"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	acmtypes "github.com/aws/aws-sdk-go-v2/service/acm/types"
	"github.com/aws/aws-sdk-go-v2/service/opensearch"
	ostypes "github.com/aws/aws-sdk-go-v2/service/opensearch/types"
	"github.com/aws/aws-sdk-go-v2/service/wafv2"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

var (
	t563ParenAside = regexp.MustCompile(`\s*\([^)]*\)$`)
	t563SDKEnum    = regexp.MustCompile(`^[A-Z][A-Z0-9]*(_[A-Z0-9]+)+$`)
)

// t563AssertWitnessRendered checks a row as the list renders it: the Status
// cell leads with the phrase of domain.TopFinding and the row colour is that
// finding's severity colour; in the detail Attention block no supporting row
// repeats the phrase and no value is a Go bool or an SDK enum token.
func t563AssertWitnessRendered(t *testing.T, td resource.ResourceTypeDef, rows []resource.Resource, r resource.Resource) domain.Finding {
	t.Helper()
	top, ok := domain.TopFinding(r.Findings)
	if !ok {
		t.Fatalf("%s/%s carries no finding", td.ShortName, r.ID)
	}
	cell, found := listStatusCellFor(t, td, rows, r.ID)
	if !found {
		t.Fatalf("%s/%s: no Status cell rendered", td.ShortName, r.ID)
	}
	if !strings.HasPrefix(cell, top.Phrase) {
		t.Errorf("%s/%s Status cell = %q, want it to lead with %q", td.ShortName, r.ID, cell, top.Phrase)
	}
	if got, want := td.ResolveColor(r), resource.ColorFromSeverity(top.Severity); got != want {
		t.Errorf("%s/%s row colour = %v, want %v from %s", td.ShortName, r.ID, got, want, top.Code)
	}
	restated := 0
	for _, v := range detailAttentionValuesFor(t, r, td.ShortName) {
		if v == top.Phrase {
			restated++
		}
		bare := strings.TrimSpace(t563ParenAside.ReplaceAllString(v, ""))
		if bare == "true" || bare == "false" || t563SDKEnum.MatchString(bare) {
			t.Errorf("%s/%s Attention value %q is a Go bool or SDK enum casing", td.ShortName, r.ID, v)
		}
	}
	if restated > 1 {
		t.Errorf("%s/%s: a supporting Attention row restates the phrase %q", td.ShortName, r.ID, top.Phrase)
	}
	return top
}

func t563Merged(t *testing.T, shortName string) (resource.ResourceTypeDef, []resource.Resource) {
	t.Helper()
	byType, cache := buildVisibilityTypeCache(t)
	td := resource.FindResourceType(shortName)
	if td == nil {
		t.Fatalf("%s not registered", shortName)
	}
	rows := byType[shortName]
	if len(rows) == 0 {
		t.Fatalf("%s: no demo rows", shortName)
	}
	return *td, mergeWave2Findings(t, *td, rows, cache, demo.NewServiceClients())
}

// The demo account holds an ACME-issued certificate, and the list shows it
// with its expiry signal.
func TestT563Demo_ACMListShowsTheACMEIssuedCertificate(t *testing.T) {
	td, rows := t563Merged(t, "acm")
	for _, r := range rows {
		cert, ok := r.RawStruct.(acmtypes.CertificateSummary)
		if !ok || cert.CertificateKeyPairOrigin != acmtypes.CertificateKeyPairOriginAcme {
			continue
		}
		top := t563AssertWitnessRendered(t, td, rows, r)
		switch top.Code {
		case "acm.expires-soon", "acm.expires-critical", "acm.expired":
		default:
			t.Errorf("ACME certificate %s top finding = %s, want its expiry signal", r.ID, top.Code)
		}
		return
	}
	t.Fatalf("no demo ACM row has CertificateKeyPairOrigin ACME among %d rows", len(rows))
}

// A REGIONAL web ACL protecting only an API Gateway stage is associated; the
// ACL protecting nothing still reads "not associated". The demo WAFv2 client
// answers an empty ResourceType as APPLICATION_LOAD_BALANCER, as AWS does,
// so the demo cannot hide a check that asks about load balancers alone.
func TestT563Demo_WAFApiGatewayOnlyACLIsNotAnOrphan(t *testing.T) {
	const apiOnly = "arn:aws:wafv2:us-east-1:123456789012:regional/webacl/acme-prod-api-waf/a1b2c3d4-5678-90ab-cdef-111111111111"

	out, err := demo.NewServiceClients().WAFv2.ListResourcesForWebACL(context.Background(), &wafv2.ListResourcesForWebACLInput{WebACLArn: aws.String(apiOnly)})
	if err != nil {
		t.Fatalf("demo ListResourcesForWebACL: %v", err)
	}
	if len(out.ResourceArns) != 0 {
		t.Errorf("demo answers an unnamed ResourceType for an API-Gateway-only ACL with %v; AWS reads it as APPLICATION_LOAD_BALANCER and answers none", out.ResourceArns)
	}

	td, rows := t563Merged(t, "waf")
	orphanPhrase := catalog.Phrase("waf.orphan")
	var sawAPIOnly, sawOrphan bool
	for _, r := range rows {
		switch {
		case r.ID == apiOnly || r.Fields["arn"] == apiOnly:
			sawAPIOnly = true
			for _, f := range r.Findings {
				if f.Code == "waf.orphan" {
					t.Errorf("%s protects an API Gateway stage but carries waf.orphan", r.Name)
				}
			}
			if cell, ok := listStatusCellFor(t, td, rows, r.ID); ok && strings.Contains(cell, orphanPhrase) {
				t.Errorf("%s Status cell = %q, names the orphan phrase", r.Name, cell)
			}
		case r.Name == fixtures.WAFOrphan:
			sawOrphan = true
			if top := t563AssertWitnessRendered(t, td, rows, r); top.Code != "waf.orphan" {
				t.Errorf("%s top finding = %s, want waf.orphan", r.Name, top.Code)
			}
		}
	}
	if !sawAPIOnly || !sawOrphan {
		t.Fatalf("demo waf rows: API-Gateway-only ACL present = %v, orphan ACL present = %v", sawAPIOnly, sawOrphan)
	}
}

// The demo region holds more OpenSearch domains than one DescribeDomains call
// accepts, and every one AWS can describe is listed once with its engine,
// instance type and endpoint rather than as a degraded name-only row. The
// demo client answers DescribeDomains as AWS does — only the named domains, and a
// ValidationException past five names — so the demo cannot hide a fetcher
// that asks for every domain at once.
func TestT563Demo_OpenSearchListsSixOrMoreDescribedDomains(t *testing.T) {
	api := demo.NewServiceClients().OpenSearch
	names, err := api.ListDomainNames(context.Background(), &opensearch.ListDomainNamesInput{})
	if err != nil {
		t.Fatalf("demo ListDomainNames: %v", err)
	}
	var all []string
	for _, d := range names.DomainNames {
		all = append(all, aws.ToString(d.DomainName))
	}
	if len(all) < 6 {
		t.Fatalf("demo region holds %d OpenSearch domains, want at least 6", len(all))
	}
	if _, sixErr := api.DescribeDomains(context.Background(), &opensearch.DescribeDomainsInput{DomainNames: all[:6]}); sixErr == nil {
		t.Errorf("demo DescribeDomains accepted six names; AWS rejects more than five")
	}
	one, err := api.DescribeDomains(context.Background(), &opensearch.DescribeDomainsInput{DomainNames: all[:1]})
	if err != nil {
		t.Errorf("demo DescribeDomains for %q: %v", all[0], err)
	} else if len(one.DomainStatusList) != 1 || aws.ToString(one.DomainStatusList[0].DomainName) != all[0] {
		t.Errorf("demo DescribeDomains for %q answered %d domains, want exactly that one", all[0], len(one.DomainStatusList))
	}

	_, rows := t563Merged(t, "opensearch")
	seen := map[string]bool{}
	described := 0
	for _, r := range rows {
		if seen[r.ID] {
			t.Errorf("%s listed twice", r.ID)
		}
		seen[r.ID] = true
		if r.Fields[awsclient.DegradedFindingField] != "" {
			alone, err := api.DescribeDomains(context.Background(), &opensearch.DescribeDomainsInput{DomainNames: []string{r.ID}})
			if err == nil && slices.ContainsFunc(alone.DomainStatusList, func(d ostypes.DomainStatus) bool { return aws.ToString(d.DomainName) == r.ID }) {
				t.Errorf("%s is listed degraded (%q) but DescribeDomains answers it when asked alone", r.ID, r.Fields[awsclient.DegradedFindingField])
			}
			continue
		}
		described++
		for _, k := range []string{"engine_version", "instance_type", "endpoint"} {
			if r.Fields[k] == "" {
				t.Errorf("%s Fields[%q] is empty", r.ID, k)
			}
		}
	}
	if described < 6 {
		t.Errorf("demo lists %d described OpenSearch domains, want at least 6", described)
	}
}
