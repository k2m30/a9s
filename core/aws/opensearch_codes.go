// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// opensearch_codes.go — canonical FindingCodes and operator sentences for
// the opensearch resource type. Every signal is wave 1: DescribeDomains is the
// fetcher's own call and DomainStatus carries all of them, so opensearch
// registers no wave-2 enricher.
package aws

import "github.com/k2m30/a9s/v3/core/domain"

const (
	CodeOpenSearchDeleting   domain.FindingCode = "opensearch.dim.deleting"
	CodeOpenSearchIsolated   domain.FindingCode = "opensearch.broken.isolated"
	CodeOpenSearchProcessing domain.FindingCode = "opensearch.warn.processing"

	CodeOpenSearchCreating     domain.FindingCode = "opensearch.state.creating"
	CodeOpenSearchUnrecognised domain.FindingCode = "opensearch.state.unrecognised"

	opensearchCodeUpdateForced   domain.FindingCode = "opensearch.update-forced"
	opensearchCodeEncryptionOff  domain.FindingCode = "opensearch.encryption-off"
	opensearchCodePublic         domain.FindingCode = "opensearch.public"
	opensearchCodePolicyUnread   domain.FindingCode = "opensearch.policy-not-inspected"
	opensearchCodeHTTPSNotForced domain.FindingCode = "opensearch.https-not-enforced"
	opensearchCodeN2NOff         domain.FindingCode = "opensearch.node-to-node-tls-off"
)
