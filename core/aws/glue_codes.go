// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// glue_codes.go — canonical wave-1 FindingCode constants for the glue
// resource type. All three read fields GetJobs already returns
// (glue.go:86 keeps the whole gluetypes.Job in RawStruct).
package aws

import "github.com/k2m30/a9s/v3/core/domain"

const (
	// CodeGlueNoSecurityConfiguration — Job.SecurityConfiguration is empty.
	// One finding covers the S3, CloudWatch-logs and job-bookmark encryption
	// settings, because all three live on the security configuration a job
	// either names or does not.
	CodeGlueNoSecurityConfiguration domain.FindingCode = "glue.no-security-configuration"

	// CodeGlueContinuousLoggingOff — the job's default arguments do not turn
	// on continuous CloudWatch logging.
	CodeGlueContinuousLoggingOff domain.FindingCode = "glue.continuous-logging-off"

	// CodeGlueArgumentSecret — a default argument value holds a credential.
	//nolint:gosec // G101 false positive: a finding code, not a credential
	CodeGlueArgumentSecret domain.FindingCode = "glue.argument-secret"
)

// S5 operator sentences.
