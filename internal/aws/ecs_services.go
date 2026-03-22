package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.Register("ecs-svc", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchECSServices(ctx, c.ECS, c.ECS, c.ECS)
	})
	resource.RegisterFieldKeys("ecs-svc", []string{"service_name", "cluster", "status", "desired_count", "running_count", "launch_type"})
}

// FetchECSServices performs a three-step fetch:
// 1. ListClusters to get cluster ARNs
// 2. ListServices per cluster to get service ARNs
// 3. DescribeServices per cluster to get full details
func FetchECSServices(
	ctx context.Context,
	listClustersAPI ECSListClustersAPI,
	listServicesAPI ECSListServicesAPI,
	describeServicesAPI ECSDescribeServicesAPI,
) ([]resource.Resource, error) {
	listOutput, err := listClustersAPI.ListClusters(ctx, &ecs.ListClustersInput{})
	if err != nil {
		return nil, fmt.Errorf("listing ECS clusters: %w", err)
	}

	var resources []resource.Resource

	for _, clusterArn := range listOutput.ClusterArns {
		svcListOutput, err := listServicesAPI.ListServices(ctx, &ecs.ListServicesInput{
			Cluster: aws.String(clusterArn),
		})
		if err != nil {
			return nil, fmt.Errorf("listing ECS services: %w", err)
		}

		if len(svcListOutput.ServiceArns) == 0 {
			continue
		}

		descOutput, err := describeServicesAPI.DescribeServices(ctx, &ecs.DescribeServicesInput{
			Cluster:  aws.String(clusterArn),
			Services: svcListOutput.ServiceArns,
		})
		if err != nil {
			return nil, fmt.Errorf("describing ECS services: %w", err)
		}

		for _, svc := range descOutput.Services {
			serviceName := ""
			if svc.ServiceName != nil {
				serviceName = *svc.ServiceName
			}

			clusterName := ""
			if svc.ClusterArn != nil {
				clusterName = *svc.ClusterArn
			}

			status := ""
			if svc.Status != nil {
				status = *svc.Status
			}

			desiredCount := fmt.Sprintf("%d", svc.DesiredCount)
			runningCount := fmt.Sprintf("%d", svc.RunningCount)
			launchType := string(svc.LaunchType)

			r := resource.Resource{
				ID:     serviceName,
				Name:   serviceName,
				Status: status,
				Fields: map[string]string{
					"service_name":  serviceName,
					"cluster":       clusterName,
					"status":        status,
					"desired_count": desiredCount,
					"running_count": runningCount,
					"launch_type":   launchType,
				},
				RawStruct:  svc,
			}

			resources = append(resources, r)
		}
	}

	return resources, nil
}
