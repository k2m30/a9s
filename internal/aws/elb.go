package aws

import (
	"context"
	"fmt"

	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.Register("elb", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchLoadBalancers(ctx, c.ELBv2)
	})
	resource.RegisterFieldKeys("elb", []string{"name", "dns_name", "type", "scheme", "state", "vpc_id"})
}

// FetchLoadBalancers calls the ELBv2 DescribeLoadBalancers API and converts the
// response into a slice of generic Resource structs.
func FetchLoadBalancers(ctx context.Context, api ELBv2DescribeLoadBalancersAPI) ([]resource.Resource, error) {
	output, err := api.DescribeLoadBalancers(ctx, &elbv2.DescribeLoadBalancersInput{})
	if err != nil {
		return nil, fmt.Errorf("fetching load balancers: %w", err)
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
			RawStruct:  lb,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
