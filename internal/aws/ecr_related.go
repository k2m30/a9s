// ecr_related.go contains ECR related-resource checker functions.
package aws

import (
	"context"
	"strings"

	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkECRLambda checks the cache for Lambda functions using image-based packaging
// (Pattern C — cache-based heuristic). Since FunctionConfiguration does not include
// the image URI, any Lambda with PackageType=Image is considered potentially related
// to this ECR repository.
func checkECRLambda(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	repoURI := res.Fields["uri"]
	if repoURI == "" {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: 0}
	}

	lambdaList, truncated, err := ecrRelatedResources(ctx, clients, cache, "lambda")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: -1, Err: err}
	}
	if lambdaList == nil {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: -1}
	}

	var ids []string
	for _, r := range lambdaList {
		raw, ok := assertStruct[lambdatypes.FunctionConfiguration](r.RawStruct)
		if ok {
			if raw.PackageType == lambdatypes.PackageTypeImage {
				ids = append(ids, r.ID)
			}
			continue
		}
		if r.Fields["package_type"] == "Image" {
			ids = append(ids, r.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: -1}
	}
	return relatedResult("lambda", ids)
}

// checkECRCodeBuild checks the cache for CodeBuild projects whose environment image
// contains this ECR repository URI (Pattern C — cache-based).
func checkECRCodeBuild(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	repoURI := res.Fields["uri"]
	if repoURI == "" {
		return resource.RelatedCheckResult{TargetType: "cb", Count: 0}
	}

	cbList, truncated, err := ecrRelatedResources(ctx, clients, cache, "cb")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "cb", Count: -1, Err: err}
	}
	if cbList == nil {
		return resource.RelatedCheckResult{TargetType: "cb", Count: -1}
	}

	var ids []string
	for _, r := range cbList {
		raw, ok := assertStruct[cbtypes.Project](r.RawStruct)
		if !ok {
			continue
		}
		if raw.Environment != nil && raw.Environment.Image != nil && strings.Contains(*raw.Environment.Image, repoURI) {
			ids = append(ids, r.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "cb", Count: -1}
	}
	return relatedResult("cb", ids)
}

// checkECRCFN checks the ECR repository's tags for aws:cloudformation:stack-name
// and matches against the CFN stack cache (Pattern C — tag-based).
func checkECRCFN(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	stackName := ecrCFNStackName(res)
	if stackName == "" {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: 0}
	}

	cfnList, truncated, err := ecrRelatedResources(ctx, clients, cache, "cfn")
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

// ecrCFNStackName extracts the aws:cloudformation:stack-name tag value from the
// resource. ECR Repository does not embed tags in the DescribeRepositories response;
// tags are fetched via a separate ListTagsForResource call. We check the Fields map
// (populated if tags were enriched) and fall back to zero if unavailable.
func ecrCFNStackName(res resource.Resource) string {
	// Tags are not present on ecrtypes.Repository directly; check enriched Fields.
	return res.Fields["cfn_stack_name"]
}


// checkECRKMS extracts the KMS key from the ECR Repository's
// EncryptionConfiguration.KmsKey field. Returns the key ID (last segment after "/").
// Pattern F — no cache needed.
func checkECRKMS(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	repo, ok := assertStruct[ecrtypes.Repository](res.RawStruct)
	if !ok || repo.EncryptionConfiguration == nil || repo.EncryptionConfiguration.KmsKey == nil || *repo.EncryptionConfiguration.KmsKey == "" {
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
	}
	keyID := *repo.EncryptionConfiguration.KmsKey
	if idx := strings.LastIndex(keyID, "/"); idx >= 0 && idx < len(keyID)-1 {
		keyID = keyID[idx+1:]
	}
	return relatedResult("kms", []string{keyID})
}






// ecrRelatedResources returns the resource list for target from cache or fetches
// the first page via the registered paginated fetcher.
func ecrRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}
