// efs_related.go contains EFS related-resource checker functions.
package aws

import (
	"context"
	"strings"

	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	efstypes "github.com/aws/aws-sdk-go-v2/service/efs/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterNavigableFields("efs", []resource.NavigableField{
		{FieldPath: "KmsKeyId", TargetType: "kms"},
	})

	resource.RegisterRelated("efs", []resource.RelatedDef{
		{TargetType: "kms", DisplayName: "KMS Keys", Checker: checkEFSKMS},
		{TargetType: "cfn", DisplayName: "CloudFormation Stacks", Checker: checkEFSCFN, NeedsTargetCache: true},
		{TargetType: "lambda", DisplayName: "Lambda Functions", Checker: nil},
	})
}

// checkEFSKMS returns the KMS key used to encrypt this EFS file system (Pattern F).
// KmsKeyId may be either a full ARN (arn:aws:kms:...:key/{id}) or a bare key ID.
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
		// Bare key ID (no ARN prefix)
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
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1}
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
