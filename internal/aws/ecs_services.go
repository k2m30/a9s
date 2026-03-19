package aws

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"

	"github.com/k2m30/a9s/internal/resource"
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
		return nil, err
	}

	var resources []resource.Resource

	for _, clusterArn := range listOutput.ClusterArns {
		svcListOutput, err := listServicesAPI.ListServices(ctx, &ecs.ListServicesInput{
			Cluster: aws.String(clusterArn),
		})
		if err != nil {
			return nil, err
		}

		if len(svcListOutput.ServiceArns) == 0 {
			continue
		}

		descOutput, err := describeServicesAPI.DescribeServices(ctx, &ecs.DescribeServicesInput{
			Cluster:  aws.String(clusterArn),
			Services: svcListOutput.ServiceArns,
		})
		if err != nil {
			return nil, err
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

			detail := map[string]string{
				"Service Name":  serviceName,
				"Cluster":       clusterName,
				"Status":        status,
				"Desired Count": desiredCount,
				"Running Count": runningCount,
				"Launch Type":   launchType,
			}

			if svc.ServiceArn != nil {
				detail["ARN"] = *svc.ServiceArn
			}

			if svc.TaskDefinition != nil {
				detail["Task Definition"] = *svc.TaskDefinition
			}

			if svc.RoleArn != nil {
				detail["Role ARN"] = *svc.RoleArn
			}

			if svc.CreatedAt != nil {
				detail["Created At"] = svc.CreatedAt.Format("2006-01-02T15:04:05Z07:00")
			}

			rawJSON := ""
			if jsonBytes, err := json.MarshalIndent(svc, "", "  "); err == nil {
				rawJSON = string(jsonBytes)
			}

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
				DetailData: detail,
				RawJSON:    rawJSON,
				RawStruct:  svc,
			}

			resources = append(resources, r)
		}
	}

	return resources, nil
}
