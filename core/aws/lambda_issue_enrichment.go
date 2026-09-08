// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// lambda_issue_enrichment.go — Wave 2 issue enrichment for the lambda
// resource type: who is allowed to invoke a function, and whether it answers
// unauthenticated HTTP.
package aws

import (
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
	const op = "GetPolicy/ListFunctionUrlConfigs"
	var mu sync.Mutex
	var failures []Failure
	_ = ForEachParallel(ctx, len(targets), EnrichmentParallelism, func(i int) {
		r := targets[i]
		policyRows, policyPublic, policyErr := lambdaPolicyExposure(ctx, api, r.ID, ownAccount)
		urlRows, urlPublic, urlErr := lambdaFunctionURLExposure(ctx, api, r.ID)

		mu.Lock()
		defer mu.Unlock()
		if policyPublic {
			setWave2Finding(&result, r.ID, lambdaCodePublicPolicy, policyRows)

		}
		if urlPublic {
			setWave2Finding(&result, r.ID, lambdaCodeFunctionURLPublic, urlRows)

		}
		switch realErr := lambdaRealErr(policyErr, urlErr); {
		case realErr != nil:
			MarkSkipped(&result, r.ID, &failures, realErr)
		case (policyErr != nil || urlErr != nil) && lambdaFunctionGone(ctx, api, r.ID):
			// The function went away between the list call and this one: a
			// race, not a failure to log — and nothing about its posture was
			// inspected, so the row must not read as clean. The healthy case
			// answers the same wire code and lands here with the function
			// still there, recording nothing.
			result.TruncatedIDs[r.ID] = true
		}
	})

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

// isLambdaNoPolicy reports the wire code Lambda answers when the thing asked
// for is absent. It is deliberately NOT a verdict: the same code means "this
// function has no resource policy / no URL config" (healthy) and "this
// function no longer exists" (a race). Which one it is takes a second
// question — see lambdaFunctionGone — asked once by the enricher rather than
// guessed at each call site.
func isLambdaNoPolicy(err error) bool {
	return ErrCodeIs(err, "ResourceNotFoundException")
}

// lambdaRealErr returns the first error that is an actual failure, skipping
// the absent-resource answers isLambdaNoPolicy names. Taking cmp.Or of the two
// raw errors instead would let a healthy "no policy" hide a genuine
// ListFunctionUrlConfigs failure behind it.
func lambdaRealErr(errs ...error) error {
	for _, err := range errs {
		if err != nil && !isLambdaNoPolicy(err) {
			return err
		}
	}
	return nil
}

// lambdaFunctionGone reports whether the function is absent, which is the one
// question GetPolicy's and ListFunctionUrlConfigs' shared error code cannot
// answer. Asked only after one of them reported absence, so a healthy
// function costs no extra call.
func lambdaFunctionGone(ctx context.Context, api LambdaGetFunctionAPI, name string) bool {
	_, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*lambda.GetFunctionOutput, error) {
		return api.GetFunction(ctx, &lambda.GetFunctionInput{FunctionName: aws.String(name)})
	})
	return isLambdaNoPolicy(err)
}
