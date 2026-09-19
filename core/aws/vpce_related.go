// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// vpce_related.go contains VPC Endpoint related-resource checker functions.
package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	r53types "github.com/aws/aws-sdk-go-v2/service/route53/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkVPCESubnet reads SubnetIds from the VpcEndpoint RawStruct directly.
// Pattern F: all data is in RawStruct.
func checkVPCESubnet(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	vpce, ok := assertStruct[ec2types.VpcEndpoint](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("subnet")
	}
	if len(vpce.SubnetIds) == 0 {
		return resource.KnownRelated("subnet", nil, false)
	}
	return relatedResult("subnet", vpce.SubnetIds)
}

// checkVPCESG reads Groups[].GroupId from the VpcEndpoint RawStruct directly.
// Pattern F: all data is in RawStruct.
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
		return resource.KnownRelated("sg", nil, false)
	}
	return relatedResult("sg", ids)
}

// checkVPCERTB reads RouteTableIds from the VpcEndpoint RawStruct directly.
// Pattern F: all data is in RawStruct (gateway-type endpoints).
func checkVPCERTB(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	vpce, ok := assertStruct[ec2types.VpcEndpoint](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("rtb")
	}
	if len(vpce.RouteTableIds) == 0 {
		return resource.KnownRelated("rtb", nil, false)
	}
	return relatedResult("rtb", vpce.RouteTableIds)
}

// checkVPCEENI reads NetworkInterfaceIds from the VpcEndpoint RawStruct directly.
// Pattern F: all data is in RawStruct (interface-type endpoints).
func checkVPCEENI(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	vpce, ok := assertStruct[ec2types.VpcEndpoint](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("eni")
	}
	if len(vpce.NetworkInterfaceIds) == 0 {
		return resource.KnownRelated("eni", nil, false)
	}
	return relatedResult("eni", vpce.NetworkInterfaceIds)
}

// checkVPCEVPC returns the VPC this endpoint is attached to (Pattern F).
// Reads vpc_id from Fields which is populated by the VPC endpoints fetcher.
func checkVPCEVPC(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	vpcID := res.Fields["vpc_id"]
	if vpcID == "" {
		return resource.KnownRelated("vpc", nil, false)
	}
	return relatedResult("vpc", []string{vpcID})
}

// checkVPCEAlarm reports CloudWatch alarms on this VPC endpoint.
// PrivateLink interface endpoints have per-endpoint alarms using dimension
// "VpcEndpointId". The endpoint ID is the res.ID; alarm cache is scanned.
func checkVPCEAlarm(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return alarmIDsByDimension(ctx, clients, cache, "", "VpcEndpointId", res.ID)
}

// checkVPCELogs reports CloudWatch Logs groups receiving VPC Flow Logs for
// this endpoint's network interfaces. Pattern C: one ec2:DescribeFlowLogs
// call filtered by resource-id; each flow log's LogGroupName, or its
// LogDestination when that is a log group ARN.
func checkVPCELogs(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	vpceID := res.ID
	if vpceID == "" {
		return resource.KnownRelated("logs", nil, false)
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
	var refs []string
	for _, fl := range out.FlowLogs {
		switch {
		case fl.LogGroupName != nil && *fl.LogGroupName != "":
			refs = append(refs, *fl.LogGroupName)
		case fl.LogDestination != nil:
			if _, isLogs := ARNForService(*fl.LogDestination, "logs"); isLogs {
				refs = append(refs, *fl.LogDestination)
			}
		}
	}
	return relatedRefs("logs", refs, refContext(clients, cache, "logs"))
}

// checkVPCER53 reports Route 53 private hosted zones associated with this VPC
// endpoint's VPC. Pattern C: one route53:ListHostedZonesByVPC call for the
// endpoint's VpcId.
func checkVPCER53(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	vpcID := res.Fields["vpc_id"]
	if vpcID == "" {
		if vpce, ok := assertStruct[ec2types.VpcEndpoint](res.RawStruct); ok && vpce.VpcId != nil {
			vpcID = *vpce.VpcId
		}
	}
	if vpcID == "" {
		return resource.KnownRelated("r53", nil, false)
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
		return resource.KnownRelated("r53", nil, false)
	}
	var ids []string
	for _, z := range out.HostedZoneSummaries {
		if z.HostedZoneId != nil {
			ids = append(ids, *z.HostedZoneId)
		}
	}
	return relatedRefs("r53", ids, refContext(clients, cache, "r53"))
}
