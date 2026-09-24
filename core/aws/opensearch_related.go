// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// opensearch_related.go contains OpenSearch Domain related-resource checker functions.
package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	acmtypes "github.com/aws/aws-sdk-go-v2/service/acm/types"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/aws-sdk-go-v2/service/opensearch"
	opensearchtypes "github.com/aws/aws-sdk-go-v2/service/opensearch/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkOpenSearchAlarms checks the cache for CloudWatch alarms with DomainName dimension matching this domain.
func checkOpenSearchAlarms(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return alarmIDsByDimension(ctx, clients, cache, "opensearch", res)
}

// checkOpenSearchLogs extracts CloudWatch log group ARNs from the domain's LogPublishingOptions.
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
		if opt.CloudWatchLogsLogGroupArn != nil {
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

// checkOpenSearchACM calls opensearch:DescribeDomainConfig and returns the
// ACM certificate attached to the domain's custom endpoint
// (DomainEndpointOptions.Options.CustomEndpointCertificateArn).
//
// The ACM fetcher (acm.go) indexes Resource.ID by DomainName. So this
// checker looks up the cert ARN against the acm cache and returns the
// matching Resource.ID (DomainName) so drill-through lands on it.
func checkOpenSearchACM(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	domainName := res.ID
	if domainName == "" {
		return foundNone("acm", "domainName")
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.OpenSearch == nil {
		return NotRead("acm")
	}
	cfgAPI, ok := c.OpenSearch.(OpenSearchDescribeDomainConfigAPI)
	if !ok {
		return NotRead("acm")
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*opensearch.DescribeDomainConfigOutput, error) {
		return cfgAPI.DescribeDomainConfig(ctx, &opensearch.DescribeDomainConfigInput{DomainName: aws.String(domainName)})
	})
	if err != nil {
		return ReadFailed("acm", err)
	}
	if out.DomainConfig == nil ||
		out.DomainConfig.DomainEndpointOptions == nil ||
		out.DomainConfig.DomainEndpointOptions.Options == nil ||
		out.DomainConfig.DomainEndpointOptions.Options.CustomEndpointCertificateArn == nil {
		return foundNone("acm", "the API answered that none is configured")
	}
	arn := *out.DomainConfig.DomainEndpointOptions.Options.CustomEndpointCertificateArn
	if arn == "" {
		return foundNone("acm", "arn")
	}

	// Reverse-scan the acm cache for a cert whose RawStruct.CertificateArn
	// matches. Return the target Resource.ID (DomainName) so drill lands.
	acmList, truncated, err := relatedResourcesFor(ctx, clients, cache, "acm")
	if err != nil {
		return ReadFailed("acm", err)
	}
	if acmList == nil {
		return NotRead("acm")
	}
	for _, acmRes := range acmList {
		cert, ok := assertStruct[acmtypes.CertificateSummary](acmRes.RawStruct)
		if !ok {
			continue
		}
		if cert.CertificateArn != nil && *cert.CertificateArn == arn {
			return relatedResultTrunc("acm", []string{acmRes.ID}, false)
		}
	}
	if truncated {
		return relatedResultTrunc("acm", nil, true)
	}
	return foundNone("acm", "the complete acm list")
}
