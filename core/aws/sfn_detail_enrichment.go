// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/sfn"
	sfntypes "github.com/aws/aws-sdk-go-v2/service/sfn/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// StateMachineEnriched wraps sfntypes.StateMachineListItem with the fields
// only DescribeStateMachine returns (Status, RoleArn, Definition). Embedding
// promotes all list-level SDK fields so YAML/JSON views render full
// state-machine metadata alongside the detail fields.
type StateMachineEnriched struct {
	sfntypes.StateMachineListItem
	Status     sfntypes.StateMachineStatus `json:"Status,omitempty" yaml:"Status,omitempty"`
	RoleArn    *string                     `json:"RoleArn,omitempty" yaml:"RoleArn,omitempty"`
	Definition any                         `json:"Definition,omitempty" yaml:"Definition,omitempty"`
}

// sfnDetailPayload bundles everything enrichSfn adds on top of the
// list-level StateMachineListItem, so fetch can hand wrap a single value.
type sfnDetailPayload struct {
	Status     sfntypes.StateMachineStatus
	RoleArn    *string
	Definition any
}

// enrichSfn resolves the ASL definition, status, and execution role for a
// Step Functions state machine via DescribeStateMachine. Uncached, unlike
// cfn/policy: the ARN is stable across a redeploy (UpdateStateMachine keeps
// it), so a session cache would serve a stale ASL definition and status
// exactly while an operator is watching a deploy — the one moment this
// detail view matters most. DescribeStateMachine is a single cheap call
// (the same reasoning that keeps lambda's GetFunction uncached), so paying
// it on every detail open is the right trade (docs/architecture.md
// §On-Demand detail enrichment).
func enrichSfn(ctx context.Context, clients any, res resource.Resource) (resource.Resource, error) {
	return enrichDetail(ctx, clients, res, detailEnrichSpec[sfntypes.StateMachineListItem, sfnDetailPayload]{
		unwrap: unwrapEnriched(func(w StateMachineEnriched) sfntypes.StateMachineListItem { return w.StateMachineListItem }),
		id: func(item sfntypes.StateMachineListItem, _ resource.Resource) (string, error) {
			if item.StateMachineArn == nil || *item.StateMachineArn == "" {
				return "", fmt.Errorf("state machine has no ARN")
			}
			return *item.StateMachineArn, nil
		},
		fetch: func(ctx context.Context, c *ServiceClients, id string, _ sfntypes.StateMachineListItem, _ resource.Resource) (sfnDetailPayload, error) {
			api, ok := c.SFN.(SFNDescribeStateMachineAPI)
			if !ok {
				return sfnDetailPayload{}, fmt.Errorf("SFN client does not support DescribeStateMachine")
			}
			out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*sfn.DescribeStateMachineOutput, error) {
				return api.DescribeStateMachine(ctx, &sfn.DescribeStateMachineInput{StateMachineArn: &id})
			})
			if err != nil {
				return sfnDetailPayload{}, err
			}
			var definition any
			if out.Definition != nil {
				definition = parseJSONOrRaw(*out.Definition)
			}
			return sfnDetailPayload{Status: out.Status, RoleArn: out.RoleArn, Definition: definition}, nil
		},
		wrap: func(item sfntypes.StateMachineListItem, payload sfnDetailPayload) any {
			return StateMachineEnriched{
				StateMachineListItem: item,
				Status:               payload.Status,
				RoleArn:              payload.RoleArn,
				Definition:           payload.Definition,
			}
		},
	})
}
