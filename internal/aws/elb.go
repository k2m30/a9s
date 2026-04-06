package aws

import (
	"context"
	"fmt"

	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("elb", []string{"name", "dns_name", "type", "scheme", "state", "vpc_id", "load_balancer_arn"})

	resource.RegisterPaginated("elb", func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchLoadBalancersPage(ctx, c.ELBv2, continuationToken)
	})

	resource.RegisterNavigableFields("elb", []resource.NavigableField{
		{FieldPath: "VpcId", TargetType: "vpc"},
		{FieldPath: "SecurityGroups", TargetType: "sg"},
		{FieldPath: "AvailabilityZones.SubnetId", TargetType: "subnet"},
	})

	resource.RegisterRelated("elb", []resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: checkELBTargetGroups, NeedsTargetCache: true},
		{TargetType: "alarm", DisplayName: "CW Alarms", Checker: checkELBAlarms, NeedsTargetCache: true},
		{TargetType: "cfn", DisplayName: "CloudFormation", Checker: checkELBCFN, NeedsTargetCache: true},
		{TargetType: "r53", DisplayName: "Route 53 Records", Checker: checkELBR53, NeedsTargetCache: true},
	})
}

// FetchLoadBalancers calls the ELBv2 DescribeLoadBalancers API and converts the
// response into a slice of generic Resource structs.
func FetchLoadBalancers(ctx context.Context, api ELBv2DescribeLoadBalancersAPI) ([]resource.Resource, error) {
	var all []resource.Resource
	token := ""
	for {
		result, err := FetchLoadBalancersPage(ctx, api, token)
		if err != nil {
			return nil, err
		}
		all = append(all, result.Resources...)
		if result.Pagination == nil || !result.Pagination.IsTruncated {
			break
		}
		token = result.Pagination.NextToken
	}
	return all, nil
}

// FetchLoadBalancersPage fetches a single page of load balancers.
func FetchLoadBalancersPage(ctx context.Context, api ELBv2DescribeLoadBalancersAPI, continuationToken string) (resource.FetchResult, error) {
	input := &elbv2.DescribeLoadBalancersInput{}
	if continuationToken != "" {
		input.Marker = &continuationToken
	}

	output, err := api.DescribeLoadBalancers(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching load balancers: %w", err)
	}

	var resources []resource.Resource

	for _, lb := range output.LoadBalancers {
		lbName := ""
		if lb.LoadBalancerName != nil {
			lbName = *lb.LoadBalancerName
		}

		dnsName := ""
		if lb.DNSName != nil {
			dnsName = *lb.DNSName
		}

		lbType := string(lb.Type)
		scheme := string(lb.Scheme)

		state := ""
		if lb.State != nil {
			state = string(lb.State.Code)
		}

		vpcID := ""
		if lb.VpcId != nil {
			vpcID = *lb.VpcId
		}

		lbArn := ""
		if lb.LoadBalancerArn != nil {
			lbArn = *lb.LoadBalancerArn
		}

		r := resource.Resource{
			ID:     lbName,
			Name:   lbName,
			Status: state,
			Fields: map[string]string{
				"name":              lbName,
				"dns_name":          dnsName,
				"type":              lbType,
				"scheme":            scheme,
				"state":             state,
				"vpc_id":            vpcID,
				"load_balancer_arn": lbArn,
			},
			RawStruct: lb,
		}

		resources = append(resources, r)
	}

	nextToken := ""
	isTruncated := false
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
			PageSize:    len(resources),
			TotalHint:   totalHint,
		},
	}, nil
}
