package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/ecs"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.Register("ecs", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchECSClusters(ctx, c.ECS, c.ECS)
	})
	resource.RegisterFieldKeys("ecs", []string{"cluster_name", "status", "running_tasks", "pending_tasks", "services_count"})
}

// FetchECSClusters performs a two-step fetch: ListClusters to get ARNs,
// then DescribeClusters for full details.
func FetchECSClusters(ctx context.Context, listAPI ECSListClustersAPI, describeAPI ECSDescribeClustersAPI) ([]resource.Resource, error) {
	listOutput, err := listAPI.ListClusters(ctx, &ecs.ListClustersInput{})
	if err != nil {
		return nil, fmt.Errorf("listing ECS clusters: %w", err)
	}

	if len(listOutput.ClusterArns) == 0 {
		return nil, nil
	}

	descOutput, err := describeAPI.DescribeClusters(ctx, &ecs.DescribeClustersInput{
		Clusters: listOutput.ClusterArns,
	})
	if err != nil {
		return nil, fmt.Errorf("describing ECS clusters: %w", err)
	}

	var resources []resource.Resource

	for _, cluster := range descOutput.Clusters {
		clusterName := ""
		if cluster.ClusterName != nil {
			clusterName = *cluster.ClusterName
		}

		status := ""
		if cluster.Status != nil {
			status = *cluster.Status
		}

		runningTasks := fmt.Sprintf("%d", cluster.RunningTasksCount)
		pendingTasks := fmt.Sprintf("%d", cluster.PendingTasksCount)
		servicesCount := fmt.Sprintf("%d", cluster.ActiveServicesCount)

		r := resource.Resource{
			ID:     clusterName,
			Name:   clusterName,
			Status: status,
			Fields: map[string]string{
				"cluster_name":   clusterName,
				"status":         status,
				"running_tasks":  runningTasks,
				"pending_tasks":  pendingTasks,
				"services_count": servicesCount,
			},
			RawStruct:  cluster,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
