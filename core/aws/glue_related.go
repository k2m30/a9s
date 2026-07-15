// glue_related.go contains Glue Job related-resource checker functions.
package aws

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/glue"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkGlueRole extracts the Role from the Glue Job RawStruct. The value may be
// a full ARN (arn:aws:iam::…:role/name) or a plain role name. The role name is
// extracted from the last path segment of an ARN, or used directly if no "/" is present.
// The role cache is then searched by name.
func checkGlueRole(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	job, ok := assertStruct[gluetypes.Job](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("role")
	}
	if job.Role == nil || *job.Role == "" {
		return resource.RelatedCheckResult{TargetType: "role", Count: 0}
	}
	// In-body: the job's Role ARN normalizes to the role name (== the role's
	// Resource.ID). Resolve by identity — no role-list fetch.
	return relatedResult("role", []string{roleNameFromARN(*job.Role)})
}

// checkGlueAlarms searches the alarm cache for alarms with a "JobName" dimension
// matching this Glue job's name.
func checkGlueAlarms(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	jobName := res.Name
	if jobName == "" {
		jobName = res.ID
	}
	if jobName == "" {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: 0}
	}

	alarmList, truncated, err := relatedResourcesFor(ctx, clients, cache, "alarm")
	if err != nil {
		return resource.ErrorRelated("alarm", err)
	}
	if alarmList == nil {
		return resource.UnknownRelated("alarm")
	}

	var ids []string
	for _, alarmRes := range alarmList {
		rawAlarm, ok := assertStruct[cwtypes.MetricAlarm](alarmRes.RawStruct)
		if !ok {
			continue
		}
		for _, d := range rawAlarm.Dimensions {
			if d.Name != nil && *d.Name == "JobName" && d.Value != nil && *d.Value == jobName {
				ids = append(ids, alarmRes.ID)
				break
			}
		}
	}
	return relatedResultTrunc("alarm", ids, truncated)
}

// checkGlueLogs searches the logs cache for the shared Glue job log groups.
// Pattern N — Glue jobs write to /aws-glue/jobs/output and /aws-glue/jobs/error
// regardless of job name (shared log groups across all Glue jobs in the account).
func checkGlueLogs(ctx context.Context, clients any, _ resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	logList, truncated, err := relatedResourcesFor(ctx, clients, cache, "logs")
	if err != nil {
		return resource.ErrorRelated("logs", err)
	}
	if logList == nil {
		return resource.UnknownRelated("logs")
	}

	var ids []string
	for _, logRes := range logList {
		if logRes.ID == "/aws-glue/jobs/output" || logRes.ID == "/aws-glue/jobs/error" {
			ids = append(ids, logRes.ID)
		}
	}
	return relatedResultTrunc("logs", ids, truncated)
}

// checkGlueCFN calls glue:GetTags(resourceArn) and looks up the
// aws:cloudformation:stack-name tag in the cfn cache. Pattern C.
// Job ARN: arn:aws:glue:REGION:ACCOUNT:job/NAME.
func checkGlueCFN(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	jobName := res.ID
	if jobName == "" {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: 0}
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.Glue == nil {
		return resource.UnknownRelated("cfn")
	}
	region := c.Region
	if region == "" {
		region = GetDefaultRegion("", "")
	}
	account := accountIDFromClients(ctx, c, c.IdentityStore())
	if account == "" {
		// Identity unresolved (STS GetCallerIdentity failed or is unavailable):
		// the ARN this checker needs cannot be constructed, so the result is
		// unknown, not a real zero.
		return resource.UnknownRelated("cfn")
	}
	jobARN := "arn:aws:glue:" + region + ":" + account + ":job/" + jobName
	tagAPI, ok := c.Glue.(GlueGetTagsAPI)
	if !ok {
		return resource.UnknownRelated("cfn")
	}
	out, err := tagAPI.GetTags(ctx, &glue.GetTagsInput{ResourceArn: aws.String(jobARN)})
	if err != nil {
		return resource.ErrorRelated("cfn", err)
	}
	stackName := out.Tags["aws:cloudformation:stack-name"]
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

// checkGlueS3 extracts the S3 bucket referenced by the job's
// Command.ScriptLocation (s3://bucket/path/to/script.py). Forward lookup
// from gluetypes.Job.
func checkGlueS3(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	job, ok := assertStruct[gluetypes.Job](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("s3")
	}
	if job.Command == nil || job.Command.ScriptLocation == nil || *job.Command.ScriptLocation == "" {
		return resource.RelatedCheckResult{TargetType: "s3", Count: 0}
	}
	bucket := bucketFromS3URI(*job.Command.ScriptLocation)
	if bucket == "" {
		return resource.RelatedCheckResult{TargetType: "s3", Count: 0}
	}
	return relatedResult("s3", []string{bucket})
}

// checkGlueKMS calls glue:GetSecurityConfiguration(name=Job.SecurityConfiguration)
// and extracts the KMS key ARNs from the encryption blocks (S3/CloudWatch/
// JobBookmarks). Pattern C — single API call per checker. When the job has
// no SecurityConfiguration, Count: 0.
func checkGlueKMS(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	job, ok := assertStruct[gluetypes.Job](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("kms")
	}
	if job.SecurityConfiguration == nil || *job.SecurityConfiguration == "" {
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.Glue == nil {
		return resource.UnknownRelated("kms")
	}
	secCfgAPI, ok := c.Glue.(GlueGetSecurityConfigurationAPI)
	if !ok {
		return resource.UnknownRelated("kms")
	}
	out, err := secCfgAPI.GetSecurityConfiguration(ctx, &glue.GetSecurityConfigurationInput{
		Name: aws.String(*job.SecurityConfiguration),
	})
	if err != nil {
		return resource.ErrorRelated("kms", err)
	}
	if out.SecurityConfiguration == nil || out.SecurityConfiguration.EncryptionConfiguration == nil {
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
	}
	enc := out.SecurityConfiguration.EncryptionConfiguration
	seen := make(map[string]struct{})
	addKey := func(arn *string) {
		if arn == nil || *arn == "" {
			return
		}
		val := *arn
		if idx := strings.LastIndex(val, "/"); idx >= 0 && idx < len(val)-1 {
			val = val[idx+1:]
		}
		seen[val] = struct{}{}
	}
	if enc.CloudWatchEncryption != nil {
		addKey(enc.CloudWatchEncryption.KmsKeyArn)
	}
	if enc.JobBookmarksEncryption != nil {
		addKey(enc.JobBookmarksEncryption.KmsKeyArn)
	}
	for _, s := range enc.S3Encryption {
		addKey(s.KmsKeyArn)
	}
	var ids []string
	for id := range seen {
		ids = append(ids, id)
	}
	return relatedResult("kms", ids)
}

// checkGlueAthena scans the athena cache for workgroups whose enriched
// Fields["glue_database"] (future enrichment) references this job's database
// targets, falling back to Count: 0 when no match is found. Uses the cache.
func checkGlueAthena(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	jobName := res.ID
	if jobName == "" {
		return resource.RelatedCheckResult{TargetType: "athena", Count: 0}
	}
	wgList, truncated, err := relatedResourcesFor(ctx, clients, cache, "athena")
	if err != nil {
		return resource.ErrorRelated("athena", err)
	}
	if wgList == nil {
		return resource.UnknownRelated("athena")
	}
	var ids []string
	for _, wg := range wgList {
		// Any workgroup tagged/named with the same job name is a convention
		// signal. Without enrichment the cache typically yields no match.
		if wg.Fields["glue_job"] == jobName || wg.ID == jobName {
			ids = append(ids, wg.ID)
		}
	}
	return relatedResultTrunc("athena", ids, truncated)
}

// checkGlueSecrets scans the job's DefaultArguments (on the RawStruct) for
// values that look like Secrets Manager references (arn:aws:secretsmanager:
// prefix). Uses res.RawStruct — no cache needed.
func checkGlueSecrets(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	job, ok := assertStruct[gluetypes.Job](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("secrets")
	}
	if len(job.DefaultArguments) == 0 {
		return resource.RelatedCheckResult{TargetType: "secrets", Count: 0}
	}
	seen := make(map[string]struct{})
	var ids []string
	for _, v := range job.DefaultArguments {
		if !strings.HasPrefix(v, "arn:aws:secretsmanager:") {
			continue
		}
		// ARN: arn:aws:secretsmanager:REGION:ACCOUNT:secret:NAME-suffix
		_, name, ok := strings.Cut(v, ":secret:")
		if !ok {
			continue
		}
		if name == "" {
			continue
		}
		if _, dup := seen[name]; dup {
			continue
		}
		seen[name] = struct{}{}
		ids = append(ids, name)
	}
	return relatedResult("secrets", ids)
}
