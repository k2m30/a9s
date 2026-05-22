package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

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
	input := &elbv2.DescribeLoadBalancersInput{
		PageSize: aws.Int32(DefaultPageSize),
	}
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

		var findings []domain.Finding
		switch state {
		case "provisioning":
			findings = []domain.Finding{{Code: CodeELBStateProvisioning, Phrase: "provisioning", Severity: domain.SevWarn, Source: "wave1"}}
		case "active_impaired":
			findings = []domain.Finding{{Code: CodeELBStateActiveImpaired, Phrase: "active impaired", Severity: domain.SevWarn, Source: "wave1"}}
		case "failed":
			findings = []domain.Finding{{Code: CodeELBStateFailed, Phrase: "failed", Severity: domain.SevBroken, Source: "wave1"}}
		}

		r := resource.Resource{
			ID:   lbName,
			Name: lbName,
			Fields: map[string]string{
				"name":              lbName,
				"dns_name":          dnsName,
				"type":              lbType,
				"scheme":            scheme,
				"state":             state,
				"vpc_id":            vpcID,
				"load_balancer_arn": lbArn,
			},
			Findings:  findings,
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
