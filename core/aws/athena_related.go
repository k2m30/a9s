// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// athena_related.go contains Athena WorkGroup related-resource checker functions.
package aws

import (
	"cmp"
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	athenatypes "github.com/aws/aws-sdk-go-v2/service/athena/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// athenaWorkGroupConfig is the workgroup's configuration: the one the list
// fetcher's GetWorkGroup put on the row, or, for a row that does not carry it,
// a GetWorkGroup of its own. nil when neither answered, which every caller
// reads as unknown; a workgroup with no configuration reads as an empty one.
func athenaWorkGroupConfig(ctx context.Context, clients any, res resource.Resource) *athenatypes.WorkGroupConfiguration {
	if row, ok := assertStruct[AthenaWorkGroupRow](res.RawStruct); ok && row.configRead {
		return cmp.Or(row.Configuration, &athenatypes.WorkGroupConfiguration{})
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.Athena == nil || res.ID == "" {
		return nil
	}
	out, err := c.Athena.GetWorkGroup(ctx, &athena.GetWorkGroupInput{WorkGroup: aws.String(res.ID)})
	// Every caller turns this nil into an unknown pivot, and the workgroup
	// row itself was never in question.
	// no finding: the related panel's own unknown is the mechanism.
	if err != nil || out == nil || out.WorkGroup == nil {
		return nil
	}
	return cmp.Or(out.WorkGroup.Configuration, &athenatypes.WorkGroupConfiguration{})
}

// checkAthenaS3 calls athena:GetWorkGroup and extracts the result-output bucket
// from Configuration.ResultConfiguration.OutputLocation (form: s3://bucket/prefix).
func checkAthenaS3(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	cfg := athenaWorkGroupConfig(ctx, clients, res)
	if cfg == nil {
		return NotRead("s3")
	}
	if cfg.ResultConfiguration == nil || cfg.ResultConfiguration.OutputLocation == nil {
		return foundNone("s3", "cfg.ResultConfiguration.OutputLocation")
	}
	return relatedRefs("s3", []string{*cfg.ResultConfiguration.OutputLocation}, refContext(clients, cache, "s3"))
}

// checkAthenaKMS counts every key the workgroup encrypts with: query results
// in S3 (ResultConfiguration), results in Athena-owned storage
// (ManagedQueryResultsConfiguration) and customer content such as notebooks
// (CustomerContentEncryptionConfiguration)
// (https://docs.aws.amazon.com/athena/latest/APIReference/API_WorkGroupConfiguration.html).
func checkAthenaKMS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	cfg := athenaWorkGroupConfig(ctx, clients, res)
	if cfg == nil {
		return NotRead("kms")
	}
	var keys []string
	if r := cfg.ResultConfiguration; r != nil && r.EncryptionConfiguration != nil {
		keys = append(keys, aws.ToString(r.EncryptionConfiguration.KmsKey))
	}
	if m := cfg.ManagedQueryResultsConfiguration; m != nil && m.EncryptionConfiguration != nil {
		keys = append(keys, aws.ToString(m.EncryptionConfiguration.KmsKey))
	}
	if cc := cfg.CustomerContentEncryptionConfiguration; cc != nil {
		keys = append(keys, aws.ToString(cc.KmsKey))
	}
	keys = nonEmpty(keys...)
	if len(keys) == 0 {
		return foundNone("kms", "cfg")
	}
	for i, k := range keys {
		keys[i] = kmsRefFromField(k, res.Type)
	}
	return kmsRelated(ctx, clients, cache, keys)
}

// checkAthenaLogs calls athena:GetWorkGroup and links the log group named by
// Configuration.MonitoringConfiguration.CloudWatchLoggingConfiguration when it
// is enabled — the only place Athena records where a workgroup writes logs.
func checkAthenaLogs(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	cfg := athenaWorkGroupConfig(ctx, clients, res)
	if cfg == nil {
		return NotRead("logs")
	}
	if cfg.MonitoringConfiguration == nil || cfg.MonitoringConfiguration.CloudWatchLoggingConfiguration == nil {
		return foundNone("logs", "cfg.MonitoringConfiguration.CloudWatchLoggingConfiguration")
	}
	cw := cfg.MonitoringConfiguration.CloudWatchLoggingConfiguration
	if !aws.ToBool(cw.Enabled) {
		return foundNone("logs", "cfg.MonitoringConfiguration.CloudWatchLoggingConfiguration.Enabled")
	}
	// Logging on with no LogGroup leaves the group to Athena, which names none
	// in the configuration.
	if aws.ToString(cw.LogGroup) == "" {
		return NotRead("logs")
	}
	return relatedResultTrunc("logs", []string{*cw.LogGroup}, false)
}

// checkAthenaRole calls athena:GetWorkGroup and extracts the ExecutionRole for
// Spark workgroups from Configuration.ExecutionRole.
func checkAthenaRole(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	cfg := athenaWorkGroupConfig(ctx, clients, res)
	if cfg == nil {
		return NotRead("role")
	}
	if cfg.ExecutionRole == nil || *cfg.ExecutionRole == "" {
		return foundNone("role", "cfg.ExecutionRole")
	}
	return relatedRefs("role", []string{*cfg.ExecutionRole}, refContext(clients, cache, "role"))
}
