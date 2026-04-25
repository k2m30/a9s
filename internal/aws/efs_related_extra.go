// efs_related_extra.go contains additional EFS related-resource checkers
// required by docs/related-resources.md.
package aws

import (
	"context"
	"strings"

	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	efstypes "github.com/aws/aws-sdk-go-v2/service/efs/types"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkEFSAlarm scans the alarm cache for CW alarms in the AWS/EFS namespace
// whose FileSystemId dimension matches this filesystem.
func checkEFSAlarm(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	fsID := res.ID
	if fsID == "" {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: 0}
	}
	alarmList, truncated, err := efsRelatedResources(ctx, clients, cache, "alarm")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1, Err: err}
	}
	if alarmList == nil {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1}
	}
	var ids []string
	for _, alarmRes := range alarmList {
		alarm, ok := assertStruct[cwtypes.MetricAlarm](alarmRes.RawStruct)
		if !ok {
			continue
		}
		for _, d := range alarm.Dimensions {
			if d.Name != nil && *d.Name == "FileSystemId" && d.Value != nil && *d.Value == fsID {
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

// checkEFSENI scans eni cache for mount-target ENIs (description contains fs-id).
func checkEFSENI(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	fsID := res.ID
	if fsID == "" {
		return resource.RelatedCheckResult{TargetType: "eni", Count: 0}
	}
	eniList, truncated, err := efsRelatedResources(ctx, clients, cache, "eni")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "eni", Count: -1, Err: err}
	}
	if eniList == nil {
		return resource.RelatedCheckResult{TargetType: "eni", Count: 0}
	}
	var ids []string
	for _, eniRes := range eniList {
		eni, ok := assertStruct[ec2types.NetworkInterface](eniRes.RawStruct)
		if !ok {
			continue
		}
		if eni.Description != nil && strings.Contains(*eni.Description, fsID) {
			ids = append(ids, eniRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("eni")
	}
	return relatedResult("eni", ids)
}

// checkEFSVPC derives the VPC via mount-target ENIs → subnet → VPC lookup.
func checkEFSVPC(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	fsID := res.ID
	if fsID == "" {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: 0}
	}
	eniList, truncated, err := efsRelatedResources(ctx, clients, cache, "eni")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: -1, Err: err}
	}
	if eniList == nil {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: 0}
	}
	vpcSet := make(map[string]struct{})
	for _, eniRes := range eniList {
		eni, ok := assertStruct[ec2types.NetworkInterface](eniRes.RawStruct)
		if !ok {
			continue
		}
		if eni.Description == nil || !strings.Contains(*eni.Description, fsID) {
			continue
		}
		if eni.VpcId != nil && *eni.VpcId != "" {
			vpcSet[*eni.VpcId] = struct{}{}
		}
	}
	var ids []string
	for id := range vpcSet {
		ids = append(ids, id)
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("vpc")
	}
	return relatedResult("vpc", ids)
}

// checkEFSBackup resolves AWS Backup PLANS that protect this EFS file system.
// The backup fetcher (backup.go) indexes Resource.ID by BackupPlanId and
// carries the plan's selected resource ARNs in Fields["resources"] as a CSV,
// so this checker reverse-scans the backup cache for plans whose resources
// CSV contains the EFS ARN. Returning recovery-point ARNs (the previous
// implementation) produced an ID-format mismatch — drill-through landed
// empty because recovery points are a different resource class and the
// backup list is keyed by plan id.
func checkEFSBackup(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	// Missing RawStruct → we can't derive the source ARN. That's an early-exit
	// (Count=0), not an error (Count=-1). Returning -1 here when the caller
	// handed us a valid truncated cache would drop the honest lower bound.
	fs, ok := assertStruct[efstypes.FileSystemDescription](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "backup", Count: 0}
	}
	if fs.FileSystemArn == nil || *fs.FileSystemArn == "" {
		return resource.RelatedCheckResult{TargetType: "backup", Count: 0}
	}
	fsARN := *fs.FileSystemArn

	plans, truncated, err := efsRelatedResources(ctx, clients, cache, "backup")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "backup", Count: -1, Err: err}
	}
	var ids []string
	for _, plan := range plans {
		if BackupPlanCoversARN(plan.Fields["resources"], plan.Fields["not_resources"], fsARN) {
			ids = append(ids, plan.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("backup")
	}
	return relatedResult("backup", ids)
}

// keep lambdatypes imported (used by checkEFSLambda in efs_related.go).
var _ = lambdatypes.FunctionConfiguration{}
