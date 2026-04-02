package aws

import (
	"context"
	"fmt"
	"sort"

	"github.com/aws/aws-sdk-go-v2/aws"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

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
		{TargetType: "eip", DisplayName: "Elastic IPs", Checker: checkEC2EIP},
		{TargetType: "ebs-snap", DisplayName: "EBS Snapshots", Checker: checkEC2EBSSnap},
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
func checkEC2TargetGroups(_ context.Context, _ interface{}, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	instanceID, vpcID, _ := ec2Identity(res)
	if instanceID == "" || vpcID == "" {
		return resource.RelatedCheckResult{TargetType: "tg", Count: 0}
	}
	tgList, ok := cache["tg"]
	if !ok {
		return resource.RelatedCheckResult{TargetType: "tg", Count: -1}
	}
	var ids []string
	for _, tgRes := range tgList {
		raw, ok := tgRes.RawStruct.(elbv2types.TargetGroup)
		if !ok {
			if p, ok := tgRes.RawStruct.(*elbv2types.TargetGroup); ok && p != nil {
				raw = *p
				ok = true
			}
		}
		targetType := tgRes.Fields["target_type"]
		tgVpcID := tgRes.Fields["vpc_id"]
		if ok {
			targetType = string(raw.TargetType)
			if raw.VpcId != nil {
				tgVpcID = *raw.VpcId
			}
		}
		if targetType != "instance" {
			continue
		}
		// Without target-health rows in cache, best available approximation is
		// VPC-level matching for instance target groups.
		if tgVpcID == vpcID {
			ids = append(ids, tgRes.ID)
		}
	}
	return relatedResult("tg", ids)
}

// checkEC2ASG checks the cache for ASGs containing this EC2 instance.
func checkEC2ASG(_ context.Context, _ interface{}, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	instanceID, _, _ := ec2Identity(res)
	if instanceID == "" {
		return resource.RelatedCheckResult{TargetType: "asg", Count: 0}
	}
	asgList, ok := cache["asg"]
	if !ok {
		return resource.RelatedCheckResult{TargetType: "asg", Count: -1}
	}
	var ids []string
	for _, asgRes := range asgList {
		raw, ok := asgRes.RawStruct.(asgtypes.AutoScalingGroup)
		if !ok {
			if p, ok := asgRes.RawStruct.(*asgtypes.AutoScalingGroup); ok && p != nil {
				raw = *p
				ok = true
			}
		}
		if !ok {
			continue
		}
		for _, inst := range raw.Instances {
			if inst.InstanceId != nil && *inst.InstanceId == instanceID {
				ids = append(ids, asgRes.ID)
				break
			}
		}
	}
	return relatedResult("asg", ids)
}

// checkEC2Alarms checks the cache for CloudWatch alarms targeting this EC2 instance.
func checkEC2Alarms(_ context.Context, _ interface{}, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	instanceID, _, _ := ec2Identity(res)
	if instanceID == "" {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: 0}
	}
	alarmList, ok := cache["alarm"]
	if !ok {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1}
	}
	var ids []string
	for _, alarmRes := range alarmList {
		raw, ok := alarmRes.RawStruct.(cwtypes.MetricAlarm)
		if !ok {
			if p, ok := alarmRes.RawStruct.(*cwtypes.MetricAlarm); ok && p != nil {
				raw = *p
				ok = true
			}
		}
		if !ok {
			continue
		}
		for _, d := range raw.Dimensions {
			if d.Value != nil && *d.Value == instanceID {
				ids = append(ids, alarmRes.ID)
				break
			}
		}
	}
	return relatedResult("alarm", ids)
}

// checkEC2CFN checks instance tags for aws:cloudformation:stack-name.
func checkEC2CFN(_ context.Context, _ interface{}, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	_, _, stackName := ec2Identity(res)
	if stackName == "" {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: 0}
	}
	cfnList, ok := cache["cfn"]
	if !ok {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1}
	}
	var ids []string
	for _, cfnRes := range cfnList {
		if cfnRes.ID == stackName || cfnRes.Name == stackName || cfnRes.Fields["stack_name"] == stackName {
			ids = append(ids, cfnRes.ID)
			continue
		}
		raw, ok := cfnRes.RawStruct.(cfntypes.Stack)
		if !ok {
			if p, ok := cfnRes.RawStruct.(*cfntypes.Stack); ok && p != nil {
				raw = *p
				ok = true
			}
		}
		if ok && raw.StackName != nil && *raw.StackName == stackName {
			ids = append(ids, cfnRes.ID)
		}
	}
	return relatedResult("cfn", ids)
}

// checkEC2EIP checks the cache for Elastic IPs associated with this EC2 instance.
func checkEC2EIP(_ context.Context, _ interface{}, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	instanceID, _, _ := ec2Identity(res)
	if instanceID == "" {
		return resource.RelatedCheckResult{TargetType: "eip", Count: 0}
	}
	eipList, ok := cache["eip"]
	if !ok {
		return resource.RelatedCheckResult{TargetType: "eip", Count: -1}
	}
	var ids []string
	for _, eipRes := range eipList {
		raw, ok := eipRes.RawStruct.(ec2types.Address)
		if !ok {
			if p, ok := eipRes.RawStruct.(*ec2types.Address); ok && p != nil {
				raw = *p
				ok = true
			}
		}
		if ok && raw.InstanceId != nil && *raw.InstanceId == instanceID {
			ids = append(ids, eipRes.ID)
			continue
		}
		if eipRes.Fields["instance_id"] == instanceID {
			ids = append(ids, eipRes.ID)
		}
	}
	return relatedResult("eip", ids)
}

// checkEC2EBSSnap checks the cache for EBS snapshots belonging to this EC2 instance.
func checkEC2EBSSnap(_ context.Context, _ interface{}, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	volumeIDs := ec2VolumeIDs(res)
	if len(volumeIDs) == 0 {
		return resource.RelatedCheckResult{TargetType: "ebs-snap", Count: 0}
	}
	snapList, ok := cache["ebs-snap"]
	if !ok {
		return resource.RelatedCheckResult{TargetType: "ebs-snap", Count: -1}
	}
	var ids []string
	for _, snapRes := range snapList {
		raw, ok := snapRes.RawStruct.(ec2types.Snapshot)
		if !ok {
			if p, ok := snapRes.RawStruct.(*ec2types.Snapshot); ok && p != nil {
				raw = *p
				ok = true
			}
		}
		volumeID := snapRes.Fields["volume_id"]
		if ok && raw.VolumeId != nil {
			volumeID = *raw.VolumeId
		}
		if _, found := volumeIDs[volumeID]; found {
			ids = append(ids, snapRes.ID)
		}
	}
	return relatedResult("ebs-snap", ids)
}

func relatedResult(target string, ids []string) resource.RelatedCheckResult {
	if len(ids) == 0 {
		return resource.RelatedCheckResult{TargetType: target, Count: 0}
	}
	set := make(map[string]struct{}, len(ids))
	uniq := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		if _, ok := set[id]; ok {
			continue
		}
		set[id] = struct{}{}
		uniq = append(uniq, id)
	}
	sort.Strings(uniq)
	return resource.RelatedCheckResult{
		TargetType:  target,
		Count:       len(uniq),
		ResourceIDs: uniq,
	}
}

func ec2Identity(res resource.Resource) (instanceID, vpcID, stackName string) {
	instanceID = res.ID
	if raw, ok := res.RawStruct.(ec2types.Instance); ok {
		if raw.InstanceId != nil {
			instanceID = *raw.InstanceId
		}
		if raw.VpcId != nil {
			vpcID = *raw.VpcId
		}
		for _, tag := range raw.Tags {
			if tag.Key == nil || tag.Value == nil {
				continue
			}
			if *tag.Key == "aws:cloudformation:stack-name" {
				stackName = *tag.Value
				break
			}
		}
		return instanceID, vpcID, stackName
	}
	if raw, ok := res.RawStruct.(*ec2types.Instance); ok && raw != nil {
		if raw.InstanceId != nil {
			instanceID = *raw.InstanceId
		}
		if raw.VpcId != nil {
			vpcID = *raw.VpcId
		}
		for _, tag := range raw.Tags {
			if tag.Key == nil || tag.Value == nil {
				continue
			}
			if *tag.Key == "aws:cloudformation:stack-name" {
				stackName = *tag.Value
				break
			}
		}
		return instanceID, vpcID, stackName
	}
	return instanceID, res.Fields["vpc_id"], ""
}

func ec2VolumeIDs(res resource.Resource) map[string]struct{} {
	ids := map[string]struct{}{}
	raw, ok := res.RawStruct.(ec2types.Instance)
	if !ok {
		if p, ok := res.RawStruct.(*ec2types.Instance); ok && p != nil {
			raw = *p
			ok = true
		}
	}
	if !ok {
		return ids
	}
	for _, bdm := range raw.BlockDeviceMappings {
		if bdm.Ebs == nil || bdm.Ebs.VolumeId == nil {
			continue
		}
		ids[*bdm.Ebs.VolumeId] = struct{}{}
	}
	return ids
}
