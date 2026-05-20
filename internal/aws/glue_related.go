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

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkGlueRole extracts the Role from the Glue Job RawStruct. The value may be
// a full ARN (arn:aws:iam::…:role/name) or a plain role name. The role name is
// extracted from the last path segment of an ARN, or used directly if no "/" is present.
// The role cache is then searched by name.
func checkGlueRole(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	job, ok := assertStruct[gluetypes.Job](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "role", Count: -1}
	}
	if job.Role == nil || *job.Role == "" {
		return resource.RelatedCheckResult{TargetType: "role", Count: 0}
	}
	roleVal := *job.Role
	roleName := roleVal
	if idx := strings.LastIndex(roleVal, "/"); idx >= 0 && idx < len(roleVal)-1 {
		roleName = roleVal[idx+1:]
	}

	roleList, _, err := glueRelatedResources(ctx, clients, cache, "role")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "role", Count: -1, Err: err}
	}
	if roleList == nil {
		return resource.RelatedCheckResult{TargetType: "role", Count: -1}
	}

	var ids []string
	for _, roleRes := range roleList {
		if roleRes.Name == roleName || roleRes.Fields["role_name"] == roleName {
			ids = append(ids, roleRes.ID)
		}
	}
	return relatedResult("role", ids)
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

	alarmList, truncated, err := glueRelatedResources(ctx, clients, cache, "alarm")
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
			if d.Name != nil && *d.Name == "JobName" && d.Value != nil && *d.Value == jobName {
				ids = append(ids, alarmRes.ID)
				break
			}
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("alarm")
	}
	return relatedResult("alarm", ids)
}

// checkGlueLogs searches the logs cache for the shared Glue job log groups.
// Pattern N — Glue jobs write to /aws-glue/jobs/output and /aws-glue/jobs/error
// regardless of job name (shared log groups across all Glue jobs in the account).
func checkGlueLogs(ctx context.Context, clients any, _ resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	logList, truncated, err := glueRelatedResources(ctx, clients, cache, "logs")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1, Err: err}
	}
	if logList == nil {
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1}
	}

	var ids []string
	for _, logRes := range logList {
		if logRes.ID == "/aws-glue/jobs/output" || logRes.ID == "/aws-glue/jobs/error" {
			ids = append(ids, logRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("logs")
	}
	return relatedResult("logs", ids)
}

// checkGlueCFN calls glue:GetTags(resourceArn) and looks up the
// aws:cloudformation:stack-name tag in the cfn cache. Pattern C.
// Job ARN: arn:aws:glue:REGION:ACCOUNT:job/NAME.
func checkGlueCFN(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	jobName := res.ID
	if jobName == "" {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: 0}
	}
	s, ok := clients.(*Scope)
	if !ok || s == nil || s.Clients == nil || s.Clients.Glue == nil {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1}
	}
	c := s.Clients
	region := regionFromEnv()
	account := accountIDFromClients(ctx, c, s.IdentityStore)
	if region == "" || account == "" {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1}
	}
	jobARN := "arn:aws:glue:" + region + ":" + account + ":job/" + jobName
	tagAPI, ok := c.Glue.(GlueGetTagsAPI)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1}
	}
	out, err := tagAPI.GetTags(ctx, &glue.GetTagsInput{ResourceArn: aws.String(jobARN)})
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1, Err: err}
	}
	stackName := out.Tags["aws:cloudformation:stack-name"]
	if stackName == "" {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: 0}
	}
	cfnList, truncated, err := glueRelatedResources(ctx, clients, cache, "cfn")
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
		return resource.ApproximateZero("cfn")
	}
	return relatedResult("cfn", ids)
}

// checkGlueS3 extracts the S3 bucket referenced by the job's
// Command.ScriptLocation (s3://bucket/path/to/script.py). Forward lookup
// from gluetypes.Job.
func checkGlueS3(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	job, ok := assertStruct[gluetypes.Job](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "s3", Count: -1}
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
		return resource.RelatedCheckResult{TargetType: "kms", Count: -1}
	}
	if job.SecurityConfiguration == nil || *job.SecurityConfiguration == "" {
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
	}
	c := serviceClientsFromAny(clients)
	if c == nil || c.Glue == nil {
		return resource.RelatedCheckResult{TargetType: "kms", Count: -1}
	}
	secCfgAPI, ok := c.Glue.(GlueGetSecurityConfigurationAPI)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "kms", Count: -1}
	}
	out, err := secCfgAPI.GetSecurityConfiguration(ctx, &glue.GetSecurityConfigurationInput{
		Name: aws.String(*job.SecurityConfiguration),
	})
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "kms", Count: -1, Err: err}
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
	wgList, truncated, err := glueRelatedResources(ctx, clients, cache, "athena")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "athena", Count: -1, Err: err}
	}
	if wgList == nil {
		return resource.RelatedCheckResult{TargetType: "athena", Count: -1}
	}
	var ids []string
	for _, wg := range wgList {
		// Any workgroup tagged/named with the same job name is a convention
		// signal. Without enrichment the cache typically yields no match.
		if wg.Fields["glue_job"] == jobName || wg.ID == jobName {
			ids = append(ids, wg.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("athena")
	}
	return relatedResult("athena", ids)
}

// checkGlueSecrets scans the job's DefaultArguments (on the RawStruct) for
// values that look like Secrets Manager references (arn:aws:secretsmanager:
// prefix). Uses res.RawStruct — no cache needed.
func checkGlueSecrets(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	job, ok := assertStruct[gluetypes.Job](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "secrets", Count: -1}
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

// glueRelatedResources returns the resource list for target from cache or by fetching the first page.
func glueRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		// Glue source closures may receive either *Scope or *ServiceClients —
		// mask the error only when no transport is reachable.
		if serviceClientsFromAny(clients) == nil {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}
