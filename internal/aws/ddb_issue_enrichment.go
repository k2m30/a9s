// ddb_issue_enrichment.go — Wave 2 issue enrichment for the ddb resource type.
package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	registerIssueEnricher("ddb", EnrichDynamoDBPITR, 100)
}

// EnrichDynamoDBPITR calls DescribeContinuousBackups for each table (cap EnrichmentCap)
// and returns a Finding when PITR is not enabled.
// Severity is "~" (informational); PITR-disabled findings do not bump the menu badge.
func EnrichDynamoDBPITR(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	fieldUpdates := make(map[string]map[string]string)
	truncatedIDs := make(map[string]bool)
	if clients.DynamoDB == nil {
		return IssueEnricherResult{Findings: findings, TruncatedIDs: truncatedIDs}, nil
	}
	truncated := len(resources) > EnrichmentCap
	for i, r := range resources {
		if i >= EnrichmentCap {
			break
		}
		name := r.Name
		if name == "" {
			name = r.ID
		}
		if name == "" {
			continue
		}
		out, err := clients.DynamoDB.DescribeContinuousBackups(ctx, &dynamodb.DescribeContinuousBackupsInput{
			TableName: aws.String(name),
		})
		if err != nil {
			// sub-call error: skip this table, mark truncated to signal incomplete data
			truncated = true
			truncatedIDs[r.ID] = true
			continue
		}
		if out.ContinuousBackupsDescription == nil {
			continue
		}
		pitr := out.ContinuousBackupsDescription.PointInTimeRecoveryDescription
		if pitr == nil {
			continue
		}
		pitrEnabled := string(pitr.PointInTimeRecoveryStatus) == "ENABLED"
		if !pitrEnabled {
			// Compute the new status value: if the table already has a non-empty
			// status phrase (e.g. "archived: kms key lost"), bump the suffix so the
			// operator sees there is an additional finding. Otherwise the phrase is
			// "PITR off" itself.
			existingStatus := r.Fields["status"]
			var newStatus string
			if existingStatus != "" {
				newStatus = resource.BumpFindingSuffix(existingStatus)
			} else {
				newStatus = "PITR off"
			}
			fieldUpdates[r.ID] = map[string]string{
				"status": newStatus,
			}
			findings[r.ID] = resource.EnrichmentFinding{
				Severity: "~",
				Summary:  "PITR off",
			}
		}
	}
	return IssueEnricherResult{IssueCount: 0, Truncated: truncated, TruncatedIDs: truncatedIDs, Findings: findings, FieldUpdates: fieldUpdates}, nil
}
