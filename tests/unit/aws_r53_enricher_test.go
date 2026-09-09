package unit

// aws_r53_enricher_test.go — Behavioral tests for EnrichRoute53Zone.
//
// Contract assertions:
//   - GetHostedZone is called once per r53 resource (keyed by zone ID).
//   - Public zones (Config.PrivateZone=false) → 0 findings (no orphan risk).
//   - Private zone with associated VPCs (VPCs non-empty) → 0 findings.
//   - Private zone with no associated VPCs (VPCs empty) → 1 finding sev "~" for that zone.
//   - clients.Route53 == nil → (EnricherResult{Findings: non-nil empty}, nil).
//   - API error → 0 findings, Truncated=true, no error returned.

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	r53types "github.com/aws/aws-sdk-go-v2/service/route53/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// r53GetHostedZoneFake implements Route53API for enrichment testing.
// It embeds the interface and overrides only GetHostedZone so the fake
// only needs to serve the single method used by EnrichRoute53Zone.
// The results map is keyed by hosted zone ID.
type r53GetHostedZoneFake struct {
	awsclient.Route53API
	// results maps zone ID → GetHostedZoneOutput.
	results map[string]*route53.GetHostedZoneOutput
	// errByID maps zone ID → error; overrides results when set.
	errByID map[string]error
}

// ListQueryLoggingConfigs and ListResourceRecordSets answer healthily rather
// than falling through to the embedded nil interface. EnrichRoute53Zone calls
// both for every public zone, and a nil embedded field dereferences into a
// SIGSEGV that takes the whole unit package down before any other test
// reports; the static interface assertion cannot catch it, because embedding
// satisfies the interface whether or not the field is set. One config and no
// records keep r53.query-logging-off and r53.dangling-record out of tests
// written about the orphan-private-zone row.
func (f *r53GetHostedZoneFake) ListQueryLoggingConfigs(
	_ context.Context,
	_ *route53.ListQueryLoggingConfigsInput,
	_ ...func(*route53.Options),
) (*route53.ListQueryLoggingConfigsOutput, error) {
	return &route53.ListQueryLoggingConfigsOutput{
		QueryLoggingConfigs: []r53types.QueryLoggingConfig{{
			Id:                        aws.String("qlc-0000000000000000"),
			HostedZoneId:              aws.String("Z0A1B2C3D4E5F6G7H8I9"),
			CloudWatchLogsLogGroupArn: aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws/route53/acme:*"),
		}},
	}, nil
}

func (f *r53GetHostedZoneFake) ListResourceRecordSets(
	_ context.Context,
	_ *route53.ListResourceRecordSetsInput,
	_ ...func(*route53.Options),
) (*route53.ListResourceRecordSetsOutput, error) {
	return &route53.ListResourceRecordSetsOutput{}, nil
}

func (f *r53GetHostedZoneFake) GetHostedZone(
	_ context.Context,
	in *route53.GetHostedZoneInput,
	_ ...func(*route53.Options),
) (*route53.GetHostedZoneOutput, error) {
	id := ""
	if in != nil && in.Id != nil {
		id = *in.Id
	}
	if f.errByID != nil {
		if err, ok := f.errByID[id]; ok {
			return nil, err
		}
	}
	out, ok := f.results[id]
	if !ok {
		return &route53.GetHostedZoneOutput{}, nil
	}
	return out, nil
}

// Compile-time check: r53GetHostedZoneFake satisfies Route53API.
var _ awsclient.Route53API = (*r53GetHostedZoneFake)(nil)

// r53ZoneResources returns a slice of r53 Resource stubs with the given zone IDs.
func r53ZoneResources(ids ...string) []resource.Resource {
	res := make([]resource.Resource, 0, len(ids))
	for _, id := range ids {
		res = append(res, resource.Resource{
			ID:   id,
			Name: id + ".example.com.",
			Fields: map[string]string{
				"zone_id":      id,
				"name":         id + ".example.com.",
				"record_count": "5",
				"private_zone": "false",
				"comment":      "",
			},
		})
	}
	return res
}

// r53ZoneOutput builds a GetHostedZoneOutput for the given zone ID, with
// PrivateZone flag and associated VPCs.
func r53ZoneOutput(zoneID string, private bool, vpcIDs []string) *route53.GetHostedZoneOutput {
	vpcs := make([]r53types.VPC, 0, len(vpcIDs))
	for _, vid := range vpcIDs {
		vpcs = append(vpcs, r53types.VPC{
			VPCId: aws.String(vid),
		})
	}
	return &route53.GetHostedZoneOutput{
		HostedZone: &r53types.HostedZone{
			Id:   aws.String(zoneID),
			Name: aws.String(zoneID + ".example.com."),
			Config: &r53types.HostedZoneConfig{
				PrivateZone: private,
			},
		},
		VPCs: vpcs,
	}
}

const (
	r53ZoneID1 = "/hostedzone/Z1AAABBBCCC111"
	r53ZoneID2 = "/hostedzone/Z2AAABBBCCC222"
)

// TestEnrichRoute53Zone_PublicZonesProduceNoFindings verifies that when both zones
// are public (Config.PrivateZone=false), no findings are produced.
func TestEnrichRoute53Zone_PublicZonesProduceNoFindings(t *testing.T) {
	fake := &r53GetHostedZoneFake{
		results: map[string]*route53.GetHostedZoneOutput{
			r53ZoneID1: r53ZoneOutput(r53ZoneID1, false, nil),
			r53ZoneID2: r53ZoneOutput(r53ZoneID2, false, nil),
		},
	}
	clients := &awsclient.ServiceClients{Route53: fake}
	resources := r53ZoneResources(r53ZoneID1, r53ZoneID2)

	result, err := awsclient.EnrichRoute53Zone(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Findings == nil {
		t.Fatal("Findings must not be nil")
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings for public zones, got %d: %v", len(result.Findings), result.Findings)
	}
}

// TestEnrichRoute53Zone_PrivateZoneWithVPCsProducesNoFindings verifies that a private
// zone with at least one associated VPC is healthy (no findings).
func TestEnrichRoute53Zone_PrivateZoneWithVPCsProducesNoFindings(t *testing.T) {
	fake := &r53GetHostedZoneFake{
		results: map[string]*route53.GetHostedZoneOutput{
			r53ZoneID1: r53ZoneOutput(r53ZoneID1, true, []string{"vpc-0a1b2c3d4e5f67890"}),
			r53ZoneID2: r53ZoneOutput(r53ZoneID2, false, nil),
		},
	}
	clients := &awsclient.ServiceClients{Route53: fake}
	resources := r53ZoneResources(r53ZoneID1, r53ZoneID2)

	result, err := awsclient.EnrichRoute53Zone(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings when private zone has VPCs, got %d: %v", len(result.Findings), result.Findings)
	}
}

// TestEnrichRoute53Zone_PrivateOrphanZoneProducesFindingSevTilde verifies that a
// private zone with no associated VPCs produces a finding with severity "~" for
// that zone only. The second zone is public and must not appear in Findings.
func TestEnrichRoute53Zone_PrivateOrphanZoneProducesFindingSevTilde(t *testing.T) {
	fake := &r53GetHostedZoneFake{
		results: map[string]*route53.GetHostedZoneOutput{
			r53ZoneID1: r53ZoneOutput(r53ZoneID1, true, []string{}), // private, no VPCs → orphan
			r53ZoneID2: r53ZoneOutput(r53ZoneID2, false, nil),       // public → no finding
		},
	}
	clients := &awsclient.ServiceClients{Route53: fake}
	resources := r53ZoneResources(r53ZoneID1, r53ZoneID2)

	result, err := awsclient.EnrichRoute53Zone(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fs, ok := result.Findings[r53ZoneID1]
	if !ok {
		t.Fatalf("expected finding keyed by %q (private orphan zone)", r53ZoneID1)
	}
	f := fs[0]
	if f.Severity != domain.SevWarn {
		t.Errorf("severity = %v, want %v", f.Severity, "~")
	}
	if _, ok := result.Findings[r53ZoneID2]; ok {
		t.Error("zone-2 (public) must NOT appear in Findings")
	}
}

// TestEnrichRoute53Zone_NilClientReturnsEmptyFindingsNoError verifies that when
// clients.Route53 is nil the enricher returns a non-nil empty Findings map and no error.
func TestEnrichRoute53Zone_NilClientReturnsEmptyFindingsNoError(t *testing.T) {
	clients := &awsclient.ServiceClients{Route53: nil}

	result, err := awsclient.EnrichRoute53Zone(context.Background(), clients, r53ZoneResources(r53ZoneID1, r53ZoneID2), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Findings == nil {
		t.Error("Findings must not be nil when Route53 client is nil")
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected empty Findings, got %d entries", len(result.Findings))
	}
}

// TestEnrichRoute53Zone_APIErrorMarksRowTruncatedIDNotBadge verifies that when the
// API call for zone-1 returns an error, the enricher marks each failing
// zone's row via TruncatedIDs, produces 0 findings for the failed zones, and
// returns a composite error containing the enricher prefix and the failing
// zone ID. r53 only ever emits "~" (informational) findings, so the
// aggregate Truncated flag must stay false — a coverage gap never
// lower-bounds the issue badge.
func TestEnrichRoute53Zone_APIErrorMarksRowTruncatedIDNotBadge(t *testing.T) {
	apiErr := errors.New("route53: GetHostedZone throttled")
	fake := &r53GetHostedZoneFake{
		errByID: map[string]error{
			r53ZoneID1: apiErr,
			r53ZoneID2: apiErr,
		},
	}
	clients := &awsclient.ServiceClients{Route53: fake}
	resources := r53ZoneResources(r53ZoneID1, r53ZoneID2)

	result, err := awsclient.EnrichRoute53Zone(context.Background(), clients, resources, nil)
	if err == nil {
		t.Fatal("enricher must surface a composite error when an API call fails")
	}
	// INVERTED for the "skipped" spec row 6: the label was "r53-enrich:", which
	// made the rendered line say the type twice ("enrich r53: GetHostedZone ..."). The
	// type comes from the registry key at the surface; the aggregate names
	// the call. Do not restore the type in the label.
	if errStr := err.Error(); !strings.Contains(errStr, "GetHostedZone") {
		t.Errorf("composite error must name the call, %q, got: %q", "GetHostedZone", errStr)
	}
	if errStr := err.Error(); !strings.Contains(errStr, r53ZoneID1) {
		t.Errorf("composite error must contain the failing zone ID %q, got: %q", r53ZoneID1, errStr)
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings on API error, got %d", len(result.Findings))
	}
	if result.Truncated {
		t.Error("Truncated must stay false: r53 only emits \"~\" findings, so an API error marks the row via TruncatedIDs, never the aggregate issue badge")
	}
	if _, marked := result.TruncatedIDs[r53ZoneID1]; !marked {
		t.Errorf("TruncatedIDs[%q] must be true", r53ZoneID1)
	}
	if _, marked := result.TruncatedIDs[r53ZoneID2]; !marked {
		t.Errorf("TruncatedIDs[%q] must be true", r53ZoneID2)
	}
}
