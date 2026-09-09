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
		TruncatedIDs:     make(map[string]string),
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

		// One of the two may have answered the absent-resource code, which
		// means either "no policy / no URL config" (healthy) or "the function
		// is gone" (a race). Only GetFunction can tell them apart, and it is a
		// network round trip: it runs HERE, on this row's own goroutine, not
		// under the mutex below. That mutex exists to record results into one
		// map; holding it across a call makes every other function in the
		// batch wait on this one's round trip, and the parallel workers fill
		// with goroutines blocked on it, so rows further down are never asked
		// about at all.
		realErr := lambdaRealErr(policyErr, urlErr)
		var absent bool
		var verifyErr error
		if realErr == nil && (policyErr != nil || urlErr != nil) {
			absent, verifyErr = lambdaFunctionAbsent(ctx, api, r.ID)
		}

		mu.Lock()
		defer mu.Unlock()
		if policyPublic {
			setWave2Finding(&result, r.ID, lambdaCodePublicPolicy, policyRows)

		}
		if urlPublic {
			setWave2Finding(&result, r.ID, lambdaCodeFunctionURLPublic, urlRows)

		}
		switch {
		case realErr != nil:
			MarkSkipped(&result, r.ID, &failures, realErr)
		case verifyErr != nil:
			// The question was not settled. Nothing about this function's
			// posture was established, so the row is uninspected — reading a
			// failed verification as "still there" reports it clean on the
			// strength of a call that never succeeded.
			MarkSkipped(&result, r.ID, &failures, verifyErr)
		case absent:
			// Gone between the list call and this one: a race, not a failure
			// to log, and nothing was inspected either. GetFunction is the
			// call that settled it, so it is the check the row names.
			markUninspected(&result, r.ID, "GetFunction")
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

// lambdaFunctionAbsent reports whether the function is absent, the one
// question GetPolicy's and ListFunctionUrlConfigs' shared error code cannot
// answer. Asked only after one of them reported absence, so a healthy
// function costs no extra call.
//
// Three answers, not two. A successful GetFunction says the function is there
// (false, nil); the absent-resource code says it is gone (true, nil); any
// other failure — denied, throttled, a reset connection — settles nothing and
// is returned, because a verification that did not happen is not the same
// answer as one that came back "present".
func lambdaFunctionAbsent(ctx context.Context, api LambdaGetFunctionAPI, name string) (bool, error) {
	_, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*lambda.GetFunctionOutput, error) {
		return api.GetFunction(ctx, &lambda.GetFunctionInput{FunctionName: aws.String(name)})
	})
	switch {
	case err == nil:
		return false, nil
	case isLambdaNoPolicy(err):
		return true, nil
	default:
		return false, err
	}
}
