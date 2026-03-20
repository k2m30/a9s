package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/ec2"

	"github.com/k2m30/a9s/internal/resource"
)

func init() {
	resource.Register("vpce", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchVPCEndpoints(ctx, c.EC2)
	})
	resource.RegisterFieldKeys("vpce", []string{"vpce_id", "service_name", "type", "state", "vpc_id"})
}

// FetchVPCEndpoints calls the EC2 DescribeVpcEndpoints API and converts the
// response into a slice of generic Resource structs.
func FetchVPCEndpoints(ctx context.Context, api EC2DescribeVpcEndpointsAPI) ([]resource.Resource, error) {
	output, err := api.DescribeVpcEndpoints(ctx, &ec2.DescribeVpcEndpointsInput{})
	if err != nil {
		return nil, fmt.Errorf("fetching VPC endpoints: %w", err)
	}

	var resources []resource.Resource

	for _, vpce := range output.VpcEndpoints {
		vpceID := ""
		if vpce.VpcEndpointId != nil {
			vpceID = *vpce.VpcEndpointId
		}

		serviceName := ""
		if vpce.ServiceName != nil {
			serviceName = *vpce.ServiceName
		}

		endpointType := string(vpce.VpcEndpointType)
		state := string(vpce.State)

		vpcID := ""
		if vpce.VpcId != nil {
			vpcID = *vpce.VpcId
		}

		r := resource.Resource{
			ID:     vpceID,
			Name:   serviceName,
			Status: state,
			Fields: map[string]string{
				"vpce_id":      vpceID,
				"service_name": serviceName,
				"type":         endpointType,
				"state":        state,
				"vpc_id":       vpcID,
			},
			RawStruct:  vpce,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
