package unit

// enrichment_unmatched_ids_test.go — Contract tests for EnricherResult.UnmatchedIDs.
//
// UnmatchedIDs carries API identifiers the enricher observed but could not normalize
// to any input Resource.ID. This replaces the previous silent-drop contract.
//
// Pattern: account-wide enrichers (EC2 instance status, EBS volume status, RDS
// maintenance) call the API once and receive results for ALL resources in the account.
// When the API returns an entry keyed by an ID that is not in the input resources
// slice (e.g., not on the current page), the enricher MUST append that identifier
// to UnmatchedIDs rather than silently dropping it.
//
// Tests use existing fake infrastructure from aws_ec2_enricher_test.go,
// enrichment_ebs_findings_test.go, and enrichment_rds_findings_test.go (same package).
//
// Tests:
//   1. TestEnrichEC2InstanceStatus_UnmatchedIDsOnUnknownInstance:
//      API returns 3 instances; only 2 are in the input slice.
//      → len(UnmatchedIDs) == 1; entry is the unknown instance ID.
//   2. TestEnrichEBSVolumeStatus_UnmatchedIDsOnUnknownVolume:
//      Same shape for volumes.
//   3. TestEnricher_UnmatchedIDs_NeverOverlapsFindings:
//      For EnrichRDSDocDBMaintenance, no ID appears in both Findings and UnmatchedIDs.
//      An unmatched ID is one the enricher could not attribute — it cannot also be
//      in Findings (which are keyed by attributed resource IDs).

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// Test 1: EC2 instance status — unknown instance produces UnmatchedIDs entry
// ---------------------------------------------------------------------------

// TestEnrichEC2InstanceStatus_UnmatchedIDsOnUnknownInstance verifies that when the
// API returns status for 3 instances but only 2 are in the input resources slice,
// the enricher appends the unknown instance ID to UnmatchedIDs.
//
// Uses ec2InstanceStatusFake defined in aws_ec2_enricher_test.go.
func TestEnrichEC2InstanceStatus_UnmatchedIDsOnUnknownInstance(t *testing.T) {
	const known1 = "i-0aaaa1111bbbbb222"
	const known2 = "i-0bbbb2222ccccc333"
	const unknown = "i-0cccc3333phantom0"

	fake := &ec2InstanceStatusFake{
		statuses: []ec2types.InstanceStatus{
			{
				InstanceId: aws.String(known1),
				SystemStatus: &ec2types.InstanceStatusSummary{
					Status: ec2types.SummaryStatusOk,
				},
				InstanceStatus: &ec2types.InstanceStatusSummary{
					Status: ec2types.SummaryStatusOk,
				},
			},
			{
				InstanceId: aws.String(known2),
				SystemStatus: &ec2types.InstanceStatusSummary{
					Status: ec2types.SummaryStatusImpaired,
				},
				InstanceStatus: &ec2types.InstanceStatusSummary{
					Status: ec2types.SummaryStatusOk,
				},
			},
			{
				InstanceId: aws.String(unknown),
				SystemStatus: &ec2types.InstanceStatusSummary{
					Status: ec2types.SummaryStatusImpaired,
				},
				InstanceStatus: &ec2types.InstanceStatusSummary{
					Status: ec2types.SummaryStatusOk,
				},
			},
		},
	}

	// Input resources: only known1 and known2 are on this page.
	resources := []resource.Resource{
		{ID: known1, Fields: map[string]string{"state": "running"}},
		{ID: known2, Fields: map[string]string{"state": "running"}},
	}

	clients := &awsclient.ServiceClients{EC2: fake}
	result, err := awsclient.EnrichEC2InstanceStatus(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// UnmatchedIDs must contain exactly the one unknown instance.
	if len(result.UnmatchedIDs) != 1 {
		t.Errorf("len(UnmatchedIDs) = %d, want 1; got: %v", len(result.UnmatchedIDs), result.UnmatchedIDs)
	} else if result.UnmatchedIDs[0] != unknown {
		t.Errorf("UnmatchedIDs[0] = %q, want %q", result.UnmatchedIDs[0], unknown)
	}

	// known1 and known2 must NOT appear in UnmatchedIDs.
	for _, id := range result.UnmatchedIDs {
		if id == known1 || id == known2 {
			t.Errorf("known input resource %q must not appear in UnmatchedIDs", id)
		}
	}
}

// ---------------------------------------------------------------------------
// Test 2: EBS volume status — unknown volume produces UnmatchedIDs entry
// ---------------------------------------------------------------------------

// TestEnrichEBSVolumeStatus_UnmatchedIDsOnUnknownVolume verifies that when the API
// returns status for 3 volumes but only 2 are in the input resources slice, the
// enricher appends the unknown volume ID to UnmatchedIDs.
//
// Uses ebsStatusFake defined in enrichment_ebs_findings_test.go.
func TestEnrichEBSVolumeStatus_UnmatchedIDsOnUnknownVolume(t *testing.T) {
	const knownVol1 = "vol-0abc000000000001"
	const knownVol2 = "vol-0abc000000000002"
	const unknownVol = "vol-0phantom000000"

	out := &ec2.DescribeVolumeStatusOutput{
		VolumeStatuses: []ec2types.VolumeStatusItem{
			{
				VolumeId:     aws.String(knownVol1),
				VolumeStatus: &ec2types.VolumeStatusInfo{Status: "impaired"},
			},
			{
				VolumeId:     aws.String(knownVol2),
				VolumeStatus: &ec2types.VolumeStatusInfo{Status: "ok"},
			},
			{
				VolumeId:     aws.String(unknownVol),
				VolumeStatus: &ec2types.VolumeStatusInfo{Status: "impaired"},
			},
		},
	}

	// Input resources: only knownVol1 and knownVol2 are on this page.
	resources := []resource.Resource{
		{ID: knownVol1, Fields: map[string]string{}},
		{ID: knownVol2, Fields: map[string]string{}},
	}

	clients := &awsclient.ServiceClients{EC2: &ebsStatusFake{volumeOutput: out}}
	result, err := awsclient.EnrichEBSVolumeStatus(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// UnmatchedIDs must contain exactly the one unknown volume.
	if len(result.UnmatchedIDs) != 1 {
		t.Errorf("len(UnmatchedIDs) = %d, want 1; got: %v", len(result.UnmatchedIDs), result.UnmatchedIDs)
	} else if result.UnmatchedIDs[0] != unknownVol {
		t.Errorf("UnmatchedIDs[0] = %q, want %q", result.UnmatchedIDs[0], unknownVol)
	}

	// Known volumes must not appear in UnmatchedIDs.
	for _, id := range result.UnmatchedIDs {
		if id == knownVol1 || id == knownVol2 {
			t.Errorf("known input resource %q must not appear in UnmatchedIDs", id)
		}
	}
}

// ---------------------------------------------------------------------------
// Test 3: UnmatchedIDs and Findings never overlap — invariant
// ---------------------------------------------------------------------------

// TestEnricher_UnmatchedIDs_NeverOverlapsFindings verifies that no identifier
// appears in both Findings (keyed by attributed resource ID) and UnmatchedIDs
// (API identifiers the enricher could not attribute). The two sets are mutually
// exclusive by definition: if an enricher could match an API identifier to an
// input resource, it goes to Findings; if not, it goes to UnmatchedIDs.
//
// We exercise EnrichRDSDocDBMaintenance with two ARNs: one matching an input
// resource ("on-page-db") and one not matching any ("off-page-db").
//
// UnmatchedIDs holds the raw API identifier (full ARN for RDS), per the contract:
// "Value = the raw identifier as seen from the API (ARN, ARN prefix, name)."
//
// Uses enrichRDSFake defined in enrichment_rds_findings_test.go.
func TestEnricher_UnmatchedIDs_NeverOverlapsFindings(t *testing.T) {
	const onPage = "on-page-db"
	const offPage = "off-page-db"
	const offPageARN = "arn:aws:rds:us-east-1:123456789012:db:" + offPage

	fake := &enrichRDSFake{
		actions: []rdstypes.ResourcePendingMaintenanceActions{
			{
				ResourceIdentifier: aws.String("arn:aws:rds:us-east-1:123456789012:db:" + onPage),
				PendingMaintenanceActionDetails: []rdstypes.PendingMaintenanceAction{
					{Action: aws.String("system-update")},
				},
			},
			{
				ResourceIdentifier: aws.String(offPageARN),
				PendingMaintenanceActionDetails: []rdstypes.PendingMaintenanceAction{
					{Action: aws.String("system-update")},
				},
			},
		},
	}

	// Only onPage is in the input; offPage is not.
	resources := []resource.Resource{{ID: onPage}}

	clients := &awsclient.ServiceClients{RDS: fake}
	result, err := awsclient.EnrichRDSDocDBMaintenance(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The unmatched entry must appear in UnmatchedIDs. RDS enricher stores the raw
	// ARN as seen from the API (not the suffix), per the UnmatchedIDs contract.
	foundUnmatched := false
	for _, id := range result.UnmatchedIDs {
		if id == offPageARN {
			foundUnmatched = true
		}
	}
	if !foundUnmatched {
		t.Errorf("UnmatchedIDs must contain the raw ARN %q (no input resource matched); got: %v", offPageARN, result.UnmatchedIDs)
	}

	// Invariant: no identifier appears in BOTH Findings keys and UnmatchedIDs.
	// Findings are keyed by Resource.ID (e.g. "on-page-db"); UnmatchedIDs hold
	// raw API identifiers (ARNs). These must not overlap.
	findingKeys := make(map[string]bool, len(result.Findings))
	for k := range result.Findings {
		findingKeys[k] = true
	}
	for _, id := range result.UnmatchedIDs {
		if findingKeys[id] {
			t.Errorf("ID %q appears in both Findings and UnmatchedIDs — invariant violated", id)
		}
	}

	// onPage must appear in Findings (it was matched).
	if _, ok := result.Findings[onPage]; !ok {
		t.Errorf("Findings must contain %q (matched input resource)", onPage)
	}
}

// ---------------------------------------------------------------------------
// Compile-time: ebsStatusFake must stub DescribeVolumeStatus correctly
// ---------------------------------------------------------------------------
// (ebsStatusFake is defined in enrichment_ebs_findings_test.go and already
// provides the required DescribeVolumeStatus override — no duplicate needed.)

// rdsUnmatchedInputFake is a local alias that reuses enrichRDSFake for
// the invariant test above. No additional type needed — enrichRDSFake is
// already defined in enrichment_rds_findings_test.go in the same package.
var _ awsclient.RDSAPI = (*enrichRDSFake)(nil)
