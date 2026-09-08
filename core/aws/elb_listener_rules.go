// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// elbListenerRulesCursor encodes both continuation dimensions this fetcher
// paginates over: Marker is the AWS-native DescribeRules page cursor;
// Skip is an in-page offset produced by this fetcher's own maxRules cap.
// DescribeRules' Marker only ever advances across whole AWS pages — it has
// no concept of "resume this same page from item 201" — so when a single AWS
// page carries more than maxRules items, resuming means re-issuing the SAME
// Marker (re-fetching the identical AWS page, which is deterministic for a
// given ListenerArn+Marker) and skipping the items already converted, rather
// than jumping straight to AWS's own NextMarker, which would silently drop
// the remainder of the current page.
type elbListenerRulesCursor struct {
	Marker string `json:"m,omitempty"`
	Skip   int    `json:"s,omitempty"`
}

// decodeELBListenerRulesCursor decodes a compound continuation token
// previously produced by elbListenerRulesCursor.encode(), via
// strictDecodeCursor. An empty token legitimately means "first page" and
// returns the zero cursor. Any other input that strictDecodeCursor rejects
// — unknown fields, trailing bytes after the JSON value, or a decode that
// lands on the exact zero value (a bare "null"/"{}", which encode() never
// itself produces since a token is only ever encoded when there is real
// state — a non-empty Marker or a positive Skip — to resume from) — has no
// legitimate origin other than this same encoder, so it is reported as an
// error rather than silently restarting from page 1 with no signal.
func decodeELBListenerRulesCursor(token string) (elbListenerRulesCursor, error) {
	if token == "" {
		return elbListenerRulesCursor{}, nil
	}
	cur, _, malformed := strictDecodeCursor[elbListenerRulesCursor](token)
	if malformed {
		return elbListenerRulesCursor{}, fmt.Errorf("decoding elb listener rules continuation token: invalid or foreign cursor %q", token)
	}
	return cur, nil
}

func (c elbListenerRulesCursor) encode() string {
	b, err := json.Marshal(c)
	if err != nil {
		return ""
	}
	return string(b)
}

// FetchELBListenerRules calls the ELBv2 DescribeRules API for one page of
// rules and converts the response into a FetchResult. DescribeRules paginates
// via Marker/NextMarker like other ELBv2 List/Describe calls. maxRules
// additionally caps the resources converted from a single response,
// independent of whether AWS itself paginated; continuationToken (see
// elbListenerRulesCursor) resumes whichever of the two caused the previous
// call to truncate — the in-page Skip offset takes priority over advancing
// to AWS's own NextMarker, so a page longer than maxRules is fully consumed
// before moving on, never partially skipped.
func FetchELBListenerRules(
	ctx context.Context,
	api ELBv2DescribeRulesAPI,
	parentCtx map[string]string,
	continuationToken string,
) (resource.FetchResult, error) {
	const maxRules = 200

	listenerArn := parentCtx["listener_arn"]

	cur, err := decodeELBListenerRulesCursor(continuationToken)
	if err != nil {
		return resource.FetchResult{}, err
	}

	input := &elbv2.DescribeRulesInput{
		ListenerArn: &listenerArn,
	}
	if cur.Marker != "" {
		input.Marker = &cur.Marker
	}

	output, err := api.DescribeRules(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("describing rules for listener %s: %w", listenerArn, err)
	}

	skip := min(cur.Skip, len(output.Rules))
	end := min(skip+maxRules, len(output.Rules))
	cappedLocally := end < len(output.Rules)

	var resources []resource.Resource
	for _, rule := range output.Rules[skip:end] {
		resources = append(resources, convertRule(rule))
	}

	next := elbListenerRulesCursor{Marker: cur.Marker, Skip: end}
	isTruncated := cappedLocally
	// A non-nil but empty NextMarker is exhaustion, not "one more page" — the
	// same nil-or-empty contract every AWS-generated paginator in this SDK
	// enforces; DescribeRules has no generated paginator of its own to
	// inherit it from. Without this check, a non-nil empty NextMarker would
	// encode a Marker="" cursor that decodeELBListenerRulesCursor rejects as
	// invalid on the very next request.
	if !cappedLocally && output.NextMarker != nil && *output.NextMarker != "" {
		next = elbListenerRulesCursor{Marker: *output.NextMarker}
		isTruncated = true
	}

	nextToken := ""
	if isTruncated {
		nextToken = next.encode()
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
