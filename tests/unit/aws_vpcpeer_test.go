package unit

// DescribeVpcPeeringConnections returns full detail (Status, ExpirationTime,
// both VpcInfo sides) in the list call, so a denial is a whole-list error and
// no row is degraded. RawStruct is the bare *ec2types.VpcPeeringConnection and
// Resource.ID is VpcPeeringConnectionId.

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	smithy "github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo/fakes"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

func fetchVpcPeerDemoPage(t *testing.T) resource.FetchResult {
	t.Helper()
	result, err := awsclient.FetchVpcPeeringConnectionsPage(context.Background(), fakes.NewEC2(), "")
	if err != nil {
		t.Fatalf("FetchVpcPeeringConnectionsPage: unexpected error %v", err)
	}
	return result
}

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

func vpcPeerAsRaw(t *testing.T, raw any) *ec2types.VpcPeeringConnection {
	t.Helper()
	v, ok := raw.(*ec2types.VpcPeeringConnection)
	if !ok {
		t.Fatalf("RawStruct = %T, want *ec2types.VpcPeeringConnection", raw)
	}
	return v
}

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

// vpcPeerConnWithStatus builds a single-connection output for states no demo
// fixture carries.
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
	// The fixture's ExpirationTime is now+72h at fixture build time, so the
	// countdown reads "3d" however long the run takes.
	if f.Phrase != "pending acceptance: expires in 3d" {
		t.Errorf("Phrase = %q, want %q", f.Phrase, "pending acceptance: expires in 3d")
	}
	if f.Severity != domain.SevWarn {
		t.Errorf("Severity = %v, want SevWarn", f.Severity)
	}
	// Detail is the static sentence FindingDef declares for vpcPeerCodePendingAcceptance;
	// the expiration date is its own "Expires" row.
	const wantDetail = "The peer has not accepted this request yet, and AWS expires it a week after creation; the countdown is in the status and the date is listed below. Ask the accepter to approve it."
	if f.Detail != wantDetail {
		t.Errorf("Detail = %q, want %q", f.Detail, wantDetail)
	}
	ad, ok := r.AttentionDetails[f.Code]
	if !ok {
		t.Fatalf("AttentionDetails[%v] not found", f.Code)
	}
	var gotExpires string
	for _, row := range ad.Rows {
		if row.Label == "Expires" {
			gotExpires = row.Value
		}
	}
	if gotExpires == "" {
		t.Errorf("Expires row missing or empty: %+v", ad.Rows)
	}
}

// 30h remaining rounds up to 2d (math.Ceil).
func TestFetchVpcPeeringConnectionsPage_StatePhrase_PendingAcceptance_RoundsUpAtDayBoundary(t *testing.T) {
	const id = "pcx-0boundary000001a"
	out := vpcPeerConnWithStatus(id, ec2types.VpcPeeringConnectionStateReasonCodePendingAcceptance)
	expiration := time.Now().Add(30 * time.Hour)
	out.VpcPeeringConnections[0].ExpirationTime = &expiration

	fake := &vpcPeerEC2Fake{listOut: out}
	result, err := awsclient.FetchVpcPeeringConnectionsPage(context.Background(), fake, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	r := mustFindVpcPeerResource(t, result.Resources, id)
	if len(r.Findings) != 1 {
		t.Fatalf("Findings: expected exactly 1, got %d: %+v", len(r.Findings), r.Findings)
	}
	f := r.Findings[0]
	const wantPhrase = "pending acceptance: expires in 2d"
	if f.Phrase != wantPhrase {
		t.Errorf("Phrase = %q, want %q (30h remaining rounds UP to 2d, not truncated to 1d)", f.Phrase, wantPhrase)
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
	const wantDetail = "The peering request was never answered and has lapsed, so no traffic crosses and the connection cannot be accepted now. Delete it and raise a new request once the other account is ready to accept."
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
	// Detail is the static sentence FindingDef declares for vpcPeerCodeRejected;
	// the accepter's status message is its own "Status message" row.
	const wantDetail = "The accepter rejected this peering request, so nothing will ever route across it; AWS keeps the record listed for a while. Delete it and request again once the other side agrees."
	if f.Detail != wantDetail {
		t.Errorf("Detail = %q, want %q", f.Detail, wantDetail)
	}
	ad, ok := r.AttentionDetails[f.Code]
	if !ok {
		t.Fatalf("AttentionDetails[%v] not found", f.Code)
	}
	var gotMessage string
	for _, row := range ad.Rows {
		if row.Label == "Status message" {
			gotMessage = row.Value
		}
	}
	if gotMessage != "Rejected by accepter: CIDR conflict" {
		t.Errorf("Status message row = %q, want %q (Status.Message verbatim)", gotMessage, "Rejected by accepter: CIDR conflict")
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
	// Detail is the static sentence FindingDef declares for vpcPeerCodeFailed;
	// the status message is its own "Status message" row.
	const wantDetail = "The peering connection failed to establish and will not recover on its own; the status message is listed below. Delete it and request a new one."
	if f.Detail != wantDetail {
		t.Errorf("Detail = %q, want %q", f.Detail, wantDetail)
	}
	ad, ok := r.AttentionDetails[f.Code]
	if !ok {
		t.Fatalf("AttentionDetails[%v] not found", f.Code)
	}
	var gotMessage string
	for _, row := range ad.Rows {
		if row.Label == "Status message" {
			gotMessage = row.Value
		}
	}
	if gotMessage != "Failed to activate the peering connection due to an internal error" {
		t.Errorf("Status message row = %q, want %q (Status.Message verbatim)", gotMessage, "Failed to activate the peering connection due to an internal error")
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
	const wantDetail = "The connection is being torn down, and when it goes, traffic between the two VPCs stops and every route pointing at it becomes a silent drop. Remove those routes, or recreate the peering if this was not intended."
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
	if f.Detail != "" {
		t.Errorf("Detail = %q, want it empty at the Dim tier", f.Detail)
	}
}

// provisioning and initiating-request have no demo fixture, so these cases
// use isolated fakes.

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
	const wantDetail = "The connection is being set up and does not carry traffic yet. Wait for it to become active, then add routes on both sides before expecting anything to cross."
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
	const wantDetail = "The request has been made and the other VPC's owner has not accepted it yet, so nothing crosses between the two. Have the accepter approve it, then add routes on both sides — an accepted peering with no routes still carries nothing."
	if f.Detail != wantDetail {
		t.Errorf("Detail = %q, want %q", f.Detail, wantDetail)
	}
}

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
	// Detail is the static sentence FindingDef declares for vpcPeerCodeCidrOverlap;
	// the overlapping range is its own "Overlapping range" row.
	const wantDetail = "The requester and accepter VPCs have overlapping address ranges, so routes into the overlap are blackholed; the range is listed below. Re-address one side, or peer a VPC that does not overlap."
	if f.Detail != wantDetail {
		t.Errorf("Detail = %q, want %q", f.Detail, wantDetail)
	}
	ad, ok := r.AttentionDetails[f.Code]
	if !ok {
		t.Fatalf("AttentionDetails[%v] not found", f.Code)
	}
	var gotRange string
	for _, row := range ad.Rows {
		if row.Label == "Overlapping range" {
			gotRange = row.Value
		}
	}
	if !strings.Contains(gotRange, "10.0.0.0/16") {
		t.Errorf("Overlapping range row = %q, want it to name %q", gotRange, "10.0.0.0/16")
	}
}

// CidrBlock and CidrBlockSet are present only on active connections; every
// non-active demo fixture has them nil.

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

// Cross-account, cross-region and DNS options are never findings; every demo
// connection is cross-account.

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

// EnrichVpcPeerRoutes scans the loaded rtb cache with no AWS call; both
// signals are "~" (SevWarn).

// vpcPeerRTBCache builds an "rtb" cache entry from the demo route tables: two
// routes into ProdPeerSharedID, one blackholed route into WarnPeerBlackholeID,
// none into WarnPeerNoRouteID.
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

	// A nil EC2 client turns any live call from the cache-scan enricher into a
	// panic.
	clients := &awsclient.ServiceClients{}
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
	const wantDetail = "The connection is active but no loaded route table sends anything to it, so it carries no traffic and looks connected while behaving as if it were not. Add a route to the peer's address range in the route tables of the subnets that need it."
	if f.Detail != wantDetail {
		t.Errorf("Detail = %q, want %q", f.Detail, wantDetail)
	}
}

func TestEnrichVpcPeerRoutes_RouteBlackholed(t *testing.T) {
	demo := fetchVpcPeerDemoPage(t)
	pcx := mustFindVpcPeerResource(t, demo.Resources, fixtures.WarnPeerBlackholeID)

	// A nil EC2 client turns any live call from the cache-scan enricher into a
	// panic.
	clients := &awsclient.ServiceClients{}
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
	const wantDetail = "A route points at this connection and its state is blackhole, so packets matching it are dropped silently, which reads as a firewall problem from the instance's side. Repoint the route at a live target or remove it."
	if f.Detail != wantDetail {
		t.Errorf("Detail = %q, want %q", f.Detail, wantDetail)
	}
}

func TestEnrichVpcPeerRoutes_ActiveRoutedNoFinding(t *testing.T) {
	demo := fetchVpcPeerDemoPage(t)
	pcx := mustFindVpcPeerResource(t, demo.Resources, fixtures.ProdPeerSharedID)

	// A nil EC2 client turns any live call from the cache-scan enricher into a
	// panic.
	clients := &awsclient.ServiceClients{}
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

	// A nil EC2 client turns any live call from the cache-scan enricher into a
	// panic.
	clients := &awsclient.ServiceClients{}
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

	// A nil EC2 client turns any live call from the cache-scan enricher into a
	// panic.
	clients := &awsclient.ServiceClients{}
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

	// A nil EC2 client turns any live call from the cache-scan enricher into a
	// panic.
	clients := &awsclient.ServiceClients{}
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
