// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// glue_issue_enrichment.go — Wave 2 issue enrichment for the glue resource type.
package aws

import (
	"context"
	"errors"
	"slices"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/glue"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// glue canonical FindingCodes.
const (
	glueCodeLatestRunFailed domain.FindingCode = "glue.latest-run-failed"
	// glueCodeSecurityConfigEncryptionOff — the security configuration the
	// job names leaves S3, CloudWatch Logs or job bookmark encryption off.
	glueCodeSecurityConfigEncryptionOff domain.FindingCode = "glue.security-configuration-encryption-off"
)

// glueSecurityConfigOffRows names each target the security configuration
// leaves without a key the account controls: an S3 setting that is absent or
// DISABLED, and a CloudWatch or job bookmark setting that is absent or
// DISABLED. It returns nil when all three are on.
func glueSecurityConfigOffRows(cfg *gluetypes.SecurityConfiguration) []domain.DetailRow {
	var ec gluetypes.EncryptionConfiguration
	if cfg != nil && cfg.EncryptionConfiguration != nil {
		ec = *cfg.EncryptionConfiguration
	}
	tier := tierOf(glueCodeSecurityConfigEncryptionOff)
	var rows []domain.DetailRow
	if !slices.ContainsFunc(ec.S3Encryption, func(e gluetypes.S3Encryption) bool {
		return e.S3EncryptionMode != "" && e.S3EncryptionMode != gluetypes.S3EncryptionModeDisabled
	}) {
		rows = append(rows, domain.DetailRow{Label: "S3 output", Value: "not encrypted", Tier: tier})
	}
	if ec.CloudWatchEncryption == nil || ec.CloudWatchEncryption.CloudWatchEncryptionMode == "" ||
		ec.CloudWatchEncryption.CloudWatchEncryptionMode == gluetypes.CloudWatchEncryptionModeDisabled {
		rows = append(rows, domain.DetailRow{Label: "CloudWatch logs", Value: "not encrypted", Tier: tier})
	}
	if ec.JobBookmarksEncryption == nil || ec.JobBookmarksEncryption.JobBookmarksEncryptionMode == "" ||
		ec.JobBookmarksEncryption.JobBookmarksEncryptionMode == gluetypes.JobBookmarksEncryptionModeDisabled {
		rows = append(rows, domain.DetailRow{Label: "Job bookmarks", Value: "not encrypted", Tier: tier})
	}
	return rows
}

// EnrichGlueJobStatus calls GetJobRuns(max:1) for each job (1 per job, cap ~50).
// Returns a Finding for each job whose latest run is FAILED, ERROR, or TIMEOUT.
// Severity is "!" (broken/degraded). Summary: "latest run <STATUS>".
func EnrichGlueJobStatus(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]string),
		FieldUpdates: make(map[string]map[string]string),
	}
	if clients.Glue == nil {
		return result, nil
	}
	truncated := false
	secConfigAPI, _ := clients.Glue.(GlueGetSecurityConfigurationAPI)
	// Many jobs name one security configuration, so each name is read once.
	var secConfigMu sync.Mutex
	secConfigs := map[string]func() (*gluetypes.SecurityConfiguration, error){}
	secConfig := func(name string) (*gluetypes.SecurityConfiguration, error) {
		secConfigMu.Lock()
		read, ok := secConfigs[name]
		if !ok {
			read = sync.OnceValues(func() (*gluetypes.SecurityConfiguration, error) {
				out, err := secConfigAPI.GetSecurityConfiguration(ctx, &glue.GetSecurityConfigurationInput{Name: aws.String(name)})
				if err != nil {
					return nil, err
				}
				if out.SecurityConfiguration == nil {
					return nil, UnusableAnswerErr{Call: "GetSecurityConfiguration", Field: "security configuration"}
				}
				return out.SecurityConfiguration, nil
			})
			secConfigs[name] = read
		}
		secConfigMu.Unlock()
		return read()
	}
	resources = capAtEnrichmentCap(&result, resources, nil, resourceIDsOf)
	n := len(resources)
	var failures, cfgFailures []Failure
	var mu sync.Mutex
	loopErr := ForEachRow(ctx, &result, resourceIDs(resources), EnrichmentParallelism, func(i int) {
		r := resources[i]
		if r.Name == "" {
			return
		}
		key := r.ID
		if key == "" {
			key = r.Name
		}
		out, err := firstPage(func() (*glue.GetJobRunsOutput, error) {
			return clients.Glue.GetJobRuns(ctx, &glue.GetJobRunsInput{
				JobName:    aws.String(r.Name),
				MaxResults: aws.Int32(1),
			})
		})
		var cfg *gluetypes.SecurityConfiguration
		var cfgErr error
		job, _ := assertStruct[gluetypes.Job](r.RawStruct)
		cfgName := aws.ToString(job.SecurityConfiguration)
		if cfgName != "" && secConfigAPI != nil {
			cfg, cfgErr = secConfig(cfgName)
		}
		mu.Lock()
		defer mu.Unlock()
		switch {
		case cfgName == "" || secConfigAPI == nil:
		case cfgErr != nil:
			MarkSkipped(&result, r.ID, &cfgFailures, cfgErr)
		default:
			if rows := glueSecurityConfigOffRows(cfg); len(rows) > 0 {
				setWave2Finding(&result, key, glueCodeSecurityConfigEncryptionOff, rows)
			}
		}
		if err != nil {
			truncated = true
			MarkSkipped(&result, r.ID, &failures, err)
			return
		}
		if len(out.JobRuns) > 0 {
			run := out.JobRuns[0]
			s := run.JobRunState
			if s == gluetypes.JobRunStateFailed || s == gluetypes.JobRunStateError || s == gluetypes.JobRunStateTimeout || s == gluetypes.JobRunStateExpired {
				stateVal := string(s)
				statePhrase := domain.HumanizeStatusPhrase(stateVal)
				var rows []domain.DetailRow
				if run.CompletedOn != nil {
					rows = append(rows, domain.DetailRow{Label: "Ended", Value: run.CompletedOn.Format("2006-01-02")})
				}
				if run.ErrorMessage != nil && *run.ErrorMessage != "" {
					rows = append(rows, domain.DetailRow{Label: "Error", Value: *run.ErrorMessage, Tier: "!"})
				}
				setWave2Finding(&result, key, glueCodeLatestRunFailed, rows, statePhrase)
				result.FieldUpdates[key] = map[string]string{"last_run": stateVal}
			} else {
				result.FieldUpdates[key] = map[string]string{"last_run": "OK"}
			}
		}
	})
	SetTruncated(&result, truncated)
	return result, errors.Join(loopErr, AggregateFailures("GetJobRuns", failures, n), AggregateFailures("GetSecurityConfiguration", cfgFailures, n))
}
