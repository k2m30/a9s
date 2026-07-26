// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"
	"strings"

	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// FetchELBListenerRules calls the ELBv2 DescribeRules API for one page of
// rules and converts the response into a FetchResult. DescribeRules paginates
// via Marker/NextMarker like other ELBv2 List/Describe calls; continuationToken
// round-trips through Marker. maxRules additionally caps the resources
// converted from a single response, independent of whether AWS itself
// paginated — IsTruncated reports true for either cause.
func FetchELBListenerRules(
	ctx context.Context,
	api ELBv2DescribeRulesAPI,
	parentCtx map[string]string,
	continuationToken string,
) (resource.FetchResult, error) {
	const maxRules = 200

	listenerArn := parentCtx["listener_arn"]

	input := &elbv2.DescribeRulesInput{
		ListenerArn: &listenerArn,
	}
	if continuationToken != "" {
		input.Marker = &continuationToken
	}

	output, err := api.DescribeRules(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("describing rules for listener %s: %w", listenerArn, err)
	}

	var resources []resource.Resource
	cappedLocally := false
	for _, rule := range output.Rules {
		if len(resources) >= maxRules {
			cappedLocally = true
			break
		}
		resources = append(resources, convertRule(rule))
	}

	nextToken := ""
	isTruncated := cappedLocally
	if output.NextMarker != nil {
		nextToken = *output.NextMarker
		isTruncated = true
	}

	totalHint := len(resources)
	if isTruncated {
		totalHint = -1
	}

	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: isTruncated,
			NextToken:   nextToken,
			TotalHint:   totalHint,
			PageSize:    len(resources),
		},
	}, nil
}

func convertRule(rule elbtypes.Rule) resource.Resource {
	ruleArn := ""
	if rule.RuleArn != nil {
		ruleArn = *rule.RuleArn
	}

	priority := ""
	if rule.Priority != nil {
		priority = *rule.Priority
	}

	isDefault := "false"
	if rule.IsDefault != nil && *rule.IsDefault {
		isDefault = "true"
	}

	conditionsSummary := BuildConditionsSummary(rule.Conditions)

	actionType := ""
	actionTarget := ""
	if len(rule.Actions) > 0 {
		action := rule.Actions[0]
		actionType = string(action.Type)

		switch action.Type {
		case elbtypes.ActionTypeEnumForward:
			if action.TargetGroupArn != nil {
				actionTarget = extractTGName(*action.TargetGroupArn)
			} else if action.ForwardConfig != nil && len(action.ForwardConfig.TargetGroups) > 0 {
				if action.ForwardConfig.TargetGroups[0].TargetGroupArn != nil {
					actionTarget = extractTGName(*action.ForwardConfig.TargetGroups[0].TargetGroupArn)
				}
			}
		case elbtypes.ActionTypeEnumRedirect:
			if action.RedirectConfig != nil {
				actionTarget = buildRedirectURL(action.RedirectConfig)
			}
		case elbtypes.ActionTypeEnumFixedResponse:
			if action.FixedResponseConfig != nil {
				statusCode := ""
				if action.FixedResponseConfig.StatusCode != nil {
					statusCode = *action.FixedResponseConfig.StatusCode
				}
				contentType := ""
				if action.FixedResponseConfig.ContentType != nil {
					contentType = *action.FixedResponseConfig.ContentType
				}
				if contentType != "" {
					actionTarget = statusCode + " " + contentType
				} else {
					actionTarget = statusCode
				}
			}
		}
	}

	return resource.Resource{
		ID:   ruleArn,
		Name: priority,
		Fields: map[string]string{
			"priority":           priority,
			"conditions_summary": conditionsSummary,
			"action_type":        actionType,
			"action_target":      actionTarget,
			"is_default":         isDefault,
		},
		RawStruct: rule,
	}
}

// BuildConditionsSummary builds a human-readable summary of rule conditions.
// Multiple conditions are joined with " AND ". Empty conditions return "".
func BuildConditionsSummary(conditions []elbtypes.RuleCondition) string {
	if len(conditions) == 0 {
		return ""
	}

	var parts []string
	for _, cond := range conditions {
		field := ""
		if cond.Field != nil {
			field = *cond.Field
		}

		switch field {
		case "path-pattern":
			if cond.PathPatternConfig != nil && len(cond.PathPatternConfig.Values) > 0 {
				parts = append(parts, "path: "+strings.Join(cond.PathPatternConfig.Values, ","))
			}
		case "host-header":
			if cond.HostHeaderConfig != nil && len(cond.HostHeaderConfig.Values) > 0 {
				parts = append(parts, "host: "+strings.Join(cond.HostHeaderConfig.Values, ","))
			}
		case "source-ip":
			if cond.SourceIpConfig != nil && len(cond.SourceIpConfig.Values) > 0 {
				parts = append(parts, "src: "+strings.Join(cond.SourceIpConfig.Values, ","))
			}
		case "http-header":
			if cond.HttpHeaderConfig != nil {
				name := ""
				if cond.HttpHeaderConfig.HttpHeaderName != nil {
					name = *cond.HttpHeaderConfig.HttpHeaderName
				}
				vals := strings.Join(cond.HttpHeaderConfig.Values, ",")
				parts = append(parts, "header: "+name+"="+vals)
			}
		case "http-request-method":
			if cond.HttpRequestMethodConfig != nil && len(cond.HttpRequestMethodConfig.Values) > 0 {
				parts = append(parts, "method: "+strings.Join(cond.HttpRequestMethodConfig.Values, ","))
			}
		case "query-string":
			if cond.QueryStringConfig != nil && len(cond.QueryStringConfig.Values) > 0 {
				var qsParts []string
				for _, kv := range cond.QueryStringConfig.Values {
					k := ""
					if kv.Key != nil {
						k = *kv.Key
					}
					v := ""
					if kv.Value != nil {
						v = *kv.Value
					}
					qsParts = append(qsParts, k+"="+v)
				}
				parts = append(parts, "query: "+strings.Join(qsParts, "&"))
			}
		default:
			if field != "" {
				parts = append(parts, field)
			}
		}
	}

	return strings.Join(parts, " AND ")
}
