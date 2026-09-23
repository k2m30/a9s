package unit

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/wafv2"
	wafv2types "github.com/aws/aws-sdk-go-v2/service/wafv2/types"
	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// wafPerTypeFake answers ListResourcesForWebACL the way WAFv2 does: only the
// resources of the requested ResourceType, and an empty ResourceType is read
// as APPLICATION_LOAD_BALANCER. Logging is on and the ACL has a blocking rule, so
// the association answer is the only thing that can raise a finding.
type wafPerTypeFake struct {
	awsclient.WAFv2API
	byType map[wafv2types.ResourceType][]string
	errFor map[wafv2types.ResourceType]error
}

func (f *wafPerTypeFake) ListResourcesForWebACL(_ context.Context, in *wafv2.ListResourcesForWebACLInput, _ ...func(*wafv2.Options)) (*wafv2.ListResourcesForWebACLOutput, error) {
	rt := in.ResourceType
	if rt == "" {
		rt = wafv2types.ResourceTypeApplicationLoadBalancer
	}
	if err := f.errFor[rt]; err != nil {
		return nil, err
	}
	return &wafv2.ListResourcesForWebACLOutput{ResourceArns: f.byType[rt]}, nil
}

func (f *wafPerTypeFake) GetLoggingConfiguration(_ context.Context, in *wafv2.GetLoggingConfigurationInput, _ ...func(*wafv2.Options)) (*wafv2.GetLoggingConfigurationOutput, error) {
	return wafLoggingOutput(aws.ToString(in.ResourceArn)), nil
}

func (f *wafPerTypeFake) GetWebACL(_ context.Context, in *wafv2.GetWebACLInput, _ ...func(*wafv2.Options)) (*wafv2.GetWebACLOutput, error) {
	return &wafv2.GetWebACLOutput{WebACL: &wafv2types.WebACL{
		Name: in.Name,
		Id:   in.Id,
		Rules: []wafv2types.Rule{{
			Name:     aws.String("block-bad-ips"),
			Priority: 0,
			Action:   &wafv2types.RuleAction{Block: &wafv2types.BlockAction{}},
		}},
	}}, nil
}

func t563WAFACL() resource.Resource {
	return resource.Resource{
		ID:   wafACLARN1,
		Name: "my-acl-1",
		Fields: map[string]string{
			"name":  "my-acl-1",
			"id":    "aaaabbbb-1111-2222-3333-444444444444",
			"arn":   wafACLARN1,
			"scope": string(wafv2types.ScopeRegional),
		},
	}
}

// ListResourcesForWebACL reports one resource type per call and defaults to
// APPLICATION_LOAD_BALANCER, so a REGIONAL web ACL protecting only an API
// Gateway stage, an AppSync API or a Cognito user pool is associated even
// though the ALB answer is empty. The ACL is unassociated only when every
// regional type answers empty; a type that cannot be asked leaves the row not
// inspected rather than claiming "none".
func TestT563_WAFOrphanCheckAsksEveryRegionalResourceType(t *testing.T) {
	denied := &smithy.GenericAPIError{Code: "AccessDeniedException", Message: "not authorized to perform wafv2:ListResourcesForWebACL"}
	cases := []struct {
		name          string
		byType        map[wafv2types.ResourceType][]string
		errFor        map[wafv2types.ResourceType]error
		wantOrphan    bool
		wantTruncated bool
	}{
		{
			name:   "API Gateway stage only",
			byType: map[wafv2types.ResourceType][]string{wafv2types.ResourceTypeApiGateway: {"arn:aws:apigateway:us-east-1::/restapis/a1b2c3d4e5/stages/prod"}},
		},
		{
			name:   "AppSync API only",
			byType: map[wafv2types.ResourceType][]string{wafv2types.ResourceTypeAppsync: {"arn:aws:appsync:us-east-1:123456789012:apis/abcdefghijklmnopqrstuvwxyz"}},
		},
		{
			name:   "Cognito user pool only",
			byType: map[wafv2types.ResourceType][]string{wafv2types.ResourceTypeCognitioUserPool: {"arn:aws:cognito-idp:us-east-1:123456789012:userpool/us-east-1_EXAMPLE1"}},
		},
		{
			name:   "load balancer only",
			byType: map[wafv2types.ResourceType][]string{wafv2types.ResourceTypeApplicationLoadBalancer: {"arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/web-alb/50dc6c495c0c9188"}},
		},
		{
			name:       "nothing of any type",
			wantOrphan: true,
		},
		{
			name:          "API Gateway answer denied, every other type empty",
			errFor:        map[wafv2types.ResourceType]error{wafv2types.ResourceTypeApiGateway: denied},
			wantTruncated: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clients := &awsclient.ServiceClients{WAFv2: &wafPerTypeFake{byType: tc.byType, errFor: tc.errFor}}

			result, err := awsclient.EnrichWAFLogging(context.Background(), clients, []resource.Resource{t563WAFACL()}, nil)
			if err != nil && !tc.wantTruncated {
				t.Fatalf("EnrichWAFLogging: %v", err)
			}

			orphan := hasCode(result.Findings[wafACLARN1], domain.FindingCode("waf.orphan"))
			if orphan != tc.wantOrphan {
				t.Errorf("waf.orphan raised = %v, want %v; findings = %+v", orphan, tc.wantOrphan, result.Findings[wafACLARN1])
			}
			if !tc.wantOrphan && len(result.Findings[wafACLARN1]) != 0 {
				t.Errorf("findings = %+v, want none", result.Findings[wafACLARN1])
			}
			_, truncated := result.TruncatedIDs[wafACLARN1]
			if truncated != tc.wantTruncated {
				t.Errorf("row not inspected = %v, want %v; TruncatedIDs = %v", truncated, tc.wantTruncated, result.TruncatedIDs)
			}
		})
	}
}
