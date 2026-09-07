// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// lambda_issue_enrichment.go — Wave 2 issue enrichment for the lambda
// resource type: who is allowed to invoke a function, and whether it answers
// unauthenticated HTTP.
package aws

import (
	"cmp"
	"context"
	"slices"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/iampolicy"
	"github.com/k2m30/a9s/v3/core/resource"
)

// lambda Wave-2 canonical FindingCodes.
const (
	lambdaCodePublicPolicy      domain.FindingCode = "lambda.public-policy"
	lambdaCodeFunctionURLPublic domain.FindingCode = "lambda.function-url-public"
)

// EnrichLambdaPosture asks, per function (capped at EnrichmentCap), who may
// invoke it: GetPolicy for the resource policy and ListFunctionUrlConfigs for
// unauthenticated function URLs. Both are read-only.
//
// A function with no resource policy answers GetPolicy with
// ResourceNotFoundException — that is the healthy answer, not a failure and
// not a coverage gap, so it is neither recorded in TruncatedIDs nor folded
// into the composite error.
//
// The two reads answer two independent questions. Losing one says nothing
// about the other, so each check's finding stands on its own and only the
// check that failed leaves the row uninspected.
func EnrichLambdaPosture(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:         make(map[string][]domain.Finding),
		AttentionDetails: make(map[string]map[domain.FindingCode]domain.AttentionDetail),
		TruncatedIDs:     make(map[string]bool),
		FieldUpdates:     make(map[string]map[string]string),
	}
	if clients.Lambda == nil {
		return result, nil
	}
	api, ok := clients.Lambda.(LambdaPostureAPI)
	if !ok {
		return result, nil
	}

	targets := make([]resource.Resource, 0, len(resources))
	for _, r := range resources {
		if r.ID != "" {
			targets = append(targets, r)
		}
	}
	targets = capAtEnrichmentCap(&result, targets, resourceIDsOf)

	ownAccount := accountIDFromClients(ctx, clients, clients.IdentityStore())
	const op = "lambda-enrich: GetPolicy/ListFunctionUrlConfigs"
	var mu sync.Mutex
	var failures []Failure
	_ = ForEachParallel(ctx, len(targets), EnrichmentParallelism, func(i int) {
		r := targets[i]
		policyRows, policyPublic, policyErr := lambdaPolicyExposure(ctx, api, r.ID, ownAccount)
		urlRows, urlPublic, urlErr := lambdaFunctionURLExposure(ctx, api, r.ID)

		mu.Lock()
		defer mu.Unlock()
		if policyPublic {
			setWave2Finding(&result, r.ID, lambdaCodePublicPolicy, "invokable by anyone", "!", "lambda",
				policyRows)

		}
		if urlPublic {
			setWave2Finding(&result, r.ID, lambdaCodeFunctionURLPublic, "function endpoint open without authentication", "!", "lambda",
				urlRows)

		}
		if err := cmp.Or(policyErr, urlErr); err != nil {
			if IsNotFoundErr(err) {
				result.TruncatedIDs[r.ID] = true
				return
			}
			MarkSkipped(&result, r.ID, &failures, err)
		}
	})
	SortFailures(failures)
	err := Finish(&result, failures, len(targets), op)
	return result, err
}

// lambdaPolicyExposure evaluates the function's resource policy through the
// shared iampolicy engine. A missing policy is the healthy answer.
func lambdaPolicyExposure(ctx context.Context, api LambdaGetPolicyAPI, name, ownAccount string) ([]domain.DetailRow, bool, error) {
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*lambda.GetPolicyOutput, error) {
		return api.GetPolicy(ctx, &lambda.GetPolicyInput{FunctionName: aws.String(name)})
	})
	if err != nil {
		if isLambdaNoPolicy(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	if out == nil || out.Policy == nil || *out.Policy == "" {
		return nil, false, nil
	}
	doc, parseErr := iampolicy.Parse(*out.Policy)
	if parseErr != nil {
		return nil, false, nil
	}
	ex := iampolicy.Evaluate(doc, ownAccount)
	if !ex.Public {
		return nil, false, nil
	}
	return []domain.DetailRow{
		{Label: "Principal", Value: "*", Tier: "!"},
		{Label: "Actions", Value: strings.Join(ex.PublicActions, ", "), Tier: "!"},
	}, true, nil
}

// lambdaFunctionURLExposure reports a function URL whose AuthType is NONE.
func lambdaFunctionURLExposure(ctx context.Context, api LambdaListFunctionUrlConfigsAPI, name string) ([]domain.DetailRow, bool, error) {
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*lambda.ListFunctionUrlConfigsOutput, error) {
		return api.ListFunctionUrlConfigs(ctx, &lambda.ListFunctionUrlConfigsInput{FunctionName: aws.String(name)})
	})
	if err != nil {
		if isLambdaNoPolicy(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	if out == nil {
		return nil, false, nil
	}
	for _, cfg := range out.FunctionUrlConfigs {
		if cfg.AuthType != lambdatypes.FunctionUrlAuthTypeNone {
			continue
		}
		rows := []domain.DetailRow{{Label: "Endpoint auth", Value: "none", Tier: "!"}}
		if cfg.Cors != nil && slices.Contains(cfg.Cors.AllowOrigins, "*") {
			rows = append(rows, domain.DetailRow{Label: "Allowed origins", Value: "any", Tier: "!"})
		}
		return rows, true, nil
	}
	return nil, false, nil
}

// isLambdaNoPolicy reports the "this function has no policy / no URL config"
// answer. Lambda returns ResourceNotFoundException for both the absent
// resource policy and the absent function-URL config, which is the healthy
// state — distinct from a deleted function, which the enricher can only see
// as the same code and therefore also treats as "nothing to report".
func isLambdaNoPolicy(err error) bool {
	return ErrCodeIs(err, "ResourceNotFoundException")
}
