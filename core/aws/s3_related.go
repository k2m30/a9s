// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// s3_related.go contains S3 bucket related-resource checker functions.
package aws

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// s3BenignAbsenceErr reports whether err is a legitimate empty state for a
// per-bucket S3 sub-resource call, rather than a hard failure: either the
// specific "not configured" ErrorCode the caller expects (e.g.
// NoSuchTagSet), or the shared IsNotFoundErr classification for a bucket
// deleted between ListBuckets and this per-bucket call. Classification is by
// ErrorCode, not a message substring scan. code is "" for a checker with no
// per-call benign code of its own (GetBucketLogging, which never errors for
// "no logging configured" — it returns 200 with LoggingEnabled == nil
// instead).
func s3BenignAbsenceErr(err error, code string) bool {
	if IsNotFoundErr(err) {
		return true
	}
	if code == "" {
		return false
	}
	return ErrCodeIs(err, code)
}

// s3NotificationRelated turns one of the bucket fetcher's comma-joined
// notification fields into a related result. Forward lookup (Pattern F): no
// cache needed, but the fetcher must have run with the notification API
// enabled for the fields to be set.
//
// The lookup either answered (a list, empty when the bucket notifies nothing
// of this kind), was refused, or could not be made from this client's region.
// The last two are not zeros: a refusal is reported as the error it was, and a
// cross-region bucket soft-truncates so the row renders "0+".
//
// idOf maps a destination ARN to the target type's Resource.ID and rejects a
// shape that would navigate nowhere.
func s3NotificationRelated(
	target, field string,
	clients any,
	res resource.Resource,
	cache resource.ResourceCache,
) resource.RelatedCheckResult {
	if msg := res.Fields["notification_error"]; msg != "" {
		return ReadFailed(target, errors.New(msg))
	}
	if res.Fields["notification_truncated"] == "true" {
		return relatedResultTrunc(target, nil, true)
	}
	return relatedRefs(target, arnsOnly(strings.Split(res.Fields[field], ",")), refContext(clients, cache, target))
}

// checkS3Lambda returns the Lambda functions this bucket's notification
// configuration targets.
func checkS3Lambda(_ context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return s3NotificationRelated("lambda", "notification_lambda", clients, res, cache)
}

// checkS3SNS returns the SNS topics this bucket's notification configuration
// targets.
func checkS3SNS(_ context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return s3NotificationRelated("sns", "notification_sns", clients, res, cache)
}

// checkS3SQS returns the SQS queues this bucket's notification configuration
// targets.
func checkS3SQS(_ context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return s3NotificationRelated("sqs", "notification_sqs", clients, res, cache)
}

// s3BucketTags reads a bucket's tags; NoSuchTagSet is a bucket with none.
func s3BucketTags(ctx context.Context, c *ServiceClients, bucket string) (map[string]string, error) {
	if c == nil || c.S3 == nil {
		return nil, errClientMissing
	}
	api, ok := c.s3For(ctx, bucket).(S3GetBucketTaggingAPI)
	if !ok {
		return nil, errClientMissing
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*s3.GetBucketTaggingOutput, error) {
		return api.GetBucketTagging(ctx, &s3.GetBucketTaggingInput{Bucket: aws.String(bucket)})
	})
	if s3BenignAbsenceErr(err, "NoSuchTagSet") {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	tags := make(map[string]string, len(out.TagSet))
	for _, t := range out.TagSet {
		tags[aws.ToString(t.Key)] = aws.ToString(t.Value)
	}
	return tags, nil
}

// checkS3CFN calls s3:GetBucketTagging to read the bucket's tags and looks up
// the aws:cloudformation:stack-name value in the cfn cache. Pattern C —
// single per-bucket API call on detail-view open.
func checkS3CFN(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	bucket := res.ID
	if bucket == "" {
		return keyMissing("cfn", "bucket")
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.S3 == nil {
		return NotRead("cfn")
	}
	tagAPI, ok := c.s3For(ctx, bucket).(S3GetBucketTaggingAPI)
	if !ok {
		return NotRead("cfn")
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*s3.GetBucketTaggingOutput, error) {
		return tagAPI.GetBucketTagging(ctx, &s3.GetBucketTaggingInput{Bucket: aws.String(bucket)})
	})
	if err != nil {
		// NoSuchTagSet is a "no tags" response, and a deleted bucket is a
		// resolved zero — both are a benign absence, not a hard failure.
		if s3BenignAbsenceErr(err, "NoSuchTagSet") {
			return foundNone("cfn", "the API answered that none is configured")
		}
		// Cross-region buckets (PermanentRedirect / IllegalLocationConstraintException):
		// soft-truncate to "0+" rather than surface a hard unknown. See s3_cross_region.go.
		if isS3CrossRegionErr(err) {
			return relatedResultTrunc("cfn", nil, true)
		}
		return ReadFailed("cfn", err)
	}
	stackName := ""
	for _, tag := range out.TagSet {
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

// checkS3KMS calls s3:GetBucketEncryption and returns the KMS key ID configured
// for server-side encryption (if SSEAlgorithm is aws:kms). Pattern C — single
// per-bucket API call.
func checkS3KMS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	bucket := res.ID
	if bucket == "" {
		return keyMissing("kms", "bucket")
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.S3 == nil {
		return NotRead("kms")
	}
	// The key encrypting a bucket lives in the bucket's region, and is read
	// there.
	bucketRegion := c.bucketRegion(ctx, bucket)
	inBucketRegion := c.InRegion(bucketRegion)
	encAPI, ok := inBucketRegion.S3.(S3GetBucketEncryptionAPI)
	if !ok {
		return NotRead("kms")
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*s3.GetBucketEncryptionOutput, error) {
		return encAPI.GetBucketEncryption(ctx, &s3.GetBucketEncryptionInput{Bucket: aws.String(bucket)})
	})
	if err != nil {
		// ServerSideEncryptionConfigurationNotFoundError means no encryption,
		// and a deleted bucket is a resolved zero — both are honest 0s.
		if s3BenignAbsenceErr(err, "ServerSideEncryptionConfigurationNotFoundError") {
			return foundNone("kms", "the API answered that none is configured")
		}
		// Cross-region buckets (PermanentRedirect / IllegalLocationConstraintException):
		// soft-truncate to "0+" rather than surface a hard unknown. See s3_cross_region.go.
		if isS3CrossRegionErr(err) {
			return relatedResultTrunc("kms", nil, true)
		}
		return ReadFailed("kms", err)
	}
	if out.ServerSideEncryptionConfiguration == nil {
		return foundNone("kms", "out.ServerSideEncryptionConfiguration")
	}
	var ids []string
	for _, rule := range out.ServerSideEncryptionConfiguration.Rules {
		if rule.ApplyServerSideEncryptionByDefault == nil {
			continue
		}
		byDefault := rule.ApplyServerSideEncryptionByDefault
		keyID := aws.ToString(byDefault.KMSMasterKeyID)
		// SSE-KMS naming no key encrypts with the AWS managed key aws/s3.
		if keyID == "" && strings.HasPrefix(string(byDefault.SSEAlgorithm), "aws:kms") {
			keyID = "alias/aws/s3"
		}
		if keyID == "" {
			continue
		}
		// KMSMasterKeyID may be a full key ARN, a full alias ARN (including
		// AWS-managed aliases like "alias/aws/s3"), or a bare ID/alias.
		ids = append(ids, kmsRefFromField(keyID, res.Type))
	}
	return kmsRelatedIn(ctx, c, cache, bucketRegion, ids)
}

// checkS3Logs calls s3:GetBucketLogging and returns the destination S3 bucket
// configured to receive this bucket's server-access logs. Pattern C — single
// per-bucket API call. S3 server-access logs are delivered to ANOTHER S3
// BUCKET (not CloudWatch Log Groups), so the pivot targets `s3` — the
// destination resource kind — and the navigation ID is the destination
// bucket name.
func checkS3Logs(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	bucket := res.ID
	if bucket == "" {
		return keyMissing("s3", "bucket")
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.S3 == nil {
		return NotRead("s3")
	}
	logAPI, ok := c.s3For(ctx, bucket).(S3GetBucketLoggingAPI)
	if !ok {
		return NotRead("s3")
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*s3.GetBucketLoggingOutput, error) {
		return logAPI.GetBucketLogging(ctx, &s3.GetBucketLoggingInput{Bucket: aws.String(bucket)})
	})
	if err != nil {
		// A deleted bucket is a resolved zero, not a hard failure.
		if s3BenignAbsenceErr(err, "") {
			return foundNone("s3", "the API answered that none is configured")
		}
		// Cross-region buckets (PermanentRedirect / IllegalLocationConstraintException):
		// soft-truncate to "0+" rather than surface a hard unknown. See s3_cross_region.go.
		if isS3CrossRegionErr(err) {
			return relatedResultTrunc("s3", nil, true)
		}
		return ReadFailed("s3", err)
	}
	if out.LoggingEnabled == nil || out.LoggingEnabled.TargetBucket == nil || *out.LoggingEnabled.TargetBucket == "" {
		return foundNone("s3", "out.LoggingEnabled.TargetBucket")
	}
	return relatedResultTrunc("s3", []string{*out.LoggingEnabled.TargetBucket}, false)
}

// checkS3Athena scans the athena cache for WorkGroups whose enriched
// Fields["result_output_location"] references this bucket. When the cache
// lacks the enrichment (common path), we emit Count: 0 — no known reference
// in cached data — rather than -1, because the scan itself is complete.
func checkS3Athena(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	bucket := res.ID
	if bucket == "" {
		return keyMissing("athena", "bucket")
	}
	wgList, truncated, err := relatedResourcesFor(ctx, clients, cache, "athena")
	if err != nil {
		return ReadFailed("athena", err)
	}
	if wgList == nil {
		return NotRead("athena")
	}
	var ids []string
	for _, wg := range wgList {
		// A workgroup whose configuration was not read may well write here,
		// and its empty result location says nothing either way, so the count
		// is a lower bound.
		if !athenaConfigRead(wg) {
			truncated = true
			continue
		}
		if s3URINames(wg.Fields["result_output_location"], bucket) {
			ids = append(ids, wg.ID)
		}
	}
	return relatedResultTrunc("athena", ids, truncated)
}

// s3URINames reports whether an s3:// URI (or bucket ARN) names bucket.
func s3URINames(uri, bucket string) bool {
	id, ok := resource.ResolveRef("s3", uri, domain.RefContext{})
	return ok && id == bucket
}

// checkS3Glue scans the glue cache for Jobs whose Command.ScriptLocation
// (s3://bucket/...) matches this bucket.
func checkS3Glue(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	bucket := res.ID
	if bucket == "" {
		return keyMissing("glue", "bucket")
	}
	jobList, truncated, err := relatedResourcesFor(ctx, clients, cache, "glue")
	if err != nil {
		return ReadFailed("glue", err)
	}
	if jobList == nil {
		return NotRead("glue")
	}
	var ids []string
	for _, jobRes := range jobList {
		job, ok := assertStruct[gluetypes.Job](jobRes.RawStruct)
		if !ok || job.Command == nil || job.Command.ScriptLocation == nil {
			continue
		}
		if s3URINames(*job.Command.ScriptLocation, bucket) {
			ids = append(ids, jobRes.ID)
		}
	}
	return relatedResultTrunc("glue", ids, truncated)
}

// checkS3Backup scans the backup cache for plans that cover this bucket,
// through BackupPlanCovers. The row carries no tags, so a plan whose verdict
// turns on a tag clause leaves the answer undecided.
func checkS3Backup(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	bucket := res.ID
	if bucket == "" {
		return keyMissing("backup", "bucket")
	}
	// An S3 bucket ARN names no region, but it does name a partition, and the
	// partition comes from the session's region. Without one there is no ARN
	// to match on, and a commercial guess would read as a proven "no plan
	// covers this bucket" in every other partition.
	region := sessionRegion(clients)
	if region == "" {
		return NotRead("backup")
	}
	bucketARN := "arn:" + PartitionForRegion(region) + ":s3:::" + bucket
	bkList, truncated, err := relatedResourcesFor(ctx, clients, cache, "backup")
	if err != nil {
		return ReadFailed("backup", err)
	}
	if bkList == nil {
		return NotRead("backup")
	}
	// A plan backs up only buckets in its own Region, and ListBuckets returns
	// buckets from every Region.
	b, _ := assertStruct[s3types.Bucket](res.RawStruct)
	switch bucketRegion := aws.ToString(b.BucketRegion); {
	case len(bkList) == 0:
	case bucketRegion == "":
		return NotRead("backup")
	case bucketRegion != region:
		return relatedResultTrunc("backup", nil, false)
	}
	c, _ := clients.(*ServiceClients)
	return backupPivot(bkList, truncated, backupTarget{arn: bucketARN, unread: "GetBucketTagging"}.withTags(s3BucketTags(ctx, c, bucket)))
}

// checkS3EBRule counts the eb-rule rows whose event pattern matches an event
// S3 sends EventBridge about this bucket. The bucket's ARN names no region but
// does name the partition, which comes from the session's region.
func checkS3EBRule(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	bucket := res.ID
	if bucket == "" {
		return keyMissing("eb-rule", "bucket")
	}
	ruleList, truncated, err := relatedResourcesFor(ctx, clients, cache, "eb-rule")
	if err != nil {
		return ReadFailed("eb-rule", err)
	}
	if ruleList == nil {
		return NotRead("eb-rule")
	}
	bucketARN := ""
	if region := sessionRegion(clients); region != "" {
		bucketARN = "arn:" + PartitionForRegion(region) + ":s3:::" + bucket
	}
	return relatedAnswer("eb-rule", ebRulesMatching(ruleList, truncated, s3Events(bucket, bucketARN)))
}

// checkS3R53 scans the r53 cache for hosted zones containing an S3-website
// alias record whose NAME (FQDN) equals this bucket's name. An alias to S3
// requires bucket-name==FQDN, so that is the join key — the bucket name
// is NEVER part of AliasTarget.DNSName (AWS returns the regional endpoint).
// The r53 fetcher pre-filters the zone's records for S3-website aliases
// and emits the FQDNs as Fields["s3website_alias_names"].
func checkS3R53(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	bucket := res.ID
	if bucket == "" {
		return keyMissing("r53", "bucket")
	}
	zoneList, truncated, err := relatedResourcesFor(ctx, clients, cache, "r53")
	if err != nil {
		return ReadFailed("r53", err)
	}
	if zoneList == nil {
		return NotRead("r53")
	}
	var ids []string
	for _, zone := range zoneList {
		// The zone fetcher reads one page of record sets. When more remain,
		// this bucket's alias record may be on one of them, so the answer is a
		// lower bound — which is what r53 → s3 already renders for the same
		// zone, and the two directions of one relationship have to agree.
		if zone.Fields["records_truncated"] == "true" {
			truncated = true
		}
		names := zone.Fields["s3website_alias_names"]
		if names == "" {
			continue
		}
		if slices.Contains(strings.Split(names, ","), bucket) {
			ids = append(ids, zone.ID)
		}
	}
	return relatedResultTrunc("r53", ids, truncated)
}

// checkS3Role resolves the roles the bucket's resource policy
// (s3:GetBucketPolicy) grants, looked up in the already-loaded `role` list.
// This is the canonical direction of the relationship — the access grant
// lives on the bucket side, not on the role's own policies.
//
// Wildcards, service principals, and cross-account role ARNs that do
// not resolve in the local `role` cache are ignored: we only surface
// concrete roles the operator can actually navigate to.
func checkS3Role(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	bucket := res.ID
	if bucket == "" {
		return keyMissing("role", "bucket")
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.S3 == nil {
		return NotRead("role")
	}
	policyAPI, ok := c.s3For(ctx, bucket).(S3GetBucketPolicyAPI)
	if !ok {
		return NotRead("role")
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*s3.GetBucketPolicyOutput, error) {
		return policyAPI.GetBucketPolicy(ctx, &s3.GetBucketPolicyInput{Bucket: aws.String(bucket)})
	})
	if err != nil {
		// NoSuchBucketPolicy is a legitimate "no policy configured"
		// response, and a deleted bucket is a resolved zero — both are
		// honest 0s, not errors.
		if s3BenignAbsenceErr(err, "NoSuchBucketPolicy") {
			return foundNone("role", "the API answered that none is configured")
		}
		// Cross-region buckets (PermanentRedirect / IllegalLocationConstraintException):
		// soft-truncate to "0+" rather than surface a hard unknown. See s3_cross_region.go.
		if isS3CrossRegionErr(err) {
			return relatedResultTrunc("role", nil, true)
		}
		return ReadFailed("role", err)
	}
	if out == nil || out.Policy == nil || *out.Policy == "" {
		return foundNone("role", "out.Policy")
	}

	// A bucket ARN names no account, so only the session's can tell a
	// foreign role from a local one.
	rc := refContext(clients, cache, "role")
	roleARNs, ok := grantedPrincipalRefs(*out.Policy, "role/")
	if !ok {
		return NotRead("role")
	}
	if len(roleARNs) == 0 {
		return foundNone("role", "roleARNs")
	}

	roleList, _, rerr := relatedResourcesFor(ctx, clients, cache, "role")
	if rerr != nil {
		return ReadFailed("role", rerr)
	}
	if roleList == nil {
		return NotRead("role")
	}

	ids, lowerBound := listedRefs("role", roleARNs, rc, roleList)
	return relatedResultTrunc("role", ids, lowerBound)
}

// checkS3Trail searches the trail cache for trails whose S3BucketName matches
// this bucket name. S3 is a reverse-lookup hub — trails reference S3 buckets,
// not the other way around.
func checkS3Trail(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	bucketName := res.ID
	if bucketName == "" {
		return keyMissing("trail", "bucketName")
	}

	trailList, truncated, err := relatedResourcesFor(ctx, clients, cache, "trail")
	if err != nil {
		return ReadFailed("trail", err)
	}
	if trailList == nil {
		return NotRead("trail")
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
	return relatedResultTrunc("trail", ids, truncated)
}

// checkS3CF searches the CloudFront cache for distributions with origins that
// reference this S3 bucket. S3OriginBucket parses the origin hostname and the
// bucket it addresses must equal this one: a host that merely carries an "s3"
// token (a proxy at assets.s3-proxy.example.com) addresses no bucket at all.
func checkS3CF(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	bucketName := res.ID
	if bucketName == "" {
		return keyMissing("cf", "bucketName")
	}

	cfList, truncated, err := relatedResourcesFor(ctx, clients, cache, "cf")
	if err != nil {
		return ReadFailed("cf", err)
	}
	if cfList == nil {
		return NotRead("cf")
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
			if bucket, ok := S3OriginBucket(*origin.DomainName); ok && bucket == bucketName {
				ids = append(ids, cfRes.ID)
				break
			}
		}
	}
	return relatedResultTrunc("cf", ids, truncated)
}
