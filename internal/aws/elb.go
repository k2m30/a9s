package aws

import (
	"context"
	"encoding/json"
	"fmt"

	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"

	"github.com/k2m30/a9s/internal/resource"
)

func init() {
	resource.Register("elb", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchLoadBalancers(ctx, c.ELBv2)
	})
}

// FetchLoadBalancers calls the ELBv2 DescribeLoadBalancers API and converts the
// response into a slice of generic Resource structs.
func FetchLoadBalancers(ctx context.Context, api ELBv2DescribeLoadBalancersAPI) ([]resource.Resource, error) {
	output, err := api.DescribeLoadBalancers(ctx, &elbv2.DescribeLoadBalancersInput{})
	if err != nil {
		return nil, err
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

		detail := map[string]string{
			"Name":     lbName,
			"DNS Name": dnsName,
			"Type":     lbType,
			"Scheme":   scheme,
			"State":    state,
			"VPC ID":   vpcID,
		}

		if lb.LoadBalancerArn != nil {
			detail["ARN"] = *lb.LoadBalancerArn
		}

		if lb.CanonicalHostedZoneId != nil {
			detail["Hosted Zone ID"] = *lb.CanonicalHostedZoneId
		}

		if lb.CreatedTime != nil {
			detail["Created Time"] = lb.CreatedTime.Format("2006-01-02T15:04:05Z07:00")
		}

		detail["IP Address Type"] = string(lb.IpAddressType)

		rawJSON := ""
		if jsonBytes, err := json.MarshalIndent(lb, "", "  "); err == nil {
			rawJSON = string(jsonBytes)
		}

		r := resource.Resource{
			ID:     lbName,
			Name:   lbName,
			Status: state,
			Fields: map[string]string{
				"name":     lbName,
				"dns_name": dnsName,
				"type":     lbType,
				"scheme":   scheme,
				"state":    state,
				"vpc_id":   vpcID,
			},
			DetailData: detail,
			RawJSON:    rawJSON,
			RawStruct:  lb,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
