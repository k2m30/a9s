package unit

// enrichment_codebuild_findings_test.go — Behavioral tests for EnrichCodeBuildStatus.
//
// Contract assertions (enricher-contract.md):
//   - Returns EnricherResult.Findings keyed by project name (r.ID).
//   - Severity "!" for all findings.
//   - Summary format: "latest build failed (<YYYY-MM-DD>)" (humanized phrase, not raw enum).
//   - Truncated = true when len(resources) > EnrichmentCap (50).
//   - Empty resources slice → non-nil empty Findings map.
//   - Successful builds (SUCCEEDED) must NOT appear in Findings.

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/codebuild"
	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// codeBuildEnrichFake implements CodeBuildAPI for enrichment testing.
// It embeds the interface and overrides only the two methods under test.
type codeBuildEnrichFake struct {
	awsclient.CodeBuildAPI
	// projectBuilds maps project name → build ID (latest build)
	projectBuilds map[string]string
	// builds maps build ID → Build struct
	builds map[string]cbtypes.Build
	// listErr and batchErr simulate API errors
	listErr  error
	batchErr error
}

func (f *codeBuildEnrichFake) ListBuildsForProject(
	_ context.Context,
	params *codebuild.ListBuildsForProjectInput,
	_ ...func(*codebuild.Options),
) (*codebuild.ListBuildsForProjectOutput, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	name := aws.ToString(params.ProjectName)
	if id, ok := f.projectBuilds[name]; ok {
		return &codebuild.ListBuildsForProjectOutput{Ids: []string{id}}, nil
	}
	return &codebuild.ListBuildsForProjectOutput{}, nil
}

func (f *codeBuildEnrichFake) BatchGetBuilds(
	_ context.Context,
	params *codebuild.BatchGetBuildsInput,
	_ ...func(*codebuild.Options),
) (*codebuild.BatchGetBuildsOutput, error) {
	if f.batchErr != nil {
		return nil, f.batchErr
	}
	var found []cbtypes.Build
	for _, id := range params.Ids {
		if b, ok := f.builds[id]; ok {
			found = append(found, b)
		}
	}
	return &codebuild.BatchGetBuildsOutput{Builds: found}, nil
}

// TestEnrichCodeBuildStatus_FailedBuildFindingKeyedByProjectName verifies findings
// are keyed by project name (r.ID) with severity "!".
func TestEnrichCodeBuildStatus_FailedBuildFindingKeyedByProjectName(t *testing.T) {
	endTime := time.Date(2026, 4, 14, 12, 0, 0, 0, time.UTC)
	fake := &codeBuildEnrichFake{
		projectBuilds: map[string]string{
			"my-project": "my-project:build-001",
		},
		builds: map[string]cbtypes.Build{
			"my-project:build-001": {
				Id:          aws.String("my-project:build-001"),
				BuildStatus: cbtypes.StatusTypeFailed,
				EndTime:     &endTime,
			},
		},
	}
	clients := &awsclient.ServiceClients{CodeBuild: fake}
	resources := []resource.Resource{{ID: "my-project"}}

	result, err := awsclient.EnrichCodeBuildStatus(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fs, ok := result.Findings["my-project"]
	if !ok {
		t.Fatalf("expected finding keyed by project name %q", "my-project")
	}
	f := fs[0]
	if f.Severity != domain.SevBroken {
		t.Errorf("severity = %v, want %v", f.Severity, "!")
	}
}

// TestEnrichCodeBuildStatus_SummaryContainsDateAndStatus verifies the summary format
// "latest build failed (<date>)" — the raw AWS enum ("FAILED") is routed through
// domain.HumanizeStatusPhrase before reaching the Wave-2 summary (11933a6f).
func TestEnrichCodeBuildStatus_SummaryContainsDateAndStatus(t *testing.T) {
	endTime := time.Date(2026, 4, 14, 12, 0, 0, 0, time.UTC)
	fake := &codeBuildEnrichFake{
		projectBuilds: map[string]string{"proj-a": "proj-a:b1"},
		builds: map[string]cbtypes.Build{
			"proj-a:b1": {
				Id:          aws.String("proj-a:b1"),
				BuildStatus: cbtypes.StatusTypeFailed,
				EndTime:     &endTime,
			},
		},
	}
	clients := &awsclient.ServiceClients{CodeBuild: fake}
	resources := []resource.Resource{{ID: "proj-a"}}

	result, err := awsclient.EnrichCodeBuildStatus(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Inverted for the spec row that gave a wave-2 phrase one owner: the
	// declared wording is "latest build <status>", so the build's end date is
	// a supporting row rather than a second shape of the phrase. The old
	// assertion described a wording no declaration carried when EndTime was
	// nil; do not restore it.
	summary := result.Findings["proj-a"][0].Phrase
	wantSummary := "latest build failed"
	if summary != wantSummary {
		t.Errorf("summary = %q, want %q", summary, wantSummary)
	}
	var endedRow string
	for _, row := range result.AttentionDetails["proj-a"]["cb.latest-build-failed"].Rows {
		if row.Label == "Ended" {
			endedRow = row.Value
		}
	}
	if endedRow != "2026-04-14" {
		t.Errorf("Ended row = %q, want the build's end date %q", endedRow, "2026-04-14")
	}
	if strings.Contains(summary, "FAILED") {
		t.Errorf("summary %q must NOT contain the raw AWS enum %q — humanize it", summary, "FAILED")
	}
}

// TestEnrichCodeBuildStatus_SucceededBuildExcluded verifies SUCCEEDED builds
// do not appear in Findings.
func TestEnrichCodeBuildStatus_SucceededBuildExcluded(t *testing.T) {
	fake := &codeBuildEnrichFake{
		projectBuilds: map[string]string{"ok-project": "ok-project:b1"},
		builds: map[string]cbtypes.Build{
			"ok-project:b1": {
				Id:          aws.String("ok-project:b1"),
				BuildStatus: cbtypes.StatusTypeSucceeded,
			},
		},
	}
	clients := &awsclient.ServiceClients{CodeBuild: fake}
	resources := []resource.Resource{{ID: "ok-project"}}

	result, err := awsclient.EnrichCodeBuildStatus(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result.Findings["ok-project"]; ok {
		t.Error("SUCCEEDED build must NOT appear in Findings")
	}
}

// TestEnrichCodeBuildStatus_MultipleFailedProjects_AllKeyedInFindings verifies
// that when several projects are probed together, every failed project gets
// its own Findings entry and the healthy one is excluded — no cross-resource
// contamination or dropped entries from the concurrent per-resource walk.
func TestEnrichCodeBuildStatus_MultipleFailedProjects_AllKeyedInFindings(t *testing.T) {
	fake := &codeBuildEnrichFake{
		projectBuilds: map[string]string{
			"fail-a": "fail-a:b1",
			"fail-b": "fail-b:b1",
			"ok-c":   "ok-c:b1",
		},
		builds: map[string]cbtypes.Build{
			"fail-a:b1": {Id: aws.String("fail-a:b1"), BuildStatus: cbtypes.StatusTypeFailed},
			"fail-b:b1": {Id: aws.String("fail-b:b1"), BuildStatus: cbtypes.StatusTypeFailed},
			"ok-c:b1":   {Id: aws.String("ok-c:b1"), BuildStatus: cbtypes.StatusTypeSucceeded},
		},
	}
	clients := &awsclient.ServiceClients{CodeBuild: fake}
	resources := []resource.Resource{
		{ID: "fail-a"},
		{ID: "fail-b"},
		{ID: "ok-c"},
	}

	result, err := awsclient.EnrichCodeBuildStatus(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Findings) != 2 {
		t.Errorf("len(Findings) = %d, want 2 (fail-a, fail-b)", len(result.Findings))
	}
	if _, ok := result.Findings["fail-a"]; !ok {
		t.Error("expected finding keyed by \"fail-a\"")
	}
	if _, ok := result.Findings["fail-b"]; !ok {
		t.Error("expected finding keyed by \"fail-b\"")
	}
	if _, ok := result.Findings["ok-c"]; ok {
		t.Error("SUCCEEDED project \"ok-c\" must NOT appear in Findings")
	}
}

// TestEnrichCodeBuildStatus_TruncatedWhenResourcesExceedCap verifies Truncated=true
// when len(resources) > EnrichmentCap (50).
func TestEnrichCodeBuildStatus_TruncatedWhenResourcesExceedCap(t *testing.T) {
	// Build 51 resources to trigger truncation.
	count := awsclient.EnrichmentCap + 1
	resources := make([]resource.Resource, count)
	projectBuilds := make(map[string]string, count)
	builds := make(map[string]cbtypes.Build, count)
	for i := range count {
		name := fmt.Sprintf("project-%03d", i)
		buildID := name + ":b1"
		resources[i] = resource.Resource{ID: name}
		projectBuilds[name] = buildID
		builds[buildID] = cbtypes.Build{
			Id:          aws.String(buildID),
			BuildStatus: cbtypes.StatusTypeSucceeded,
		}
	}
	fake := &codeBuildEnrichFake{projectBuilds: projectBuilds, builds: builds}
	clients := &awsclient.ServiceClients{CodeBuild: fake}

	result, err := awsclient.EnrichCodeBuildStatus(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Truncated {
		t.Errorf("Truncated must be true when len(resources)=%d > EnrichmentCap=%d",
			count, awsclient.EnrichmentCap)
	}
}

// TestEnrichCodeBuildStatus_AtCapNotTruncated verifies Truncated=false
// when len(resources) == EnrichmentCap exactly.
func TestEnrichCodeBuildStatus_AtCapNotTruncated(t *testing.T) {
	count := awsclient.EnrichmentCap // exactly 50
	resources := make([]resource.Resource, count)
	projectBuilds := make(map[string]string, count)
	builds := make(map[string]cbtypes.Build, count)
	for i := range count {
		name := fmt.Sprintf("project-%03d", i)
		buildID := name + ":b1"
		resources[i] = resource.Resource{ID: name}
		projectBuilds[name] = buildID
		builds[buildID] = cbtypes.Build{
			Id:          aws.String(buildID),
			BuildStatus: cbtypes.StatusTypeSucceeded,
		}
	}
	fake := &codeBuildEnrichFake{projectBuilds: projectBuilds, builds: builds}
	clients := &awsclient.ServiceClients{CodeBuild: fake}

	result, err := awsclient.EnrichCodeBuildStatus(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Truncated {
		t.Errorf("Truncated must be false when len(resources)=%d == EnrichmentCap=%d",
			count, awsclient.EnrichmentCap)
	}
}

// TestEnrichCodeBuildStatus_EmptyResourcesReturnsEmptyFindings verifies empty resources
// returns non-nil empty Findings (not an error).
func TestEnrichCodeBuildStatus_EmptyResourcesReturnsEmptyFindings(t *testing.T) {
	fake := &codeBuildEnrichFake{}
	clients := &awsclient.ServiceClients{CodeBuild: fake}

	result, err := awsclient.EnrichCodeBuildStatus(context.Background(), clients, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Findings == nil {
		t.Error("Findings must not be nil on empty resources")
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected empty Findings, got %d entries", len(result.Findings))
	}
}
