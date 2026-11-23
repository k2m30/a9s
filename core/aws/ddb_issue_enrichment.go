// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// ddb_issue_enrichment.go — Wave 2 issue enrichment for the ddb resource type.
package aws

import (
	"context"
	"errors"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// ddb canonical FindingCodes.
const (
	ddbCodePITROff            domain.FindingCode = "ddb.pitr-off"
	ddbCodeCrossAccountPolicy domain.FindingCode = "ddb.cross-account-policy"
	ddbCodePublicPolicy       domain.FindingCode = "ddb.public-policy"
)

// EnrichDynamoDBPITR calls DescribeContinuousBackups for each table (cap EnrichmentCap)
// and returns a Finding when PITR is not enabled.
// Severity is "~" (informational); PITR-disabled findings do not bump the menu badge.
func EnrichDynamoDBPITR(ctx context.Context, clients *ServiceClients, resources []resource.Resource, cache resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]string),
		FieldUpdates: make(map[string]map[string]string),
	}
	// Backup coverage runs before the client guard below: a type whose own API
	// client is missing is still either selected by a plan or not, and the tag
	// read it may need is skipped along with everything else in that case.
	var tagRead backupTagReader
	if clients != nil && clients.DynamoDB != nil {
		if api, ok := clients.DynamoDB.(DynamoDBListTagsOfResourceAPI); ok {
			tagRead = func(ctx context.Context, arn string) (map[string]string, error) {
				return dynamoDBTagsForARN(ctx, api, arn)
			}
		}
	}
	targetOf, tagErr := backupTagsAccessor(ctx, cache, resources, backupTargetFromFields("DescribeTable"), tagRead, &result, "ListTagsOfResource")
	addBackupCoverage(cache, CodeDDBNotInBackupPlan, resources, targetOf, &result)

	if clients.DynamoDB == nil {
		return result, tagErr
	}
	resources = capAtEnrichmentCap(&result, resources, nil, resourceIDsOf)
	n := len(resources)
	var pitrFailures []Failure
	var mu sync.Mutex
	loopErr := ForEachRow(ctx, &result, resourceIDs(resources), EnrichmentParallelism, func(i int) {
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
			MarkSkipped(&result, r.ID, &pitrFailures, err)
			return
		}
		if out.ContinuousBackupsDescription == nil || out.ContinuousBackupsDescription.PointInTimeRecoveryDescription == nil {
			MarkSkipped(&result, r.ID, &pitrFailures, UnusableAnswerErr{Call: "DescribeContinuousBackups", Field: "point-in-time recovery description"})
			return
		}
		pitr := out.ContinuousBackupsDescription.PointInTimeRecoveryDescription
		pitrEnabled := string(pitr.PointInTimeRecoveryStatus) == "ENABLED"
		if !pitrEnabled {
			setWave2Finding(&result, r.ID, ddbCodePITROff, nil)
		}
	})
	pitrErr := AggregateFailures("DescribeContinuousBackups", pitrFailures, n)
	err := enrichDDBResourcePolicies(ctx, clients, resources, &result)
	return result, errors.Join(tagErr, loopErr, pitrErr, err)
}

// enrichDDBResourcePolicies raises nothing for a table with no policy at all,
// which is the common case.
func enrichDDBResourcePolicies(ctx context.Context, clients *ServiceClients, resources []resource.Resource, result *IssueEnricherResult) error {
	ownAccount := accountIDFromClients(ctx, clients, clients.IdentityStore())
	// GetResourcePolicy takes the ARN only DescribeTable returns, so a table
	// without one stays uninspected on that call, the way backup coverage
	// marks it (backupTargetFromFields); the fetch has already reported the
	// failure.
	for _, r := range resources {
		if r.Fields["arn"] == "" {
			markUninspected(result, r.ID, "DescribeTable")
		}
	}
	resources = capAtEnrichmentCap(result, resources, func(r resource.Resource) bool {
		return r.Fields["arn"] != ""
	}, resourceIDsOf)
	n := len(resources)
	var failures []Failure
	var mu sync.Mutex
	loopErr := ForEachRow(ctx, result, resourceIDs(resources), EnrichmentParallelism, func(i int) {
		r := resources[i]
		name := r.Name
		if name == "" {
			name = r.ID
		}
		if name == "" {
			return
		}
		out, err := clients.DynamoDB.GetResourcePolicy(ctx, &dynamodb.GetResourcePolicyInput{
			ResourceArn: aws.String(r.Fields["arn"]),
		})
		mu.Lock()
		defer mu.Unlock()
		switch {
		case isDDBPolicyAbsent(err):
			return
		case err != nil:
			MarkSkipped(result, r.ID, &failures, err)
			return
		}
		var policy string
		if out != nil {
			policy = aws.ToString(out.Policy)
		}
		readPolicy("ddb", policy, ownAccount, r.Fields["arn"]).raise(result, r.ID)
	})
	return errors.Join(loopErr, AggregateFailures("GetResourcePolicy", failures, n))
}

// isDDBPolicyAbsent reports whether err is DynamoDB's way of saying the table
// has no resource policy — the normal case, not a failure. Matched on the
// error code rather than the modeled type so a response the SDK could not
// bind to *PolicyNotFoundException still classifies correctly.
func isDDBPolicyAbsent(err error) bool {
	if err == nil {
		return false
	}
	return ErrCodeIs(err, "PolicyNotFoundException")
}

// dynamoDBTagsForARN reads one table's tags. The call answers ten tags a page,
// so it walks the pages; a table whose tags run past the page cap is reported
// as unreadable rather than as carrying only the tags read so far, which would
// let a selection on a later tag read as no selection at all.
func dynamoDBTagsForARN(ctx context.Context, api DynamoDBListTagsOfResourceAPI, arn string) (map[string]string, error) {
	list, complete, err := PageAll(ctx, PerParentPageCap, func(ctx context.Context, token *string) ([]ddbtypes.Tag, *string, error) {
		out, err := api.ListTagsOfResource(ctx, &dynamodb.ListTagsOfResourceInput{
			ResourceArn: aws.String(arn),
			NextToken:   token,
		})
		if err != nil {
			return nil, nil, err
		}
		return out.Tags, out.NextToken, nil
	})
	if err != nil {
		return nil, err
	}
	if !complete {
		return nil, errPageCap
	}
	tags := make(map[string]string, len(list))
	for _, t := range list {
		tags[aws.ToString(t.Key)] = aws.ToString(t.Value)
	}
	return tags, nil
}
