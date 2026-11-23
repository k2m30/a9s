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
	var failures []Failure
	attempted := 0
	walk := parentChildWalk{
		listParents: func(ctx context.Context, token *string) ([]string, *string, error) {
			out, err := listClustersAPI.ListClusters(ctx, &ecs.ListClustersInput{NextToken: token})
			if err != nil {
				return nil, nil, fmt.Errorf("listing ECS clusters: %w", err)
			}
			return out.ClusterArns, out.NextToken, nil
		},
		listOp: "ecs-svc: ListServices",
		listChildren: func(ctx context.Context, clusterArn string, token *string) ([]string, *string, error) {
			out, err := listServicesAPI.ListServices(ctx, &ecs.ListServicesInput{
				Cluster:    aws.String(clusterArn),
				MaxResults: aws.Int32(100),
				NextToken:  token,
			})
			if err != nil {
				return nil, nil, err
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
			attempted += len(serviceArns)
			var ans listedAnswer
			var services []ecstypes.Service
			if err == nil {
				services, ans.gone = descOutput.Services, ecsGone(descOutput.Failures)
			}
			for _, svc := range services {
				serviceName := ""
				if svc.ServiceName != nil {
					serviceName = *svc.ServiceName
				}

				clusterName, _ := ecsRefToID(aws.ToString(svc.ClusterArn), domain.RefContext{})

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
				ans.rows = append(ans.rows, r)
			}
			clusterName, _ := ecsRefToID(clusterArn, domain.RefContext{})
			// A listed service belongs to the cluster being walked, so its row ID
			// pairs that cluster with the name the ARN carries.
			nameOf := func(arn string) string {
				id, _ := ecsSvcRefToID(arn, domain.RefContext{})
				if _, name, qualified := strings.Cut(id, "/"); qualified {
					return name
				}
				return id
			}
			idOf := func(arn string) string { return ecsSvcID(clusterName, nameOf(arn)) }
			rows, batchFailures := keepListed("DescribeServices", serviceArns, idOf, ans, err, func(arn string, err error) resource.Resource {
				serviceName := nameOf(arn)
				r := DegradedDetails("ecs-svc", idOf(arn), err)
				r.Name = serviceName
				r.Fields["service_name"], r.Fields["cluster"], r.Fields["arn"] = serviceName, clusterName, arn
				return r
			})
			failures = append(failures, batchFailures...)
			return rows, nil
		},
		batch: 10,
	}
	result, err := walk.page(ctx, continuationToken)
	return result, JoinAggregates(err, AggregateFailures("ecs-svc: DescribeServices", failures, attempted))
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

// ecsSvcStatuses is the Service.status domain the Amazon ECS API Reference
// states ("The valid values are ACTIVE, DRAINING, or INACTIVE"); the SDK types
// it as a plain string.
// https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_Service.html
var ecsSvcStatuses = []string{"ACTIVE", "DRAINING", "INACTIVE"} //nolint:gochecknoglobals // static AWS status domain

func ecsSvcFindings(status string, desiredCount, runningCount int32) []domain.Finding {
	var posture []domain.Finding
	// A service that asks for nothing is idle by design, not short of capacity.
	switch {
	case desiredCount <= 0:
	case runningCount == 0:
		posture = append(posture, wave1Finding(CodeECSSvcNoTasksRunning))
	case runningCount < desiredCount:
		posture = append(posture, wave1Finding(CodeECSSvcTasksBelowDesired))
	}
	return withState("ecs-svc", status, posture)
}
