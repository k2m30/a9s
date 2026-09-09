package unit

// prowler_w2_opensearch_test.go — opensearch rows 22–24 of the w2 Prowler
// batch: reachable outside a VPC, HTTPS not enforced, node-to-node encryption
// off.
//
// EnrichOpenSearchDomains makes no API call — it reads Fields the fetcher
// populated. Driving the fetcher and the enricher back to back is therefore
// the only way to prove the whole path: a field the fetcher never writes makes
// the enricher silently clean, which no enricher-only test would catch.

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/opensearch"
	ostypes "github.com/aws/aws-sdk-go-v2/service/opensearch/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

const (
	w2OSCodePublic   = "opensearch.public"
	w2OSCodeHTTPSOff = "opensearch.https-not-enforced"
	w2OSCodeN2NOff   = "opensearch.node-to-node-tls-off"
	// d1 moved these three checks to wave 1 with the rest of the type's signals.
	w2OSSource       = "wave1"
	w2OSPublicPolicy = `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":"*","Action":"es:*","Resource":"arn:aws:es:eu-central-1:123456789012:domain/acme-search/*"}]}`
	w2OSScopedPolicy = `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":"arn:aws:iam::123456789012:role/app"},"Action":"es:ESHttpGet","Resource":"*"}]}`
	w2OSCondPolicy   = `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":"*","Action":"es:ESHttpGet","Resource":"*","Condition":{"IpAddress":{"aws:SourceIp":["203.0.113.0/24"]}}}]}`
)

var w2OSNow = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

type w2OSFake struct {
	domains []ostypes.DomainStatus
}

func (f *w2OSFake) ListDomainNames(_ context.Context, _ *opensearch.ListDomainNamesInput, _ ...func(*opensearch.Options)) (*opensearch.ListDomainNamesOutput, error) {
	names := make([]ostypes.DomainInfo, 0, len(f.domains))
	for _, d := range f.domains {
		names = append(names, ostypes.DomainInfo{DomainName: d.DomainName, EngineType: ostypes.EngineTypeOpenSearch})
	}
	return &opensearch.ListDomainNamesOutput{DomainNames: names}, nil
}

func (f *w2OSFake) DescribeDomains(_ context.Context, _ *opensearch.DescribeDomainsInput, _ ...func(*opensearch.Options)) (*opensearch.DescribeDomainsOutput, error) {
	return &opensearch.DescribeDomainsOutput{DomainStatusList: f.domains}, nil
}

// w2OSDomain returns an otherwise-healthy VPC-attached domain: HTTPS enforced,
// node-to-node encryption on, encryption at rest on, a same-account access
// policy, no pending software update.
func w2OSDomain(name string) ostypes.DomainStatus {
	return ostypes.DomainStatus{
		DomainName:    aws.String(name),
		DomainId:      aws.String("123456789012/" + name),
		ARN:           aws.String("arn:aws:es:eu-central-1:123456789012:domain/" + name),
		EngineVersion: aws.String("OpenSearch_2.13"),
		Created:       aws.Bool(true),
		Deleted:       aws.Bool(false),
		Processing:    aws.Bool(false),
		Endpoint:      aws.String("vpc-" + name + ".eu-central-1.es.amazonaws.com"),
		ClusterConfig: &ostypes.ClusterConfig{
			InstanceType:  ostypes.OpenSearchPartitionInstanceTypeT3SmallSearch,
			InstanceCount: aws.Int32(2),
		},
		VPCOptions: &ostypes.VPCDerivedInfo{
			VPCId:     aws.String("vpc-0acme00000000001"),
			SubnetIds: []string{"subnet-0acme0000000001"},
		},
		AccessPolicies:              aws.String(w2OSScopedPolicy),
		DomainEndpointOptions:       &ostypes.DomainEndpointOptions{EnforceHTTPS: aws.Bool(true)},
		NodeToNodeEncryptionOptions: &ostypes.NodeToNodeEncryptionOptions{Enabled: aws.Bool(true)},
		EncryptionAtRestOptions:     &ostypes.EncryptionAtRestOptions{Enabled: aws.Bool(true)},
		ServiceSoftwareOptions:      &ostypes.ServiceSoftwareOptions{UpdateAvailable: aws.Bool(false), CurrentVersion: aws.String("R20240101")},
	}
}

// w2OSRun drives the fetcher and then the registered enricher, returning both
// the produced resources and the enrichment result.
func w2OSRun(t *testing.T, domains ...ostypes.DomainStatus) (map[string]resource.Resource, awsclient.IssueEnricherResult) {
	t.Helper()
	fake := &w2OSFake{domains: domains}
	rs, err := awsclient.FetchOpenSearchDomainsAt(context.Background(), fake, fake, w2OSNow)
	if err != nil {
		t.Fatalf("FetchOpenSearchDomainsAt: %v", err)
	}
	// Inverted in d1: the three network-posture checks read the DescribeDomains
	// response the fetcher already holds and made no AWS call, so they are wave-1
	// findings on the row and opensearch registers no wave 2 at all. The result
	// is rebuilt from the fetched rows so the assertions below are unchanged.
	res := awsclient.IssueEnricherResult{
		Findings:         map[string][]domain.Finding{},
		AttentionDetails: map[string]map[domain.FindingCode]domain.AttentionDetail{},
		TruncatedIDs:     map[string]string{},
	}
	for _, r := range rs {
		if len(r.Findings) > 0 {
			res.Findings[r.ID] = r.Findings
		}
		if len(r.AttentionDetails) > 0 {
			res.AttentionDetails[r.ID] = r.AttentionDetails
		}
	}
	return w2ByID(rs), res
}

// ---------------------------------------------------------------------------
// row 22 — reachable outside a VPC
// ---------------------------------------------------------------------------

// Public reachability needs both halves: no VPC boundary AND an access policy
// anyone can use. Either one alone still leaves a gate in front of the data.
func TestW2OpenSearchPublicNeedsBothNoVPCAndOpenPolicy(t *testing.T) {
	open := w2OSDomain("acme-search")
	open.VPCOptions = nil
	open.AccessPolicies = aws.String(w2OSPublicPolicy)

	publicEndpointScopedPolicy := w2OSDomain("acme-search-scoped")
	publicEndpointScopedPolicy.VPCOptions = nil

	vpcOpenPolicy := w2OSDomain("acme-search-vpc")
	vpcOpenPolicy.AccessPolicies = aws.String(w2OSPublicPolicy)

	fields, res := w2OSRun(t, open, publicEndpointScopedPolicy, vpcOpenPolicy, w2OSDomain("acme-search-safe"))

	w2AssertFinding(t, res.Findings["acme-search"], w2OSCodePublic, "reachable outside a VPC", domain.SevBroken, w2OSSource)
	rows := w2Rows(t, res, "acme-search", w2OSCodePublic)
	w2AssertRow(t, rows, "Endpoint", "public")
	w2AssertRow(t, rows, "Access policy", "open")
	w2AssertFindingDef(t, "opensearch", w2OSCodePublic, "reachable outside a VPC", domain.SevBroken, "wave1")

	w2AssertNoCode(t, res.Findings["acme-search-scoped"], w2OSCodePublic)
	w2AssertNoCode(t, res.Findings["acme-search-vpc"], w2OSCodePublic)
	w2AssertNoCode(t, res.Findings["acme-search-safe"], w2OSCodePublic)

	// The enricher makes no API call, so it can only see what the fetcher
	// wrote. Pin the two fields it depends on.
	if got := fields["acme-search"].Fields["vpc_enabled"]; got != "false" {
		t.Errorf("acme-search vpc_enabled = %q, want \"false\"", got)
	}
	if got := fields["acme-search"].Fields["access_policy_public"]; got != "true" {
		t.Errorf("acme-search access_policy_public = %q, want \"true\"", got)
	}
	if got := fields["acme-search-vpc"].Fields["vpc_enabled"]; got != "true" {
		t.Errorf("acme-search-vpc vpc_enabled = %q, want \"true\"", got)
	}
}

// A wildcard principal fenced by a source-IP condition is a scoped grant.
func TestW2OpenSearchConditionedPolicyIsNotOpen(t *testing.T) {
	d := w2OSDomain("acme-search-cond")
	d.VPCOptions = nil
	d.AccessPolicies = aws.String(w2OSCondPolicy)

	fields, res := w2OSRun(t, d)

	w2AssertNoCode(t, res.Findings["acme-search-cond"], w2OSCodePublic)
	if got := fields["acme-search-cond"].Fields["access_policy_public"]; got != "false" {
		t.Errorf("access_policy_public = %q on a conditioned policy, want \"false\"", got)
	}
}

// ---------------------------------------------------------------------------
// rows 23 & 24 — transport encryption
// ---------------------------------------------------------------------------

func TestW2OpenSearchHTTPSNotEnforced(t *testing.T) {
	off := w2OSDomain("acme-search-http")
	off.DomainEndpointOptions = &ostypes.DomainEndpointOptions{EnforceHTTPS: aws.Bool(false)}

	absent := w2OSDomain("acme-search-nohttpsopt")
	absent.DomainEndpointOptions = nil

	_, res := w2OSRun(t, off, absent, w2OSDomain("acme-search-safe"))

	w2AssertFinding(t, res.Findings["acme-search-http"], w2OSCodeHTTPSOff, "HTTPS not enforced", domain.SevWarn, w2OSSource)
	w2AssertNoRows(t, res, "acme-search-http", w2OSCodeHTTPSOff)
	w2AssertFinding(t, res.Findings["acme-search-nohttpsopt"], w2OSCodeHTTPSOff, "HTTPS not enforced", domain.SevWarn, w2OSSource)
	w2AssertNoCode(t, res.Findings["acme-search-safe"], w2OSCodeHTTPSOff)
	w2AssertFindingDef(t, "opensearch", w2OSCodeHTTPSOff, "HTTPS not enforced", domain.SevWarn, "wave1")
}

func TestW2OpenSearchNodeToNodeEncryptionOff(t *testing.T) {
	off := w2OSDomain("acme-search-n2n")
	off.NodeToNodeEncryptionOptions = &ostypes.NodeToNodeEncryptionOptions{Enabled: aws.Bool(false)}

	absent := w2OSDomain("acme-search-non2nopt")
	absent.NodeToNodeEncryptionOptions = nil

	_, res := w2OSRun(t, off, absent, w2OSDomain("acme-search-safe"))

	w2AssertFinding(t, res.Findings["acme-search-n2n"], w2OSCodeN2NOff, "node-to-node encryption off", domain.SevWarn, w2OSSource)
	w2AssertNoRows(t, res, "acme-search-n2n", w2OSCodeN2NOff)
	w2AssertFinding(t, res.Findings["acme-search-non2nopt"], w2OSCodeN2NOff, "node-to-node encryption off", domain.SevWarn, w2OSSource)
	w2AssertNoCode(t, res.Findings["acme-search-safe"], w2OSCodeN2NOff)
	w2AssertFindingDef(t, "opensearch", w2OSCodeN2NOff, "node-to-node encryption off", domain.SevWarn, "wave1")
}

// ---------------------------------------------------------------------------
// independence and lifecycle
// ---------------------------------------------------------------------------

// Three conditions on one domain stay three findings, each with its own rows.
func TestW2OpenSearchThreeConditionsOnOneDomain(t *testing.T) {
	bad := w2OSDomain("acme-search-worst")
	bad.VPCOptions = nil
	bad.AccessPolicies = aws.String(w2OSPublicPolicy)
	bad.DomainEndpointOptions = &ostypes.DomainEndpointOptions{EnforceHTTPS: aws.Bool(false)}
	bad.NodeToNodeEncryptionOptions = &ostypes.NodeToNodeEncryptionOptions{Enabled: aws.Bool(false)}

	_, res := w2OSRun(t, bad)

	w2AssertFinding(t, res.Findings["acme-search-worst"], w2OSCodePublic, "reachable outside a VPC", domain.SevBroken, w2OSSource)
	w2AssertFinding(t, res.Findings["acme-search-worst"], w2OSCodeHTTPSOff, "HTTPS not enforced", domain.SevWarn, w2OSSource)
	w2AssertFinding(t, res.Findings["acme-search-worst"], w2OSCodeN2NOff, "node-to-node encryption off", domain.SevWarn, w2OSSource)

	w2AssertNoRows(t, res, "acme-search-worst", w2OSCodeHTTPSOff)
	w2AssertNoRows(t, res, "acme-search-worst", w2OSCodeN2NOff)
}

// A domain being deleted emits no posture finding — the existing enricher
// already guards this for its own signals and the new rows must not slip past.
func TestW2OpenSearchDeletedDomainEmitsNoPostureFinding(t *testing.T) {
	gone := w2OSDomain("acme-search-gone")
	gone.Deleted = aws.Bool(true)
	gone.VPCOptions = nil
	gone.AccessPolicies = aws.String(w2OSPublicPolicy)
	gone.DomainEndpointOptions = &ostypes.DomainEndpointOptions{EnforceHTTPS: aws.Bool(false)}
	gone.NodeToNodeEncryptionOptions = &ostypes.NodeToNodeEncryptionOptions{Enabled: aws.Bool(false)}

	_, res := w2OSRun(t, gone)

	for _, code := range []string{w2OSCodePublic, w2OSCodeHTTPSOff, w2OSCodeN2NOff} {
		w2AssertNoCode(t, res.Findings["acme-search-gone"], code)
	}
}
