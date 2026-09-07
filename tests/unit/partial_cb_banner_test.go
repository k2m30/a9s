package unit_test

// partial_cb_banner_test.go — the codebuild cap fix removed the `truncated`
// local and both `result.Truncated = truncated` assignments that overwrote it.
// Truncated is what puts the "+" on the menu's issue badge, so these pin that
// the banner still lights when the walk was cut short and still stays dark
// when it was not.

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	codebuild "github.com/aws/aws-sdk-go-v2/service/codebuild"
	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"
	smithy "github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

// cbBannerFake gives every project one failed build, and fails the list call
// for whichever project names it is told to.
type cbBannerFake struct {
	awsclient.CodeBuildAPI

	failFor map[string]bool
}

func (f *cbBannerFake) ListBuildsForProject(_ context.Context, in *codebuild.ListBuildsForProjectInput, _ ...func(*codebuild.Options)) (*codebuild.ListBuildsForProjectOutput, error) {
	name := aws.ToString(in.ProjectName)
	if f.failFor[name] {
		return nil, &smithy.GenericAPIError{
			Code:    "AccessDeniedException",
			Message: "User is not authorized to perform: codebuild:ListBuildsForProject",
			Fault:   smithy.FaultClient,
		}
	}
	return &codebuild.ListBuildsForProjectOutput{Ids: []string{name + ":a1b2c3d4-1111-2222-3333-444455556666"}}, nil
}

func (f *cbBannerFake) BatchGetBuilds(_ context.Context, in *codebuild.BatchGetBuildsInput, _ ...func(*codebuild.Options)) (*codebuild.BatchGetBuildsOutput, error) {
	out := &codebuild.BatchGetBuildsOutput{}
	for _, id := range in.Ids {
		out.Builds = append(out.Builds, cbtypes.Build{
			Id:            aws.String(id),
			BuildStatus:   cbtypes.StatusTypeFailed,
			BuildComplete: true,
			EndTime:       aws.Time(time.Now().Add(-3 * time.Hour)),
			Phases: []cbtypes.BuildPhase{
				{PhaseType: cbtypes.BuildPhaseTypeBuild, PhaseStatus: cbtypes.StatusTypeFailed},
			},
		})
	}
	return out, nil
}

// TestPartialCB_TheIssueBadgeStillSaysTheCountIsALowerBound drives the real
// enricher and then the controller the menu reads, so the assertion is on what
// an operator sees: "issues:N+" when the walk did not reach every project, and
// "issues:N" when it did. The suffix comes from MenuEntry.IssueBadge.Truncated
// (internal/tui/views/mainmenu.go entryIssueBadge), which is fed by the
// enricher's own Truncated flag.
func TestPartialCB_TheIssueBadgeStillSaysTheCountIsALowerBound(t *testing.T) {
	for _, tc := range []struct {
		name          string
		projects      int
		failFor       map[string]bool
		wantTruncated bool
	}{
		{
			// One project past the cap is never asked about, so the count of
			// failing builds is a lower bound.
			name:          "past the cap",
			projects:      awsclient.EnrichmentCap + 1,
			wantTruncated: true,
		},
		{
			// Exactly at the cap nothing is dropped and every project
			// answered, so the count is the whole truth and carries no "+".
			name:          "at the cap",
			projects:      awsclient.EnrichmentCap,
			wantTruncated: false,
		},
		{
			// Under the cap, but one project's build list was denied. Its
			// builds could be failing, so the count is a lower bound again —
			// this is the branch whose write the fix rerouted.
			name:          "one project denied",
			projects:      3,
			failFor:       map[string]bool{"acme-build-001": true},
			wantTruncated: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows := make([]resource.Resource, 0, tc.projects)
			for i := range tc.projects {
				id := fmt.Sprintf("acme-build-%03d", i)
				rows = append(rows, resource.Resource{ID: id, Name: id, Fields: map[string]string{}})
			}

			res, _ := awsclient.EnrichCodeBuildStatus(context.Background(),
				&awsclient.ServiceClients{CodeBuild: &cbBannerFake{failFor: tc.failFor}}, rows, nil)

			if res.Truncated != tc.wantTruncated {
				t.Errorf("enricher Truncated = %v, want %v", res.Truncated, tc.wantTruncated)
			}

			c := newTestController(t)
			c.ApplyEnrichmentState("cb", len(res.Findings), res.Truncated, res.Findings, res.AttentionDetails)
			snap := c.Snapshot()
			entry := menuEntryFor(snap.Body.Menu, "cb")
			if entry == nil {
				t.Fatal("the menu has no entry for cb")
			}
			if entry.IssueBadge.Truncated != tc.wantTruncated {
				t.Errorf("cb IssueBadge.Truncated = %v, want %v — the badge suffix is the \"+\" on issues:%d",
					entry.IssueBadge.Truncated, tc.wantTruncated, entry.IssueBadge.Count)
			}
			if entry.IssueBadge.Count != len(res.Findings) {
				t.Errorf("cb IssueBadge.Count = %d, want %d", entry.IssueBadge.Count, len(res.Findings))
			}
		})
	}
}
