// eb_issue_enrichment.go — Wave 2 issue enrichment for the eb resource type.
package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk/types"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// eb canonical FindingCodes.
const (
	ebCodeEnvironmentCauses domain.FindingCode = "eb.environment-causes"
)

// EnrichEBEnvironmentHealth calls DescribeEnvironmentHealth for each Elastic
// Beanstalk environment (1 per environment, cap 50). Returns an informational
// "~" finding for each environment with a non-empty Causes slice.
// Summary: "EB causes: <first cause>". IssueCount is always 0 — causes are
// informational signals, not broken-state indicators.
func EnrichEBEnvironmentHealth(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string]domain.Finding),
		TruncatedIDs: make(map[string]bool),
	}
	if clients.ElasticBeanstalk == nil {
		return result, nil
	}
	truncated := len(resources) > EnrichmentCap
	for i, r := range resources {
		if i >= EnrichmentCap {
			break
		}
		name := r.Name
		if name == "" {
			name = r.Fields["environment_name"]
		}
		if name == "" {
			continue
		}
		out, err := clients.ElasticBeanstalk.DescribeEnvironmentHealth(ctx, &elasticbeanstalk.DescribeEnvironmentHealthInput{
			EnvironmentName: aws.String(name),
			AttributeNames:  []ebtypes.EnvironmentHealthAttribute{ebtypes.EnvironmentHealthAttributeCauses},
		})
		if err != nil {
			truncated = true
			result.TruncatedIDs[r.ID] = true
			continue
		}
		if len(out.Causes) == 0 {
			continue
		}
		firstCause := out.Causes[0]
		rows := []domain.DetailRow{
			{Label: "Cause", Value: firstCause, Tier: "~"},
		}
		// Record additional causes as extra rows.
		for _, cause := range out.Causes[1:] {
			rows = append(rows, domain.DetailRow{Label: "Cause", Value: cause, Tier: "~"})
		}
		// Key on resource ID (environment ID) for registry consistency.
		// Fall back to name if ID is not set.
		key := r.ID
		if key == "" {
			key = name
		}
		setWave2Finding(&result, key, ebCodeEnvironmentCauses, fmt.Sprintf("EB causes: %s", firstCause), "~", "eb", rows)
	}
	result.IssueCount = 0
	result.Truncated = truncated
	return result, nil
}
