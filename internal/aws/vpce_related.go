// vpce_related.go contains VPC Endpoint related-resource checker functions.
package aws

import (
	"context"
	"strings"

	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	r53types "github.com/aws/aws-sdk-go-v2/service/route53/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkVPCESubnet reads SubnetIds from the VpcEndpoint RawStruct directly.
// Pattern F: all data is in RawStruct, no cache lookup needed.
func checkVPCESubnet(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	vpce, ok := assertStruct[ec2types.VpcEndpoint](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("subnet")
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
		return resource.UnknownRelated("sg")
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
		return resource.UnknownRelated("rtb")
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
		return resource.UnknownRelated("eni")
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
		return resource.ErrorRelated("alarm", err)
	}
	if alarmList == nil {
		return resource.UnknownRelated("alarm")
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
		return resource.UnknownRelated("logs")
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
		return resource.ErrorRelated("logs", err)
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
// endpoint's VPC. Pattern C: one route53:ListHostedZonesByVPC call for the
// endpoint's VpcId.
func checkVPCER53(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	vpcID := res.Fields["vpc_id"]
	if vpcID == "" {
		if vpce, ok := assertStruct[ec2types.VpcEndpoint](res.RawStruct); ok && vpce.VpcId != nil {
			vpcID = *vpce.VpcId
		}
	}
	if vpcID == "" {
		return resource.RelatedCheckResult{TargetType: "r53", Count: 0}
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.Route53 == nil {
		return resource.UnknownRelated("r53")
	}
	api, ok := c.Route53.(Route53ListHostedZonesByVPCAPI)
	if !ok {
		return resource.UnknownRelated("r53")
	}
	region := c.Region
	if region == "" {
		region = GetDefaultRegion("", "")
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*route53.ListHostedZonesByVPCOutput, error) {
		return api.ListHostedZonesByVPC(ctx, &route53.ListHostedZonesByVPCInput{
			VPCId:     &vpcID,
			VPCRegion: r53types.VPCRegion(region),
		})
	})
	if err != nil {
		return resource.ErrorRelated("r53", err)
	}
	if out == nil || len(out.HostedZoneSummaries) == 0 {
		return resource.RelatedCheckResult{TargetType: "r53", Count: 0}
	}
	var ids []string
	for _, z := range out.HostedZoneSummaries {
		if z.HostedZoneId != nil && *z.HostedZoneId != "" {
			ids = append(ids, *z.HostedZoneId)
		}
	}
	return relatedResult("r53", ids)
}
