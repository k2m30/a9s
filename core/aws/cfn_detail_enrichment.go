// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// StackEnriched wraps cfntypes.Stack with the template body GetTemplate
// returns. Embedding promotes all SDK fields so YAML/JSON views render full
// stack metadata alongside the template.
type StackEnriched struct {
	cfntypes.Stack
	TemplateBody any `json:"TemplateBody,omitempty" yaml:"TemplateBody,omitempty"`
}

// enrichCfn fetches the template body for a CloudFormation stack via
// GetTemplate. Session-cached with a version-aware key: cacheKey folds in
// the list row's LastUpdatedTime (falling back to CreationTime), so a stack
// updated in-session gets a new key on its next refreshed list row — a
// natural cache miss that forces a fresh GetTemplate instead of serving a
// pre-update template. Stale entries under the old version's key are never
// evicted, but that's bounded and small per session (docs/architecture.md
// §On-Demand detail enrichment).
func enrichCfn(ctx context.Context, clients any, res resource.Resource) (resource.Resource, error) {
	return enrichDetail(ctx, clients, res, detailEnrichSpec[cfntypes.Stack, any]{
		unwrap: unwrapEnriched(func(w StackEnriched) cfntypes.Stack { return w.Stack }),
		id: func(stack cfntypes.Stack, _ resource.Resource) (string, error) {
			// Prefer StackId (stable across renames), fall back to StackName.
			switch {
			case stack.StackId != nil && *stack.StackId != "":
				return *stack.StackId, nil
			case stack.StackName != nil && *stack.StackName != "":
				return *stack.StackName, nil
			default:
				return "", fmt.Errorf("stack has no identifier")
			}
		},
		cache: detailDocsCache,
		cacheKey: func(id string, stack cfntypes.Stack, _ resource.Resource) string {
			version := stack.CreationTime
			if stack.LastUpdatedTime != nil {
				version = stack.LastUpdatedTime
			}
			var unix int64
			if version != nil {
				unix = version.Unix()
			}
			return "cfn:" + id + ":" + strconv.FormatInt(unix, 10)
		},
		fetch: func(ctx context.Context, c *ServiceClients, id string, _ cfntypes.Stack, _ resource.Resource) (any, error) {
			api, ok := c.CloudFormation.(CFNGetTemplateAPI)
			if !ok {
				return nil, fmt.Errorf("CloudFormation client does not support GetTemplate")
			}
			out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*cloudformation.GetTemplateOutput, error) {
				return api.GetTemplate(ctx, &cloudformation.GetTemplateInput{StackName: &id})
			})
			if err != nil {
				return nil, err
			}
			if out.TemplateBody == nil {
				return nil, nil
			}
			// CFN templates are frequently authored as YAML, not JSON —
			// parseJSONOrRaw's raw-string fallback renders as a block scalar
			// rather than failing enrichment.
			return parseJSONOrRaw(*out.TemplateBody), nil
		},
		wrap: func(stack cfntypes.Stack, body any) any {
			return StackEnriched{Stack: stack, TemplateBody: body}
		},
	})
}
