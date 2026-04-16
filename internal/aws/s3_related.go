// s3_related.go contains S3 bucket related-resource checker functions.
package aws

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkS3Lambda returns the Lambda function ARNs referenced by this bucket's
// notification configuration. The bucket fetcher populates Fields["notification_lambda"]
// via GetBucketNotificationConfiguration (first Lambda target). Forward lookup
// (Pattern F): no cache needed, but the fetcher must have run with the notification
// API enabled for the field to be set. When the field is absent we can't tell
// the difference between "no notifications" and "notifications not enriched":
// Count: 0 is safest because the fetcher either enriches or leaves it "".
func checkS3Lambda(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	arn := res.Fields["notification_lambda"]
	if arn == "" {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: 0}
	}
	// Lambda ARN format: arn:aws:lambda:region:account:function:NAME[:VERSION]
	parts := strings.Split(arn, ":")
	if len(parts) < 7 {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: 0}
	}
	name := parts[6]
	if name == "" {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: 0}
	}
	return relatedResult("lambda", []string{name})
}

// checkS3SNS returns the SNS topic from the bucket's notification configuration,
// populated in Fields["notification_sns"] by GetBucketNotificationConfiguration.
func checkS3SNS(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	arn := res.Fields["notification_sns"]
	if arn == "" {
		return resource.RelatedCheckResult{TargetType: "sns", Count: 0}
	}
	// SNS topic ARN: arn:aws:sns:region:account:TopicName
	parts := strings.Split(arn, ":")
	if len(parts) < 6 {
		return resource.RelatedCheckResult{TargetType: "sns", Count: 0}
	}
	name := parts[5]
	if name == "" {
		return resource.RelatedCheckResult{TargetType: "sns", Count: 0}
	}
	return relatedResult("sns", []string{name})
}

// checkS3SQS returns the SQS queue from the bucket's notification configuration,
// populated in Fields["notification_sqs"] by GetBucketNotificationConfiguration.
func checkS3SQS(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	arn := res.Fields["notification_sqs"]
	if arn == "" {
		return resource.RelatedCheckResult{TargetType: "sqs", Count: 0}
	}
	// SQS queue ARN: arn:aws:sqs:region:account:QueueName
	parts := strings.Split(arn, ":")
	if len(parts) < 6 {
		return resource.RelatedCheckResult{TargetType: "sqs", Count: 0}
	}
	name := parts[5]
	if name == "" {
		return resource.RelatedCheckResult{TargetType: "sqs", Count: 0}
	}
	return relatedResult("sqs", []string{name})
}

// checkS3CFN calls s3:GetBucketTagging to read the bucket's tags and looks up
// the aws:cloudformation:stack-name value in the cfn cache. Pattern C —
// single per-bucket API call on detail-view open.
func checkS3CFN(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	bucket := res.ID
	if bucket == "" {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: 0}
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.S3 == nil {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1}
	}
	tagAPI, ok := c.S3.(S3GetBucketTaggingAPI)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1}
	}
	out, err := tagAPI.GetBucketTagging(ctx, &s3.GetBucketTaggingInput{Bucket: aws.String(bucket)})
	if err != nil {
		// NoSuchTagSet is a "no tags" response, not a hard failure.
		if strings.Contains(err.Error(), "NoSuchTagSet") {
			return resource.RelatedCheckResult{TargetType: "cfn", Count: 0}
		}
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1, Err: err}
	}
	stackName := ""
	for _, tag := range out.TagSet {
		if tag.Key != nil && *tag.Key == "aws:cloudformation:stack-name" && tag.Value != nil {
			stackName = *tag.Value
			break
		}
	}
	if stackName == "" {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: 0}
	}
	cfnList, truncated, err := s3RelatedResources(ctx, clients, cache, "cfn")
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

// checkS3KMS calls s3:GetBucketEncryption and returns the KMS key ID configured
// for server-side encryption (if SSEAlgorithm is aws:kms). Pattern C — single
// per-bucket API call.
func checkS3KMS(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	bucket := res.ID
	if bucket == "" {
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.S3 == nil {
		return resource.RelatedCheckResult{TargetType: "kms", Count: -1}
	}
	encAPI, ok := c.S3.(S3GetBucketEncryptionAPI)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "kms", Count: -1}
	}
	out, err := encAPI.GetBucketEncryption(ctx, &s3.GetBucketEncryptionInput{Bucket: aws.String(bucket)})
	if err != nil {
		// ServerSideEncryptionConfigurationNotFoundError means no encryption — honest 0.
		if strings.Contains(err.Error(), "ServerSideEncryptionConfigurationNotFoundError") {
			return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
		}
		return resource.RelatedCheckResult{TargetType: "kms", Count: -1, Err: err}
	}
	if out.ServerSideEncryptionConfiguration == nil {
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
	}
	var ids []string
	for _, rule := range out.ServerSideEncryptionConfiguration.Rules {
		if rule.ApplyServerSideEncryptionByDefault == nil {
			continue
		}
		keyID := ""
		if rule.ApplyServerSideEncryptionByDefault.KMSMasterKeyID != nil {
			keyID = *rule.ApplyServerSideEncryptionByDefault.KMSMasterKeyID
		}
		if keyID == "" {
			continue
		}
		// KMSMasterKeyID may be a full ARN (arn:aws:kms:…:key/ID) or bare ID/alias.
		if idx := strings.LastIndex(keyID, "/"); idx >= 0 && idx < len(keyID)-1 {
			keyID = keyID[idx+1:]
		}
		ids = append(ids, keyID)
	}
	return relatedResult("kms", ids)
}

// checkS3Logs calls s3:GetBucketLogging and returns the target bucket
// configured as the server-access-log destination. Pattern C — single
// per-bucket API call. The target is an S3 bucket (not a CloudWatch log
// group), so we emit a "logs"-targeted entry IDed by the destination bucket.
func checkS3Logs(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	bucket := res.ID
	if bucket == "" {
		return resource.RelatedCheckResult{TargetType: "logs", Count: 0}
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.S3 == nil {
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1}
	}
	logAPI, ok := c.S3.(S3GetBucketLoggingAPI)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1}
	}
	out, err := logAPI.GetBucketLogging(ctx, &s3.GetBucketLoggingInput{Bucket: aws.String(bucket)})
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1, Err: err}
	}
	if out.LoggingEnabled == nil || out.LoggingEnabled.TargetBucket == nil || *out.LoggingEnabled.TargetBucket == "" {
		return resource.RelatedCheckResult{TargetType: "logs", Count: 0}
	}
	return relatedResult("logs", []string{*out.LoggingEnabled.TargetBucket})
}

// checkS3Athena scans the athena cache for WorkGroups whose enriched
// Fields["result_output_location"] references this bucket. When the cache
// lacks the enrichment (common path), we emit Count: 0 — no known reference
// in cached data — rather than -1, because the scan itself is complete.
func checkS3Athena(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	bucket := res.ID
	if bucket == "" {
		return resource.RelatedCheckResult{TargetType: "athena", Count: 0}
	}
	wgList, truncated, err := s3RelatedResources(ctx, clients, cache, "athena")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "athena", Count: -1, Err: err}
	}
	if wgList == nil {
		return resource.RelatedCheckResult{TargetType: "athena", Count: -1}
	}
	var ids []string
	for _, wg := range wgList {
		if bucketFromS3URI(wg.Fields["result_output_location"]) == bucket {
			ids = append(ids, wg.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "athena", Count: -1}
	}
	return relatedResult("athena", ids)
}

// checkS3Glue scans the glue cache for Jobs whose Command.ScriptLocation
// (s3://bucket/...) matches this bucket.
func checkS3Glue(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	bucket := res.ID
	if bucket == "" {
		return resource.RelatedCheckResult{TargetType: "glue", Count: 0}
	}
	jobList, truncated, err := s3RelatedResources(ctx, clients, cache, "glue")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "glue", Count: -1, Err: err}
	}
	if jobList == nil {
		return resource.RelatedCheckResult{TargetType: "glue", Count: -1}
	}
	var ids []string
	for _, jobRes := range jobList {
		job, ok := assertStruct[gluetypes.Job](jobRes.RawStruct)
		if !ok || job.Command == nil || job.Command.ScriptLocation == nil {
			continue
		}
		if bucketFromS3URI(*job.Command.ScriptLocation) == bucket {
			ids = append(ids, jobRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "glue", Count: -1}
	}
	return relatedResult("glue", ids)
}

// checkS3Backup scans the backup cache for recovery points whose ResourceArn
// identifies this bucket (arn:aws:s3:::BUCKET). The backup cache holds backup
// plans/jobs, not per-resource recovery points — so a full scan rarely matches.
// When the cache lacks the needed shape, Count: 0 (scan complete, no hits).
func checkS3Backup(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	bucket := res.ID
	if bucket == "" {
		return resource.RelatedCheckResult{TargetType: "backup", Count: 0}
	}
	bucketARN := "arn:aws:s3:::" + bucket
	bkList, truncated, err := s3RelatedResources(ctx, clients, cache, "backup")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "backup", Count: -1, Err: err}
	}
	if bkList == nil {
		return resource.RelatedCheckResult{TargetType: "backup", Count: -1}
	}
	var ids []string
	for _, bk := range bkList {
		if bk.Fields["resource_arn"] == bucketARN || strings.Contains(bk.Fields["resources"], bucketARN) {
			ids = append(ids, bk.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "backup", Count: -1}
	}
	return relatedResult("backup", ids)
}

// checkS3EBRule scans the eb-rule cache for rules whose enriched
// Fields["target_arns"] contains this bucket's ARN. Target ARNs are not part
// of ListRules response; they arrive via the eb_rule_targets child view or
// future enrichment. Count: 0 when no match in cache.
func checkS3EBRule(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	bucket := res.ID
	if bucket == "" {
		return resource.RelatedCheckResult{TargetType: "eb-rule", Count: 0}
	}
	bucketARN := "arn:aws:s3:::" + bucket
	ruleList, truncated, err := s3RelatedResources(ctx, clients, cache, "eb-rule")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "eb-rule", Count: -1, Err: err}
	}
	if ruleList == nil {
		return resource.RelatedCheckResult{TargetType: "eb-rule", Count: -1}
	}
	var ids []string
	for _, ruleRes := range ruleList {
		if strings.Contains(ruleRes.Fields["target_arns"], bucketARN) {
			ids = append(ids, ruleRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "eb-rule", Count: -1}
	}
	return relatedResult("eb-rule", ids)
}

// checkS3IAMUser scans the iam-user cache for users whose enriched policy
// documents mention this bucket's ARN. Policy bodies are not loaded into the
// user list by default — ListUsers returns only summary metadata. Count: 0
// when no match.
func checkS3IAMUser(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	bucket := res.ID
	if bucket == "" {
		return resource.RelatedCheckResult{TargetType: "iam-user", Count: 0}
	}
	bucketARN := "arn:aws:s3:::" + bucket
	userList, truncated, err := s3RelatedResources(ctx, clients, cache, "iam-user")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "iam-user", Count: -1, Err: err}
	}
	if userList == nil {
		return resource.RelatedCheckResult{TargetType: "iam-user", Count: -1}
	}
	var ids []string
	for _, userRes := range userList {
		if strings.Contains(userRes.Fields["policy_resources"], bucketARN) {
			ids = append(ids, userRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "iam-user", Count: -1}
	}
	return relatedResult("iam-user", ids)
}

// checkS3R53 scans the r53 cache for hosted zones whose enriched
// Fields["alias_targets"] reference this bucket's website endpoint (BUCKET.s3-website*).
// Hosted-zone records are lazy-loaded per zone; bucket-pointing aliases rarely
// appear in the summary cache. Count: 0 when no match.
func checkS3R53(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	bucket := res.ID
	if bucket == "" {
		return resource.RelatedCheckResult{TargetType: "r53", Count: 0}
	}
	zoneList, truncated, err := s3RelatedResources(ctx, clients, cache, "r53")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "r53", Count: -1, Err: err}
	}
	if zoneList == nil {
		return resource.RelatedCheckResult{TargetType: "r53", Count: -1}
	}
	probe := bucket + ".s3"
	var ids []string
	for _, zone := range zoneList {
		if strings.Contains(zone.Fields["alias_targets"], probe) {
			ids = append(ids, zone.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "r53", Count: -1}
	}
	return relatedResult("r53", ids)
}

// checkS3Role scans the role cache for roles whose enriched
// Fields["policy_resources"] mention this bucket's ARN. Policies are not
// loaded in the role list by default. Count: 0 when no match.
func checkS3Role(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	bucket := res.ID
	if bucket == "" {
		return resource.RelatedCheckResult{TargetType: "role", Count: 0}
	}
	bucketARN := "arn:aws:s3:::" + bucket
	roleList, truncated, err := s3RelatedResources(ctx, clients, cache, "role")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "role", Count: -1, Err: err}
	}
	if roleList == nil {
		return resource.RelatedCheckResult{TargetType: "role", Count: -1}
	}
	var ids []string
	for _, roleRes := range roleList {
		if strings.Contains(roleRes.Fields["policy_resources"], bucketARN) {
			ids = append(ids, roleRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "role", Count: -1}
	}
	return relatedResult("role", ids)
}

// checkS3WAF scans the waf cache for WebACLs whose enriched
// Fields["log_destination_arns"] reference this bucket. WAF LoggingConfiguration
// destinations are not in the summary list; requires per-ACL GetLoggingConfiguration
// enrichment. Count: 0 when no match in cached data.
func checkS3WAF(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	bucket := res.ID
	if bucket == "" {
		return resource.RelatedCheckResult{TargetType: "waf", Count: 0}
	}
	bucketARN := "arn:aws:s3:::" + bucket
	wafList, truncated, err := s3RelatedResources(ctx, clients, cache, "waf")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "waf", Count: -1, Err: err}
	}
	if wafList == nil {
		return resource.RelatedCheckResult{TargetType: "waf", Count: -1}
	}
	var ids []string
	for _, wafRes := range wafList {
		if strings.Contains(wafRes.Fields["log_destination_arns"], bucketARN) {
			ids = append(ids, wafRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "waf", Count: -1}
	}
	return relatedResult("waf", ids)
}

// checkS3Trail searches the trail cache for trails whose S3BucketName matches
// this bucket name. S3 is a reverse-lookup hub — trails reference S3 buckets,
// not the other way around.
func checkS3Trail(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	bucketName := res.ID
	if bucketName == "" {
		return resource.RelatedCheckResult{TargetType: "trail", Count: 0}
	}

	trailList, truncated, err := s3RelatedResources(ctx, clients, cache, "trail")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "trail", Count: -1, Err: err}
	}
	if trailList == nil {
		return resource.RelatedCheckResult{TargetType: "trail", Count: -1}
	}

	var ids []string
	for _, trailRes := range trailList {
		trail, ok := assertStruct[cloudtrailtypes.Trail](trailRes.RawStruct)
		if !ok {
			continue
		}
		if trail.S3BucketName == nil || *trail.S3BucketName == "" {
			continue
		}
		if *trail.S3BucketName == bucketName {
			ids = append(ids, trailRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "trail", Count: -1}
	}
	return relatedResult("trail", ids)
}

// checkS3CF searches the CloudFront cache for distributions with origins that
// reference this S3 bucket. Origin DomainName formats:
//   - {bucket}.s3.amazonaws.com
//   - {bucket}.s3-website.{region}.amazonaws.com
//   - {bucket}.s3.{region}.amazonaws.com
func checkS3CF(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	bucketName := res.ID
	if bucketName == "" {
		return resource.RelatedCheckResult{TargetType: "cf", Count: 0}
	}

	cfList, truncated, err := s3RelatedResources(ctx, clients, cache, "cf")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "cf", Count: -1, Err: err}
	}
	if cfList == nil {
		return resource.RelatedCheckResult{TargetType: "cf", Count: -1}
	}

	var ids []string
	for _, cfRes := range cfList {
		dist, ok := assertStruct[cftypes.DistributionSummary](cfRes.RawStruct)
		if !ok {
			continue
		}
		if dist.Origins == nil {
			continue
		}
		for _, origin := range dist.Origins.Items {
			if origin.DomainName == nil {
				continue
			}
			if strings.Contains(*origin.DomainName, bucketName+".s3") {
				ids = append(ids, cfRes.ID)
				break
			}
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "cf", Count: -1}
	}
	return relatedResult("cf", ids)
}

// s3RelatedResources returns the resource list for target from cache or by
// fetching the first page via the registered paginated fetcher.
func s3RelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}

func init() {
	resource.RegisterRelated("s3", []resource.RelatedDef{
		{TargetType: "trail", DisplayName: "CloudTrail Trails", Checker: checkS3Trail, NeedsTargetCache: true},
		{TargetType: "cf", DisplayName: "CloudFront", Checker: checkS3CF, NeedsTargetCache: true},
		{TargetType: "lambda", DisplayName: "Lambda (notifications)", Checker: checkS3Lambda},
		{TargetType: "sns", DisplayName: "SNS (notifications)", Checker: checkS3SNS},
		{TargetType: "sqs", DisplayName: "SQS (notifications)", Checker: checkS3SQS},
		{TargetType: "cfn", DisplayName: "CloudFormation", Checker: checkS3CFN},
		{TargetType: "kms", DisplayName: "KMS Key", Checker: checkS3KMS},
		{TargetType: "logs", DisplayName: "Log Groups", Checker: checkS3Logs},
		{TargetType: "athena", DisplayName: "Athena WorkGroups", Checker: checkS3Athena},
		{TargetType: "glue", DisplayName: "Glue Jobs", Checker: checkS3Glue},
		{TargetType: "backup", DisplayName: "Backup", Checker: checkS3Backup},
		{TargetType: "eb-rule", DisplayName: "EventBridge Rules", Checker: checkS3EBRule},
		{TargetType: "iam-user", DisplayName: "IAM Users", Checker: checkS3IAMUser},
		{TargetType: "r53", DisplayName: "Route 53", Checker: checkS3R53},
		{TargetType: "role", DisplayName: "IAM Roles", Checker: checkS3Role},
		{TargetType: "waf", DisplayName: "WAF", Checker: checkS3WAF},
	})
}
