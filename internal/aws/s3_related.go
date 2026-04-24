// s3_related.go contains S3 bucket related-resource checker functions.
package aws

import (
	"context"
	"encoding/json"
	"slices"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
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
// The SNS fetcher indexes Resource.ID by full topic ARN (sns.go — TopicArn),
// so this checker must return the ARN unchanged — stripping to the bare topic
// name breaks drill-through.
func checkS3SNS(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	arn := res.Fields["notification_sns"]
	if arn == "" {
		return resource.RelatedCheckResult{TargetType: "sns", Count: 0}
	}
	// Basic ARN shape guard — arn:aws:sns:region:account:TopicName has 6 parts.
	if parts := strings.Split(arn, ":"); len(parts) < 6 || parts[5] == "" {
		return resource.RelatedCheckResult{TargetType: "sns", Count: 0}
	}
	return relatedResult("sns", []string{arn})
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
		return resource.ApproximateZero("cfn")
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
		return resource.ApproximateZero("cfn")
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

// checkS3Logs calls s3:GetBucketLogging and returns the destination S3 bucket
// configured to receive this bucket's server-access logs. Pattern C — single
// per-bucket API call. S3 server-access logs are delivered to ANOTHER S3
// BUCKET (not CloudWatch Log Groups), so the pivot targets `s3` — the
// destination resource kind — and the navigation ID is the destination
// bucket name. Spec §2 `logs` subsection + a9s-devops (2026-04-20).
func checkS3Logs(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	bucket := res.ID
	if bucket == "" {
		return resource.RelatedCheckResult{TargetType: "s3", Count: 0}
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.S3 == nil {
		return resource.RelatedCheckResult{TargetType: "s3", Count: -1}
	}
	logAPI, ok := c.S3.(S3GetBucketLoggingAPI)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "s3", Count: -1}
	}
	out, err := logAPI.GetBucketLogging(ctx, &s3.GetBucketLoggingInput{Bucket: aws.String(bucket)})
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "s3", Count: -1, Err: err}
	}
	if out.LoggingEnabled == nil || out.LoggingEnabled.TargetBucket == nil || *out.LoggingEnabled.TargetBucket == "" {
		return resource.RelatedCheckResult{TargetType: "s3", Count: 0}
	}
	return relatedResult("s3", []string{*out.LoggingEnabled.TargetBucket})
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
		return resource.ApproximateZero("athena")
	}
	var ids []string
	for _, wg := range wgList {
		if bucketFromS3URI(wg.Fields["result_output_location"]) == bucket {
			ids = append(ids, wg.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("athena")
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
		return resource.ApproximateZero("glue")
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
		return resource.ApproximateZero("glue")
	}
	return relatedResult("glue", ids)
}

// checkS3Backup scans the backup cache for plans that cover this bucket.
// Two matching paths are applied per cached plan:
//   - Legacy: Fields["resource_arn"] exactly equals the bucket ARN
//     (recovery-point-shaped cache entries; unrelated to BackupSelection).
//   - Selection: BackupPlanCoversARN checks Fields["resources"] (may contain
//     wildcard patterns such as arn:aws:s3:::*) and Fields["not_resources"]
//     (exclusion list). A plan covers this bucket iff any Resources entry
//     matches AND no NotResources entry matches.
//
// No live API call is made — this is a pure cache scan.
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
		return resource.ApproximateZero("backup")
	}
	var ids []string
	for _, bk := range bkList {
		if bk.Fields["resource_arn"] == bucketARN || BackupPlanCoversARN(bk.Fields["resources"], bk.Fields["not_resources"], bucketARN) {
			ids = append(ids, bk.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("backup")
	}
	return relatedResult("backup", ids)
}

// checkS3EBRule scans the eb-rule cache for rules whose EventPattern filters
// on `source=aws.s3` AND `detail.bucket.name` containing this bucket. Spec
// §2: "rules with EventPattern.source=['aws.s3'] AND EventPattern.detail.
// bucket.name matching this bucket". Event-pattern is the only standard
// join between an S3 bucket and an EventBridge rule (per a9s-devops).
func checkS3EBRule(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	bucket := res.ID
	if bucket == "" {
		return resource.RelatedCheckResult{TargetType: "eb-rule", Count: 0}
	}
	ruleList, truncated, err := s3RelatedResources(ctx, clients, cache, "eb-rule")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "eb-rule", Count: -1, Err: err}
	}
	if ruleList == nil {
		return resource.ApproximateZero("eb-rule")
	}
	bucketQuoted := `"` + bucket + `"`
	var ids []string
	for _, ruleRes := range ruleList {
		pattern := ruleRes.Fields["event_pattern"]
		if pattern == "" {
			continue
		}
		if !strings.Contains(pattern, `"aws.s3"`) {
			continue
		}
		if !strings.Contains(pattern, bucketQuoted) {
			continue
		}
		ids = append(ids, ruleRes.ID)
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("eb-rule")
	}
	return relatedResult("eb-rule", ids)
}

// checkS3R53 scans the r53 cache for hosted zones containing an S3-website
// alias record whose NAME (FQDN) equals this bucket's name. Spec §2: "alias
// to S3 requires bucket-name==FQDN; that's the join key" — the bucket name
// is NEVER part of AliasTarget.DNSName (AWS returns the regional endpoint).
// The r53 fetcher pre-filters the zone's records for S3-website aliases
// and emits the FQDNs as Fields["s3website_alias_names"].
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
		return resource.ApproximateZero("r53")
	}
	var ids []string
	for _, zone := range zoneList {
		names := zone.Fields["s3website_alias_names"]
		if names == "" {
			continue
		}
		if slices.Contains(strings.Split(names, ","), bucket) {
			ids = append(ids, zone.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("r53")
	}
	return relatedResult("r53", ids)
}

// checkS3Role resolves roles named as AWS principals in the bucket's
// resource policy. Spec §2: "Call s3:GetBucketPolicy, parse the JSON
// policy document for Statement[].Principal.AWS entries matching IAM
// role ARNs, look each up in the already-loaded `role` list." This is
// the canonical direction of the relationship — the access grant lives
// on the bucket side, not on the role's own policies.
//
// Wildcards, service principals, and cross-account role ARNs that do
// not resolve in the local `role` cache are ignored: we only surface
// concrete roles the operator can actually navigate to.
func checkS3Role(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	bucket := res.ID
	if bucket == "" {
		return resource.RelatedCheckResult{TargetType: "role", Count: 0}
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.S3 == nil {
		return resource.RelatedCheckResult{TargetType: "role", Count: -1}
	}
	policyAPI, ok := c.S3.(S3GetBucketPolicyAPI)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "role", Count: -1}
	}
	out, err := policyAPI.GetBucketPolicy(ctx, &s3.GetBucketPolicyInput{Bucket: aws.String(bucket)})
	if err != nil {
		// NoSuchBucketPolicy is a legitimate "no policy configured"
		// response — honest 0, not error.
		if strings.Contains(err.Error(), "NoSuchBucketPolicy") {
			return resource.RelatedCheckResult{TargetType: "role", Count: 0}
		}
		return resource.RelatedCheckResult{TargetType: "role", Count: -1, Err: err}
	}
	if out == nil || out.Policy == nil || *out.Policy == "" {
		return resource.RelatedCheckResult{TargetType: "role", Count: 0}
	}

	principalARNs := extractBucketPolicyAWSPrincipals(*out.Policy)
	if len(principalARNs) == 0 {
		return resource.RelatedCheckResult{TargetType: "role", Count: 0}
	}

	roleList, truncated, rerr := s3RelatedResources(ctx, clients, cache, "role")
	if rerr != nil {
		return resource.RelatedCheckResult{TargetType: "role", Count: -1, Err: rerr}
	}
	if roleList == nil {
		return resource.ApproximateZero("role")
	}

	// Match role ARNs against the loaded role cache. Anything that
	// doesn't resolve locally (wildcards, cross-account, services) is
	// dropped — the pivot is "navigate to this role in the list".
	var ids []string
	for _, principalARN := range principalARNs {
		name := roleNameFromARN(principalARN)
		if name == "" {
			continue
		}
		for _, roleRes := range roleList {
			if roleRes.ID == name || roleRes.Name == name {
				ids = append(ids, roleRes.ID)
				break
			}
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("role")
	}
	return relatedResult("role", ids)
}

// extractBucketPolicyAWSPrincipals parses a bucket-policy JSON document
// and returns every concrete Statement[].Principal.AWS role-ARN it names.
// Accepts the AWS-canonical shapes (string, []string) and filters to
// entries that look like IAM role ARNs; wildcards ("*"), service
// principals ({Service: ...}), and malformed entries are dropped.
func extractBucketPolicyAWSPrincipals(doc string) []string {
	var parsed struct {
		Statement []struct {
			Principal any `json:"Principal"`
		} `json:"Statement"`
	}
	if err := json.Unmarshal([]byte(doc), &parsed); err != nil {
		return nil
	}
	var arns []string
	for _, stmt := range parsed.Statement {
		// Principal may be a string "*" or a map {"AWS": ..., "Service": ...}.
		m, ok := stmt.Principal.(map[string]any)
		if !ok {
			continue
		}
		aws := m["AWS"]
		switch v := aws.(type) {
		case string:
			if isIAMRoleARN(v) {
				arns = append(arns, v)
			}
		case []any:
			for _, item := range v {
				if s, ok := item.(string); ok && isIAMRoleARN(s) {
					arns = append(arns, s)
				}
			}
		}
	}
	return arns
}

// isIAMRoleARN reports whether s looks like an IAM role ARN
// (arn:aws:iam::<account>:role/<name>). Rejects wildcards, account-root
// ARNs, and user ARNs — the role pivot only surfaces role principals.
// Role name extraction reuses roleNameFromARN from iam_roles_related.go.
func isIAMRoleARN(s string) bool {
	return strings.HasPrefix(s, "arn:aws:iam::") && strings.Contains(s, ":role/")
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
		return resource.ApproximateZero("trail")
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
		return resource.ApproximateZero("trail")
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
		return resource.ApproximateZero("cf")
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
		return resource.ApproximateZero("cf")
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
		{TargetType: "s3", DisplayName: "Access Log Bucket", Checker: checkS3Logs},
		{TargetType: "athena", DisplayName: "Athena WorkGroups", Checker: checkS3Athena},
		{TargetType: "glue", DisplayName: "Glue Jobs", Checker: checkS3Glue},
		{TargetType: "backup", DisplayName: "Backup", Checker: checkS3Backup},
		{TargetType: "eb-rule", DisplayName: "EventBridge Rules", Checker: checkS3EBRule},
		{TargetType: "r53", DisplayName: "Route 53", Checker: checkS3R53},
		{TargetType: "role", DisplayName: "IAM Roles", Checker: checkS3Role},
	})
}
