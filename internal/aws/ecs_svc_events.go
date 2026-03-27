package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("ecs_svc_events", []string{"timestamp", "message"})

	resource.RegisterPaginatedChild("ecs_svc_events", func(ctx context.Context, clients interface{}, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchEcsSvcEvents(ctx, c.ECS, parentCtx["cluster"], parentCtx["service_name"], continuationToken)
	})

	resource.RegisterChildType(resource.ResourceTypeDef{
		Name:      "Service Events",
		ShortName: "ecs_svc_events",
		Columns:   resource.EcsSvcEventColumns(),
	})
}

// FetchEcsSvcEvents calls the ECS DescribeServices API and extracts the
// Events list from the service. At most 100 events are returned (the API
// default maximum), so no pagination is needed. Uses FetchResult for consistency.
func FetchEcsSvcEvents(
	ctx context.Context,
	api ECSDescribeServicesAPI,
	cluster, serviceName string,
	continuationToken string,
) (resource.FetchResult, error) {
	output, err := api.DescribeServices(ctx, &ecs.DescribeServicesInput{
		Cluster:  aws.String(cluster),
		Services: []string{serviceName},
	})
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("describing ECS service events for %s: %w", serviceName, err)
	}

	if len(output.Services) == 0 {
		return resource.FetchResult{
			Pagination: &resource.PaginationMeta{IsTruncated: false},
		}, nil
	}

	svc := output.Services[0]
	var resources []resource.Resource

	for _, event := range svc.Events {
		id := ""
		if event.Id != nil {
			id = *event.Id
		}

		timestamp := ""
		name := ""
		if event.CreatedAt != nil {
			timestamp = event.CreatedAt.UTC().Format("2006-01-02 15:04:05")
			name = event.CreatedAt.UTC().Format("2006-01-02 15:04:05")
		}

		message := ""
		if event.Message != nil {
			message = strings.ReplaceAll(*event.Message, "\n", " ")
		}

		r := resource.Resource{
			ID:   id,
			Name: name,
			Fields: map[string]string{
				"timestamp": timestamp,
				"message":   message,
			},
			RawStruct: event,
		}

		resources = append(resources, r)
	}

	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: false,
			TotalHint:   len(resources),
			PageSize:    len(resources),
		},
	}, nil
}
