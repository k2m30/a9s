// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// lambda_issue_enrichment.go — Wave 2 issue enrichment for the lambda
// resource type: the function's lifecycle state, who is allowed to invoke
// it, and whether it answers unauthenticated HTTP.
package aws

import (
	"cmp"
	"context"
	"errors"
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

// EnrichLambdaPosture asks, per function (capped at EnrichmentCap), GetPolicy
// for the resource policy, ListFunctionUrlConfigs for unauthenticated function
// URLs, and GetFunction for its lifecycle state — ListFunctions returns none
// of State, StateReasonCode or LastUpdateStatus. All three are read-only.
//
// A function with no resource policy answers GetPolicy with
// ResourceNotFoundException — that is the healthy answer, not a failure and
// not a coverage gap, so it is neither recorded in TruncatedIDs nor folded
// into the composite error. GetFunction answering the same code means the
// function is gone.
//
// The policy and URL reads answer two independent questions. Losing one says
// nothing about the other, so each check's finding stands on its own and only
// the check that failed leaves the row uninspected.
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

	targets := capAtEnrichmentCap(&result, resources, func(r resource.Resource) bool { return r.ID != "" }, resourceIDsOf)

	ownAccount := accountIDFromClients(ctx, clients, clients.IdentityStore())
	const op = "function state, policy and URL posture"
	var mu sync.Mutex
	var failures []Failure
	loopErr := ForEachRow(ctx, &result, resourceIDs(targets), EnrichmentParallelism, func(i int) {
		r := targets[i]
		// Every read runs here, on this row's own goroutine; the mutex below
		// only records results, so no row waits on another's round trip.
		policyRows, policyPublic, policyErr := lambdaPolicyExposure(ctx, api, r.ID, ownAccount)
		urlRows, urlPublic, urlErr := lambdaFunctionURLExposure(ctx, api, r.ID)
		cfg, getErr := lambdaGetConfiguration(ctx, api, r.ID)

		mu.Lock()
		defer mu.Unlock()
		if isLambdaNoPolicy(getErr) {
			// Gone between the list call and this one: a race, not a failure
			// to log, and nothing was inspected either.
			markUninspected(&result, r.ID, "GetFunction")
			return
		}
		if cfg != nil {
			result.FieldUpdates[r.ID] = map[string]string{
				"state":              string(cfg.State),
				"last_update_status": string(cfg.LastUpdateStatus),
			}
			if code := lambdaLifecycleCode(cfg); code != "" {
				setWave2Finding(&result, r.ID, code, nil)
			}
		}
		if policyPublic {
			setWave2Finding(&result, r.ID, lambdaCodePublicPolicy, policyRows)
		}
		if urlPublic {
			setWave2Finding(&result, r.ID, lambdaCodeFunctionURLPublic, urlRows)
		}
		var unusable UnusableAnswerErr
		switch err := cmp.Or(getErr, lambdaRealErr(policyErr, urlErr)); {
		case errors.As(err, &unusable):
			MarkUnusable(&result, r.ID, &failures, unusable.Error())
		case err != nil:
			MarkSkipped(&result, r.ID, &failures, err)
		}
	})

	err := errors.Join(loopErr, Finish(&result, failures, len(targets), op))
	return result, err
}

// lambdaLifecycleCode is the one lifecycle finding GetFunction's answer
// earns: a failed last update first, since the function then runs a version
// other than the one configured, then the function's own state.
func lambdaLifecycleCode(cfg *lambdatypes.FunctionConfiguration) domain.FindingCode {
	switch {
	case cfg.LastUpdateStatus == lambdatypes.LastUpdateStatusFailed:
		return CodeLambdaLastUpdateFailed
	case cfg.State == lambdatypes.StatePending:
		return CodeLambdaStatePending
	case cfg.State == lambdatypes.StateFailed:
		return CodeLambdaStateFailed
	case cfg.State == lambdatypes.StateInactive:
		return CodeLambdaInactive
	}
	return ""
}

// lambdaGetConfiguration reads the function's configuration through
// GetFunction. An answer without one is an error, never an empty state.
func lambdaGetConfiguration(ctx context.Context, api LambdaGetFunctionAPI, name string) (*lambdatypes.FunctionConfiguration, error) {
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*lambda.GetFunctionOutput, error) {
		return api.GetFunction(ctx, &lambda.GetFunctionInput{FunctionName: aws.String(name)})
	})
	if err != nil {
		return nil, err
	}
	if out == nil || out.Configuration == nil {
		return nil, UnusableAnswerErr{Call: "GetFunction", Field: "configuration"}
	}
	return out.Configuration, nil
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
		return nil, false, parseErr
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
// for is absent: "no resource policy / no URL config" (healthy) from GetPolicy
// and ListFunctionUrlConfigs, "the function no longer exists" from
// GetFunction.
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
