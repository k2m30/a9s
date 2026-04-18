// efs_related_extra.go contains additional EFS related-resource checkers
// required by docs/related-resources.md.
package aws

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/backup"
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

// checkEFSEC2 scans ec2 cache for instances whose ENIs mount this filesystem.
// Cross-reference via the eni cache (EFS mount targets have ENIs with the
// filesystem ID in their description, and are attached to an EC2 when an
// instance mounts them).
func checkEFSEC2(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	fsID := res.ID
	if fsID == "" {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: 0}
	}
	eniList, truncated, err := efsRelatedResources(ctx, clients, cache, "eni")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: -1, Err: err}
	}
	if eniList == nil {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: 0}
	}
	instanceSet := make(map[string]struct{})
	for _, eniRes := range eniList {
		eni, ok := assertStruct[ec2types.NetworkInterface](eniRes.RawStruct)
		if !ok {
			continue
		}
		if eni.Description == nil || !strings.Contains(*eni.Description, fsID) {
			continue
		}
		if eni.Attachment != nil && eni.Attachment.InstanceId != nil && *eni.Attachment.InstanceId != "" {
			instanceSet[*eni.Attachment.InstanceId] = struct{}{}
		}
	}
	var ids []string
	for id := range instanceSet {
		ids = append(ids, id)
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("ec2")
	}
	return relatedResult("ec2", ids)
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

// checkEFSBackup resolves AWS Backup recovery points for this EFS file system via
// backup:ListRecoveryPointsByResource (Pattern A: 1 API call).
// The EFS ARN is read from FileSystemArn in RawStruct.
func checkEFSBackup(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	fs, ok := assertStruct[efstypes.FileSystemDescription](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "backup", Count: -1}
	}
	if fs.FileSystemArn == nil || *fs.FileSystemArn == "" {
		return resource.RelatedCheckResult{TargetType: "backup", Count: 0}
	}
	fsARN := *fs.FileSystemArn

	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.Backup == nil {
		return resource.RelatedCheckResult{TargetType: "backup", Count: -1}
	}
	api, ok := c.Backup.(BackupListRecoveryPointsByResourceAPI)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "backup", Count: -1}
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*backup.ListRecoveryPointsByResourceOutput, error) {
		return api.ListRecoveryPointsByResource(ctx, &backup.ListRecoveryPointsByResourceInput{
			ResourceArn: &fsARN,
		})
	})
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "backup", Count: -1, Err: err}
	}
	var ids []string
	for _, rp := range out.RecoveryPoints {
		if rp.RecoveryPointArn != nil && *rp.RecoveryPointArn != "" {
			ids = append(ids, *rp.RecoveryPointArn)
		}
	}
	return relatedResult("backup", ids)
}

// keep lambdatypes imported (used by checkEFSLambda in efs_related.go).
var _ = lambdatypes.FunctionConfiguration{}
