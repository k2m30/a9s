// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"

	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

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

		findings := elbStateFindings(state)

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

// elbStateFindings is the one predicate for a load balancer's state. active
// reports nothing. colorELB runs it over Fields for rows built outside the
// fetcher.
func elbStateFindings(state string) []domain.Finding {
	switch state {
	case "provisioning":
		return []domain.Finding{{Code: CodeELBStateProvisioning, Phrase: "provisioning", Detail: catalog.Detail(CodeELBStateProvisioning), Severity: domain.SevWarn, Source: "wave1"}}
	case "active_impaired":
		return []domain.Finding{{Code: CodeELBStateActiveImpaired, Phrase: "active impaired", Detail: catalog.Detail(CodeELBStateActiveImpaired), Severity: domain.SevWarn, Source: "wave1"}}
	case "failed":
		return []domain.Finding{{Code: CodeELBStateFailed, Phrase: "failed", Detail: catalog.Detail(CodeELBStateFailed), Severity: domain.SevBroken, Source: "wave1"}}
	}
	return nil
}
