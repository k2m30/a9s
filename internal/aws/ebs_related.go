package aws

import (
	"context"
	"strings"

	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkEBSEC2 returns the EC2 instance this volume is attached to (Pattern F).
func checkEBSEC2(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	instanceID := res.Fields["attached_to"]
	if instanceID == "" {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: 0}
	}
	return relatedResult("ec2", []string{instanceID})
}

// checkEBSSnap searches the ebs-snap cache for snapshots of this volume (Pattern C).
func checkEBSSnap(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	volID := res.ID
	if volID == "" {
		return resource.RelatedCheckResult{TargetType: "ebs-snap", Count: 0}
	}

	snapList, truncated, err := ebsRelatedResources(ctx, clients, cache, "ebs-snap")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "ebs-snap", Count: -1, Err: err}
	}
	if snapList == nil {
		return resource.RelatedCheckResult{TargetType: "ebs-snap", Count: -1}
	}

	var ids []string
	for _, r := range snapList {
		if r.Fields["volume_id"] == volID {
			ids = append(ids, r.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("ebs-snap")
	}
	return relatedResult("ebs-snap", ids)
}

// checkEBSKMS returns the KMS key used to encrypt this volume (Pattern F).
func checkEBSKMS(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	vol, ok := assertStruct[ec2types.Volume](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "kms", Count: -1}
	}
	if vol.KmsKeyId == nil || *vol.KmsKeyId == "" {
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
	}
	arn := *vol.KmsKeyId
	idx := strings.LastIndex(arn, "/")
	if idx < 0 || idx == len(arn)-1 {
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
	}
	return relatedResult("kms", []string{arn[idx+1:]})
}

// checkEBSAlarm searches the alarm cache for alarms with a VolumeId dimension
// matching this volume.
func checkEBSAlarm(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	volID := res.ID
	if volID == "" {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: 0}
	}

	alarmList, truncated, err := ebsRelatedResources(ctx, clients, cache, "alarm")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1, Err: err}
	}
	if alarmList == nil {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1}
	}

	var ids []string
	for _, alarmRes := range alarmList {
		rawAlarm, ok := assertStruct[cwtypes.MetricAlarm](alarmRes.RawStruct)
		if !ok {
			continue
		}
		for _, d := range rawAlarm.Dimensions {
			if d.Name != nil && *d.Name == "VolumeId" && d.Value != nil && *d.Value == volID {
				ids = append(ids, alarmRes.ID)
				break
			}
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("alarm")
	}
	return relatedResult("alarm", ids)
}

// checkEBSCFN matches the volume's aws:cloudformation:stack-name tag to a
// CFN stack in the cache. Pattern C — Volume.Tags is populated from
// DescribeVolumes.
func checkEBSCFN(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	vol, ok := assertStruct[ec2types.Volume](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: 0}
	}
	stackName := ""
	for _, tag := range vol.Tags {
		if tag.Key != nil && *tag.Key == "aws:cloudformation:stack-name" && tag.Value != nil {
			stackName = *tag.Value
			break
		}
	}
	if stackName == "" {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: 0}
	}

	cfnList, truncated, err := ebsRelatedResources(ctx, clients, cache, "cfn")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1, Err: err}
	}
	if cfnList == nil {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1}
	}

	var ids []string
	for _, cfnRes := range cfnList {
		if cfnRes.ID == stackName || cfnRes.Name == stackName || cfnRes.Fields["stack_name"] == stackName {
			ids = append(ids, cfnRes.ID)
			continue
		}
		rawCFN, cfnOk := assertStruct[cfntypes.Stack](cfnRes.RawStruct)
		if cfnOk && rawCFN.StackName != nil && *rawCFN.StackName == stackName {
			ids = append(ids, cfnRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("cfn")
	}
	return relatedResult("cfn", ids)
}

// checkEBSBackup scans the backup cache for backup plans whose selection
// tags (BackupSelection.ListOfTags, joined into Fields["selection_tags"] by
// the backup fetcher) match this volume's own tags. Zero extra calls — a
// pure cross-reference of the already-loaded backup cache, per
// docs/resources/ebs.md ("sibling-list cross-ref when backup list is loaded,
// otherwise the panel renders an empty backup group").
func checkEBSBackup(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	volID := res.ID
	if volID == "" {
		return resource.RelatedCheckResult{TargetType: "backup", Count: 0}
	}
	tags := map[string]string{}
	if vol, ok := assertStruct[ec2types.Volume](res.RawStruct); ok {
		for _, t := range vol.Tags {
			if t.Key != nil && t.Value != nil {
				tags[*t.Key] = *t.Value
			}
		}
	}

	backupList, truncated, err := ebsRelatedResources(ctx, clients, cache, "backup")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "backup", Count: -1, Err: err}
	}
	if backupList == nil {
		return resource.RelatedCheckResult{TargetType: "backup", Count: -1}
	}

	var ids []string
	for _, planRes := range backupList {
		if backupSelectionTagsMatch(planRes.Fields["selection_tags"], tags) {
			ids = append(ids, planRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("backup")
	}
	return relatedResult("backup", ids)
}

// ebsRelatedResources returns the resource list for target from cache or fetches
// the first page via the registered paginated fetcher.
func ebsRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}
