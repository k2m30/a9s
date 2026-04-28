package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	autoscalingtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("asg", []string{
		"asg_name", "min_size", "max_size", "desired", "instances", "status",
		"instances_unhealthy_count", "in_service_count", "suspended_processes",
	})

	resource.RegisterPaginated("asg", func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchAutoScalingGroupsPage(ctx, c.AutoScaling, continuationToken)
	})

	resource.RegisterRelated("asg", []resource.RelatedDef{
		{TargetType: "ec2", DisplayName: "EC2 Instances", Checker: checkASGEC2},
		{TargetType: "tg", DisplayName: "Target Groups", Checker: checkASGTG},
		{TargetType: "subnet", DisplayName: "Subnets", Checker: checkASGSubnets},
		{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: checkASGAlarm, NeedsTargetCache: true},
		{TargetType: "ng", DisplayName: "EKS Node Groups", Checker: checkASGNG, NeedsTargetCache: true},
		{TargetType: "ami", DisplayName: "AMI", Checker: checkASGAMI, NeedsTargetCache: false},
		{TargetType: "elb", DisplayName: "Load Balancers", Checker: checkASGELB, NeedsTargetCache: false},
		{TargetType: "role", DisplayName: "IAM Roles", Checker: checkASGRole, NeedsTargetCache: false},
		{TargetType: "sg", DisplayName: "Security Groups", Checker: checkASGSG, NeedsTargetCache: false},
		{TargetType: "sns", DisplayName: "SNS Topics", Checker: checkASGSNS, NeedsTargetCache: false},
		{TargetType: "vpc", DisplayName: "VPCs", Checker: checkASGVPC, NeedsTargetCache: false},
	})

	// autoscalingtypes.Group: TargetGroupARNs[] — list of TG ARNs; VPCZoneIdentifier — CSV subnet IDs
	resource.RegisterDefaultNavFields("asg", []resource.NavigableField{
		{FieldPath: "TargetGroupARNs", TargetType: "tg"},
		{FieldPath: "VPCZoneIdentifier", TargetType: "subnet"},
	})
}

// FetchAutoScalingGroups calls the AutoScaling DescribeAutoScalingGroups API and converts the
// response into a slice of generic Resource structs.
func FetchAutoScalingGroups(ctx context.Context, api ASGDescribeAutoScalingGroupsAPI) ([]resource.Resource, error) {
	var all []resource.Resource
	token := ""
	for {
		result, err := FetchAutoScalingGroupsPage(ctx, api, token)
		if err != nil {
			return nil, err
		}
		all = append(all, result.Resources...)
		if result.Pagination == nil || !result.Pagination.IsTruncated {
			break
		}
		token = result.Pagination.NextToken
	}
	return all, nil
}

// FetchAutoScalingGroupsPage fetches a single page of Auto Scaling groups.
func FetchAutoScalingGroupsPage(ctx context.Context, api ASGDescribeAutoScalingGroupsAPI, continuationToken string) (resource.FetchResult, error) {
	input := &autoscaling.DescribeAutoScalingGroupsInput{
		MaxRecords: aws.Int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	output, err := api.DescribeAutoScalingGroups(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching Auto Scaling groups: %w", err)
	}

	var resources []resource.Resource

	for _, asg := range output.AutoScalingGroups {
		asgName := ""
		if asg.AutoScalingGroupName != nil {
			asgName = *asg.AutoScalingGroupName
		}

		minSize := ""
		if asg.MinSize != nil {
			minSize = fmt.Sprintf("%d", *asg.MinSize)
		}

		maxSize := ""
		if asg.MaxSize != nil {
			maxSize = fmt.Sprintf("%d", *asg.MaxSize)
		}

		desired := ""
		if asg.DesiredCapacity != nil {
			desired = fmt.Sprintf("%d", *asg.DesiredCapacity)
		}

		instances := fmt.Sprintf("%d", len(asg.Instances))

		status := ""
		if asg.Status != nil {
			status = *asg.Status
		}

		unhealthyCount := 0
		inServiceCount := 0
		for _, inst := range asg.Instances {
			if inst.HealthStatus != nil && *inst.HealthStatus == "Unhealthy" {
				unhealthyCount++
			}
			if inst.LifecycleState == autoscalingtypes.LifecycleStateInService {
				inServiceCount++
			}
		}

		var suspendedNames []string
		for _, sp := range asg.SuspendedProcesses {
			if sp.ProcessName != nil {
				suspendedNames = append(suspendedNames, *sp.ProcessName)
			}
		}
		suspendedProcesses := strings.Join(suspendedNames, ",")

		r := resource.Resource{
			ID:   asgName,
			Name: asgName,
			// Status: removed — PR-03b migrates fetcher to Findings for lifecycle states.
			Fields: map[string]string{
				"asg_name":                  asgName,
				"min_size":                  minSize,
				"max_size":                  maxSize,
				"desired":                   desired,
				"instances":                 instances,
				"status":                    status,
				"instances_unhealthy_count": fmt.Sprintf("%d", unhealthyCount),
				"in_service_count":          fmt.Sprintf("%d", inServiceCount),
				"suspended_processes":       suspendedProcesses,
			},
			RawStruct: asg,
		}

		// Phase 03 PR-03b: emit canonical Findings for "Delete in progress".
		// Empty status → healthy (no Finding). Structural signals (unhealthy
		// instance count, in_service < min) are handled by the Color func.
		if status == "Delete in progress" {
			r.Findings = []domain.Finding{{
				Code: CodeASGStateDeleting, Phrase: "delete in progress",
				Severity: domain.SevWarn, Source: "wave1",
			}}
		}

		resources = append(resources, r)
	}

	nextToken := ""
	isTruncated := false
	if output.NextToken != nil {
		nextToken = *output.NextToken
		isTruncated = true
	}

	totalHint := len(resources)
	if isTruncated {
		totalHint = -1
	}

	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: isTruncated,
			NextToken:   nextToken,
			PageSize:    len(resources),
			TotalHint:   totalHint,
		},
	}, nil
}
