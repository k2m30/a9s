// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// opensearch_related.go contains OpenSearch Domain related-resource checker functions.
package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/aws-sdk-go-v2/service/opensearch"
	opensearchtypes "github.com/aws/aws-sdk-go-v2/service/opensearch/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkOpenSearchAlarms checks the cache for CloudWatch alarms with DomainName dimension matching this domain.
func checkOpenSearchAlarms(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return alarmIDsByDimension(ctx, clients, cache, "opensearch", res)
}

// checkOpenSearchLogs extracts the CloudWatch log groups of the domain's
// enabled LogPublishingOptions: Enabled is "whether the log should be
// published"
// (https://docs.aws.amazon.com/opensearch-service/latest/APIReference/API_LogPublishingOption.html).
func checkOpenSearchLogs(_ context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	domain, ok := assertStruct[opensearchtypes.DomainStatus](res.RawStruct)
	if !ok {
		return NotRead("logs")
	}
	if len(domain.LogPublishingOptions) == 0 {
		return foundNone("logs", "domain.LogPublishingOptions")
	}

	var arns []string
	for _, opt := range domain.LogPublishingOptions {
		if aws.ToBool(opt.Enabled) && opt.CloudWatchLogsLogGroupArn != nil {
			arns = append(arns, *opt.CloudWatchLogsLogGroupArn)
		}
	}
	return relatedRefs("logs", arns, refContext(clients, cache, "logs"))
}

// checkOpenSearchSG extracts security group IDs from the OpenSearch Domain's
// VPCOptions.SecurityGroupIds slice (only present for VPC-attached domains).
func checkOpenSearchSG(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	domain, ok := assertStruct[opensearchtypes.DomainStatus](res.RawStruct)
	if !ok {
		return NotRead("sg")
	}
	if domain.VPCOptions == nil {
		return foundNone("sg", "domain.VPCOptions")
	}
	var ids []string
	for _, sgID := range domain.VPCOptions.SecurityGroupIds {
		if sgID != "" {
			ids = append(ids, sgID)
		}
	}
	return relatedResultTrunc("sg", ids, false)
}

// checkOpenSearchVPC returns the VPC this OpenSearch domain is attached to.
// Reads VPCOptions.VPCId from the DomainStatus RawStruct.
// Returns Count: 0 for public domains not attached to a VPC.
func checkOpenSearchVPC(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	domain, ok := assertStruct[opensearchtypes.DomainStatus](res.RawStruct)
	if !ok {
		return NotRead("vpc")
	}
	if domain.VPCOptions == nil || domain.VPCOptions.VPCId == nil || *domain.VPCOptions.VPCId == "" {
		return foundNone("vpc", "domain.VPCOptions.VPCId")
	}
	return relatedResultTrunc("vpc", []string{*domain.VPCOptions.VPCId}, false)
}

// checkOpenSearchKMS extracts the KMS key ID from the OpenSearch domain's
// EncryptionAtRestOptions.KmsKeyId field.
func checkOpenSearchKMS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	domain, ok := assertStruct[opensearchtypes.DomainStatus](res.RawStruct)
	if !ok {
		// Structural assertion failure — RawStruct isn't a DomainStatus. This
		// is "unknown" (cannot determine), not "no KMS key": return -1 so the
		// UI renders "?" rather than falsely reporting 0. Matches
		// checkOpenSearchCFN / VPC / Subnet / SG / Logs.
		return NotRead("kms")
	}
	if domain.EncryptionAtRestOptions == nil ||
		domain.EncryptionAtRestOptions.KmsKeyId == nil ||
		*domain.EncryptionAtRestOptions.KmsKeyId == "" {
		// Legitimately no KMS key configured (encryption-off) — 0 is correct.
		return foundNone("kms", "EncryptionAtRestOptions.KmsKeyId")
	}
	keyID := kmsRefFromField(*domain.EncryptionAtRestOptions.KmsKeyId, res.Type)
	return kmsRelated(ctx, clients, cache, []string{keyID})
}

// checkOpenSearchCFN calls opensearch:ListTags(ARN=DomainStatus.ARN) and
// looks up the aws:cloudformation:stack-name tag in the cfn cache.
func checkOpenSearchCFN(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	domain, ok := assertStruct[opensearchtypes.DomainStatus](res.RawStruct)
	if !ok {
		return NotRead("cfn")
	}
	if domain.ARN == nil || *domain.ARN == "" {
		return foundNone("cfn", "domain.ARN")
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.OpenSearch == nil {
		return NotRead("cfn")
	}
	tagAPI, ok := c.OpenSearch.(OpenSearchListTagsAPI)
	if !ok {
		return NotRead("cfn")
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*opensearch.ListTagsOutput, error) {
		return tagAPI.ListTags(ctx, &opensearch.ListTagsInput{ARN: aws.String(*domain.ARN)})
	})
	if err != nil {
		return ReadFailed("cfn", err)
	}
	stackName := ""
	for _, tag := range out.TagList {
		if tag.Key != nil && *tag.Key == "aws:cloudformation:stack-name" && tag.Value != nil {
			stackName = *tag.Value
			break
		}
	}
	if stackName == "" {
		return foundNone("cfn", "stackName")
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
		rawCFN, cfnOk := assertStruct[cfntypes.Stack](cfnRes.RawStruct)
		if cfnOk && rawCFN.StackName != nil && *rawCFN.StackName == stackName {
			ids = append(ids, cfnRes.ID)
		}
	}
	return relatedResultTrunc("cfn", ids, truncated)
}

// checkOpenSearchSubnet returns the subnets the VPC-attached domain is deployed
// into (VPCOptions.SubnetIds).
func checkOpenSearchSubnet(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	domain, ok := assertStruct[opensearchtypes.DomainStatus](res.RawStruct)
	if !ok {
		return NotRead("subnet")
	}
	if domain.VPCOptions == nil {
		return foundNone("subnet", "domain.VPCOptions")
	}
	var ids []string
	for _, id := range domain.VPCOptions.SubnetIds {
		if id != "" {
			ids = append(ids, id)
		}
	}
	return relatedResultTrunc("subnet", ids, false)
}

// checkOpenSearchACM reports the certificate on the domain's custom endpoint,
// DomainEndpointOptions.CustomEndpointCertificateArn "for your security
// certificate, managed in AWS Certificate Manager", which DescribeDomains puts
// on the row
// (https://docs.aws.amazon.com/opensearch-service/latest/APIReference/API_DomainEndpointOptions.html).
// With the custom endpoint disabled the certificate terminates nothing.
func checkOpenSearchACM(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	domain, ok := assertStruct[opensearchtypes.DomainStatus](res.RawStruct)
	if !ok {
		return NotRead("acm")
	}
	opts := domain.DomainEndpointOptions
	if opts == nil || !aws.ToBool(opts.CustomEndpointEnabled) {
		return foundNone("acm", "DomainEndpointOptions.CustomEndpointEnabled")
	}
	return listedRelated(ctx, clients, cache, "acm", []string{aws.ToString(opts.CustomEndpointCertificateArn)}, false)
}
