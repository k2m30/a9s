// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// FunctionEnriched wraps lambdatypes.FunctionConfiguration — GetFunction's
// Configuration replaces the list-level one entirely (it carries
// State/StateReason/LastUpdateStatus* fields the list call omits) — plus the
// Code/Concurrency fields only GetFunction returns.
type FunctionEnriched struct {
	lambdatypes.FunctionConfiguration
	Code        *lambdatypes.FunctionCodeLocation `json:"Code,omitempty" yaml:"Code,omitempty"`
	Concurrency *lambdatypes.Concurrency          `json:"Concurrency,omitempty" yaml:"Concurrency,omitempty"`
}

// lambdaDetailPayload is enrichLambda's fetch result: GetFunction's three
// enrichment-relevant fields, bundled so wrap can apply Configuration's
// nil-vs-present distinction independently of Code/Concurrency.
type lambdaDetailPayload struct {
	Configuration *lambdatypes.FunctionConfiguration
	Code          *lambdatypes.FunctionCodeLocation
	Concurrency   *lambdatypes.Concurrency
}

// enrichLambda fetches the full function configuration (including
// deployment state) plus code location and reserved concurrency via
// GetFunction. Uncached: a single cheap call, and State/StateReason/
// LastUpdateStatus* are exactly the fields that change during a deploy —
// the moment an operator is most likely to open this detail view.
func enrichLambda(ctx context.Context, clients any, res resource.Resource) (resource.Resource, error) {
	return enrichDetail(ctx, clients, res, detailEnrichSpec[lambdatypes.FunctionConfiguration, lambdaDetailPayload]{
		unwrap: unwrapEnriched(func(w FunctionEnriched) lambdatypes.FunctionConfiguration { return w.FunctionConfiguration }),
		id: func(cfg lambdatypes.FunctionConfiguration, _ resource.Resource) (string, error) {
			// Prefer FunctionName, fall back to FunctionArn.
			switch {
			case cfg.FunctionName != nil && *cfg.FunctionName != "":
				return *cfg.FunctionName, nil
			case cfg.FunctionArn != nil && *cfg.FunctionArn != "":
				return *cfg.FunctionArn, nil
			default:
				return "", fmt.Errorf("function has no name or ARN")
			}
		},
		fetch: func(ctx context.Context, c *ServiceClients, id string, _ lambdatypes.FunctionConfiguration, _ resource.Resource) (lambdaDetailPayload, error) {
			api, ok := c.Lambda.(LambdaGetFunctionAPI)
			if !ok {
				return lambdaDetailPayload{}, fmt.Errorf("AWS Lambda client does not support GetFunction")
			}
			out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*lambda.GetFunctionOutput, error) {
				return api.GetFunction(ctx, &lambda.GetFunctionInput{FunctionName: &id})
			})
			if err != nil {
				return lambdaDetailPayload{}, err
			}
			return lambdaDetailPayload{Configuration: out.Configuration, Code: out.Code, Concurrency: out.Concurrency}, nil
		},
		wrap: func(cfg lambdatypes.FunctionConfiguration, payload lambdaDetailPayload) any {
			enriched := FunctionEnriched{FunctionConfiguration: cfg, Code: payload.Code, Concurrency: payload.Concurrency}
			if payload.Configuration != nil {
				enriched.FunctionConfiguration = *payload.Configuration
			}
			return enriched
		},
	})
}
