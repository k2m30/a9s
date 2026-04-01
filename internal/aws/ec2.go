package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("ec2", []string{"instance_id", "name", "state", "type", "private_ip", "public_ip", "launch_time", "lifecycle"})

	resource.RegisterPaginated("ec2", func(ctx context.Context, clients interface{}, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchEC2InstancesPage(ctx, c.EC2, continuationToken)
	})

	resource.RegisterRelated("ec2", []resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: checkEC2TargetGroups},
		{TargetType: "asg", DisplayName: "Auto Scaling Groups", Checker: checkEC2ASG},
		{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: checkEC2Alarms},
		{TargetType: "cfn", DisplayName: "CloudFormation Stacks", Checker: checkEC2CFN},
	})

	resource.RegisterNavigableFields("ec2", []resource.NavigableField{
		{FieldPath: "VpcId", TargetType: "vpc"},
		{FieldPath: "SubnetId", TargetType: "subnet"},
		{FieldPath: "ImageId", TargetType: "ami"},
	})
}

// FetchEC2Instances calls the EC2 DescribeInstances API and returns all pages
// of instances. Used by existing tests and the legacy fetcher.
func FetchEC2Instances(ctx context.Context, api EC2DescribeInstancesAPI) ([]resource.Resource, error) {
	var all []resource.Resource
	token := ""
	for {
		result, err := FetchEC2InstancesPage(ctx, api, token)
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

// FetchEC2InstancesPage calls the EC2 DescribeInstances API and returns
// a single page of instances. Pass an empty continuationToken for the first page.
func FetchEC2InstancesPage(ctx context.Context, api EC2DescribeInstancesAPI, continuationToken string) (resource.FetchResult, error) {
	input := &ec2.DescribeInstancesInput{
		MaxResults: aws.Int32(200),
	}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	output, err := api.DescribeInstances(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching EC2 instances: %w", err)
	}

	var resources []resource.Resource
	for _, reservation := range output.Reservations {
		for _, inst := range reservation.Instances {
			// Extract instance ID
			instanceID := ""
			if inst.InstanceId != nil {
				instanceID = *inst.InstanceId
			}

			// Extract Name tag
			name := ""
			for _, tag := range inst.Tags {
				if tag.Key != nil && *tag.Key == "Name" {
					if tag.Value != nil {
						name = *tag.Value
					}
					break
				}
			}

			// Extract state
			state := string(inst.State.Name)

			// Extract instance type
			instanceType := string(inst.InstanceType)

			// Extract private IP
			privateIP := ""
			if inst.PrivateIpAddress != nil {
				privateIP = *inst.PrivateIpAddress
			}

			// Extract public IP (may be nil)
			publicIP := ""
			if inst.PublicIpAddress != nil {
				publicIP = *inst.PublicIpAddress
			}

			// Format launch time
			launchTime := ""
			if inst.LaunchTime != nil {
				launchTime = inst.LaunchTime.Format("2006-01-02 15:04")
			}

			// Extract lifecycle (on-demand if empty)
			lifecycle := "on-demand"
			if inst.InstanceLifecycle != "" {
				lifecycle = string(inst.InstanceLifecycle)
			}

			r := resource.Resource{
				ID:     instanceID,
				Name:   name,
				Status: state,
				Fields: map[string]string{
					"instance_id": instanceID,
					"name":        name,
					"state":       state,
					"type":        instanceType,
					"private_ip":  privateIP,
					"public_ip":   publicIP,
					"launch_time": launchTime,
					"lifecycle":   lifecycle,
				},
				RawStruct: inst,
			}

			resources = append(resources, r)
		}
	}

	// Build pagination metadata
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

// checkEC2TargetGroups checks the cache for target groups referencing this EC2 instance.
func checkEC2TargetGroups(_ context.Context, _ interface{}, _ resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	// Check cache for "tg" resources — real matching logic comes in per-resource issues
	if tgs, ok := cache["tg"]; ok {
		_ = tgs // placeholder: iterate and match by instance ID
	}
	return resource.RelatedCheckResult{TargetType: "tg", Count: -1}
}

// checkEC2ASG checks the cache for ASGs containing this EC2 instance.
func checkEC2ASG(_ context.Context, _ interface{}, _ resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	if asgs, ok := cache["asg"]; ok {
		_ = asgs
	}
	return resource.RelatedCheckResult{TargetType: "asg", Count: -1}
}

// checkEC2Alarms checks the cache for CloudWatch alarms targeting this EC2 instance.
func checkEC2Alarms(_ context.Context, _ interface{}, _ resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	if alarms, ok := cache["alarm"]; ok {
		_ = alarms
	}
	return resource.RelatedCheckResult{TargetType: "alarm", Count: -1}
}

// checkEC2CFN checks instance tags for aws:cloudformation:stack-name.
func checkEC2CFN(_ context.Context, _ interface{}, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	// Check for CFN tags on the instance
	return resource.RelatedCheckResult{TargetType: "cfn", Count: -1}
}

