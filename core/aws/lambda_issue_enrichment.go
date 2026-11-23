// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// lambda_issue_enrichment.go — Wave 2 issue enrichment for the lambda
// resource type: the function's lifecycle state, who is allowed to invoke
// it, and whether it answers unauthenticated HTTP.
package aws

import (
	"context"
	"errors"
	"slices"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// lambda Wave-2 canonical FindingCodes.
const (
	lambdaCodePublicPolicy       domain.FindingCode = "lambda.public-policy"
	lambdaCodeCrossAccountPolicy domain.FindingCode = "lambda.cross-account-policy"
	lambdaCodeFunctionURLPublic  domain.FindingCode = "lambda.function-url-public"
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
		policy, policyErr := lambdaPolicy(ctx, api, r.ID)
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
		class, state := catalog.LifecycleNormal, ""
		if cfg != nil {
			state = string(cfg.State)
			class = LifecycleClassOf("lambda", state)
			result.FieldUpdates[r.ID] = map[string]string{
				"state":              string(cfg.State),
				"last_update_status": string(cfg.LastUpdateStatus),
			}
			if f, ok := StateFinding("lambda", string(cfg.State)); ok {
				setWave2Finding(&result, r.ID, f.Code, nil, domain.HumanizeStatusPhrase(string(cfg.State)))
			}
			if cfg.LastUpdateStatus == lambdatypes.LastUpdateStatusFailed {
				setWave2Finding(&result, r.ID, CodeLambdaLastUpdateFailed, nil)
			}
		}
		if policyErr == nil {
			readPolicy("lambda", policy, ownAccount, r.Fields["arn"]).raiseIn(&result, r.ID, class, state)
		}
		if urlPublic && catalog.Applies(lambdaCodeFunctionURLPublic, class, state) {
			setWave2Finding(&result, r.ID, lambdaCodeFunctionURLPublic, urlRows)
		}
		// Each read that failed is recorded: two calls refused for two
		// different actions are two grants the operator needs.
		for _, err := range []error{getErr, policyErr, urlErr} {
			if err != nil && !isLambdaNoPolicy(err) {
				MarkSkipped(&result, r.ID, &failures, err)
			}
		}
	})

	err := errors.Join(loopErr, AggregateFailures(op, failures, len(targets)))
	return result, err
}

// lambdaGetConfiguration reads the function's configuration through
// GetFunction. An answer without one is an error, never an empty state.
func lambdaGetConfiguration(ctx context.Context, api LambdaGetFunctionAPI, name string) (*lambdatypes.FunctionConfiguration, error) {
	out, err := api.GetFunction(ctx, &lambda.GetFunctionInput{FunctionName: aws.String(name)})
	if err != nil {
		return nil, err
	}
	if out == nil || out.Configuration == nil {
		return nil, UnusableAnswerErr{Call: "GetFunction", Field: "configuration"}
	}
	return out.Configuration, nil
}

// lambdaPolicy reads the function's resource policy. A missing policy is the
// healthy answer, returned as ResourceNotFoundException.
func lambdaPolicy(ctx context.Context, api LambdaGetPolicyAPI, name string) (string, error) {
	out, err := api.GetPolicy(ctx, &lambda.GetPolicyInput{FunctionName: aws.String(name)})
	if err != nil {
		return "", err
	}
	if out == nil {
		return "", nil
	}
	return aws.ToString(out.Policy), nil
}

// lambdaFunctionURLExposure reports a function URL whose AuthType is NONE.
func lambdaFunctionURLExposure(ctx context.Context, api LambdaListFunctionUrlConfigsAPI, name string) ([]domain.DetailRow, bool, error) {
	// ListFunctionUrlConfigs pages by Marker and NextMarker
	// (https://docs.aws.amazon.com/lambda/latest/api/API_ListFunctionUrlConfigs.html).
	configs, complete, err := PageAll(ctx, PerParentPageCap, func(ctx context.Context, marker *string) ([]lambdatypes.FunctionUrlConfig, *string, error) {
		out, err := api.ListFunctionUrlConfigs(ctx, &lambda.ListFunctionUrlConfigsInput{FunctionName: aws.String(name), Marker: marker})
		if err != nil {
			return nil, nil, err
		}
		if out == nil {
			return nil, nil, UnusableAnswerErr{Call: "ListFunctionUrlConfigs", Field: "function URL configs"}
		}
		return out.FunctionUrlConfigs, out.NextMarker, nil
	})
	if err != nil {
		return nil, false, err
	}
	if !complete {
		return nil, false, errPageCap
	}
	for _, cfg := range configs {
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
