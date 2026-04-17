// opensearch_related.go contains OpenSearch Domain related-resource checker functions.
package aws

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/opensearch"
	opensearchtypes "github.com/aws/aws-sdk-go-v2/service/opensearch/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterRelated("opensearch", []resource.RelatedDef{
		{TargetType: "alarm", DisplayName: "CW Alarms", Checker: checkOpenSearchAlarms, NeedsTargetCache: true},
		{TargetType: "logs", DisplayName: "Log Groups", Checker: checkOpenSearchLogs, NeedsTargetCache: false},
		{TargetType: "sg", DisplayName: "Security Groups", Checker: checkOpenSearchSG},
		{TargetType: "vpc", DisplayName: "VPC", Checker: checkOpenSearchVPC},
		{TargetType: "kms", DisplayName: "KMS Key", Checker: checkOpenSearchKMS},
		{TargetType: "cfn", DisplayName: "CloudFormation", Checker: checkOpenSearchCFN},
		{TargetType: "subnet", DisplayName: "Subnets", Checker: checkOpenSearchSubnet},
		{TargetType: "acm", DisplayName: "ACM Certificates", Checker: checkOpenSearchACM},
		{TargetType: "role", DisplayName: "IAM Roles", Checker: checkOpenSearchRole},
	})

	// opensearchtypes.DomainStatus: EncryptionAtRestOptions.KmsKeyId
	// VPCOptions: VPCId, SubnetIds, SecurityGroupIds
	resource.RegisterNavigableFields("opensearch", []resource.NavigableField{
		{FieldPath: "EncryptionAtRestOptions.KmsKeyId", TargetType: "kms"},
		{FieldPath: "VPCOptions.VPCId", TargetType: "vpc"},
		{FieldPath: "VPCOptions.SubnetIds", TargetType: "subnet"},
		{FieldPath: "VPCOptions.SecurityGroupIds", TargetType: "sg"},
	})
}


// checkOpenSearchAlarms checks the cache for CloudWatch alarms with DomainName dimension matching this domain.
func checkOpenSearchAlarms(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	domainName := res.ID
	if domainName == "" {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: 0}
	}

	alarmList, truncated, err := opensearchRelatedResources(ctx, clients, cache, "alarm")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1, Err: err}
	}
	if alarmList == nil {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1}
	}

	var ids []string
	for _, alarmRes := range alarmList {
		rawAlarm, ok := assertStruct[cwtypes.MetricAlarm](alarmRes.RawStruct)
		if !ok {
			continue
		}
		for _, d := range rawAlarm.Dimensions {
			if d.Name != nil && *d.Name == "DomainName" && d.Value != nil && *d.Value == domainName {
				ids = append(ids, alarmRes.ID)
				break
			}
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1}
	}
	return relatedResult("alarm", ids)
}

// checkOpenSearchLogs extracts CloudWatch log group ARNs from the domain's LogPublishingOptions.
// Pattern F — reads from RawStruct, no cache needed.
func checkOpenSearchLogs(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	domain, ok := assertStruct[opensearchtypes.DomainStatus](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1}
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
		return resource.RelatedCheckResult{TargetType: "sg", Count: -1}
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
		return resource.RelatedCheckResult{TargetType: "vpc", Count: -1}
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
	if !ok || domain.EncryptionAtRestOptions == nil ||
		domain.EncryptionAtRestOptions.KmsKeyId == nil ||
		*domain.EncryptionAtRestOptions.KmsKeyId == "" {
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
	}
	keyID := *domain.EncryptionAtRestOptions.KmsKeyId
	if idx := strings.LastIndex(keyID, "/"); idx >= 0 && idx < len(keyID)-1 {
		keyID = keyID[idx+1:]
	}
	return relatedResult("kms", []string{keyID})
}

// checkOpenSearchCFN calls opensearch:ListTags(ARN=DomainStatus.ARN) and
// looks up the aws:cloudformation:stack-name tag in the cfn cache. Pattern C.
func checkOpenSearchCFN(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	domain, ok := assertStruct[opensearchtypes.DomainStatus](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1}
	}
	if domain.ARN == nil || *domain.ARN == "" {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: 0}
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.OpenSearch == nil {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1}
	}
	tagAPI, ok := c.OpenSearch.(OpenSearchListTagsAPI)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1}
	}
	out, err := tagAPI.ListTags(ctx, &opensearch.ListTagsInput{ARN: aws.String(*domain.ARN)})
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1, Err: err}
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
	cfnList, truncated, err := opensearchRelatedResources(ctx, clients, cache, "cfn")
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
		rawCFN, cfnOk := assertStruct[cfntypes.Stack](cfnRes.RawStruct)
		if cfnOk && rawCFN.StackName != nil && *rawCFN.StackName == stackName {
			ids = append(ids, cfnRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1}
	}
	return relatedResult("cfn", ids)
}

// checkOpenSearchSubnet returns the subnets the VPC-attached domain is deployed
// into (VPCOptions.SubnetIds). Pattern F — no cache needed.
func checkOpenSearchSubnet(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	domain, ok := assertStruct[opensearchtypes.DomainStatus](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: -1}
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
// ACM certificate ARN attached to the domain's custom endpoint
// (DomainEndpointOptions.Options.CustomEndpointCertificateArn). Pattern C.
func checkOpenSearchACM(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	domainName := res.ID
	if domainName == "" {
		return resource.RelatedCheckResult{TargetType: "acm", Count: 0}
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.OpenSearch == nil {
		return resource.RelatedCheckResult{TargetType: "acm", Count: -1}
	}
	cfgAPI, ok := c.OpenSearch.(OpenSearchDescribeDomainConfigAPI)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "acm", Count: -1}
	}
	out, err := cfgAPI.DescribeDomainConfig(ctx, &opensearch.DescribeDomainConfigInput{DomainName: aws.String(domainName)})
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "acm", Count: -1, Err: err}
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
	// ACM cert ARN: arn:aws:acm:REGION:ACCOUNT:certificate/ID — ID is the last segment.
	certID := arn
	if idx := strings.LastIndex(arn, "/"); idx >= 0 && idx < len(arn)-1 {
		certID = arn[idx+1:]
	}
	return relatedResult("acm", []string{certID})
}

// opensearchRelatedResources returns the resource list for target from cache or by fetching the first page.
func opensearchRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}


