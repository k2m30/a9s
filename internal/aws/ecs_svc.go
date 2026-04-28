package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("ecs-svc", []string{"service_name", "cluster", "status", "desired_count", "running_count", "launch_type", "task_definition"})

	resource.RegisterPaginated("ecs-svc", func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchECSServicesPage(ctx, c.ECS, c.ECS, c.ECS, continuationToken)
	})
}

// FetchECSServicesPage fetches one page of ECS clusters using the continuationToken,
// then for each cluster in that page fetches all services via ListServices+DescribeServices.
// IsTruncated reflects whether ListClusters has more pages beyond this one.
func FetchECSServicesPage(
	ctx context.Context,
	listClustersAPI ECSListClustersAPI,
	listServicesAPI ECSListServicesAPI,
	describeServicesAPI ECSDescribeServicesAPI,
	continuationToken string,
) (resource.FetchResult, error) {
	input := &ecs.ListClustersInput{}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	listOutput, err := listClustersAPI.ListClusters(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("listing ECS clusters: %w", err)
	}

	var resources []resource.Resource

	for _, clusterArn := range listOutput.ClusterArns {
		svcListOutput, err := listServicesAPI.ListServices(ctx, &ecs.ListServicesInput{
			Cluster: aws.String(clusterArn),
		})
		if err != nil {
			return resource.FetchResult{}, fmt.Errorf("listing ECS services: %w", err)
		}

		if len(svcListOutput.ServiceArns) == 0 {
			continue
		}

		descOutput, err := describeServicesAPI.DescribeServices(ctx, &ecs.DescribeServicesInput{
			Cluster:  aws.String(clusterArn),
			Services: svcListOutput.ServiceArns,
		})
		if err != nil {
			return resource.FetchResult{}, fmt.Errorf("describing ECS services: %w", err)
		}

		for _, svc := range descOutput.Services {
			serviceName := ""
			if svc.ServiceName != nil {
				serviceName = *svc.ServiceName
			}

			clusterName := ""
			if svc.ClusterArn != nil {
				arn := *svc.ClusterArn
				if idx := strings.LastIndex(arn, "/"); idx >= 0 {
					clusterName = arn[idx+1:]
				} else {
					clusterName = arn
				}
			}

			status := ""
			if svc.Status != nil {
				status = *svc.Status
			}

			desiredCount := fmt.Sprintf("%d", svc.DesiredCount)
			runningCount := fmt.Sprintf("%d", svc.RunningCount)
			launchType := string(svc.LaunchType)

			taskDefinition := ""
			if svc.TaskDefinition != nil {
				taskDefinition = *svc.TaskDefinition
			}

			// PR-03c: emit wave1 Findings for non-healthy lifecycle states.
			// ACTIVE → no Finding (healthy). Fields["status"] is still populated
			// so the existing structural Color path works as fallback.
			var findings []domain.Finding
			switch status {
			case "DRAINING":
				findings = []domain.Finding{{Code: CodeECSSvcStateDraining, Phrase: "draining", Severity: domain.SevWarn, Source: "wave1"}}
			case "INACTIVE":
				findings = []domain.Finding{{Code: CodeECSSvcStateInactive, Phrase: "inactive", Severity: domain.SevBroken, Source: "wave1"}}
			}

			r := resource.Resource{
				ID:   serviceName,
				Name: serviceName,
				Fields: map[string]string{
					"service_name":    serviceName,
					"cluster":         clusterName,
					"status":          status,
					"desired_count":   desiredCount,
					"running_count":   runningCount,
					"launch_type":     launchType,
					"task_definition": taskDefinition,
				},
				Findings:  findings,
				RawStruct: svc,
			}

			resources = append(resources, r)
		}
	}

	nextToken := ""
	isTruncated := false
	if listOutput.NextToken != nil {
		nextToken = *listOutput.NextToken
		isTruncated = true
	}

	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: isTruncated,
			NextToken:   nextToken,
			PageSize:    len(resources),
			TotalHint:   -1,
		},
	}, nil
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
	var allResources []resource.Resource
	continuationToken := ""

	for {
		result, err := FetchECSServicesPage(ctx, listClustersAPI, listServicesAPI, describeServicesAPI, continuationToken)
		if err != nil {
			return nil, err
		}

		allResources = append(allResources, result.Resources...)

		if result.Pagination == nil || !result.Pagination.IsTruncated {
			break
		}
		continuationToken = result.Pagination.NextToken
	}

	return allResources, nil
}
