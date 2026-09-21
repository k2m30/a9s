// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"cmp"
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// FetchECSServicesPage fetches one page of ECS services: ListClusters, every
// ListServices page of each cluster, and DescribeServices for at most 10
// services a call, through parentChildWalk.
func FetchECSServicesPage(
	ctx context.Context,
	listClustersAPI ECSListClustersAPI,
	listServicesAPI ECSListServicesAPI,
	describeServicesAPI ECSDescribeServicesAPI,
	continuationToken string,
) (resource.FetchResult, error) {
	walk := parentChildWalk{
		listParents: func(ctx context.Context, token *string) ([]string, *string, error) {
			out, err := listClustersAPI.ListClusters(ctx, &ecs.ListClustersInput{NextToken: token})
			if err != nil {
				return nil, nil, fmt.Errorf("listing ECS clusters: %w", err)
			}
			return out.ClusterArns, out.NextToken, nil
		},
		listChildren: func(ctx context.Context, clusterArn string, token *string) ([]string, *string, error) {
			out, err := listServicesAPI.ListServices(ctx, &ecs.ListServicesInput{
				Cluster:    aws.String(clusterArn),
				MaxResults: aws.Int32(100),
				NextToken:  token,
			})
			if err != nil {
				return nil, nil, fmt.Errorf("listing ECS services: %w", err)
			}
			return out.ServiceArns, out.NextToken, nil
		},
		describe: func(ctx context.Context, clusterArn string, serviceArns []string) ([]resource.Resource, error) {
			// Tags are returned only when named in Include; the CloudFormation
			// pivot matches on the stack-name tag.
			descOutput, err := describeServicesAPI.DescribeServices(ctx, &ecs.DescribeServicesInput{
				Cluster:  aws.String(clusterArn),
				Services: serviceArns,
				Include:  []ecstypes.ServiceField{ecstypes.ServiceFieldTags},
			})
			if err != nil {
				return nil, fmt.Errorf("describing ECS services: %w", err)
			}
			resources := make([]resource.Resource, 0, len(descOutput.Services))
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

				arn := ""
				if svc.ServiceArn != nil {
					arn = *svc.ServiceArn
				}

				// Fields["status"] stays populated for the structural Color fallback.
				findings := ecsSvcFindings(status, svc.DesiredCount, svc.RunningCount)

				r := resource.Resource{
					ID:   ecsSvcID(clusterName, serviceName),
					Name: serviceName,
					Fields: map[string]string{
						"service_name":    serviceName,
						"cluster":         clusterName,
						"status":          status,
						"desired_count":   desiredCount,
						"running_count":   runningCount,
						"launch_type":     launchType,
						"task_definition": taskDefinition,
						"arn":             arn,
					},
					Findings:  findings,
					RawStruct: svc,
				}

				resources = append(resources, r)
			}
			return resources, nil
		},
		batch: 10,
	}
	return walk.page(ctx, continuationToken)
}

// ecsSvcID is the row identity of an ECS service: AWS scopes a service name
// to its cluster, so two clusters may each run one called "api".
func ecsSvcID(cluster, serviceName string) string {
	return cluster + "/" + serviceName
}

// ecsSvcName is a service row's own name, as every AWS API that takes a
// service takes it.
func ecsSvcName(res resource.Resource) string {
	return cmp.Or(res.Fields["service_name"], res.Name)
}

// ecsSvcFindings is the one predicate for a service's health: its lifecycle
// state, then whether it is running the tasks it asks for. colorECSSvc runs it
// over Fields for rows built outside the fetcher.
func ecsSvcFindings(status string, desiredCount, runningCount int32) []domain.Finding {
	var findings []domain.Finding
	switch status {
	case "DRAINING":
		findings = append(findings, wave1Finding(CodeECSSvcStateDraining))
	case "INACTIVE":
		findings = append(findings, wave1Finding(CodeECSSvcStateInactive))
	}
	// A service that asks for nothing is idle by design, not short of capacity.
	switch {
	case desiredCount <= 0:
	case runningCount == 0:
		findings = append(findings, wave1Finding(CodeECSSvcNoTasksRunning))
	case runningCount < desiredCount:
		findings = append(findings, wave1Finding(CodeECSSvcTasksBelowDesired))
	}
	return findings
}
