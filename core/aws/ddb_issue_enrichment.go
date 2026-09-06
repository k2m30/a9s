// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// ddb_issue_enrichment.go — Wave 2 issue enrichment for the ddb resource type.
package aws

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	smithy "github.com/aws/smithy-go"

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

// S5 operator sentences for the resource-policy findings.
const (
	ddbCrossAccountPolicyDetail = "The table's resource policy grants access to an AWS account outside this one. Confirm each account belongs to a partner you meant to share with, and remove the rest."
	ddbPublicPolicyDetail       = "The table's resource policy allows any AWS principal, so anyone with an AWS account can reach it. Replace the wildcard principal with the specific roles that need access."
)

// EnrichDynamoDBPITR calls DescribeContinuousBackups for each table (cap EnrichmentCap)
// and returns a Finding when PITR is not enabled.
// Severity is "~" (informational); PITR-disabled findings do not bump the menu badge.
func EnrichDynamoDBPITR(ctx context.Context, clients *ServiceClients, resources []resource.Resource, cache resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]bool),
		FieldUpdates: make(map[string]map[string]string),
	}
	// Backup coverage is a cache-only join, so it runs before the client guard
	// below: a type whose own API client is missing is still either selected by
	// a plan or not.
	addBackupCoverage(cache, "ddb", resources, backupARNFromField, &result)

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
			// by domain.StatusPhrase(r.Findings) — not by writing
			// FieldUpdates["status"] here.
			setWave2Finding(&result, r.ID, ddbCodePITROff, "point-in-time recovery disabled", "~", "ddb", nil, "")
		}
	})
	err := enrichDDBResourcePolicies(ctx, clients, resources, &result)
	return result, err
}

// enrichDDBResourcePolicies reads each table's resource policy (cap
// EnrichmentCap) and classifies it through the shared policy engine: a
// wildcard principal is the "!" row, a named foreign account the "~" row.
// A table with no policy at all is the common case and is not a finding.
func enrichDDBResourcePolicies(ctx context.Context, clients *ServiceClients, resources []resource.Resource, result *IssueEnricherResult) error {
	ownAccount := accountIDFromClients(ctx, clients, clients.IdentityStore())
	n := min(len(resources), EnrichmentCap)
	if n < len(resources) {
		result.Truncated = true
	}
	var failures []string
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
			MarkSkipped(result, r.ID, &failures, "GetResourcePolicy", err)
			return
		}
		doc, parseErr := iampolicy.Parse(aws.ToString(out.Policy))
		if parseErr != nil {
			MarkSkipped(result, r.ID, &failures, "GetResourcePolicy", parseErr)
			return
		}
		ex := iampolicy.Evaluate(doc, ownAccount)
		if ex.Public {
			setWave2Finding(result, r.ID, ddbCodePublicPolicy, "resource policy open to anyone", "!", "ddb",
				[]domain.DetailRow{
					{Label: "Principal", Value: "*", Tier: "!"},
					{Label: "Actions", Value: strings.Join(ex.PublicActions, ", ")},
				}, ddbPublicPolicyDetail)
			return
		}
		// Without a resolved own-account ID every principal reads as foreign,
		// which would report the account's own roles as an outside grant.
		if ownAccount != "" && len(ex.CrossAccount) > 0 {
			setWave2Finding(result, r.ID, ddbCodeCrossAccountPolicy, "resource policy grants another account", "~", "ddb",
				[]domain.DetailRow{{Label: "Accounts", Value: strings.Join(ex.CrossAccount, ", "), Tier: "~"}},
				ddbCrossAccountPolicyDetail)
		}
	})
	return Finish(result, failures, n, "ddb-enrich: GetResourcePolicy")
}

// isDDBPolicyAbsent reports whether err is DynamoDB's way of saying the table
// has no resource policy — the normal case, not a failure. Matched on the
// error code rather than the modeled type so a response the SDK could not
// bind to *PolicyNotFoundException still classifies correctly.
func isDDBPolicyAbsent(err error) bool {
	if err == nil {
		return false
	}
	var notFound *ddbtypes.PolicyNotFoundException
	if errors.As(err, &notFound) {
		return true
	}
	var apiErr smithy.APIError
	return errors.As(err, &apiErr) && apiErr.ErrorCode() == "PolicyNotFoundException"
}
