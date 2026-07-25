// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// ecr_related.go contains ECR related-resource checker functions.
package aws

import (
	"context"
	"encoding/json"
	"slices"
	"strings"

	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
	eventbridgetypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkECRLambda checks the cache for Lambda functions using image-based packaging
// (Pattern C — cache-based heuristic). Since FunctionConfiguration does not include
// the image URI, any Lambda with PackageType=Image is considered potentially related
// to this ECR repository.
func checkECRLambda(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	repoURI := res.Fields["uri"]
	if repoURI == "" {
		return resource.KnownRelated("lambda", nil, false)
	}

	lambdaList, truncated, err := ecrRelatedResources(ctx, clients, cache, "lambda")
	if err != nil {
		return resource.ErrorRelated("lambda", err)
	}
	if lambdaList == nil {
		return resource.UnknownRelated("lambda")
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
	return relatedResultTrunc("lambda", ids, truncated)
}

// checkECRCodeBuild checks the cache for CodeBuild projects whose environment image
// contains this ECR repository URI (Pattern C — cache-based).
func checkECRCodeBuild(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	repoURI := res.Fields["uri"]
	if repoURI == "" {
		return resource.KnownRelated("cb", nil, false)
	}

	cbList, truncated, err := ecrRelatedResources(ctx, clients, cache, "cb")
	if err != nil {
		return resource.ErrorRelated("cb", err)
	}
	if cbList == nil {
		return resource.UnknownRelated("cb")
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
	return relatedResultTrunc("cb", ids, truncated)
}

// checkECRCFN checks the ECR repository's tags for aws:cloudformation:stack-name
// and matches against the CFN stack cache (Pattern C — tag-based).
// One ecr:ListTagsForResource call per open repository.
func checkECRCFN(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	stackName, err := ecrCFNStackName(ctx, clients, res)
	if err != nil {
		return resource.ErrorRelated("cfn", err)
	}
	if stackName == "" {
		return resource.KnownRelated("cfn", nil, false)
	}

	cfnList, truncated, err := ecrRelatedResources(ctx, clients, cache, "cfn")
	if err != nil {
		return resource.ErrorRelated("cfn", err)
	}
	if cfnList == nil {
		return resource.UnknownRelated("cfn")
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
	return relatedResultTrunc("cfn", ids, truncated)
}

// ecrCFNStackName extracts the aws:cloudformation:stack-name tag value from the
// repository. ECR Repository does not embed tags in the DescribeRepositories
// response; tags are fetched via a single ecr:ListTagsForResource call keyed
// on the repository ARN. Returns "" (no error) when the tag is absent.
func ecrCFNStackName(ctx context.Context, clients any, res resource.Resource) (string, error) {
	repo, ok := assertStruct[ecrtypes.Repository](res.RawStruct)
	if !ok || repo.RepositoryArn == nil || *repo.RepositoryArn == "" {
		return "", nil
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.ECR == nil {
		return "", nil
	}
	api, ok := c.ECR.(ECRListTagsForResourceAPI)
	if !ok {
		return "", nil
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*ecr.ListTagsForResourceOutput, error) {
		return api.ListTagsForResource(ctx, &ecr.ListTagsForResourceInput{ResourceArn: repo.RepositoryArn})
	})
	if err != nil {
		return "", err
	}
	if out == nil {
		return "", nil
	}
	for _, tag := range out.Tags {
		if tag.Key != nil && *tag.Key == "aws:cloudformation:stack-name" && tag.Value != nil {
			return *tag.Value, nil
		}
	}
	return "", nil
}

// checkECRKMS extracts the KMS key from the ECR Repository's
// EncryptionConfiguration.KmsKey field. Returns the key ID (last segment after "/").
// Pattern F — no cache needed.
func checkECRKMS(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	repo, ok := assertStruct[ecrtypes.Repository](res.RawStruct)
	if !ok || repo.EncryptionConfiguration == nil || repo.EncryptionConfiguration.KmsKey == nil || *repo.EncryptionConfiguration.KmsKey == "" {
		return resource.KnownRelated("kms", nil, false)
	}
	keyID := kmsKeyIDFromField(*repo.EncryptionConfiguration.KmsKey, res.Type)
	return relatedResult("kms", []string{keyID})
}

// checkECREbRule is a reverse-scan checker for the ecr→eb-rule relationship.
// Iterates cache["eb-rule"]; for each rule, checks if rule.EventPattern JSON
// contains source: ["aws.ecr"] AND (detail.repository-name == repo name OR
// resources containing the repo ARN). NeedsTargetCache: true.
func checkECREbRule(_ context.Context, _ any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	repo, ok := assertStruct[ecrtypes.Repository](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("eb-rule")
	}
	repoName := ""
	if repo.RepositoryName != nil {
		repoName = *repo.RepositoryName
	}
	if repoName == "" {
		return resource.KnownRelated("eb-rule", nil, false)
	}
	repoARN := ""
	if repo.RepositoryArn != nil {
		repoARN = *repo.RepositoryArn
	}

	entry, ok := cache["eb-rule"]
	if !ok {
		return resource.UnknownRelated("eb-rule")
	}

	var ids []string
	for _, ruleRes := range entry.Resources {
		raw, ok := assertStruct[eventbridgetypes.Rule](ruleRes.RawStruct)
		if !ok {
			continue
		}
		if raw.EventPattern == nil || *raw.EventPattern == "" {
			continue
		}
		if ecrEbRuleMatches(*raw.EventPattern, repoName, repoARN) {
			ids = append(ids, ruleRes.ID)
		}
	}
	return relatedResultTrunc("eb-rule", ids, entry.IsTruncated)
}

// ecrEbRuleMatches returns true if the EventPattern JSON has source ["aws.ecr"]
// and references the repository by name or ARN.
func ecrEbRuleMatches(pattern, repoName, repoARN string) bool {
	var p map[string]json.RawMessage
	if err := json.Unmarshal([]byte(pattern), &p); err != nil {
		return false
	}

	// Check source includes "aws.ecr"
	if src, ok := p["source"]; ok {
		var sources []string
		if err := json.Unmarshal(src, &sources); err != nil || !slices.Contains(sources, "aws.ecr") {
			return false
		}
	} else {
		return false
	}

	// Check detail.repository-name or resources match.
	// If a repository-name filter is present but doesn't match, return false.
	// Only fall through to "no filter → broad match" when no filter key exists.
	hasRepoFilter := false
	if detail, ok := p["detail"]; ok {
		var d map[string]json.RawMessage
		if err := json.Unmarshal(detail, &d); err == nil {
			if rn, ok := d["repository-name"]; ok {
				hasRepoFilter = true
				var names []string
				if err := json.Unmarshal(rn, &names); err == nil && slices.Contains(names, repoName) {
					return true
				}
			}
		}
	}
	if repoARN != "" {
		if resources, ok := p["resources"]; ok {
			hasRepoFilter = true
			var res []string
			if err := json.Unmarshal(resources, &res); err == nil {
				for _, r := range res {
					if strings.Contains(r, repoARN) {
						return true
					}
				}
			}
		}
	}
	if hasRepoFilter {
		// A filter existed but didn't match — not related.
		return false
	}
	// Source matches aws.ecr with no repository filter — treat as broad match.
	return true
}
