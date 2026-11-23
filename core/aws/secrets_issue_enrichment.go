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
	"github.com/k2m30/a9s/v3/core/resource"
)

// secrets resource-policy FindingCodes.
const (
	secretsCodePublicPolicy       domain.FindingCode = "secrets.public-policy"
	secretsCodeCrossAccountPolicy domain.FindingCode = "secrets.cross-account-policy"
)

// EnrichSecretsPolicy raises nothing for a secret with no resource policy
// attached (ResourcePolicy nil), which is the normal case.
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
	resources = capAtEnrichmentCap(&result, resources, nil, resourceIDsOf)
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
		out, err := policyAPI.GetResourcePolicy(ctx, &secretsmanager.GetResourcePolicyInput{
			SecretId: aws.String(secretID),
		})
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			MarkSkipped(&result, r.ID, &failures, err)
			return
		}
		if out == nil {
			return
		}
		readPolicy("secrets", aws.ToString(out.ResourcePolicy), ownAccount, r.Fields["arn"]).raise(&result, r.ID)
	})
	err := errors.Join(loopErr, Finish(&result, failures, n, "secrets-policy"))
	return result, err
}
