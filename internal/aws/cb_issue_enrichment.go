// cb_issue_enrichment.go — Wave 2 issue enrichment for the cb resource type.
package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/codebuild"
	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// cb canonical FindingCodes.
const (
	cbCodeLatestBuildFailed domain.FindingCode = "cb.latest-build-failed"
)

// EnrichCodeBuildStatus calls BatchGetBuilds for the latest build of each project
// and returns a Finding for every project whose latest build is not SUCCEEDED.
// Severity is "!" (broken/degraded). Summary: "latest build FAILED (<date>)".
func EnrichCodeBuildStatus(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string]domain.Finding),
		TruncatedIDs: make(map[string]bool),
		FieldUpdates: make(map[string]map[string]string),
	}
	if clients.CodeBuild == nil || len(resources) == 0 {
		return result, nil
	}
	names := make([]string, 0, len(resources))
	for _, r := range resources {
		if r.ID != "" {
			names = append(names, r.ID)
		}
	}
	if len(names) == 0 {
		return result, nil
	}
	buildIDToProject := make(map[string]string, len(names))
	var buildIDs []string
	truncated := len(resources) > EnrichmentCap
	for _, name := range names {
		if len(buildIDs) >= EnrichmentCap {
			break
		}
		out, err := clients.CodeBuild.ListBuildsForProject(ctx, &codebuild.ListBuildsForProjectInput{
			ProjectName: aws.String(name),
			SortOrder:   cbtypes.SortOrderTypeDescending,
		})
		if err != nil {
			truncated = true
			result.TruncatedIDs[name] = true
			continue
		}
		if len(out.Ids) > 0 {
			id := out.Ids[0]
			buildIDs = append(buildIDs, id)
			buildIDToProject[id] = name
		}
	}
	if len(buildIDs) == 0 {
		result.Truncated = truncated
		return result, nil
	}
	builds, err := clients.CodeBuild.BatchGetBuilds(ctx, &codebuild.BatchGetBuildsInput{
		Ids: buildIDs,
	})
	if err != nil {
		return IssueEnricherResult{TruncatedIDs: result.TruncatedIDs}, err
	}
	for _, b := range builds.Builds {
		if b.Id == nil {
			continue
		}
		projectName := buildIDToProject[*b.Id]
		if projectName == "" {
			continue
		}
		switch b.BuildStatus {
		case cbtypes.StatusTypeSucceeded, cbtypes.StatusTypeInProgress, cbtypes.StatusTypeStopped:
			result.FieldUpdates[projectName] = map[string]string{"last_build": "OK"}
			continue
		}
		statusVal := string(b.BuildStatus)
		lastBuildVal := statusVal
		if b.EndTime != nil {
			elapsed := time.Since(*b.EndTime)
			hours := int(elapsed.Hours())
			lastBuildVal = fmt.Sprintf("%s %dh ago", statusVal, hours)
		}
		rows := []domain.DetailRow{
			{Label: "Status", Value: statusVal, Tier: "!"},
		}
		if b.EndTime != nil {
			rows = append(rows, domain.DetailRow{Label: "Ended", Value: b.EndTime.Format("2006-01-02")})
		}
		// Append the latest failed phase if build is not complete.
		if !b.BuildComplete {
			if b.CurrentPhase != nil && *b.CurrentPhase != "" {
				rows = append(rows, domain.DetailRow{Label: "Current Phase", Value: *b.CurrentPhase, Tier: "~"})
			}
		} else {
			// Find the latest failed phase.
			for i := len(b.Phases) - 1; i >= 0; i-- {
				ph := b.Phases[i]
				if ph.PhaseStatus == cbtypes.StatusTypeFailed {
					rows = append(rows, domain.DetailRow{Label: "Phase", Value: string(ph.PhaseType), Tier: "!"})
					break
				}
			}
		}
		summary := fmt.Sprintf("latest build %s", statusVal)
		if b.EndTime != nil {
			summary = fmt.Sprintf("latest build %s (%s)", statusVal, b.EndTime.Format("2006-01-02"))
		}
		setWave2Finding(&result, projectName, cbCodeLatestBuildFailed, summary, "!", "cb", rows)
		result.FieldUpdates[projectName] = map[string]string{"last_build": lastBuildVal}
	}
	result.IssueCount = len(result.Findings)
	result.Truncated = truncated
	return result, nil
}
