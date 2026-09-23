// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// glue_related.go contains Glue Job related-resource checker functions.
package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
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
		return resource.ProvenZero("role", "job.Role")
	}
	// The job's Role ARN normalizes to the role name, which is the role's
	// Resource.ID, so it resolves by identity.
	return relatedRefs("role", []string{*job.Role}, refContext(clients, cache, "role"))
}

func checkGlueAlarms(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return alarmIDsByDimension(ctx, clients, cache, "glue", res)
}

// checkGlueLogs searches the logs cache for the shared Glue job log groups.
// Glue jobs write to /aws-glue/jobs/output and /aws-glue/jobs/error
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
// aws:cloudformation:stack-name tag in the cfn cache.
// Job ARN: arn:<partition>:glue:REGION:ACCOUNT:job/NAME.
func checkGlueCFN(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	jobName := res.ID
	if jobName == "" {
		return resource.ProvenZero("cfn", "jobName")
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.Glue == nil {
		return resource.UnknownRelated("cfn")
	}
	region := sessionRegion(c)
	account := accountIDFromClients(ctx, c, c.IdentityStore())
	if account == "" || region == "" {
		// Identity or region unresolved (STS GetCallerIdentity failed or is
		// unavailable; the session recorded no region): a job ARN carries both
		// in segments of its own, so the ARN this checker needs cannot be
		// constructed and the result is unknown, not a real zero.
		return resource.UnknownRelated("cfn")
	}
	jobARN := "arn:" + PartitionForRegion(region) + ":glue:" + region + ":" + account + ":job/" + jobName
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
		return resource.ProvenZero("cfn", "stackName")
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
func checkGlueS3(_ context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	job, ok := assertStruct[gluetypes.Job](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("s3")
	}
	if job.Command == nil || job.Command.ScriptLocation == nil || *job.Command.ScriptLocation == "" {
		return resource.ProvenZero("s3", "job.Command.ScriptLocation")
	}
	return relatedRefs("s3", []string{*job.Command.ScriptLocation}, refContext(clients, cache, "s3"))
}

// checkGlueKMS calls glue:GetSecurityConfiguration(name=Job.SecurityConfiguration)
// and extracts the KMS key ARNs from the encryption blocks (S3/CloudWatch/
// JobBookmarks). When the job has no SecurityConfiguration, Count: 0.
func checkGlueKMS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	job, ok := assertStruct[gluetypes.Job](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("kms")
	}
	if job.SecurityConfiguration == nil || *job.SecurityConfiguration == "" {
		return resource.ProvenZero("kms", "job.SecurityConfiguration")
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
		return resource.ProvenZero("kms", "out.SecurityConfiguration.EncryptionConfiguration")
	}
	enc := out.SecurityConfiguration.EncryptionConfiguration
	var refs []string
	addKey := func(arn *string) {
		if arn != nil {
			refs = append(refs, *arn)
		}
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
	return kmsRelated(ctx, clients, cache, refs)
}

// checkGlueSecrets scans the job's DefaultArguments (on the RawStruct) for
// values that look like Secrets Manager references (arn:aws:secretsmanager:
// prefix).
func checkGlueSecrets(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	job, ok := assertStruct[gluetypes.Job](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("secrets")
	}
	if len(job.DefaultArguments) == 0 {
		return resource.ProvenZero("secrets", "job.DefaultArguments")
	}
	var refs []string
	for _, v := range job.DefaultArguments {
		if isSecret(v) {
			refs = append(refs, v)
		}
	}
	return listedRelated(ctx, clients, cache, "secrets", refs, false)
}
