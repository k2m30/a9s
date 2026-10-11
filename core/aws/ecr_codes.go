// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// ecr_codes.go — FindingCode constants for the repository settings of the ecr
// resource type. Tag mutability is readable from the DescribeRepositories
// output FetchECRRepositories already holds; scan on push also needs the
// registry scanning configuration, so EnrichECRRepository raises it. The other
// wave-2 codes live next to EnrichECRRepository in ecr_issue_enrichment.go.
package aws

import "github.com/k2m30/a9s/v3/core/domain"

const (
	// CodeECRScanOnPushOff — no registry scanning rule scans the repository
	// on push or continuously, and either the registry has rules, uses
	// enhanced scanning, or the repository's own ScanOnPush is off.
	CodeECRScanOnPushOff domain.FindingCode = "ecr.scan-on-push-off"

	// CodeECRMutableTags — ImageTagMutability is MUTABLE or
	// MUTABLE_WITH_EXCLUSION.
	CodeECRMutableTags domain.FindingCode = "ecr.mutable-tags"
)
