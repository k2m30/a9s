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
	resource.RegisterFieldKeys("elb_listeners", []string{
		"port", "protocol", "default_action_type", "default_action_target",
		"ssl_policy", "certificate_short",
	})

	resource.RegisterChildFetcher("elb_listeners", func(ctx context.Context, clients interface{}, parentCtx resource.ParentContext) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchELBListeners(ctx, c.ELBv2, parentCtx)
	})

	resource.RegisterChildType(resource.ResourceTypeDef{
		Name:      "ELB Listeners",
		ShortName: "elb_listeners",
		Columns:   resource.ELBListenerColumns(),
	})
}

// FetchELBListeners calls the ELBv2 DescribeListeners API and converts the
// response into a slice of generic Resource structs.
func FetchELBListeners(
	ctx context.Context,
	api ELBv2DescribeListenersAPI,
	parentCtx map[string]string,
) ([]resource.Resource, error) {
	const maxListeners = 200

	lbArn := parentCtx["load_balancer_arn"]

	var resources []resource.Resource
	var marker *string

	for {
		input := &elbv2.DescribeListenersInput{
			LoadBalancerArn: &lbArn,
			Marker:          marker,
		}

		output, err := api.DescribeListeners(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("describing listeners for %s: %w", lbArn, err)
		}

		for _, listener := range output.Listeners {
			resources = append(resources, convertListener(listener))

			if len(resources) >= maxListeners {
				return resources, nil
			}
		}

		if output.NextMarker == nil {
			break
		}
		marker = output.NextMarker
	}

	return resources, nil
}

func convertListener(listener elbtypes.Listener) resource.Resource {
	arn := ""
	if listener.ListenerArn != nil {
		arn = *listener.ListenerArn
	}

	port := ""
	if listener.Port != nil {
		port = fmt.Sprintf("%d", *listener.Port)
	}

	protocol := string(listener.Protocol)

	sslPolicy := ""
	if listener.SslPolicy != nil {
		sslPolicy = *listener.SslPolicy
	}

	actionType := ""
	actionTarget := ""
	if len(listener.DefaultActions) > 0 {
		action := listener.DefaultActions[0]
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
			if action.FixedResponseConfig != nil && action.FixedResponseConfig.StatusCode != nil {
				actionTarget = *action.FixedResponseConfig.StatusCode
			}
		}
	}

	certShort := ""
	if len(listener.Certificates) > 0 && listener.Certificates[0].CertificateArn != nil {
		certShort = extractCertID(*listener.Certificates[0].CertificateArn)
	}

	return resource.Resource{
		ID:     arn,
		Name:   port,
		Status: "",
		Fields: map[string]string{
			"port":                  port,
			"protocol":              protocol,
			"default_action_type":   actionType,
			"default_action_target": actionTarget,
			"ssl_policy":            sslPolicy,
			"certificate_short":     certShort,
		},
		RawStruct: listener,
	}
}

// extractTGName extracts the target group name from an ARN like:
// arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/api-prod-tg/abc123
func extractTGName(arn string) string {
	parts := strings.Split(arn, "/")
	if len(parts) >= 2 {
		return parts[len(parts)-2]
	}
	return arn
}

// buildRedirectURL builds a human-readable redirect URL from RedirectConfig.
func buildRedirectURL(cfg *elbtypes.RedirectActionConfig) string {
	proto := "#{protocol}"
	if cfg.Protocol != nil {
		proto = *cfg.Protocol
	}
	host := "#{host}"
	if cfg.Host != nil {
		host = *cfg.Host
	}
	port := "#{port}"
	if cfg.Port != nil {
		port = *cfg.Port
	}
	path := "#{path}"
	if cfg.Path != nil {
		path = *cfg.Path
	}
	query := "#{query}"
	if cfg.Query != nil {
		query = *cfg.Query
	}
	return fmt.Sprintf("%s://%s:%s%s?%s", proto, host, port, path, query)
}

// extractCertID extracts the certificate ID from the ARN.
// "arn:aws:acm:us-east-1:123456789012:certificate/abc-def-123" -> "abc-def-123"
func extractCertID(arn string) string {
	parts := strings.Split(arn, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return arn
}
