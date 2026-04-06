package aws

import (
	"context"
	"fmt"
	"strings"

	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("elb_listener_rules", []string{
		"priority", "conditions_summary", "action_type", "action_target", "is_default",
	})

	resource.RegisterPaginatedChild("elb_listener_rules", func(ctx context.Context, clients any, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchELBListenerRules(ctx, c.ELBv2, parentCtx, continuationToken)
	})

	resource.RegisterChildType(resource.ResourceTypeDef{
		Name:      "Listener Rules",
		ShortName: "elb_listener_rules",
		Columns:   resource.ELBListenerRuleColumns(),
		CopyField: "conditions_summary",
	})
}

// FetchELBListenerRules calls the ELBv2 DescribeRules API and converts the
// response into a FetchResult. This is a single-call API (no pagination from AWS),
// but uses FetchResult for consistency with the paginated child fetcher interface.
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

	output, err := api.DescribeRules(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("describing rules for listener %s: %w", listenerArn, err)
	}

	var resources []resource.Resource
	for _, rule := range output.Rules {
		resources = append(resources, convertRule(rule))
		if len(resources) >= maxRules {
			break
		}
	}

	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: false,
			TotalHint:   len(resources),
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
		ID:     ruleArn,
		Name:   priority,
		Status: "",
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
