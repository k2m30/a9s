// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// opensearch_codes.go — canonical FindingCodes and S5 operator sentences for
// the opensearch resource type. Every signal is wave 1: DescribeDomains is the
// fetcher's own call and DomainStatus carries all of them, so opensearch
// registers no wave-2 enricher.
package aws

import "github.com/k2m30/a9s/v3/core/domain"

const (
	CodeOpenSearchDeleting   domain.FindingCode = "opensearch.dim.deleting"
	CodeOpenSearchIsolated   domain.FindingCode = "opensearch.broken.isolated"
	CodeOpenSearchProcessing domain.FindingCode = "opensearch.warn.processing"

	opensearchCodeUpdateForced   domain.FindingCode = "opensearch.update-forced"
	opensearchCodeEncryptionOff  domain.FindingCode = "opensearch.encryption-off"
	opensearchCodePublic         domain.FindingCode = "opensearch.public"
	opensearchCodeHTTPSNotForced domain.FindingCode = "opensearch.https-not-enforced"
	opensearchCodeN2NOff         domain.FindingCode = "opensearch.node-to-node-tls-off"
)
