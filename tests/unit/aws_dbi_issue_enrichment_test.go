package unit

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

type dbiMaintenanceFake struct {
	awsclient.RDSAPI
	pages [][]rdstypes.ResourcePendingMaintenanceActions
	call  int
	err   error
}

func (f *dbiMaintenanceFake) DescribePendingMaintenanceActions(
	_ context.Context,
	_ *rds.DescribePendingMaintenanceActionsInput,
	_ ...func(*rds.Options),
) (*rds.DescribePendingMaintenanceActionsOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	if len(f.pages) == 0 {
		return &rds.DescribePendingMaintenanceActionsOutput{}, nil
	}
	idx := f.call
	if idx >= len(f.pages) {
		return &rds.DescribePendingMaintenanceActionsOutput{}, nil
	}
	f.call++
	var marker *string
	if f.call < len(f.pages) {
		marker = aws.String("next")
	}
	return &rds.DescribePendingMaintenanceActionsOutput{
		PendingMaintenanceActions: f.pages[idx],
		Marker:                    marker,
	}, nil
}

// buildDbiResources converts all DBIFixtures.Instances into []resource.Resource
// by running them through FetchRDSInstancesPage (so Resource.Status is
// correctly derived by the fetcher, not hardcoded in tests).
func buildDbiResources(t *testing.T) []resource.Resource {
	t.Helper()
	fix := fixtures.NewDBIFixtures()
	mock := &fakeRDSDescribeDBInstances{Output: &rds.DescribeDBInstancesOutput{DBInstances: fix.Instances}}
	result, err := awsclient.FetchRDSInstancesPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("buildDbiResources: FetchRDSInstancesPage error: %v", err)
	}
	return result.Resources
}

func TestDBI_Enrich_MaintenancePending_HealthyRow(t *testing.T) {
	resources := buildDbiResources(t)
	fix := fixtures.NewDBIFixtures()

	fake := &dbiMaintenanceFake{
		pages: [][]rdstypes.ResourcePendingMaintenanceActions{fix.PendingMaintenanceActions},
	}
	clients := &awsclient.ServiceClients{RDS: fake}

	result, err := awsclient.EnrichDBIMaintenance(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("EnrichDBIMaintenance error: %v", err)
	}

	findings, ok := result.Findings[fixtures.MaintDbiScheduledID]
	if !ok {
		t.Fatalf("expected finding for %q; Findings keys = %v", fixtures.MaintDbiScheduledID, findingKeys(result.Findings))
	}
	finding := findings[0]
	if finding.Severity != domain.SevWarn {
		t.Errorf("Severity = %v, want SevWarn", finding.Severity)
	}

	// Concrete details (Action, Description) belong in AttentionDetail rows and
	// in Detail, not in the phrase.
	if finding.Phrase != "maintenance scheduled" {
		t.Errorf("Phrase = %q, want %q", finding.Phrase, "maintenance scheduled")
	}
	if strings.Contains(finding.Phrase, "system-update") || strings.Contains(finding.Phrase, "New minor engine patch") {
		t.Errorf("Phrase must not embed Row content; got %q", finding.Phrase)
	}

	// Detail is the one static sentence FindingDef declares for
	// dbiCodePendingMaintenance (catalog_databases.go); the concrete facts live
	// in the AttentionDetail rows.
	const wantDetail = "AWS has a maintenance action pending for this instance and will apply it in a maintenance window of its choosing once the target date passes; the action, apply method and earliest date are listed below. Apply it yourself in a window that suits you."
	if finding.Detail != wantDetail {
		t.Errorf("Detail = %q, want %q", finding.Detail, wantDetail)
	}
	wantRows := map[string]string{
		"Action":          "system-update",
		"Description":     "New minor engine patch 16.2.3",
		"Earliest Target": "2026-04-01",
	}
	gotRows := map[string]string{}
	for _, r := range result.AttentionDetails[fixtures.MaintDbiScheduledID][finding.Code].Rows {
		gotRows[r.Label] = r.Value
	}
	for label, val := range wantRows {
		if gotRows[label] != val {
			t.Errorf("Rows[%q] = %q, want %q", label, gotRows[label], val)
		}
	}

	// FieldUpdates must be nil/empty — the merged display phrase is
	// computed by phraseFromFindings(r.Findings) at render time.
	if updates, ok := result.FieldUpdates[fixtures.MaintDbiScheduledID]; ok && len(updates) != 0 {
		t.Errorf("AS-140: expected empty FieldUpdates for %q (status overlay removed); got %v", fixtures.MaintDbiScheduledID, updates)
	}

}

func TestDBI_Enrich_MaintenancePending_NilDescription(t *testing.T) {
	const resourceID = "inline-no-desc"
	const arn = "arn:aws:rds:us-east-1:123456789012:db:" + resourceID

	fake := &dbiMaintenanceFake{
		pages: [][]rdstypes.ResourcePendingMaintenanceActions{
			{
				{
					ResourceIdentifier: aws.String(arn),
					PendingMaintenanceActionDetails: []rdstypes.PendingMaintenanceAction{
						{Action: aws.String("os-upgrade"), Description: nil},
					},
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{RDS: fake}
	resources := []resource.Resource{
		{ID: resourceID, Name: resourceID, Fields: map[string]string{"status": ""}},
	}

	result, err := awsclient.EnrichDBIMaintenance(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("EnrichDBIMaintenance error: %v", err)
	}
	findings, ok := result.Findings[resourceID]
	if !ok {
		t.Fatalf("expected finding for %q", resourceID)
	}
	finding := findings[0]
	if finding.Phrase != "maintenance scheduled" {
		t.Errorf("Phrase = %q, want %q", finding.Phrase, "maintenance scheduled")
	}
	var labels []string
	for _, r := range result.AttentionDetails[resourceID][finding.Code].Rows {
		labels = append(labels, r.Label)
	}
	for _, l := range labels {
		if l == "Description" {
			t.Errorf("Rows must omit Description when source field is nil; got labels=%v", labels)
		}
	}

	const wantDetail = "AWS has a maintenance action pending for this instance and will apply it in a maintenance window of its choosing once the target date passes; the action, apply method and earliest date are listed below. Apply it yourself in a window that suits you."
	if finding.Detail != wantDetail {
		t.Errorf("Detail = %q, want %q (nil Description must not blank the definition's sentence)", finding.Detail, wantDetail)
	}
}

func TestDBI_Enrich_NonHealthyStatus_NoFieldUpdates(t *testing.T) {
	const resourceID = "inline-already-warning"
	const arn = "arn:aws:rds:us-east-1:123456789012:db:" + resourceID

	fake := &dbiMaintenanceFake{
		pages: [][]rdstypes.ResourcePendingMaintenanceActions{
			{
				{
					ResourceIdentifier: aws.String(arn),
					PendingMaintenanceActionDetails: []rdstypes.PendingMaintenanceAction{
						{Action: aws.String("os-upgrade"), Description: aws.String("OS patch")},
					},
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{RDS: fake}
	resources := []resource.Resource{
		{
			ID:     resourceID,
			Name:   resourceID,
			Fields: map[string]string{"status": "publicly accessible"},
		},
	}

	result, err := awsclient.EnrichDBIMaintenance(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("EnrichDBIMaintenance error: %v", err)
	}

	if _, ok := result.Findings[resourceID]; !ok {
		t.Errorf("expected finding for %q on non-healthy row (S5 still needed)", resourceID)
	}

	if updates, ok := result.FieldUpdates[resourceID]; ok && len(updates) != 0 {
		t.Errorf("AS-140: expected empty FieldUpdates for %q (status overlay removed); got %v", resourceID, updates)
	}
}

func TestDBI_Enrich_NoMatchNoFinding(t *testing.T) {
	fake := &dbiMaintenanceFake{
		pages: [][]rdstypes.ResourcePendingMaintenanceActions{
			{
				{
					ResourceIdentifier: aws.String("arn:aws:rds:us-east-1:123456789012:db:not-in-resources"),
					PendingMaintenanceActionDetails: []rdstypes.PendingMaintenanceAction{
						{Action: aws.String("system-update")},
					},
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{RDS: fake}
	resources := []resource.Resource{
		{ID: "other-instance", Fields: map[string]string{}},
	}

	result, err := awsclient.EnrichDBIMaintenance(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("EnrichDBIMaintenance error: %v", err)
	}
	if len(result.Findings) != 0 {
		t.Errorf("Findings must be empty when no ARN matches; got %v", result.Findings)
	}
}

func TestDBI_Enrich_NilRDSClient(t *testing.T) {
	clients := &awsclient.ServiceClients{RDS: nil}
	result, err := awsclient.EnrichDBIMaintenance(context.Background(), clients, nil, nil)
	if err != nil {
		t.Fatalf("EnrichDBIMaintenance error: %v", err)
	}
	if result.Findings == nil {
		t.Error("Findings must not be nil even when RDS client is nil")
	}
	if len(result.Findings) != 0 {
		t.Errorf("len(Findings) = %d, want 0 (nil RDS client, no resources)", len(result.Findings))
	}
}

func TestDBI_Enrich_Pagination(t *testing.T) {
	past := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	page1 := []rdstypes.ResourcePendingMaintenanceActions{
		{
			ResourceIdentifier: aws.String("arn:aws:rds:us-east-1:123456789012:db:dbi-page1"),
			PendingMaintenanceActionDetails: []rdstypes.PendingMaintenanceAction{
				{Action: aws.String("system-update"), AutoAppliedAfterDate: aws.Time(past)},
			},
		},
	}
	page2 := []rdstypes.ResourcePendingMaintenanceActions{
		{
			ResourceIdentifier: aws.String("arn:aws:rds:us-east-1:123456789012:db:dbi-page2"),
			PendingMaintenanceActionDetails: []rdstypes.PendingMaintenanceAction{
				{Action: aws.String("os-upgrade"), AutoAppliedAfterDate: aws.Time(past)},
			},
		},
	}

	fake := &dbiMaintenanceFake{pages: [][]rdstypes.ResourcePendingMaintenanceActions{page1, page2}}
	clients := &awsclient.ServiceClients{RDS: fake}
	resources := []resource.Resource{
		{ID: "dbi-page1", Fields: map[string]string{"status": ""}},
		{ID: "dbi-page2", Fields: map[string]string{"status": ""}},
	}

	result, err := awsclient.EnrichDBIMaintenance(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("EnrichDBIMaintenance error: %v", err)
	}
	if _, ok := result.Findings["dbi-page1"]; !ok {
		t.Error("expected finding for dbi-page1 (from page 1)")
	}
	if _, ok := result.Findings["dbi-page2"]; !ok {
		t.Error("expected finding for dbi-page2 (from page 2)")
	}
}

func TestDBI_Enrich_Wave1PlusWave2_NoFieldUpdates(t *testing.T) {
	const resourceID = fixtures.WarnDbiPublicMaintID
	const arn = fixtures.WarnDbiPublicMaintARN

	fake := &dbiMaintenanceFake{
		pages: [][]rdstypes.ResourcePendingMaintenanceActions{
			{
				{
					ResourceIdentifier: aws.String(arn),
					PendingMaintenanceActionDetails: []rdstypes.PendingMaintenanceAction{
						{Action: aws.String("system-update"), Description: aws.String("OS patch")},
					},
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{RDS: fake}
	resources := []resource.Resource{
		{
			ID:     resourceID,
			Name:   resourceID,
			Fields: map[string]string{"status": "publicly accessible"},
		},
	}

	result, err := awsclient.EnrichDBIMaintenance(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("EnrichDBIMaintenance error: %v", err)
	}

	findings, ok := result.Findings[resourceID]
	if !ok {
		t.Fatalf("expected finding for %q; Findings keys = %v", resourceID, findingKeys(result.Findings))
	}
	finding := findings[0]
	if finding.Severity != domain.SevWarn {
		t.Errorf("Severity = %v, want SevWarn", finding.Severity)
	}

	if updates, ok := result.FieldUpdates[resourceID]; ok && len(updates) != 0 {
		t.Errorf("AS-140: expected empty FieldUpdates for %q (status overlay removed); got %v", resourceID, updates)
	}

}

func TestDBI_Enrich_Wave1PlusWave2_PostPR03eShape_NoFieldUpdates(t *testing.T) {
	const resourceID = fixtures.WarnDbiPublicMaintID
	const arn = fixtures.WarnDbiPublicMaintARN

	fake := &dbiMaintenanceFake{
		pages: [][]rdstypes.ResourcePendingMaintenanceActions{
			{
				{
					ResourceIdentifier: aws.String(arn),
					PendingMaintenanceActionDetails: []rdstypes.PendingMaintenanceAction{
						{Action: aws.String("os-upgrade"), Description: aws.String("Kernel security patch")},
					},
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{RDS: fake}

	resources := []resource.Resource{
		{
			ID:   resourceID,
			Name: resourceID,
			Findings: []domain.Finding{
				{Code: awsclient.CodeDBIPubliclyAccessible, Phrase: "publicly accessible", Severity: domain.SevWarn, Source: "wave1"},
			},
			Fields: map[string]string{"status": "publicly accessible"},
		},
	}

	result, err := awsclient.EnrichDBIMaintenance(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("EnrichDBIMaintenance error: %v", err)
	}

	if _, ok := result.Findings[resourceID]; !ok {
		t.Errorf("expected finding for %q on post-PR-03e wave-1 shape", resourceID)
	}

	if updates, ok := result.FieldUpdates[resourceID]; ok && len(updates) != 0 {
		t.Errorf("AS-140: expected empty FieldUpdates for %q (status overlay removed); got %v", resourceID, updates)
	}
}

func TestDBI_Enrich_Wave1MultiPlusWave2_NoFieldUpdates(t *testing.T) {
	const resourceID = "inline-3warn-plus-maint"
	const arn = "arn:aws:rds:us-east-1:123456789012:db:" + resourceID

	fake := &dbiMaintenanceFake{
		pages: [][]rdstypes.ResourcePendingMaintenanceActions{
			{
				{
					ResourceIdentifier: aws.String(arn),
					PendingMaintenanceActionDetails: []rdstypes.PendingMaintenanceAction{
						{Action: aws.String("system-update"), Description: aws.String("Minor patch")},
					},
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{RDS: fake}
	resources := []resource.Resource{
		{
			ID:     resourceID,
			Name:   resourceID,
			Fields: map[string]string{"status": "no automated backups (+2)"},
		},
	}

	result, err := awsclient.EnrichDBIMaintenance(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("EnrichDBIMaintenance error: %v", err)
	}

	if _, ok := result.Findings[resourceID]; !ok {
		t.Errorf("expected finding for %q (wave-2 must emit even when wave-1 has +N)", resourceID)
	}
	if updates, ok := result.FieldUpdates[resourceID]; ok && len(updates) != 0 {
		t.Errorf("AS-140: expected empty FieldUpdates for %q (status overlay removed); got %v", resourceID, updates)
	}
}

func TestDBI_Enrich_HealthyPlusWave2_NoFieldUpdates_Regression(t *testing.T) {
	fix := fixtures.NewDBIFixtures()
	resources := buildDbiResources(t)

	fake := &dbiMaintenanceFake{
		pages: [][]rdstypes.ResourcePendingMaintenanceActions{fix.PendingMaintenanceActions},
	}
	clients := &awsclient.ServiceClients{RDS: fake}

	result, err := awsclient.EnrichDBIMaintenance(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("EnrichDBIMaintenance error: %v", err)
	}

	if _, ok := result.Findings[fixtures.MaintDbiScheduledID]; !ok {
		t.Errorf("expected finding for %q", fixtures.MaintDbiScheduledID)
	}
	if updates, ok := result.FieldUpdates[fixtures.MaintDbiScheduledID]; ok && len(updates) != 0 {
		t.Errorf("AS-140: expected empty FieldUpdates for %q (status overlay removed); got %v", fixtures.MaintDbiScheduledID, updates)
	}
}

func TestDBI_Enrich_VariousExistingStatuses_NoFieldUpdates(t *testing.T) {
	cases := []struct {
		name           string
		existingStatus string
	}{
		{name: "no_existing_suffix", existingStatus: "publicly accessible"},
		{name: "existing_suffix_1", existingStatus: "publicly accessible (+1)"},
		{name: "existing_suffix_9", existingStatus: "publicly accessible (+9)"},
		{name: "unparsable_suffix", existingStatus: "foo (+bar)"},
		{name: "healthy_empty_status", existingStatus: ""},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			id := "inline-bump-" + tc.name
			arn := "arn:aws:rds:us-east-1:123456789012:db:" + id

			fake := &dbiMaintenanceFake{
				pages: [][]rdstypes.ResourcePendingMaintenanceActions{
					{
						{
							ResourceIdentifier: aws.String(arn),
							PendingMaintenanceActionDetails: []rdstypes.PendingMaintenanceAction{
								{Action: aws.String("system-update")},
							},
						},
					},
				},
			}
			clients := &awsclient.ServiceClients{RDS: fake}
			resources := []resource.Resource{
				{
					ID:     id,
					Name:   id,
					Fields: map[string]string{"status": tc.existingStatus},
				},
			}

			result, err := awsclient.EnrichDBIMaintenance(context.Background(), clients, resources, nil)
			if err != nil {
				t.Fatalf("EnrichDBIMaintenance error: %v", err)
			}

			if _, ok := result.Findings[id]; !ok {
				t.Errorf("expected finding for %q", id)
			}

			if updates, ok := result.FieldUpdates[id]; ok && len(updates) != 0 {
				t.Errorf("AS-140: expected empty FieldUpdates for %q (existingStatus=%q); got %v", id, tc.existingStatus, updates)
			}
		})
	}
}

func findingKeys(m map[string][]domain.Finding) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
