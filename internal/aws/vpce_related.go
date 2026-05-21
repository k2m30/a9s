// vpce_related.go contains VPC Endpoint related-resource checker functions.
package aws

import (
	"context"
	"strings"

	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkVPCESubnet reads SubnetIds from the VpcEndpoint RawStruct directly.
// Pattern F: all data is in RawStruct, no cache lookup needed.
func checkVPCESubnet(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	vpce, ok := assertStruct[ec2types.VpcEndpoint](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: -1}
	}
	if len(vpce.SubnetIds) == 0 {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: 0}
	}
	return relatedResult("subnet", vpce.SubnetIds)
}

// checkVPCESG reads Groups[].GroupId from the VpcEndpoint RawStruct directly.
// Pattern F: all data is in RawStruct, no cache lookup needed.
func checkVPCESG(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	vpce, ok := assertStruct[ec2types.VpcEndpoint](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "sg", Count: -1}
	}
	var ids []string
	for _, g := range vpce.Groups {
		if g.GroupId != nil && *g.GroupId != "" {
			ids = append(ids, *g.GroupId)
		}
	}
	if len(ids) == 0 {
		return resource.RelatedCheckResult{TargetType: "sg", Count: 0}
	}
	return relatedResult("sg", ids)
}

// checkVPCERTB reads RouteTableIds from the VpcEndpoint RawStruct directly.
// Pattern F: all data is in RawStruct, no cache lookup needed (gateway-type endpoints).
func checkVPCERTB(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	vpce, ok := assertStruct[ec2types.VpcEndpoint](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "rtb", Count: -1}
	}
	if len(vpce.RouteTableIds) == 0 {
		return resource.RelatedCheckResult{TargetType: "rtb", Count: 0}
	}
	return relatedResult("rtb", vpce.RouteTableIds)
}

// checkVPCEENI reads NetworkInterfaceIds from the VpcEndpoint RawStruct directly.
// Pattern F: all data is in RawStruct, no cache lookup needed (interface-type endpoints).
func checkVPCEENI(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	vpce, ok := assertStruct[ec2types.VpcEndpoint](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "eni", Count: -1}
	}
	if len(vpce.NetworkInterfaceIds) == 0 {
		return resource.RelatedCheckResult{TargetType: "eni", Count: 0}
	}
	return relatedResult("eni", vpce.NetworkInterfaceIds)
}

// checkVPCEVPC returns the VPC this endpoint is attached to (Pattern F).
// Reads vpc_id from Fields which is populated by the VPC endpoints fetcher.
func checkVPCEVPC(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	vpcID := res.Fields["vpc_id"]
	if vpcID == "" {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: 0}
	}
	return relatedResult("vpc", []string{vpcID})
}

// checkVPCEACM reports the ACM cert on a PrivateLink/Gateway endpoint's custom
// DNS. The list response doesn't carry cert details — requires
// ModifyVpcEndpoint + PrivateDnsNameConfiguration per endpoint. Returns -1.
func checkVPCEACM(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	if res.ID == "" {
		return resource.RelatedCheckResult{TargetType: "acm", Count: 0}
	}
	return resource.RelatedCheckResult{TargetType: "acm", Count: -1}
}

// checkVPCEAlarm reports CloudWatch alarms on this VPC endpoint.
// PrivateLink interface endpoints have per-endpoint alarms using dimension
// "VpcEndpointId". The endpoint ID is the res.ID; alarm cache is scanned.
func checkVPCEAlarm(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	vpceID := res.ID
	if vpceID == "" {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: 0}
	}

	alarmList, truncated, err := FetchRelatedTarget(ctx, clients, cache, "alarm")
	if err != nil {
		if _, sok := clients.(*ServiceClients); !sok {
			alarmList, truncated, err = nil, false, nil
		}
	}
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1, Err: err}
	}
	if alarmList == nil {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1}
	}
	var ids []string
	for _, alarmRes := range alarmList {
		raw, ok := assertStruct[cwtypes.MetricAlarm](alarmRes.RawStruct)
		if !ok {
			continue
		}
		for _, d := range raw.Dimensions {
			if d.Name != nil && *d.Name == "VpcEndpointId" && d.Value != nil && *d.Value == vpceID {
				ids = append(ids, alarmRes.ID)
				break
			}
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("alarm")
	}
	return relatedResult("alarm", ids)
}

// checkVPCECF reports CloudFront distributions fronting this VPC endpoint.
// CloudFront->VPCE mapping goes through CloudFront VPC Origins, which is not
// on DistributionSummary. Returns Count: -1.
func checkVPCECF(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	if res.ID == "" {
		return resource.RelatedCheckResult{TargetType: "cf", Count: 0}
	}
	return resource.RelatedCheckResult{TargetType: "cf", Count: -1}
}

// checkVPCELogs reports CloudWatch Logs groups receiving VPC Flow Logs for
// this endpoint's network interfaces. Pattern C: one ec2:DescribeFlowLogs
// call filtered by resource-id; extract LogGroupName or parse log-group name
// from LogDestination ARN.
func checkVPCELogs(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	vpceID := res.ID
	if vpceID == "" {
		return resource.RelatedCheckResult{TargetType: "logs", Count: 0}
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.EC2 == nil {
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1}
	}
	filterName := "resource-id"
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*ec2.DescribeFlowLogsOutput, error) {
		return c.EC2.DescribeFlowLogs(ctx, &ec2.DescribeFlowLogsInput{
			Filter: []ec2types.Filter{
				{Name: &filterName, Values: []string{vpceID}},
			},
		})
	})
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1, Err: err}
	}
	seen := make(map[string]bool)
	var ids []string
	for _, fl := range out.FlowLogs {
		name := ""
		if fl.LogGroupName != nil && *fl.LogGroupName != "" {
			name = *fl.LogGroupName
		} else if fl.LogDestination != nil && *fl.LogDestination != "" {
			name = *fl.LogDestination
			if strings.Contains(name, ":log-group:") {
				parts := strings.Split(name, ":log-group:")
				if len(parts) == 2 {
					name = parts[1]
					if colon := strings.Index(name, ":"); colon >= 0 {
						name = name[:colon]
					}
				}
			}
		}
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		ids = append(ids, name)
	}
	return relatedResult("logs", ids)
}

// checkVPCER53 reports Route 53 private hosted zones associated with this VPC
// endpoint's PrivateDns. The associated-zones list lives on
// route53:ListHostedZonesByVPC — not in the r53 hosted-zone cache. Returns -1.
func checkVPCER53(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	if res.ID == "" {
		return resource.RelatedCheckResult{TargetType: "r53", Count: 0}
	}
	return resource.RelatedCheckResult{TargetType: "r53", Count: -1}
}

// checkVPCES3 reports S3 buckets associated with a Gateway-type VPC endpoint
// via its policy/allow-list. Policy text lives on VpcEndpoint.PolicyDocument —
// parsing reliably requires JSON parsing and authoritative bucket matching.
// For Gateway endpoints the service name indicates com.amazonaws.<region>.s3;
// determining which buckets are accessible needs policy interpretation — not
// straightforward. Returns Count: -1.
func checkVPCES3(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	if res.ID == "" {
		return resource.RelatedCheckResult{TargetType: "s3", Count: 0}
	}
	return resource.RelatedCheckResult{TargetType: "s3", Count: -1}
}

// checkVPCETG reports target groups pointing at this VPC endpoint as a target.
// Target groups with target_type=ip can target VPCE IP addresses, but the TG
// list cache does not include registered targets — DescribeTargetHealth per TG
// is required. Returns Count: -1.
func checkVPCETG(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	if res.ID == "" {
		return resource.RelatedCheckResult{TargetType: "tg", Count: 0}
	}
	return resource.RelatedCheckResult{TargetType: "tg", Count: -1}
}

// checkVPCEWAF reports WAF Web ACLs associated with this endpoint. VPCE itself
// has no Web ACL binding in the list response; WAF associations are resolved
// from the WAF side via wafv2:ListResourcesForWebACL. Returns Count: -1.
func checkVPCEWAF(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	if res.ID == "" {
		return resource.RelatedCheckResult{TargetType: "waf", Count: 0}
	}
	return resource.RelatedCheckResult{TargetType: "waf", Count: -1}
}
