// interfaces_networking.go adds AWS API interfaces used by networking/edge
// related-panel Pattern C checkers. Kept separate from interfaces.go to
// minimise merge churn with other ongoing resource batches.
package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/wafv2"
)

// ELBv2DescribeTagsAPI lists tags on one or more ELB/TG/listener/rule ARNs.
type ELBv2DescribeTagsAPI interface {
	DescribeTags(ctx context.Context, params *elbv2.DescribeTagsInput, optFns ...func(*elbv2.Options)) (*elbv2.DescribeTagsOutput, error)
}

// WAFv2GetWebACLForResourceAPI is the forward "resource → WebACL" WAF lookup.
type WAFv2GetWebACLForResourceAPI interface {
	GetWebACLForResource(ctx context.Context, params *wafv2.GetWebACLForResourceInput, optFns ...func(*wafv2.Options)) (*wafv2.GetWebACLForResourceOutput, error)
}

// CloudFrontListDistributionsByWebACLIdAPI is the forward "WebACL → CF distributions" lookup.
type CloudFrontListDistributionsByWebACLIdAPI interface {
	ListDistributionsByWebACLId(ctx context.Context, params *cloudfront.ListDistributionsByWebACLIdInput, optFns ...func(*cloudfront.Options)) (*cloudfront.ListDistributionsByWebACLIdOutput, error)
}

// APIGatewayV2GetDomainNamesAPI lists custom domain names registered for
// HTTP/WebSocket APIs. Used to resolve apigw→acm, apigw→r53.
type APIGatewayV2GetDomainNamesAPI interface {
	GetDomainNames(ctx context.Context, params *apigatewayv2.GetDomainNamesInput, optFns ...func(*apigatewayv2.Options)) (*apigatewayv2.GetDomainNamesOutput, error)
}

// APIGatewayV2GetApiMappingsAPI returns API→stage mappings for a given
// custom domain. Used with GetDomainNames to determine which domains map
// to a given API.
type APIGatewayV2GetApiMappingsAPI interface {
	GetApiMappings(ctx context.Context, params *apigatewayv2.GetApiMappingsInput, optFns ...func(*apigatewayv2.Options)) (*apigatewayv2.GetApiMappingsOutput, error)
}

// APIGatewayV2GetIntegrationsAPI lists integrations (Lambda, SFN, SNS, HTTP)
// for a given API. Used to resolve apigw→lambda/sfn/sns.
type APIGatewayV2GetIntegrationsAPI interface {
	GetIntegrations(ctx context.Context, params *apigatewayv2.GetIntegrationsInput, optFns ...func(*apigatewayv2.Options)) (*apigatewayv2.GetIntegrationsOutput, error)
}

// EC2DescribeTransitGatewayVpcAttachmentsAPI enumerates subnets attached
// to a Transit Gateway via VPC attachments. Used to resolve tgw→subnet.
type EC2DescribeTransitGatewayVpcAttachmentsAPI interface {
	DescribeTransitGatewayVpcAttachments(ctx context.Context, params *ec2.DescribeTransitGatewayVpcAttachmentsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeTransitGatewayVpcAttachmentsOutput, error)
}
