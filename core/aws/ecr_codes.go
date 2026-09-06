// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// ecr_codes.go — canonical wave-1 FindingCode constants for the ecr resource
// type. Both conditions are readable from the DescribeRepositories output
// FetchECRRepositories already holds. The wave-2 codes live next to
// EnrichECRRepository in ecr_issue_enrichment.go.
package aws

import "github.com/k2m30/a9s/v3/core/domain"

const (
	// CodeECRScanOnPushOff — ImageScanningConfiguration is absent or
	// ScanOnPush is false.
	CodeECRScanOnPushOff domain.FindingCode = "ecr.scan-on-push-off"

	// CodeECRMutableTags — ImageTagMutability is MUTABLE.
	CodeECRMutableTags domain.FindingCode = "ecr.mutable-tags"
)

// S5 operator sentences.
