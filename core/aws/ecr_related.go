// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// ecr_related.go contains ECR related-resource checker functions.
package aws

import (
	"context"
	"slices"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// ecrRepoURI is the "<registry>/<repository>" URI AWS gives the repository a
// row stands for.
func ecrRepoURI(res resource.Resource) string {
	if raw, ok := assertStruct[ecrtypes.Repository](res.RawStruct); ok && raw.RepositoryUri != nil && *raw.RepositoryUri != "" {
		return *raw.RepositoryUri
	}
	return res.Fields["uri"]
}

// ecrWorkloadRepos is the ecr pivot of a workload that runs container
// images: the loaded repositories those images name.
func ecrWorkloadRepos(ctx context.Context, clients any, cache resource.ResourceCache, images []string) resource.RelatedCheckResult {
	if len(images) == 0 {
		return foundNone("ecr", "container images")
	}
	list, truncated, err := relatedResourcesFor(ctx, clients, cache, "ecr")
	if err != nil {
		return ReadFailed("ecr", err)
	}
	if list == nil {
		return NotRead("ecr")
	}
	var ids []string
	for _, repoRes := range list {
		uri := ecrRepoURI(repoRes)
		if slices.ContainsFunc(images, func(img string) bool { return imageRefersToRepo(img, uri) }) {
			ids = append(ids, repoRes.ID)
		}
	}
	return relatedResultTrunc("ecr", ids, truncated)
}

// checkECRLambda reports the Lambda functions running an image of this
// repository. FunctionConfiguration carries no image URI, so the image of
// each container-packaged function is read with one lambda:GetFunction call.
func checkECRLambda(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	repoURI := ecrRepoURI(res)
	if repoURI == "" {
		return foundNone("lambda", "repoURI")
	}

	lambdaList, truncated, err := relatedResourcesFor(ctx, clients, cache, "lambda")
	if err != nil {
		return ReadFailed("lambda", err)
	}
	if lambdaList == nil {
		return NotRead("lambda")
	}

	var ids []string
	var reads rowReads
	for _, r := range lambdaList {
		if !lambdaRunsImage(r) {
			continue
		}
		image, err := lambdaImageURI(ctx, clients, r.ID)
		if err != nil {
			reads.fail(r.ID, err)
			continue
		}
		reads.read++
		if imageRefersToRepo(image, repoURI) {
			ids = append(ids, r.ID)
		}
	}
	return reads.answer("lambda", "ecr-related: GetFunction", ids, truncated)
}

// checkECRCodeBuild reports the CodeBuild projects whose build environment
// runs an image of this repository (Pattern C — cache-based).
func checkECRCodeBuild(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	repoURI := ecrRepoURI(res)
	if repoURI == "" {
		return foundNone("cb", "repoURI")
	}

	cbList, truncated, err := relatedResourcesFor(ctx, clients, cache, "cb")
	if err != nil {
		return ReadFailed("cb", err)
	}
	if cbList == nil {
		return NotRead("cb")
	}

	var ids []string
	for _, r := range cbList {
		raw, ok := assertStruct[cbtypes.Project](r.RawStruct)
		if !ok {
			continue
		}
		if raw.Environment != nil && raw.Environment.Image != nil && imageRefersToRepo(*raw.Environment.Image, repoURI) {
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
		return ReadFailed("cfn", err)
	}
	if stackName == "" {
		return unreadZero(res, foundNone("cfn", "stackName"))
	}

	cfnList, truncated, err := relatedResourcesFor(ctx, clients, cache, "cfn")
	if err != nil {
		return ReadFailed("cfn", err)
	}
	if cfnList == nil {
		return NotRead("cfn")
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
	return unreadZeroScanned(res, len(cfnList), relatedResultTrunc("cfn", ids, truncated))
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
func checkECRKMS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	repo, ok := assertStruct[ecrtypes.Repository](res.RawStruct)
	if !ok || repo.EncryptionConfiguration == nil || repo.EncryptionConfiguration.KmsKey == nil || *repo.EncryptionConfiguration.KmsKey == "" {
		if res.RawStruct == nil {
			return NotRead("kms")
		}
		return foundNone("kms", "repo.EncryptionConfiguration.KmsKey")
	}
	keyID := kmsRefFromField(*repo.EncryptionConfiguration.KmsKey, res.Type)
	return kmsRelated(ctx, clients, cache, []string{keyID})
}

// checkECREbRule counts the eb-rule rows whose event pattern matches an event
// ECR emits about this repository.
func checkECREbRule(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	repo, ok := assertStruct[ecrtypes.Repository](res.RawStruct)
	if !ok {
		return NotRead("eb-rule")
	}
	repoName := aws.ToString(repo.RepositoryName)
	if repoName == "" {
		return foundNone("eb-rule", "repoName")
	}
	ruleList, truncated, err := relatedResourcesFor(ctx, clients, cache, "eb-rule")
	if err != nil {
		return ReadFailed("eb-rule", err)
	}
	if ruleList == nil {
		return NotRead("eb-rule")
	}
	return relatedAnswer("eb-rule", ebRulesMatching(ruleList, truncated, ecrEvents(repoName, aws.ToString(repo.RepositoryArn))))
}
