package unit

// aws_vpcpeer_test.go — fetcher + Wave 2 issue-enrichment tests for EC2 VPC
// Peering Connections (docs/resources/vpc-peer.md §3/§4,
// docs/resources/vpc-peer-impl-plan.md §0/§1).
//
// vpc-peer is a SINGLE-CALL fetcher — DescribeVpcPeeringConnections returns
// full detail (Status, ExpirationTime, both VpcInfo sides) in the list call
// itself. Unlike lt (in-fetcher N+1), there is NO per-connection describe,
// NO degraded-row story: a DescribeVpcPeeringConnections denial is a
// whole-list error, never an empty success and never a rich-degraded row.
// RawStruct is the bare *ec2types.VpcPeeringConnection (no composite wrapper
// like LTRaw), Resource.ID is VpcPeeringConnectionId.
//
// Fetcher-written findings (Source "wave1") cover the state machine +
// active-only CIDR overlap. EnrichVpcPeerRoutes is a separate cache-scan
// enricher (zero AWS calls, mirrors EnrichLTDeprecatedAMI/
// snapshot_cross_ref.go) scanning the already-loaded "rtb" cache for two
// derived "~" (SevWarn) signals: no_local_route and route_blackholed.

import (
	"context"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	smithy "github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo/fakes"
	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

// fetchVpcPeerDemoPage fetches the shared demo fixture page. Unlike lt, the
// demo set has no details-denied witness (no degraded-row story exists for
// this type per docs/resources/vpc-peer-impl-plan.md §0), so a non-nil error
// here always fails the test.
func fetchVpcPeerDemoPage(t *testing.T) resource.FetchResult {
	t.Helper()
	result, err := awsclient.FetchVpcPeeringConnectionsPage(context.Background(), fakes.NewEC2(), "")
	if err != nil {
		t.Fatalf("FetchVpcPeeringConnectionsPage: unexpected error %v", err)
	}
	return result
}

// mustFindVpcPeerResource returns the resource with the given ID from a
// slice of fetched resources, failing the test if absent.
func mustFindVpcPeerResource(t *testing.T, resources []resource.Resource, id string) resource.Resource {
	t.Helper()
	for _, r := range resources {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("resource %q not found in fetch result", id)
	return resource.Resource{}
}

// vpcPeerAsRaw asserts RawStruct is the pinned bare *ec2types.VpcPeeringConnection
// shape (no composite wrapper, unlike LTRaw — docs/resources/vpc-peer-impl-plan.md
// §0: single-call type, no describe, no degraded rows).
func vpcPeerAsRaw(t *testing.T, raw any) *ec2types.VpcPeeringConnection {
	t.Helper()
	v, ok := raw.(*ec2types.VpcPeeringConnection)
	if !ok {
		t.Fatalf("RawStruct = %T, want *ec2types.VpcPeeringConnection", raw)
	}
	return v
}

// vpcPeerEC2Fake is a minimal EC2 fake exercising only DescribeVpcPeeringConnections
// — the fetcher takes the narrow EC2DescribeVpcPeeringConnectionsAPI interface
// (single-call, no N+1, mirrors the lt/FetchLaunchTemplatesPage narrow-interface
// convention), so this fake structurally cannot serve any other operation.
type vpcPeerEC2Fake struct {
	listOut  *ec2.DescribeVpcPeeringConnectionsOutput
	listErr  error
	gotInput *ec2.DescribeVpcPeeringConnectionsInput
}

func (f *vpcPeerEC2Fake) DescribeVpcPeeringConnections(
	_ context.Context, input *ec2.DescribeVpcPeeringConnectionsInput, _ ...func(*ec2.Options),
) (*ec2.DescribeVpcPeeringConnectionsOutput, error) {
	f.gotInput = input
	return f.listOut, f.listErr
}

var _ awsclient.EC2DescribeVpcPeeringConnectionsAPI = (*vpcPeerEC2Fake)(nil)

// vpcPeerConnWithStatus builds a minimal single-connection DescribeVpcPeeringConnections
// output for a given state-reason code, for states with no shared demo fixture
// witness (provisioning, initiating-request).
func vpcPeerConnWithStatus(id string, code ec2types.VpcPeeringConnectionStateReasonCode) *ec2.DescribeVpcPeeringConnectionsOutput {
	return &ec2.DescribeVpcPeeringConnectionsOutput{
		VpcPeeringConnections: []ec2types.VpcPeeringConnection{
			{
				VpcPeeringConnectionId: aws.String(id),
				Status:                 &ec2types.VpcPeeringConnectionStateReason{Code: code},
				RequesterVpcInfo:       &ec2types.VpcPeeringConnectionVpcInfo{VpcId: aws.String("vpc-0requesterside0001"), OwnerId: aws.String("123456789012")},
				AccepterVpcInfo:        &ec2types.VpcPeeringConnectionVpcInfo{VpcId: aws.String("vpc-0accepterside0001"), OwnerId: aws.String("210987654321")},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// healthy_silence / E7 sanity
// ---------------------------------------------------------------------------

func TestFetchVpcPeeringConnectionsPage_ActiveSilence(t *testing.T) {
	result := fetchVpcPeerDemoPage(t)
	r := mustFindVpcPeerResource(t, result.Resources, fixtures.ProdPeerSharedID)

	if len(r.Findings) != 0 {
		t.Errorf("Findings: expected 0 for an active, disjoint-CIDR, routed connection, got %d: %+v", len(r.Findings), r.Findings)
	}
	if r.Fields["status"] != "" {
		t.Errorf(`Fields["status"] = %q, want "" (S4 blank on a healthy row)`, r.Fields["status"])
	}
}

func TestFetchVpcPeeringConnectionsPage_ResourceIDAndRawStructMapping(t *testing.T) {
	result := fetchVpcPeerDemoPage(t)
	r := mustFindVpcPeerResource(t, result.Resources, fixtures.ProdPeerSharedID)

	if r.ID != fixtures.ProdPeerSharedID {
		t.Errorf("ID = %q, want VpcPeeringConnectionId %q", r.ID, fixtures.ProdPeerSharedID)
	}
	raw := vpcPeerAsRaw(t, r.RawStruct)
	if aws.ToString(raw.VpcPeeringConnectionId) != fixtures.ProdPeerSharedID {
		t.Errorf("RawStruct.VpcPeeringConnectionId = %q, want %q", aws.ToString(raw.VpcPeeringConnectionId), fixtures.ProdPeerSharedID)
	}
}

// ---------------------------------------------------------------------------
// state_phrases — one subtest per §4 state, exact Code/Phrase/Severity;
// rejected/failed Detail == Status.Message verbatim (anti-duplication: Phrase
// stays the bare state word, never absorbs the message).
// ---------------------------------------------------------------------------

func TestFetchVpcPeeringConnectionsPage_StatePhrase_PendingAcceptance(t *testing.T) {
	result := fetchVpcPeerDemoPage(t)
	r := mustFindVpcPeerResource(t, result.Resources, fixtures.WarnPeerPendingID)

	if len(r.Findings) != 1 {
		t.Fatalf("Findings: expected exactly 1, got %d: %+v", len(r.Findings), r.Findings)
	}
	f := r.Findings[0]
	if f.Code != "vpc-peer.warn.pending_acceptance" {
		t.Errorf("Code = %q, want %q", f.Code, "vpc-peer.warn.pending_acceptance")
	}
	// The fixture's ExpirationTime is evergreen (now+72h, fixed at fixture
	// build time); the "3d" countdown must read stable for the entire test
	// run regardless of elapsed wall-clock time since fixture build
	// (docs/resources/vpc-peer.md §4 pending-acceptance: "never rots").
	if f.Phrase != "pending acceptance: expires in 3d" {
		t.Errorf("Phrase = %q, want %q", f.Phrase, "pending acceptance: expires in 3d")
	}
	if f.Severity != domain.SevWarn {
		t.Errorf("Severity = %v, want SevWarn", f.Severity)
	}
	const wantPrefix = "The peer has not accepted; AWS expires the request on "
	if !strings.HasPrefix(f.Detail, wantPrefix) || !strings.HasSuffix(f.Detail, ".") {
		t.Errorf("Detail = %q, want prefix %q and a trailing period (S5 template + the actual expiration date)", f.Detail, wantPrefix)
	}
}

func TestFetchVpcPeeringConnectionsPage_StatePhrase_Expired(t *testing.T) {
	result := fetchVpcPeerDemoPage(t)
	r := mustFindVpcPeerResource(t, result.Resources, fixtures.WarnPeerExpiredID)

	if len(r.Findings) != 1 {
		t.Fatalf("Findings: expected exactly 1, got %d: %+v", len(r.Findings), r.Findings)
	}
	f := r.Findings[0]
	if f.Code != "vpc-peer.warn.expired" {
		t.Errorf("Code = %q, want %q", f.Code, "vpc-peer.warn.expired")
	}
	if f.Phrase != "expired: never accepted" {
		t.Errorf("Phrase = %q, want %q", f.Phrase, "expired: never accepted")
	}
	if f.Severity != domain.SevWarn {
		t.Errorf("Severity = %v, want SevWarn", f.Severity)
	}
	const wantDetail = "The peering request expired unaccepted; recreate it if still needed."
	if f.Detail != wantDetail {
		t.Errorf("Detail = %q, want %q", f.Detail, wantDetail)
	}
}

func TestFetchVpcPeeringConnectionsPage_StatePhrase_Rejected(t *testing.T) {
	result := fetchVpcPeerDemoPage(t)
	r := mustFindVpcPeerResource(t, result.Resources, fixtures.BrokenPeerRejectedID)

	if len(r.Findings) != 1 {
		t.Fatalf("Findings: expected exactly 1, got %d: %+v", len(r.Findings), r.Findings)
	}
	f := r.Findings[0]
	if f.Code != "vpc-peer.broken.rejected" {
		t.Errorf("Code = %q, want %q", f.Code, "vpc-peer.broken.rejected")
	}
	if f.Phrase != "rejected" {
		t.Errorf(`Phrase = %q, want exactly %q (anti-duplication: the message lives in Detail, never appended to Phrase)`, f.Phrase, "rejected")
	}
	if f.Severity != domain.SevBroken {
		t.Errorf("Severity = %v, want SevBroken", f.Severity)
	}
	const wantDetail = "Rejected by accepter: CIDR conflict"
	if f.Detail != wantDetail {
		t.Errorf("Detail = %q, want %q (Status.Message verbatim)", f.Detail, wantDetail)
	}
}

func TestFetchVpcPeeringConnectionsPage_StatePhrase_Failed(t *testing.T) {
	result := fetchVpcPeerDemoPage(t)
	r := mustFindVpcPeerResource(t, result.Resources, fixtures.BrokenPeerFailedID)

	if len(r.Findings) != 1 {
		t.Fatalf("Findings: expected exactly 1, got %d: %+v", len(r.Findings), r.Findings)
	}
	f := r.Findings[0]
	if f.Code != "vpc-peer.broken.failed" {
		t.Errorf("Code = %q, want %q", f.Code, "vpc-peer.broken.failed")
	}
	if f.Phrase != "failed" {
		t.Errorf(`Phrase = %q, want exactly %q (anti-duplication: the message lives in Detail, never appended to Phrase)`, f.Phrase, "failed")
	}
	if f.Severity != domain.SevBroken {
		t.Errorf("Severity = %v, want SevBroken", f.Severity)
	}
	const wantDetail = "Failed to activate the peering connection due to an internal error"
	if f.Detail != wantDetail {
		t.Errorf("Detail = %q, want %q (Status.Message verbatim)", f.Detail, wantDetail)
	}
}

func TestFetchVpcPeeringConnectionsPage_StatePhrase_Deleting(t *testing.T) {
	result := fetchVpcPeerDemoPage(t)
	r := mustFindVpcPeerResource(t, result.Resources, fixtures.WarnPeerDeletingID)

	if len(r.Findings) != 1 {
		t.Fatalf("Findings: expected exactly 1, got %d: %+v", len(r.Findings), r.Findings)
	}
	f := r.Findings[0]
	if f.Code != "vpc-peer.warn.deleting" {
		t.Errorf("Code = %q, want %q", f.Code, "vpc-peer.warn.deleting")
	}
	if f.Phrase != "deleting" {
		t.Errorf("Phrase = %q, want %q", f.Phrase, "deleting")
	}
	if f.Severity != domain.SevWarn {
		t.Errorf("Severity = %v, want SevWarn", f.Severity)
	}
	const wantDetail = "Peering connection is being deleted."
	if f.Detail != wantDetail {
		t.Errorf("Detail = %q, want %q", f.Detail, wantDetail)
	}
}

func TestFetchVpcPeeringConnectionsPage_StatePhrase_Deleted(t *testing.T) {
	result := fetchVpcPeerDemoPage(t)
	r := mustFindVpcPeerResource(t, result.Resources, fixtures.DimPeerDeletedID)

	if len(r.Findings) != 1 {
		t.Fatalf("Findings: expected exactly 1, got %d: %+v", len(r.Findings), r.Findings)
	}
	f := r.Findings[0]
	if f.Code != "vpc-peer.dim.deleted" {
		t.Errorf("Code = %q, want %q", f.Code, "vpc-peer.dim.deleted")
	}
	if f.Phrase != "deleted" {
		t.Errorf("Phrase = %q, want %q", f.Phrase, "deleted")
	}
	if f.Severity != domain.SevDim {
		t.Errorf("Severity = %v, want SevDim", f.Severity)
	}
	const wantDetail = "AWS keeps deleted connections listed for a window."
	if f.Detail != wantDetail {
		t.Errorf("Detail = %q, want %q", f.Detail, wantDetail)
	}
}

// provisioning and initiating-request have no shared demo fixture witness
// (the fixture list covers the other 8 states plus overlap/no-route/blackhole);
// isolated adversarial fakes mirror aws_lt_test.go's ltEC2Fake pattern.

func TestFetchVpcPeeringConnectionsPage_StatePhrase_Provisioning(t *testing.T) {
	const id = "pcx-0provisioning0001a"
	fake := &vpcPeerEC2Fake{listOut: vpcPeerConnWithStatus(id, ec2types.VpcPeeringConnectionStateReasonCodeProvisioning)}
	result, err := awsclient.FetchVpcPeeringConnectionsPage(context.Background(), fake, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	r := mustFindVpcPeerResource(t, result.Resources, id)
	if len(r.Findings) != 1 {
		t.Fatalf("Findings: expected exactly 1, got %d: %+v", len(r.Findings), r.Findings)
	}
	f := r.Findings[0]
	if f.Code != "vpc-peer.warn.provisioning" {
		t.Errorf("Code = %q, want %q", f.Code, "vpc-peer.warn.provisioning")
	}
	if f.Phrase != "provisioning" {
		t.Errorf("Phrase = %q, want %q", f.Phrase, "provisioning")
	}
	if f.Severity != domain.SevWarn {
		t.Errorf("Severity = %v, want SevWarn", f.Severity)
	}
	const wantDetail = "Peering connection is being provisioned."
	if f.Detail != wantDetail {
		t.Errorf("Detail = %q, want %q", f.Detail, wantDetail)
	}
}

func TestFetchVpcPeeringConnectionsPage_StatePhrase_InitiatingRequest(t *testing.T) {
	const id = "pcx-0initiating00001a"
	fake := &vpcPeerEC2Fake{listOut: vpcPeerConnWithStatus(id, ec2types.VpcPeeringConnectionStateReasonCodeInitiatingRequest)}
	result, err := awsclient.FetchVpcPeeringConnectionsPage(context.Background(), fake, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	r := mustFindVpcPeerResource(t, result.Resources, id)
	if len(r.Findings) != 1 {
		t.Fatalf("Findings: expected exactly 1, got %d: %+v", len(r.Findings), r.Findings)
	}
	f := r.Findings[0]
	if f.Code != "vpc-peer.warn.initiating" {
		t.Errorf("Code = %q, want %q", f.Code, "vpc-peer.warn.initiating")
	}
	if f.Phrase != "initiating" {
		t.Errorf("Phrase = %q, want %q", f.Phrase, "initiating")
	}
	if f.Severity != domain.SevWarn {
		t.Errorf("Severity = %v, want SevWarn", f.Severity)
	}
	const wantDetail = "Peering request is being initiated."
	if f.Detail != wantDetail {
		t.Errorf("Detail = %q, want %q", f.Detail, wantDetail)
	}
}

// ---------------------------------------------------------------------------
// cidr_overlap_active — active + identical CidrBlockSets on both sides ->
// warn + both ranges named in Detail.
// ---------------------------------------------------------------------------

func TestFetchVpcPeeringConnectionsPage_CIDROverlapActive(t *testing.T) {
	result := fetchVpcPeerDemoPage(t)
	r := mustFindVpcPeerResource(t, result.Resources, fixtures.WarnPeerOverlapID)

	if len(r.Findings) != 1 {
		t.Fatalf("Findings: expected exactly 1 (active, no state degradation — overlap only), got %d: %+v", len(r.Findings), r.Findings)
	}
	f := r.Findings[0]
	if f.Code != "vpc-peer.warn.cidr_overlap" {
		t.Errorf("Code = %q, want %q", f.Code, "vpc-peer.warn.cidr_overlap")
	}
	if f.Phrase != "CIDR overlap with peer" {
		t.Errorf("Phrase = %q, want %q", f.Phrase, "CIDR overlap with peer")
	}
	if f.Severity != domain.SevWarn {
		t.Errorf("Severity = %v, want SevWarn", f.Severity)
	}
	const wantPrefix = "Requester and accepter CIDR ranges overlap: "
	const wantSuffix = "overlapping subsets blackhole."
	if !strings.HasPrefix(f.Detail, wantPrefix) || !strings.HasSuffix(f.Detail, wantSuffix) {
		t.Errorf("Detail = %q, want prefix %q and suffix %q", f.Detail, wantPrefix, wantSuffix)
	}
	if !strings.Contains(f.Detail, "10.0.0.0/16") {
		t.Errorf("Detail = %q, want it to name the overlapping range %q", f.Detail, "10.0.0.0/16")
	}
}

// ---------------------------------------------------------------------------
// cidr_nil_safe — every non-active demo fixture has nil CidrBlock/CidrBlockSet
// (load-bearing SDK fact: CIDR info is active-only); the overlap check must
// never panic and must never fire on any of them.
// ---------------------------------------------------------------------------

func TestFetchVpcPeeringConnectionsPage_CIDRNilSafeNoOverlapFinding(t *testing.T) {
	result := fetchVpcPeerDemoPage(t)

	nonActiveIDs := []string{
		fixtures.WarnPeerPendingID, fixtures.WarnPeerExpiredID,
		fixtures.BrokenPeerRejectedID, fixtures.BrokenPeerFailedID,
		fixtures.WarnPeerDeletingID, fixtures.DimPeerDeletedID,
	}
	for _, id := range nonActiveIDs {
		r := mustFindVpcPeerResource(t, result.Resources, id)
		for _, f := range r.Findings {
			if f.Code == "vpc-peer.warn.cidr_overlap" {
				t.Errorf("resource %q: non-active connection must never raise a CIDR overlap finding (CIDR fields are nil), got %+v", id, f)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// list_denied_is_error — the ONLY API call for this type; a denial must
// never render as an empty successful result (no degraded-row story exists).
// ---------------------------------------------------------------------------

func TestFetchVpcPeeringConnectionsPage_ListDeniedIsError(t *testing.T) {
	fake := &vpcPeerEC2Fake{
		listErr: &smithy.GenericAPIError{
			Code:    "UnauthorizedOperation",
			Message: "You are not authorized to perform ec2:DescribeVpcPeeringConnections",
		},
	}
	result, err := awsclient.FetchVpcPeeringConnectionsPage(context.Background(), fake, "")
	if err == nil {
		t.Fatal("FetchVpcPeeringConnectionsPage must return a non-nil error when DescribeVpcPeeringConnections is denied — never an empty successful result")
	}
	if !strings.Contains(err.Error(), "UnauthorizedOperation") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "UnauthorizedOperation")
	}
	if len(result.Resources) != 0 {
		t.Errorf("Resources: expected 0 on error, got %d", len(result.Resources))
	}
}

// ---------------------------------------------------------------------------
// pagination — single paginated call: incoming continuationToken must be
// forwarded as NextToken on the request; a returned NextToken must mark the
// result truncated and carry through for the next page.
// ---------------------------------------------------------------------------

func TestFetchVpcPeeringConnectionsPage_PaginationTokenPropagation(t *testing.T) {
	fake := &vpcPeerEC2Fake{
		listOut: &ec2.DescribeVpcPeeringConnectionsOutput{
			VpcPeeringConnections: []ec2types.VpcPeeringConnection{
				{
					VpcPeeringConnectionId: aws.String("pcx-0paginationwitness1"),
					Status:                 &ec2types.VpcPeeringConnectionStateReason{Code: ec2types.VpcPeeringConnectionStateReasonCodeActive},
				},
			},
			NextToken: aws.String("page-2-token"),
		},
	}
	result, err := awsclient.FetchVpcPeeringConnectionsPage(context.Background(), fake, "incoming-token")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if fake.gotInput == nil || aws.ToString(fake.gotInput.NextToken) != "incoming-token" {
		t.Errorf("request NextToken = %q, want %q (incoming continuationToken must be forwarded)", aws.ToString(fake.gotInput.NextToken), "incoming-token")
	}
	if result.Pagination == nil || !result.Pagination.IsTruncated {
		t.Fatalf("Pagination.IsTruncated = false, want true (output.NextToken was set)")
	}
	if result.Pagination.NextToken != "page-2-token" {
		t.Errorf("Pagination.NextToken = %q, want %q", result.Pagination.NextToken, "page-2-token")
	}
}

// ---------------------------------------------------------------------------
// wave3_anti — every demo connection is cross-account (docs/resources/vpc-peer.md
// §3.1: "all seven live-witnessed rows are cross-account"); cross-account/
// cross-region and DNS options must never surface as a finding.
// ---------------------------------------------------------------------------

func TestFetchVpcPeeringConnectionsPage_WaveThreeAntiTests(t *testing.T) {
	result := fetchVpcPeerDemoPage(t)

	forbidden := []string{"cross-account", "cross-region", "ClassicLink", "AllowDnsResolutionFromRemoteVpc", "pending-acceptance", "initiating-request"}
	for _, r := range result.Resources {
		for _, f := range r.Findings {
			for _, s := range forbidden {
				if strings.Contains(f.Phrase, s) || strings.Contains(f.Detail, s) {
					t.Errorf("resource %q: Finding %+v must never surface Wave-3/raw-enum text %q", r.ID, f, s)
				}
			}
		}
	}
}

// ---------------------------------------------------------------------------
// EnrichVpcPeerRoutes — cache-scan enricher (zero SDK calls, mirrors
// EnrichLTDeprecatedAMI's signature over the loaded "rtb" ResourceCache).
// Both derived signals ship as "~" (SevWarn) background checks, matching the
// lt deprecated-AMI treatment (spec amended 2026-07-15).
// ---------------------------------------------------------------------------

// vpcPeerRTBCache builds an "rtb" ResourceCache entry from the REAL demo rtb
// fixtures (FetchRouteTablesPage + fakes.NewEC2()), so the enricher tests
// exercise the actual shipped fixture graph (two routes into ProdPeerSharedID,
// one blackholed route into WarnPeerBlackholeID, none into WarnPeerNoRouteID).
func vpcPeerRTBCache(t *testing.T) resource.ResourceCache {
	t.Helper()
	result, err := awsclient.FetchRouteTablesPage(context.Background(), fakes.NewEC2(), "")
	if err != nil {
		t.Fatalf("FetchRouteTablesPage: %v", err)
	}
	return resource.ResourceCache{
		"rtb": resource.ResourceCacheEntry{Resources: result.Resources},
	}
}

func TestEnrichVpcPeerRoutes_NoLocalRoute(t *testing.T) {
	demo := fetchVpcPeerDemoPage(t)
	pcx := mustFindVpcPeerResource(t, demo.Resources, fixtures.WarnPeerNoRouteID)

	clients := &awsclient.ServiceClients{EC2: fakes.NewEC2()}
	result, err := awsclient.EnrichVpcPeerRoutes(context.Background(), clients, []resource.Resource{pcx}, vpcPeerRTBCache(t))
	if err != nil {
		t.Fatalf("EnrichVpcPeerRoutes returned error: %v", err)
	}

	findings := result.Findings[fixtures.WarnPeerNoRouteID]
	if len(findings) != 1 {
		t.Fatalf("Findings[%s]: expected exactly 1, got %d: %+v", fixtures.WarnPeerNoRouteID, len(findings), findings)
	}
	f := findings[0]
	if f.Code != "vpc-peer.warn.no_local_route" {
		t.Errorf("Code = %q, want %q", f.Code, "vpc-peer.warn.no_local_route")
	}
	if f.Phrase != "no local route to peer" {
		t.Errorf("Phrase = %q, want %q", f.Phrase, "no local route to peer")
	}
	if f.Severity != domain.SevWarn {
		t.Errorf("Severity = %v, want SevWarn (the \"~\" tier)", f.Severity)
	}
	const wantDetail = "No loaded route table routes to this peering connection."
	if f.Detail != wantDetail {
		t.Errorf("Detail = %q, want %q", f.Detail, wantDetail)
	}
}

func TestEnrichVpcPeerRoutes_RouteBlackholed(t *testing.T) {
	demo := fetchVpcPeerDemoPage(t)
	pcx := mustFindVpcPeerResource(t, demo.Resources, fixtures.WarnPeerBlackholeID)

	clients := &awsclient.ServiceClients{EC2: fakes.NewEC2()}
	result, err := awsclient.EnrichVpcPeerRoutes(context.Background(), clients, []resource.Resource{pcx}, vpcPeerRTBCache(t))
	if err != nil {
		t.Fatalf("EnrichVpcPeerRoutes returned error: %v", err)
	}

	findings := result.Findings[fixtures.WarnPeerBlackholeID]
	if len(findings) != 1 {
		t.Fatalf("Findings[%s]: expected exactly 1, got %d: %+v", fixtures.WarnPeerBlackholeID, len(findings), findings)
	}
	f := findings[0]
	if f.Code != "vpc-peer.warn.route_blackholed" {
		t.Errorf("Code = %q, want %q", f.Code, "vpc-peer.warn.route_blackholed")
	}
	if f.Phrase != "route to peer blackholed" {
		t.Errorf("Phrase = %q, want %q", f.Phrase, "route to peer blackholed")
	}
	if f.Severity != domain.SevWarn {
		t.Errorf("Severity = %v, want SevWarn (the \"~\" tier)", f.Severity)
	}
	const wantDetail = "A route references this connection but its state is blackhole."
	if f.Detail != wantDetail {
		t.Errorf("Detail = %q, want %q", f.Detail, wantDetail)
	}
}

func TestEnrichVpcPeerRoutes_ActiveRoutedNoFinding(t *testing.T) {
	demo := fetchVpcPeerDemoPage(t)
	pcx := mustFindVpcPeerResource(t, demo.Resources, fixtures.ProdPeerSharedID)

	clients := &awsclient.ServiceClients{EC2: fakes.NewEC2()}
	result, err := awsclient.EnrichVpcPeerRoutes(context.Background(), clients, []resource.Resource{pcx}, vpcPeerRTBCache(t))
	if err != nil {
		t.Fatalf("EnrichVpcPeerRoutes returned error: %v", err)
	}
	if len(result.Findings[fixtures.ProdPeerSharedID]) != 0 {
		t.Errorf("Findings[%s]: expected 0 (two active, non-blackholed routes reference it), got %+v",
			fixtures.ProdPeerSharedID, result.Findings[fixtures.ProdPeerSharedID])
	}
}

func TestEnrichVpcPeerRoutes_NonActiveGuardedNoFinding(t *testing.T) {
	demo := fetchVpcPeerDemoPage(t)
	pcx := mustFindVpcPeerResource(t, demo.Resources, fixtures.WarnPeerPendingID)

	clients := &awsclient.ServiceClients{EC2: fakes.NewEC2()}
	result, err := awsclient.EnrichVpcPeerRoutes(context.Background(), clients, []resource.Resource{pcx}, vpcPeerRTBCache(t))
	if err != nil {
		t.Fatalf("EnrichVpcPeerRoutes returned error: %v", err)
	}
	if len(result.Findings[fixtures.WarnPeerPendingID]) != 0 {
		t.Errorf("Findings[%s]: expected 0 (both derived checks are active-only), got %+v",
			fixtures.WarnPeerPendingID, result.Findings[fixtures.WarnPeerPendingID])
	}
}

func TestEnrichVpcPeerRoutes_RTBCacheAbsentSkipsSilently(t *testing.T) {
	demo := fetchVpcPeerDemoPage(t)
	noRoute := mustFindVpcPeerResource(t, demo.Resources, fixtures.WarnPeerNoRouteID)
	blackhole := mustFindVpcPeerResource(t, demo.Resources, fixtures.WarnPeerBlackholeID)

	clients := &awsclient.ServiceClients{EC2: fakes.NewEC2()}
	result, err := awsclient.EnrichVpcPeerRoutes(context.Background(), clients, []resource.Resource{noRoute, blackhole}, resource.ResourceCache{})
	if err != nil {
		t.Fatalf("EnrichVpcPeerRoutes returned error: %v", err)
	}
	if len(result.Findings[fixtures.WarnPeerNoRouteID]) != 0 {
		t.Errorf("Findings[%s]: expected 0 when the rtb cache is not loaded, got %+v", fixtures.WarnPeerNoRouteID, result.Findings[fixtures.WarnPeerNoRouteID])
	}
	if len(result.Findings[fixtures.WarnPeerBlackholeID]) != 0 {
		t.Errorf("Findings[%s]: expected 0 when the rtb cache is not loaded, got %+v", fixtures.WarnPeerBlackholeID, result.Findings[fixtures.WarnPeerBlackholeID])
	}
}

func TestEnrichVpcPeerRoutes_RTBCacheTruncatedSkipsSilently(t *testing.T) {
	demo := fetchVpcPeerDemoPage(t)
	noRoute := mustFindVpcPeerResource(t, demo.Resources, fixtures.WarnPeerNoRouteID)
	blackhole := mustFindVpcPeerResource(t, demo.Resources, fixtures.WarnPeerBlackholeID)

	cache := vpcPeerRTBCache(t)
	entry := cache["rtb"]
	entry.IsTruncated = true
	cache["rtb"] = entry

	clients := &awsclient.ServiceClients{EC2: fakes.NewEC2()}
	result, err := awsclient.EnrichVpcPeerRoutes(context.Background(), clients, []resource.Resource{noRoute, blackhole}, cache)
	if err != nil {
		t.Fatalf("EnrichVpcPeerRoutes returned error: %v", err)
	}
	if len(result.Findings[fixtures.WarnPeerNoRouteID]) != 0 {
		t.Errorf("Findings[%s]: expected 0 when the rtb cache is truncated (never a guess), got %+v", fixtures.WarnPeerNoRouteID, result.Findings[fixtures.WarnPeerNoRouteID])
	}
	if len(result.Findings[fixtures.WarnPeerBlackholeID]) != 0 {
		t.Errorf("Findings[%s]: expected 0 when the rtb cache is truncated (never a guess), got %+v", fixtures.WarnPeerBlackholeID, result.Findings[fixtures.WarnPeerBlackholeID])
	}
}
