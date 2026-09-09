// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// ddb_issue_enrichment.go — Wave 2 issue enrichment for the ddb resource type.
package aws

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/iampolicy"
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
	arnAndTags, tagErr := backupTagsAccessor(ctx, cache, resources, tagRead, &result, "ListTagsOfResource")
	addBackupCoverage(cache, "ddb", CodeDDBNotInBackupPlan, resources, arnAndTags, &result)

	if clients.DynamoDB == nil {
		return result, tagErr
	}
	resources = capAtEnrichmentCap(&result, resources, resourceIDsOf)
	n := len(resources)
	var pitrFailures []Failure
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
			MarkSkipped(&result, r.ID, &pitrFailures, err)
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
			// by domain.StatusPhrase(r.Findings) — not by writing
			// FieldUpdates["status"] here.
			setWave2Finding(&result, r.ID, ddbCodePITROff, nil)
		}
	})
	// AggregateFailures rather than Finish: this pass emits only "~", so a
	// table it could not read is a coverage gap on that row, never a lower
	// bound on the issue count the badge shows.

	pitrErr := AggregateFailures("DescribeContinuousBackups", pitrFailures, n)
	err := enrichDDBResourcePolicies(ctx, clients, resources, &result)
	return result, errors.Join(tagErr, pitrErr, err)
}

// enrichDDBResourcePolicies reads each table's resource policy (cap
// EnrichmentCap) and classifies it through the shared policy engine: a
// wildcard principal is the "!" row, a named foreign account the "~" row.
// A table with no policy at all is the common case and is not a finding.
func enrichDDBResourcePolicies(ctx context.Context, clients *ServiceClients, resources []resource.Resource, result *IssueEnricherResult) error {
	ownAccount := accountIDFromClients(ctx, clients, clients.IdentityStore())
	resources = capAtEnrichmentCap(result, resources, resourceIDsOf)
	n := len(resources)
	if n < len(resources) {
		SetTruncated(result, true)
	}
	var failures []Failure
	var mu sync.Mutex
	_ = ForEachParallel(ctx, n, EnrichmentParallelism, func(i int) {
		r := resources[i]
		name := r.Name
		if name == "" {
			name = r.ID
		}
		if name == "" || resourceIsTearingDown(r.RawStruct) {
			return
		}
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*dynamodb.GetResourcePolicyOutput, error) {
			return clients.DynamoDB.GetResourcePolicy(ctx, &dynamodb.GetResourcePolicyInput{
				ResourceArn: aws.String(r.Fields["arn"]),
			})
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
		if out == nil || aws.ToString(out.Policy) == "" {
			// No document is the same answer as PolicyNotFoundException above,
			// and reading it as a parse failure would report a check that
			// never failed.
			return
		}
		doc, parseErr := iampolicy.Parse(aws.ToString(out.Policy))
		if parseErr != nil {
			MarkSkipped(result, r.ID, &failures, parseErr)
			return
		}
		ex := iampolicy.Evaluate(doc, ownAccount)
		if ex.Public {
			setWave2Finding(result, r.ID, ddbCodePublicPolicy, []domain.DetailRow{
				{Label: "Principal", Value: "*", Tier: "!"},
				{Label: "Actions", Value: strings.Join(ex.PublicActions, ", ")},
			})

			return
		}
		// Without a resolved own-account ID every principal reads as foreign,
		// which would report the account's own roles as an outside grant.
		if ownAccount != "" && len(ex.CrossAccount) > 0 {
			setWave2Finding(result, r.ID, ddbCodeCrossAccountPolicy, []domain.DetailRow{{Label: "Accounts", Value: strings.Join(ex.CrossAccount, ", "), Tier: tierOf(ddbCodeCrossAccountPolicy)}})

		}
	})
	return Finish(result, failures, n, "GetResourcePolicy")
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
	tags := map[string]string{}
	var token *string
	for range PerParentPageCap {
		out, err := api.ListTagsOfResource(ctx, &dynamodb.ListTagsOfResourceInput{
			ResourceArn: aws.String(arn),
			NextToken:   token,
		})
		if err != nil {
			return nil, err
		}
		for _, t := range out.Tags {
			tags[aws.ToString(t.Key)] = aws.ToString(t.Value)
		}
		if out.NextToken == nil || *out.NextToken == "" {
			return tags, nil
		}
		token = out.NextToken
	}
	return nil, fmt.Errorf("ListTagsOfResource: more than %d pages of tags for %s", PerParentPageCap, arn)
}
