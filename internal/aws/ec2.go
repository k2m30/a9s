package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("ec2", []string{"instance_id", "name", "state", "type", "private_ip", "public_ip", "launch_time", "lifecycle", "image_id", "vpc_id", "system_status", "instance_status"})

	resource.RegisterFieldAliases("ec2", map[string]string{
		"instance_id":  "InstanceId",
		"type":         "InstanceType",
		"state":        "State",
		"lifecycle":    "InstanceLifecycle",
		"image_id":     "ImageId",
		"key_name":     "KeyName",
		"vpc_id":       "VpcId",
		"subnet_id":    "SubnetId",
		"private_ip":   "PrivateIpAddress",
		"private_dns":  "PrivateDnsName",
		"public_ip":    "PublicIpAddress",
		"iam_profile":  "IamInstanceProfile",
		"architecture": "Architecture",
		"platform":     "Platform",
		"launch_time":  "LaunchTime",
	})

	resource.RegisterPaginated("ec2", func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchEC2InstancesPage(ctx, c.EC2, continuationToken)
	})

	resource.RegisterRelated("ec2", []resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: checkEC2TargetGroups, NeedsTargetCache: true},
		{TargetType: "asg", DisplayName: "Auto Scaling Groups", Checker: checkEC2ASG, NeedsTargetCache: true},
		{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: checkEC2Alarms, NeedsTargetCache: true},
		{TargetType: "ng", DisplayName: "EKS Node Groups", Checker: checkEC2NodeGroups, NeedsTargetCache: true},
		{TargetType: "cfn", DisplayName: "CloudFormation Stacks", Checker: checkEC2CFN, NeedsTargetCache: true},
		{TargetType: "eip", DisplayName: "Elastic IPs", Checker: checkEC2EIP, NeedsTargetCache: true},
		{TargetType: "ebs", DisplayName: "EBS Volumes", Checker: checkEC2EBS},
		{TargetType: "ebs-snap", DisplayName: "EBS Snapshots", Checker: checkEC2EBSSnap, NeedsTargetCache: true},
		{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: checkEC2CloudTrailEvents, NeedsTargetCache: true},
		{TargetType: "sg", DisplayName: "Security Groups", Checker: checkEC2SG},
		{TargetType: "vpc", DisplayName: "VPC", Checker: checkEC2VPC},
		{TargetType: "role", DisplayName: "IAM Role", Checker: checkEC2Role},
		{TargetType: "kms", DisplayName: "KMS Key", Checker: checkEC2KMS},
		{TargetType: "ami", DisplayName: "AMIs", Checker: checkEC2AMI},
		{TargetType: "backup", DisplayName: "AWS Backups", Checker: checkEC2Backup},
		{TargetType: "eni", DisplayName: "Network Interfaces", Checker: checkEC2ENI},
		{TargetType: "logs", DisplayName: "Log Groups", Checker: checkEC2Logs},
		{TargetType: "ssm", DisplayName: "SSM Parameters", Checker: checkEC2SSM},
		{TargetType: "subnet", DisplayName: "Subnets", Checker: checkEC2Subnet},
	})

	resource.RegisterNavigableFields("ec2", []resource.NavigableField{
		{FieldPath: "VpcId", TargetType: "vpc"},
		{FieldPath: "SubnetId", TargetType: "subnet"},
		{FieldPath: "ImageId", TargetType: "ami"},
		{FieldPath: "BlockDeviceMappings.Ebs.VolumeId", TargetType: "ebs"},
		{FieldPath: "SecurityGroups.GroupId", TargetType: "sg"},
		{FieldPath: "NetworkInterfaces.NetworkInterfaceId", TargetType: "eni"},
		{FieldPath: "IamInstanceProfile.Arn", TargetType: "role"},
	})
}

// FetchEC2Instances calls the EC2 DescribeInstances API and returns all pages
// of instances. Used by existing tests and the legacy fetcher.
func FetchEC2Instances(ctx context.Context, api EC2FetchInstancesAPI) ([]resource.Resource, error) {
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
func FetchEC2InstancesPage(ctx context.Context, api EC2FetchInstancesAPI, continuationToken string) (resource.FetchResult, error) {
	input := &ec2.DescribeInstancesInput{
		MaxResults: aws.Int32(DefaultPageSize),
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

	// Enrich with status check data (graceful degradation on error).
	if len(resources) > 0 {
		enrichEC2StatusChecks(ctx, api, resources)
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

// enrichEC2StatusChecks calls DescribeInstanceStatus for the page's resources
// and merges system_status/instance_status into each resource's Fields map.
// Errors are silently ignored (graceful degradation per design spec).
func enrichEC2StatusChecks(ctx context.Context, api EC2DescribeInstanceStatusAPI, resources []resource.Resource) {
	// Collect instance IDs.
	ids := make([]string, 0, len(resources))
	for _, r := range resources {
		if r.ID != "" {
			ids = append(ids, r.ID)
		}
	}
	if len(ids) == 0 {
		return
	}

	// Build a map from instance ID to (systemStatus, instanceStatus).
	statusMap := make(map[string][2]string, len(ids))

	// DescribeInstanceStatus accepts max 100 IDs per call.
	const batchSize = 100
	for start := 0; start < len(ids); start += batchSize {
		end := min(start+batchSize, len(ids))
		batch := ids[start:end]

		out, err := api.DescribeInstanceStatus(ctx, &ec2.DescribeInstanceStatusInput{
			InstanceIds:         batch,
			IncludeAllInstances: aws.Bool(true),
		})
		if err != nil {
			// Non-fatal: skip enrichment for this batch.
			continue
		}

		for _, s := range out.InstanceStatuses {
			if s.InstanceId == nil {
				continue
			}
			sysStatus := ""
			instStatus := ""
			if s.SystemStatus != nil {
				sysStatus = string(s.SystemStatus.Status)
			}
			if s.InstanceStatus != nil {
				instStatus = string(s.InstanceStatus.Status)
			}
			statusMap[*s.InstanceId] = [2]string{sysStatus, instStatus}
		}
	}

	// Merge status fields into resources and promote Status for running instances.
	for i, r := range resources {
		if pair, ok := statusMap[r.ID]; ok {
			if resources[i].Fields == nil {
				resources[i].Fields = make(map[string]string)
			}
			sysStatus := pair[0]
			instStatus := pair[1]
			if sysStatus != "" {
				resources[i].Fields["system_status"] = sysStatus
			}
			if instStatus != "" {
				resources[i].Fields["instance_status"] = instStatus
			}
			// Promote Resource.Status for running instances only.
			// A running instance whose status checks report "impaired" or
			// "initializing" must surface as an issue in the menu badge and
			// ctrl+z filter. We promote Status so the Color func's fallback path
			// (via r.Status) also works for test doubles without a Color func.
			if resources[i].Fields["state"] == "running" {
				if sysStatus == "impaired" || instStatus == "impaired" {
					resources[i].Status = "impaired"
				} else if sysStatus == "initializing" || instStatus == "initializing" {
					if resources[i].Status != "impaired" { // impaired has priority
						resources[i].Status = "initializing"
					}
				}
			}
		}
	}
}
