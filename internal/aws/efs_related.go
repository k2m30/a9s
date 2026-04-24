// efs_related.go contains EFS related-resource checker functions.
package aws

import (
	"context"
	"slices"
	"strings"

	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/efs"
	efstypes "github.com/aws/aws-sdk-go-v2/service/efs/types"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterNavigableFields("efs", []resource.NavigableField{
		{FieldPath: "KmsKeyId", TargetType: "kms"},
	})

	resource.RegisterRelated("efs", []resource.RelatedDef{
		{TargetType: "kms", DisplayName: "KMS Keys", Checker: checkEFSKMS},
		{TargetType: "cfn", DisplayName: "CloudFormation Stacks", Checker: checkEFSCFN, NeedsTargetCache: true},
		{TargetType: "sg", DisplayName: "Security Groups", Checker: checkEFSSG, NeedsTargetCache: false},
		{TargetType: "subnet", DisplayName: "Subnets", Checker: checkEFSSubnet, NeedsTargetCache: false},
		{TargetType: "lambda", DisplayName: "Lambda Functions", Checker: checkEFSLambda, NeedsTargetCache: false},
		{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: checkEFSAlarm, NeedsTargetCache: true},
		{TargetType: "backup", DisplayName: "Backup Plans", Checker: checkEFSBackup, NeedsTargetCache: true},
		// EC2 pivot intentionally removed: EC2→EFS mounting happens at the
		// guest OS level via DNS lookup of mt ENIs. AWS exposes no API edge
		// linking instance → filesystem — mount-target ENIs are
		// RequesterManaged with no Attachment.InstanceId, so a checker can
		// only return zero or heuristic noise. Honest drop beats a registered
		// pivot that always returns Count=0 (U9 violation).
		{TargetType: "ecs-task", DisplayName: "ECS Tasks", Checker: checkEFSECSTask, NeedsTargetCache: true},
		{TargetType: "eni", DisplayName: "Network Interfaces", Checker: checkEFSENI, NeedsTargetCache: true},
		{TargetType: "vpc", DisplayName: "VPC", Checker: checkEFSVPC, NeedsTargetCache: true},
	})
}

// checkEFSKMS returns the KMS key used to encrypt this EFS file system (Pattern F).
// KmsKeyId may be either a full ARN (arn:aws:kms:...:key/{id}) or a bare key ID.
//
// The checker emits the key ID blindly; the related-check orchestrator's
// lazy-add path (RegisterFetchByIDs for "kms") fetches the key metadata on
// demand when the ID is not already in the customer-managed kms cache. That
// keeps this checker simple AND lets AWS-managed keys (aws/elasticfilesystem,
// etc.) drill into a real entry — both the count and the drill land on the
// same resource.
func checkEFSKMS(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	fs, ok := assertStruct[efstypes.FileSystemDescription](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "kms", Count: -1}
	}
	if fs.KmsKeyId == nil || *fs.KmsKeyId == "" {
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
	}
	val := *fs.KmsKeyId
	idx := strings.LastIndex(val, "/")
	var keyID string
	switch {
	case idx < 0:
		keyID = val
	case idx == len(val)-1:
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
	default:
		keyID = val[idx+1:]
	}
	return relatedResult("kms", []string{keyID})
}

// checkEFSCFN checks EFS file system tags for aws:cloudformation:stack-name
// and matches against the CFN stack cache (Pattern C — tag-based).
func checkEFSCFN(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	stackName := efsCFNStackName(res)
	if stackName == "" {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: 0}
	}

	cfnList, truncated, err := efsRelatedResources(ctx, clients, cache, "cfn")
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
		raw, ok := assertStruct[cfntypes.Stack](cfnRes.RawStruct)
		if ok && raw.StackName != nil && *raw.StackName == stackName {
			ids = append(ids, cfnRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("cfn")
	}
	return relatedResult("cfn", ids)
}

// efsCFNStackName extracts the aws:cloudformation:stack-name tag value from the
// EFS file system's Tags slice.
func efsCFNStackName(res resource.Resource) string {
	fs, ok := assertStruct[efstypes.FileSystemDescription](res.RawStruct)
	if !ok {
		return ""
	}
	for _, tag := range fs.Tags {
		if tag.Key != nil && *tag.Key == "aws:cloudformation:stack-name" && tag.Value != nil {
			return *tag.Value
		}
	}
	return ""
}

// checkEFSSG finds security groups for this EFS file system by scanning the ENI
// cache for mount-target ENIs whose Description contains the filesystem ID (Pattern C).
func checkEFSSG(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	fsID := res.ID
	if fsID == "" {
		return resource.RelatedCheckResult{TargetType: "sg", Count: 0}
	}

	eniList, truncated, err := efsRelatedResources(ctx, clients, cache, "eni")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "sg", Count: -1, Err: err}
	}
	if eniList == nil {
		return resource.RelatedCheckResult{TargetType: "sg", Count: -1}
	}

	sgSet := make(map[string]struct{})
	for _, eniRes := range eniList {
		eni, ok := assertStruct[ec2types.NetworkInterface](eniRes.RawStruct)
		if !ok {
			continue
		}
		if eni.Description == nil || !strings.Contains(*eni.Description, fsID) {
			continue
		}
		for _, sg := range eni.Groups {
			if sg.GroupId != nil && *sg.GroupId != "" {
				sgSet[*sg.GroupId] = struct{}{}
			}
		}
	}

	var ids []string
	for id := range sgSet {
		ids = append(ids, id)
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("sg")
	}
	return relatedResult("sg", ids)
}

// checkEFSSubnet finds subnets for this EFS file system by scanning the ENI
// cache for mount-target ENIs whose Description contains the filesystem ID (Pattern C).
func checkEFSSubnet(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	fsID := res.ID
	if fsID == "" {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: 0}
	}

	eniList, truncated, err := efsRelatedResources(ctx, clients, cache, "eni")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: -1, Err: err}
	}
	if eniList == nil {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: -1}
	}

	subnetSet := make(map[string]struct{})
	for _, eniRes := range eniList {
		eni, ok := assertStruct[ec2types.NetworkInterface](eniRes.RawStruct)
		if !ok {
			continue
		}
		if eni.Description == nil || !strings.Contains(*eni.Description, fsID) {
			continue
		}
		if eni.SubnetId != nil && *eni.SubnetId != "" {
			subnetSet[*eni.SubnetId] = struct{}{}
		}
	}

	var ids []string
	for id := range subnetSet {
		ids = append(ids, id)
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("subnet")
	}
	return relatedResult("subnet", ids)
}

// efsRelatedResources returns the resource list for target from cache or fetches
// the first page via the registered paginated fetcher.
func efsRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}

// checkEFSLambda finds Lambda functions that mount this EFS file system via
// FileSystemConfigs. Lambda FileSystemConfigs carry EFS *access-point* ARNs,
// not filesystem ARNs, so the link requires resolving the filesystem's access
// points via efs:DescribeAccessPoints (Pattern A + C): collect this file
// system's access point ARNs, then scan the lambda cache for
// FunctionConfiguration.FileSystemConfigs entries whose Arn is in that set.
// Returns Count: -1 when no live EFS client is available to list access points.
func checkEFSLambda(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	fsID := res.ID
	if fsID == "" {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: 0}
	}

	c, cok := clients.(*ServiceClients)
	if !cok || c == nil || c.EFS == nil {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: -1}
	}

	apOut, err := c.EFS.DescribeAccessPoints(ctx, &efs.DescribeAccessPointsInput{
		FileSystemId: &fsID,
	})
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: -1, Err: err}
	}
	apARNs := make(map[string]struct{})
	for _, ap := range apOut.AccessPoints {
		if ap.AccessPointArn != nil && *ap.AccessPointArn != "" {
			apARNs[*ap.AccessPointArn] = struct{}{}
		}
	}
	if len(apARNs) == 0 {
		// No access points exist for this filesystem — no Lambda can mount it.
		return resource.RelatedCheckResult{TargetType: "lambda", Count: 0}
	}

	lambdaList, truncated, err := efsRelatedResources(ctx, clients, cache, "lambda")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: -1, Err: err}
	}
	if lambdaList == nil {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: -1}
	}

	var ids []string
	for _, lRes := range lambdaList {
		fn, ok := assertStruct[lambdatypes.FunctionConfiguration](lRes.RawStruct)
		if !ok {
			continue
		}
		for _, cfg := range fn.FileSystemConfigs {
			if cfg.Arn == nil {
				continue
			}
			if _, matched := apARNs[*cfg.Arn]; matched {
				ids = append(ids, lRes.ID)
				break
			}
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("lambda")
	}
	return relatedResult("lambda", ids)
}

// checkEFSECSTask is a reverse-scan checker for the efs→ecs-task relationship.
// Pattern C+reverse: iterate cache["ecs-task"]; for each task read
// Fields["efs_file_system_ids"] (comma-separated list of EFS file-system IDs
// joined by the ecs-task fetcher via DescribeTaskDefinition) and match against
// this filesystem's ID.
// NeedsTargetCache: true.
func checkEFSECSTask(_ context.Context, _ any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	fsID := res.ID
	if fsID == "" {
		return resource.RelatedCheckResult{TargetType: "ecs-task", Count: 0}
	}

	entry, ok := cache["ecs-task"]
	if !ok {
		return resource.UnknownRelated("ecs-task")
	}

	var ids []string
	for _, tRes := range entry.Resources {
		joined := tRes.Fields["efs_file_system_ids"]
		if joined == "" {
			continue
		}
		if slices.Contains(strings.Split(joined, ","), fsID) {
			ids = append(ids, tRes.ID)
		}
	}
	result := relatedResult("ecs-task", ids)
	result.Approximate = entry.IsTruncated
	return result
}
