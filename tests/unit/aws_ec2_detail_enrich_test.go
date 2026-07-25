package unit

// aws_ec2_detail_enrich_test.go — coverage for enrichEc2
// (core/aws/ec2_detail_enrichment.go), the on-demand detail enricher
// registered for the "ec2" resource type (#261).
//
// Covers:
//   - wrong clients type / nil DetailEnrichmentCtx / nil Clients → error
//     (ec2 has no DetailDocs dependency — uncached, per the contract)
//   - wrong RawStruct type → error
//   - missing InstanceId → error
//   - success: valid base64 user-data decoded to plaintext
//   - invalid base64 falls back to the raw attribute value
//   - valid gzip (base64+gzip magic) user-data decompressed to plaintext
//   - corrupt gzip (bad header after the magic) falls back to the raw base64
//     string (un-gunzipped bytes fail the utf8.Valid gate)
//   - binary non-UTF-8, non-gzip user-data falls back to the raw base64 string
//   - oversized gzip user-data is truncated at the 1 MiB decompression cap
//   - nil/empty UserData attribute → ""
//   - InstanceEnriched re-enrichment path accepted as RawStruct
//   - API error propagated
//   - registry sanity: GetDetailEnricher("ec2") non-nil
//
// The EC2 fake must implement all 21 methods of the EC2API aggregate (the
// static type of ServiceClients.EC2) plus EC2DescribeInstanceAttributeAPI,
// which the enricher type-asserts separately — mirrors enrichPolicyIAM's
// full-interface stub in aws_iam_policies_enrich_test.go.

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

// ---------------------------------------------------------------------------
// enrichEc2Fake — full EC2API + EC2DescribeInstanceAttributeAPI fake
// ---------------------------------------------------------------------------

type enrichEc2Fake struct {
	describeAttrFn    func(*ec2.DescribeInstanceAttributeInput) (*ec2.DescribeInstanceAttributeOutput, error)
	describeAttrCalls int
}

func (f *enrichEc2Fake) DescribeInstanceAttribute(_ context.Context, in *ec2.DescribeInstanceAttributeInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstanceAttributeOutput, error) {
	f.describeAttrCalls++
	if f.describeAttrFn != nil {
		return f.describeAttrFn(in)
	}
	return &ec2.DescribeInstanceAttributeOutput{}, nil
}

// --- Stubs for the rest of EC2API (unused by enrichEc2) ---

func (f *enrichEc2Fake) DescribeInstances(_ context.Context, _ *ec2.DescribeInstancesInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	return &ec2.DescribeInstancesOutput{}, nil
}
func (f *enrichEc2Fake) DescribeVpcs(_ context.Context, _ *ec2.DescribeVpcsInput, _ ...func(*ec2.Options)) (*ec2.DescribeVpcsOutput, error) {
	return &ec2.DescribeVpcsOutput{}, nil
}
func (f *enrichEc2Fake) DescribeSecurityGroups(_ context.Context, _ *ec2.DescribeSecurityGroupsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
	return &ec2.DescribeSecurityGroupsOutput{}, nil
}
func (f *enrichEc2Fake) DescribeSubnets(_ context.Context, _ *ec2.DescribeSubnetsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error) {
	return &ec2.DescribeSubnetsOutput{}, nil
}
func (f *enrichEc2Fake) DescribeRouteTables(_ context.Context, _ *ec2.DescribeRouteTablesInput, _ ...func(*ec2.Options)) (*ec2.DescribeRouteTablesOutput, error) {
	return &ec2.DescribeRouteTablesOutput{}, nil
}
func (f *enrichEc2Fake) DescribeNatGateways(_ context.Context, _ *ec2.DescribeNatGatewaysInput, _ ...func(*ec2.Options)) (*ec2.DescribeNatGatewaysOutput, error) {
	return &ec2.DescribeNatGatewaysOutput{}, nil
}
func (f *enrichEc2Fake) DescribeInternetGateways(_ context.Context, _ *ec2.DescribeInternetGatewaysInput, _ ...func(*ec2.Options)) (*ec2.DescribeInternetGatewaysOutput, error) {
	return &ec2.DescribeInternetGatewaysOutput{}, nil
}
func (f *enrichEc2Fake) DescribeAddresses(_ context.Context, _ *ec2.DescribeAddressesInput, _ ...func(*ec2.Options)) (*ec2.DescribeAddressesOutput, error) {
	return &ec2.DescribeAddressesOutput{}, nil
}
func (f *enrichEc2Fake) DescribeTransitGateways(_ context.Context, _ *ec2.DescribeTransitGatewaysInput, _ ...func(*ec2.Options)) (*ec2.DescribeTransitGatewaysOutput, error) {
	return &ec2.DescribeTransitGatewaysOutput{}, nil
}
func (f *enrichEc2Fake) DescribeTransitGatewayAttachments(_ context.Context, _ *ec2.DescribeTransitGatewayAttachmentsInput, _ ...func(*ec2.Options)) (*ec2.DescribeTransitGatewayAttachmentsOutput, error) {
	return &ec2.DescribeTransitGatewayAttachmentsOutput{}, nil
}
func (f *enrichEc2Fake) DescribeTransitGatewayVpcAttachments(_ context.Context, _ *ec2.DescribeTransitGatewayVpcAttachmentsInput, _ ...func(*ec2.Options)) (*ec2.DescribeTransitGatewayVpcAttachmentsOutput, error) {
	return &ec2.DescribeTransitGatewayVpcAttachmentsOutput{}, nil
}
func (f *enrichEc2Fake) DescribeTransitGatewayRouteTables(_ context.Context, _ *ec2.DescribeTransitGatewayRouteTablesInput, _ ...func(*ec2.Options)) (*ec2.DescribeTransitGatewayRouteTablesOutput, error) {
	return &ec2.DescribeTransitGatewayRouteTablesOutput{}, nil
}
func (f *enrichEc2Fake) DescribeVpcEndpoints(_ context.Context, _ *ec2.DescribeVpcEndpointsInput, _ ...func(*ec2.Options)) (*ec2.DescribeVpcEndpointsOutput, error) {
	return &ec2.DescribeVpcEndpointsOutput{}, nil
}
func (f *enrichEc2Fake) DescribeNetworkInterfaces(_ context.Context, _ *ec2.DescribeNetworkInterfacesInput, _ ...func(*ec2.Options)) (*ec2.DescribeNetworkInterfacesOutput, error) {
	return &ec2.DescribeNetworkInterfacesOutput{}, nil
}
func (f *enrichEc2Fake) DescribeVolumes(_ context.Context, _ *ec2.DescribeVolumesInput, _ ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error) {
	return &ec2.DescribeVolumesOutput{}, nil
}
func (f *enrichEc2Fake) DescribeSnapshots(_ context.Context, _ *ec2.DescribeSnapshotsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSnapshotsOutput, error) {
	return &ec2.DescribeSnapshotsOutput{}, nil
}
func (f *enrichEc2Fake) DescribeImages(_ context.Context, _ *ec2.DescribeImagesInput, _ ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
	return &ec2.DescribeImagesOutput{}, nil
}
func (f *enrichEc2Fake) DescribeInstanceStatus(_ context.Context, _ *ec2.DescribeInstanceStatusInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstanceStatusOutput, error) {
	return &ec2.DescribeInstanceStatusOutput{}, nil
}
func (f *enrichEc2Fake) DescribeVolumeStatus(_ context.Context, _ *ec2.DescribeVolumeStatusInput, _ ...func(*ec2.Options)) (*ec2.DescribeVolumeStatusOutput, error) {
	return &ec2.DescribeVolumeStatusOutput{}, nil
}
func (f *enrichEc2Fake) DescribeFlowLogs(_ context.Context, _ *ec2.DescribeFlowLogsInput, _ ...func(*ec2.Options)) (*ec2.DescribeFlowLogsOutput, error) {
	return &ec2.DescribeFlowLogsOutput{}, nil
}
func (f *enrichEc2Fake) DescribeLaunchTemplateVersions(_ context.Context, _ *ec2.DescribeLaunchTemplateVersionsInput, _ ...func(*ec2.Options)) (*ec2.DescribeLaunchTemplateVersionsOutput, error) {
	return &ec2.DescribeLaunchTemplateVersionsOutput{}, nil
}
func (f *enrichEc2Fake) DescribeLaunchTemplates(_ context.Context, _ *ec2.DescribeLaunchTemplatesInput, _ ...func(*ec2.Options)) (*ec2.DescribeLaunchTemplatesOutput, error) {
	return &ec2.DescribeLaunchTemplatesOutput{}, nil
}
func (f *enrichEc2Fake) DescribeVpcPeeringConnections(_ context.Context, _ *ec2.DescribeVpcPeeringConnectionsInput, _ ...func(*ec2.Options)) (*ec2.DescribeVpcPeeringConnectionsOutput, error) {
	return &ec2.DescribeVpcPeeringConnectionsOutput{}, nil
}

var _ awsclient.EC2API = (*enrichEc2Fake)(nil)
var _ awsclient.EC2DescribeInstanceAttributeAPI = (*enrichEc2Fake)(nil)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func ec2Enricher(t *testing.T) resource.DetailEnricher {
	t.Helper()
	e := resource.GetDetailEnricher("ec2")
	if e == nil {
		t.Fatal("ec2 detail enricher not registered")
	}
	return e
}

func makeEc2Ctx(client awsclient.EC2API) *awsclient.DetailEnrichmentCtx {
	return &awsclient.DetailEnrichmentCtx{Clients: &awsclient.ServiceClients{EC2: client}}
}

const ec2TestInstanceID = "i-0abc123def4567890"

func makeEc2Instance(instanceID string) ec2types.Instance {
	var idPtr *string
	if instanceID != "" {
		idPtr = aws.String(instanceID)
	}
	return ec2types.Instance{
		InstanceId:       idPtr,
		InstanceType:     ec2types.InstanceTypeT3Medium,
		PrivateIpAddress: aws.String("10.0.1.42"),
		State:            &ec2types.InstanceState{Name: ec2types.InstanceStateNameRunning},
	}
}

func makeEc2Res(instanceID string) resource.Resource {
	return resource.Resource{ID: instanceID, RawStruct: makeEc2Instance(instanceID)}
}

const ec2TestUserDataPlain = "#!/bin/bash\nyum update -y\necho \"export APP_ENV=production\" >> /etc/environment\n"

// ---------------------------------------------------------------------------
// Tests: invalid context
// ---------------------------------------------------------------------------

func TestEnrichEc2_WrongClientsType_ReturnsError(t *testing.T) {
	enricher := ec2Enricher(t)
	res := makeEc2Res(ec2TestInstanceID)

	_, err := enricher(context.Background(), "not-a-detail-ctx", res)
	if err == nil {
		t.Fatal("expected error for wrong clients type, got nil")
	}
}

func TestEnrichEc2_NilDetailEnrichmentCtx_ReturnsError(t *testing.T) {
	enricher := ec2Enricher(t)
	res := makeEc2Res(ec2TestInstanceID)

	_, err := enricher(context.Background(), (*awsclient.DetailEnrichmentCtx)(nil), res)
	if err == nil {
		t.Fatal("expected error for nil DetailEnrichmentCtx, got nil")
	}
}

func TestEnrichEc2_NilClients_ReturnsError(t *testing.T) {
	enricher := ec2Enricher(t)
	res := makeEc2Res(ec2TestInstanceID)
	ctx := &awsclient.DetailEnrichmentCtx{Clients: nil}

	_, err := enricher(context.Background(), ctx, res)
	if err == nil {
		t.Fatal("expected error for nil Clients, got nil")
	}
}

// ---------------------------------------------------------------------------
// Tests: bad RawStruct / missing InstanceId
// ---------------------------------------------------------------------------

func TestEnrichEc2_WrongRawStructType_ReturnsError(t *testing.T) {
	enricher := ec2Enricher(t)
	res := resource.Resource{ID: ec2TestInstanceID, RawStruct: "not-an-instance"}

	_, err := enricher(context.Background(), makeEc2Ctx(&enrichEc2Fake{}), res)
	if err == nil {
		t.Fatal("expected error for wrong RawStruct type, got nil")
	}
}

func TestEnrichEc2_NoInstanceId_ReturnsError(t *testing.T) {
	enricher := ec2Enricher(t)
	res := makeEc2Res("")

	_, err := enricher(context.Background(), makeEc2Ctx(&enrichEc2Fake{}), res)
	if err == nil {
		t.Fatal("expected error for instance with no InstanceId, got nil")
	}
}

// ---------------------------------------------------------------------------
// Tests: success — valid base64, invalid base64, empty/nil
// ---------------------------------------------------------------------------

func TestEnrichEc2_ValidBase64_Decoded(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte(ec2TestUserDataPlain))
	fake := &enrichEc2Fake{
		describeAttrFn: func(_ *ec2.DescribeInstanceAttributeInput) (*ec2.DescribeInstanceAttributeOutput, error) {
			return &ec2.DescribeInstanceAttributeOutput{
				UserData: &ec2types.AttributeValue{Value: aws.String(encoded)},
			}, nil
		},
	}

	enricher := ec2Enricher(t)
	res := makeEc2Res(ec2TestInstanceID)

	got, err := enricher(context.Background(), makeEc2Ctx(fake), res)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.describeAttrCalls != 1 {
		t.Errorf("DescribeInstanceAttribute called %d times, want 1", fake.describeAttrCalls)
	}
	enriched, ok := got.RawStruct.(awsclient.InstanceEnriched)
	if !ok {
		t.Fatalf("RawStruct = %T, want InstanceEnriched", got.RawStruct)
	}
	if enriched.UserData != ec2TestUserDataPlain {
		t.Errorf("enriched.UserData = %q, want decoded %q", enriched.UserData, ec2TestUserDataPlain)
	}
	if enriched.InstanceId == nil || *enriched.InstanceId != ec2TestInstanceID {
		t.Errorf("enriched.InstanceId = %v, want %q", enriched.InstanceId, ec2TestInstanceID)
	}
}

func TestEnrichEc2_InvalidBase64_FallsBackToRawValue(t *testing.T) {
	const garbage = "not-valid-base64-!!!"
	fake := &enrichEc2Fake{
		describeAttrFn: func(_ *ec2.DescribeInstanceAttributeInput) (*ec2.DescribeInstanceAttributeOutput, error) {
			return &ec2.DescribeInstanceAttributeOutput{
				UserData: &ec2types.AttributeValue{Value: aws.String(garbage)},
			}, nil
		},
	}

	enricher := ec2Enricher(t)
	res := makeEc2Res(ec2TestInstanceID)

	got, err := enricher(context.Background(), makeEc2Ctx(fake), res)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	enriched := got.RawStruct.(awsclient.InstanceEnriched)
	if enriched.UserData != garbage {
		t.Errorf("enriched.UserData = %q, want raw fallback %q", enriched.UserData, garbage)
	}
}

// gzipThenBase64 compresses data with gzip and base64-encodes the result —
// the shape enrichEc2's decode chain expects for cloud-init-style
// gzip-compressed user data (base64 → gzip magic → gunzip).
func gzipThenBase64(t *testing.T, data []byte) string {
	t.Helper()
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(data); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func TestEnrichEc2_ValidGzipUserData_Decompressed(t *testing.T) {
	const script = "#!/bin/bash\napt-get update"
	encoded := gzipThenBase64(t, []byte(script))
	fake := &enrichEc2Fake{
		describeAttrFn: func(_ *ec2.DescribeInstanceAttributeInput) (*ec2.DescribeInstanceAttributeOutput, error) {
			return &ec2.DescribeInstanceAttributeOutput{
				UserData: &ec2types.AttributeValue{Value: aws.String(encoded)},
			}, nil
		},
	}

	enricher := ec2Enricher(t)
	res := makeEc2Res(ec2TestInstanceID)

	got, err := enricher(context.Background(), makeEc2Ctx(fake), res)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	enriched := got.RawStruct.(awsclient.InstanceEnriched)
	if enriched.UserData != script {
		t.Errorf("enriched.UserData = %q, want decompressed %q", enriched.UserData, script)
	}
}

func TestEnrichEc2_CorruptGzipUserData_FallsBackToRawBase64(t *testing.T) {
	// Gzip magic (0x1f, 0x8b) followed by bytes that are not a valid gzip
	// header — gunzipUserData must fail (verified: gzip.NewReader returns
	// "gzip: invalid header" for this exact fixture), so enrichEc2 falls
	// through with the un-gunzipped bytes. Those bytes start with 0x8b, an
	// invalid UTF-8 lead byte, so the utf8.Valid gate also fails and
	// enrichEc2 must fall back to the original base64 string.
	garbage := []byte{0x1f, 0x8b, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a}
	encoded := base64.StdEncoding.EncodeToString(garbage)

	fake := &enrichEc2Fake{
		describeAttrFn: func(_ *ec2.DescribeInstanceAttributeInput) (*ec2.DescribeInstanceAttributeOutput, error) {
			return &ec2.DescribeInstanceAttributeOutput{
				UserData: &ec2types.AttributeValue{Value: aws.String(encoded)},
			}, nil
		},
	}

	enricher := ec2Enricher(t)
	res := makeEc2Res(ec2TestInstanceID)

	got, err := enricher(context.Background(), makeEc2Ctx(fake), res)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	enriched := got.RawStruct.(awsclient.InstanceEnriched)
	if enriched.UserData != encoded {
		t.Errorf("enriched.UserData = %q, want original base64 fallback %q", enriched.UserData, encoded)
	}
}

func TestEnrichEc2_BinaryNonUTF8NonGzip_FallsBackToRawBase64(t *testing.T) {
	// Not gzip-magic-prefixed (first byte 0xff, not 0x1f) so the gunzip
	// branch is skipped entirely; the raw bytes are not valid UTF-8 either,
	// so enrichEc2 must fall back to the original base64 string.
	binary := []byte{0xff, 0xfe, 0x00}
	encoded := base64.StdEncoding.EncodeToString(binary)

	fake := &enrichEc2Fake{
		describeAttrFn: func(_ *ec2.DescribeInstanceAttributeInput) (*ec2.DescribeInstanceAttributeOutput, error) {
			return &ec2.DescribeInstanceAttributeOutput{
				UserData: &ec2types.AttributeValue{Value: aws.String(encoded)},
			}, nil
		},
	}

	enricher := ec2Enricher(t)
	res := makeEc2Res(ec2TestInstanceID)

	got, err := enricher(context.Background(), makeEc2Ctx(fake), res)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	enriched := got.RawStruct.(awsclient.InstanceEnriched)
	if enriched.UserData != encoded {
		t.Errorf("enriched.UserData = %q, want original base64 fallback %q", enriched.UserData, encoded)
	}
}

// TestEnrichEc2_OversizedGzipUserData_FallsBackToRawBase64 pins gunzipUserData's
// explicit-error-above-the-cap contract (#261 boundary-sealing wave, item f):
// an oversized decompressed payload must never be presented as complete,
// truncated content. gunzipUserData itself is unexported (core/aws), so the
// only externally observable proof that it returned an error rather than a
// truncated []byte is that decodeUserData's caller-side fallback chain takes
// over exactly like any other unrecoverable gunzip failure (see
// TestEnrichEc2_CorruptGzipUserData_FallsBackToRawBase64) — the still-gzipped
// bytes fail the utf8.Valid gate, so enrichEc2 falls back to the original
// base64 string instead of a silently truncated 1 MiB prefix.
func TestEnrichEc2_OversizedGzipUserData_FallsBackToRawBase64(t *testing.T) {
	const oneMiB = 1 << 20
	large := strings.Repeat("A", oneMiB*2) // 2 MiB decompressed, well past the 1 MiB cap
	encoded := gzipThenBase64(t, []byte(large))

	fake := &enrichEc2Fake{
		describeAttrFn: func(_ *ec2.DescribeInstanceAttributeInput) (*ec2.DescribeInstanceAttributeOutput, error) {
			return &ec2.DescribeInstanceAttributeOutput{
				UserData: &ec2types.AttributeValue{Value: aws.String(encoded)},
			}, nil
		},
	}

	enricher := ec2Enricher(t)
	res := makeEc2Res(ec2TestInstanceID)

	got, err := enricher(context.Background(), makeEc2Ctx(fake), res)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	enriched := got.RawStruct.(awsclient.InstanceEnriched)
	if enriched.UserData != encoded {
		t.Errorf("enriched.UserData = %q, want original base64 fallback %q (oversized decompression must error, never truncate)", enriched.UserData, encoded)
	}
}

// TestEnrichEc2_AtCapGzipUserData_DecodesIntact pins the cap's other edge:
// a payload that decompresses to EXACTLY maxUserDataDecompressedSize (1 MiB)
// is at-or-below the cap and must still decode in full, not trip the
// oversized fallback above.
func TestEnrichEc2_AtCapGzipUserData_DecodesIntact(t *testing.T) {
	const oneMiB = 1 << 20
	atCap := strings.Repeat("B", oneMiB)
	encoded := gzipThenBase64(t, []byte(atCap))

	fake := &enrichEc2Fake{
		describeAttrFn: func(_ *ec2.DescribeInstanceAttributeInput) (*ec2.DescribeInstanceAttributeOutput, error) {
			return &ec2.DescribeInstanceAttributeOutput{
				UserData: &ec2types.AttributeValue{Value: aws.String(encoded)},
			}, nil
		},
	}

	enricher := ec2Enricher(t)
	res := makeEc2Res(ec2TestInstanceID)

	got, err := enricher(context.Background(), makeEc2Ctx(fake), res)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	enriched := got.RawStruct.(awsclient.InstanceEnriched)
	if enriched.UserData != atCap {
		t.Errorf("enriched.UserData length = %d, want the full %d-byte at-cap payload decoded intact (not the base64 fallback)", len(enriched.UserData), oneMiB)
	}
}

func TestEnrichEc2_NilUserDataAttribute_EmptyString(t *testing.T) {
	fake := &enrichEc2Fake{
		describeAttrFn: func(_ *ec2.DescribeInstanceAttributeInput) (*ec2.DescribeInstanceAttributeOutput, error) {
			return &ec2.DescribeInstanceAttributeOutput{UserData: nil}, nil
		},
	}

	enricher := ec2Enricher(t)
	res := makeEc2Res(ec2TestInstanceID)

	got, err := enricher(context.Background(), makeEc2Ctx(fake), res)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	enriched := got.RawStruct.(awsclient.InstanceEnriched)
	if enriched.UserData != "" {
		t.Errorf("enriched.UserData = %q, want empty string for nil UserData attribute", enriched.UserData)
	}
}

func TestEnrichEc2_EmptyUserDataValue_EmptyString(t *testing.T) {
	fake := &enrichEc2Fake{
		describeAttrFn: func(_ *ec2.DescribeInstanceAttributeInput) (*ec2.DescribeInstanceAttributeOutput, error) {
			return &ec2.DescribeInstanceAttributeOutput{
				UserData: &ec2types.AttributeValue{Value: aws.String("")},
			}, nil
		},
	}

	enricher := ec2Enricher(t)
	res := makeEc2Res(ec2TestInstanceID)

	got, err := enricher(context.Background(), makeEc2Ctx(fake), res)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	enriched := got.RawStruct.(awsclient.InstanceEnriched)
	if enriched.UserData != "" {
		t.Errorf("enriched.UserData = %q, want empty string for empty UserData value", enriched.UserData)
	}
}

// ---------------------------------------------------------------------------
// Tests: re-enrichment path
// ---------------------------------------------------------------------------

func TestEnrichEc2_InstanceEnrichedRawStruct_Accepted(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte(ec2TestUserDataPlain))
	fake := &enrichEc2Fake{
		describeAttrFn: func(_ *ec2.DescribeInstanceAttributeInput) (*ec2.DescribeInstanceAttributeOutput, error) {
			return &ec2.DescribeInstanceAttributeOutput{
				UserData: &ec2types.AttributeValue{Value: aws.String(encoded)},
			}, nil
		},
	}
	enricher := ec2Enricher(t)

	res := resource.Resource{
		ID: ec2TestInstanceID,
		RawStruct: awsclient.InstanceEnriched{
			Instance: makeEc2Instance(ec2TestInstanceID),
			UserData: "stale-previous-userdata",
		},
	}

	got, err := enricher(context.Background(), makeEc2Ctx(fake), res)
	if err != nil {
		t.Fatalf("unexpected error on InstanceEnriched re-enrichment: %v", err)
	}
	enriched, ok := got.RawStruct.(awsclient.InstanceEnriched)
	if !ok {
		t.Fatalf("RawStruct = %T, want InstanceEnriched", got.RawStruct)
	}
	if enriched.UserData != ec2TestUserDataPlain {
		t.Errorf("enriched.UserData = %q, want refreshed %q", enriched.UserData, ec2TestUserDataPlain)
	}
}

// ---------------------------------------------------------------------------
// Tests: API error propagation
// ---------------------------------------------------------------------------

func TestEnrichEc2_APIError_Propagated(t *testing.T) {
	fake := &enrichEc2Fake{
		describeAttrFn: func(_ *ec2.DescribeInstanceAttributeInput) (*ec2.DescribeInstanceAttributeOutput, error) {
			return nil, errFake("DescribeInstanceAttribute: access denied")
		},
	}
	enricher := ec2Enricher(t)
	res := makeEc2Res(ec2TestInstanceID)

	_, err := enricher(context.Background(), makeEc2Ctx(fake), res)
	if err == nil {
		t.Fatal("expected error from API failure, got nil")
	}
}

// ---------------------------------------------------------------------------
// Tests: registry sanity
// ---------------------------------------------------------------------------

func TestDetailEnricherRegistry_Ec2_IsNonNil(t *testing.T) {
	e := resource.GetDetailEnricher("ec2")
	if e == nil {
		t.Fatal("ec2 detail enricher must be registered and non-nil")
	}
}
