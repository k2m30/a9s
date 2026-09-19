// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// secrets_issue_enrichment.go — Wave 2 issue enrichment for the secrets resource type.
package aws

import (
	"context"
	"errors"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/iampolicy"
	"github.com/k2m30/a9s/v3/core/resource"
)

// secrets resource-policy FindingCodes.
const (
	secretsCodePublicPolicy       domain.FindingCode = "secrets.public-policy"
	secretsCodeCrossAccountPolicy domain.FindingCode = "secrets.cross-account-policy"
)

// EnrichSecretsPolicy calls GetResourcePolicy per secret (cap EnrichmentCap)
// and classifies the attached resource policy through iampolicy.
//
// Findings:
//   - Exposure.Public → "!" finding "resource policy open to anyone"
//   - Exposure.CrossAccount non-empty and not public → "~" finding
//     "resource policy grants another account"
//
// A secret with no resource policy attached (ResourcePolicy nil) is the
// normal case and yields nothing, and a secret already scheduled for deletion
// is skipped entirely. Skip when clients.SecretsManager == nil.
func EnrichSecretsPolicy(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]string),
		FieldUpdates: make(map[string]map[string]string),
	}
	if clients.SecretsManager == nil {
		return result, nil
	}
	policyAPI, ok := clients.SecretsManager.(SecretsManagerGetResourcePolicyAPI)
	if !ok {
		return result, nil
	}
	ownAccount := accountIDFromClients(ctx, clients, clients.IdentityStore())

	var failures []Failure
	resources = capAtEnrichmentCap(&result, resources, func(r resource.Resource) bool {
		return r.Fields["status"] != "DELETED"
	}, resourceIDsOf)
	n := len(resources)
	var mu sync.Mutex
	loopErr := ForEachRow(ctx, &result, resourceIDs(resources), EnrichmentParallelism, func(i int) {
		r := resources[i]
		secretID := r.Fields["arn"]
		if secretID == "" {
			secretID = r.ID
		}
		if secretID == "" {
			return
		}
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*secretsmanager.GetResourcePolicyOutput, error) {
			return policyAPI.GetResourcePolicy(ctx, &secretsmanager.GetResourcePolicyInput{
				SecretId: aws.String(secretID),
			})
		})
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			MarkSkipped(&result, r.ID, &failures, err)
			return
		}
		if out == nil || aws.ToString(out.ResourcePolicy) == "" {
			return
		}
		doc, perr := iampolicy.Parse(aws.ToString(out.ResourcePolicy))
		if perr != nil {
			MarkSkipped(&result, r.ID, &failures, perr)
			return
		}
		ex := iampolicy.Evaluate(doc, ownAccount)
		switch {
		case ex.Public:
			setWave2Finding(&result, r.ID, secretsCodePublicPolicy, publicPolicyRows(ex))

		case len(ex.CrossAccount) > 0:
			setWave2Finding(&result, r.ID, secretsCodeCrossAccountPolicy, crossAccountPolicyRows(ex))

		}
	})
	err := errors.Join(loopErr, Finish(&result, failures, n, "secrets-policy"))
	return result, err
}
