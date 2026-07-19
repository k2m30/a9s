// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// opensearch_related.go contains OpenSearch Domain related-resource checker functions.
package aws

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	acmtypes "github.com/aws/aws-sdk-go-v2/service/acm/types"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/aws-sdk-go-v2/service/opensearch"
	opensearchtypes "github.com/aws/aws-sdk-go-v2/service/opensearch/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkOpenSearchAlarms checks the cache for CloudWatch alarms with DomainName dimension matching this domain.
func checkOpenSearchAlarms(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return alarmIDsByDimension(ctx, clients, cache, "", "DomainName", res.ID)
}

// checkOpenSearchLogs extracts CloudWatch log group ARNs from the domain's LogPublishingOptions.
// Pattern F — reads from RawStruct, no cache needed.
func checkOpenSearchLogs(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	domain, ok := assertStruct[opensearchtypes.DomainStatus](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("logs")
	}
	if len(domain.LogPublishingOptions) == 0 {
		return resource.RelatedCheckResult{TargetType: "logs", Count: 0}
	}

	seen := make(map[string]struct{})
	var ids []string
	for _, opt := range domain.LogPublishingOptions {
		if opt.CloudWatchLogsLogGroupArn == nil || *opt.CloudWatchLogsLogGroupArn == "" {
			continue
		}
		arn := *opt.CloudWatchLogsLogGroupArn
		// ARN format: arn:aws:logs:region:account:log-group:/name:*
		// Extract log group name by splitting on ":log-group:" and stripping trailing ":*"
		parts := strings.SplitN(arn, ":log-group:", 2)
		if len(parts) != 2 {
			continue
		}
		logGroupName := strings.TrimSuffix(parts[1], ":*")
		if logGroupName == "" {
			continue
		}
		if _, exists := seen[logGroupName]; !exists {
			seen[logGroupName] = struct{}{}
			ids = append(ids, logGroupName)
		}
	}
	if len(ids) == 0 {
		return resource.RelatedCheckResult{TargetType: "logs", Count: 0}
	}
	return relatedResult("logs", ids)
}

// checkOpenSearchSG extracts security group IDs from the OpenSearch Domain's
// VPCOptions.SecurityGroupIds slice (only present for VPC-attached domains).
// Pattern F — no cache needed.
func checkOpenSearchSG(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	domain, ok := assertStruct[opensearchtypes.DomainStatus](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("sg")
	}
	if domain.VPCOptions == nil {
		return resource.RelatedCheckResult{TargetType: "sg", Count: 0}
	}
	var ids []string
	for _, sgID := range domain.VPCOptions.SecurityGroupIds {
		if sgID != "" {
			ids = append(ids, sgID)
		}
	}
	return relatedResult("sg", ids)
}

// checkOpenSearchVPC returns the VPC this OpenSearch domain is attached to (Pattern R).
// Reads VPCOptions.VPCId from the DomainStatus RawStruct.
// Returns Count: 0 for public domains not attached to a VPC.
func checkOpenSearchVPC(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	domain, ok := assertStruct[opensearchtypes.DomainStatus](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("vpc")
	}
	if domain.VPCOptions == nil || domain.VPCOptions.VPCId == nil || *domain.VPCOptions.VPCId == "" {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: 0}
	}
	return relatedResult("vpc", []string{*domain.VPCOptions.VPCId})
}

// checkOpenSearchKMS extracts the KMS key ID from the OpenSearch domain's
// EncryptionAtRestOptions.KmsKeyId field. Pattern F — no cache needed.
func checkOpenSearchKMS(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	domain, ok := assertStruct[opensearchtypes.DomainStatus](res.RawStruct)
	if !ok {
		// Structural assertion failure — RawStruct isn't a DomainStatus. This
		// is "unknown" (cannot determine), not "no KMS key". Pattern-F
		// contract: return -1 so the UI renders "?" rather than falsely
		// reporting 0. Matches checkOpenSearchCFN / VPC / Subnet / SG / Logs.
		return resource.UnknownRelated("kms")
	}
	if domain.EncryptionAtRestOptions == nil ||
		domain.EncryptionAtRestOptions.KmsKeyId == nil ||
		*domain.EncryptionAtRestOptions.KmsKeyId == "" {
		// Legitimately no KMS key configured (encryption-off) — 0 is correct.
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
	}
	keyID := kmsKeyIDFromField(*domain.EncryptionAtRestOptions.KmsKeyId, res.Type)
	return relatedResult("kms", []string{keyID})
}

// checkOpenSearchCFN calls opensearch:ListTags(ARN=DomainStatus.ARN) and
// looks up the aws:cloudformation:stack-name tag in the cfn cache. Pattern C.
func checkOpenSearchCFN(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	domain, ok := assertStruct[opensearchtypes.DomainStatus](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("cfn")
	}
	if domain.ARN == nil || *domain.ARN == "" {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: 0}
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.OpenSearch == nil {
		return resource.UnknownRelated("cfn")
	}
	tagAPI, ok := c.OpenSearch.(OpenSearchListTagsAPI)
	if !ok {
		return resource.UnknownRelated("cfn")
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*opensearch.ListTagsOutput, error) {
		return tagAPI.ListTags(ctx, &opensearch.ListTagsInput{ARN: aws.String(*domain.ARN)})
	})
	if err != nil {
		return resource.ErrorRelated("cfn", err)
	}
	stackName := ""
	for _, tag := range out.TagList {
		if tag.Key != nil && *tag.Key == "aws:cloudformation:stack-name" && tag.Value != nil {
			stackName = *tag.Value
			break
		}
	}
	if stackName == "" {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: 0}
	}
	cfnList, truncated, err := relatedResourcesFor(ctx, clients, cache, "cfn")
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
		rawCFN, cfnOk := assertStruct[cfntypes.Stack](cfnRes.RawStruct)
		if cfnOk && rawCFN.StackName != nil && *rawCFN.StackName == stackName {
			ids = append(ids, cfnRes.ID)
		}
	}
	return relatedResultTrunc("cfn", ids, truncated)
}

// checkOpenSearchSubnet returns the subnets the VPC-attached domain is deployed
// into (VPCOptions.SubnetIds). Pattern F — no cache needed.
func checkOpenSearchSubnet(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	domain, ok := assertStruct[opensearchtypes.DomainStatus](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("subnet")
	}
	if domain.VPCOptions == nil {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: 0}
	}
	var ids []string
	for _, id := range domain.VPCOptions.SubnetIds {
		if id != "" {
			ids = append(ids, id)
		}
	}
	return relatedResult("subnet", ids)
}

// checkOpenSearchACM calls opensearch:DescribeDomainConfig and returns the
// ACM certificate attached to the domain's custom endpoint
// (DomainEndpointOptions.Options.CustomEndpointCertificateArn). Pattern C.
//
// The ACM fetcher (acm.go) indexes Resource.ID by DomainName. So this
// checker looks up the cert ARN against the acm cache and returns the
// matching Resource.ID (DomainName) so drill-through lands on it.
// Returning the bare cert ID (last segment of the ARN) — as the original
// implementation did — produces an unnavigable ID-format mismatch.
func checkOpenSearchACM(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	domainName := res.ID
	if domainName == "" {
		return resource.RelatedCheckResult{TargetType: "acm", Count: 0}
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.OpenSearch == nil {
		return resource.UnknownRelated("acm")
	}
	cfgAPI, ok := c.OpenSearch.(OpenSearchDescribeDomainConfigAPI)
	if !ok {
		return resource.UnknownRelated("acm")
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*opensearch.DescribeDomainConfigOutput, error) {
		return cfgAPI.DescribeDomainConfig(ctx, &opensearch.DescribeDomainConfigInput{DomainName: aws.String(domainName)})
	})
	if err != nil {
		return resource.ErrorRelated("acm", err)
	}
	if out.DomainConfig == nil ||
		out.DomainConfig.DomainEndpointOptions == nil ||
		out.DomainConfig.DomainEndpointOptions.Options == nil ||
		out.DomainConfig.DomainEndpointOptions.Options.CustomEndpointCertificateArn == nil {
		return resource.RelatedCheckResult{TargetType: "acm", Count: 0}
	}
	arn := *out.DomainConfig.DomainEndpointOptions.Options.CustomEndpointCertificateArn
	if arn == "" {
		return resource.RelatedCheckResult{TargetType: "acm", Count: 0}
	}

	// Reverse-scan the acm cache for a cert whose RawStruct.CertificateArn
	// matches. Return the target Resource.ID (DomainName) so drill lands.
	acmList, truncated, err := relatedResourcesFor(ctx, clients, cache, "acm")
	if err != nil {
		return resource.ErrorRelated("acm", err)
	}
	if acmList == nil {
		return resource.UnknownRelated("acm")
	}
	for _, acmRes := range acmList {
		cert, ok := assertStruct[acmtypes.CertificateSummary](acmRes.RawStruct)
		if !ok {
			continue
		}
		if cert.CertificateArn != nil && *cert.CertificateArn == arn {
			return relatedResult("acm", []string{acmRes.ID})
		}
	}
	if truncated {
		return relatedResultTrunc("acm", nil, true)
	}
	return resource.RelatedCheckResult{TargetType: "acm", Count: 0}
}
