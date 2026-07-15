// ddb_issue_enrichment.go — Wave 2 issue enrichment for the ddb resource type.
package aws

import (
	"context"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// ddb canonical FindingCodes.
const (
	ddbCodePITROff domain.FindingCode = "ddb.pitr-off"
)

// EnrichDynamoDBPITR calls DescribeContinuousBackups for each table (cap EnrichmentCap)
// and returns a Finding when PITR is not enabled.
// Severity is "~" (informational); PITR-disabled findings do not bump the menu badge.
func EnrichDynamoDBPITR(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]bool),
		FieldUpdates: make(map[string]map[string]string),
	}
	if clients.DynamoDB == nil {
		return result, nil
	}
	n := min(len(resources), EnrichmentCap)
	var mu sync.Mutex
	_ = ForEachParallel(ctx, n, EnrichmentParallelism, func(i int) {
		r := resources[i]
		name := r.Name
		if name == "" {
			name = r.ID
		}
		if name == "" {
			return
		}
		out, err := clients.DynamoDB.DescribeContinuousBackups(ctx, &dynamodb.DescribeContinuousBackupsInput{
			TableName: aws.String(name),
		})
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			// sub-call error: skip this table, mark truncated to signal incomplete data
			result.TruncatedIDs[r.ID] = true
			return
		}
		if out.ContinuousBackupsDescription == nil {
			return
		}
		pitr := out.ContinuousBackupsDescription.PointInTimeRecoveryDescription
		if pitr == nil {
			return
		}
		pitrEnabled := string(pitr.PointInTimeRecoveryStatus) == "ENABLED"
		if !pitrEnabled {
			// Emit only the Finding entry. The merged display
			// phrase (e.g. "archived: kms key lost") is computed at render time
			// by phraseFromFindings(r.Findings) — not by writing
			// FieldUpdates["status"] here.
			setWave2Finding(&result, r.ID, ddbCodePITROff, "point-in-time recovery disabled", "~", "ddb", nil, "")
		}
	})
	result.IssueCount = 0
	// "~"-only enrichment: EnrichmentCap bounds informational coverage, never the issue count — so it never lower-bounds the issue badge (cf. EnrichSESAccount).
	result.Truncated = false
	return result, nil
}
