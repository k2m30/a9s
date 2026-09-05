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

const (
	opensearchUpdateForcedDetail   = "AWS will apply this update automatically once the scheduled date passes; upgrade on your own schedule before then to control the maintenance window."
	opensearchEncryptionOffDetail  = "Data at rest is stored unencrypted. Enabling encryption at rest requires creating a new domain and migrating data — it cannot be turned on in place."
	opensearchPublicDetail         = "The domain sits outside a VPC and its access policy allows any principal, so the search endpoint is reachable from the internet. Move the domain into a VPC, or scope the access policy to named principals."
	opensearchHTTPSNotForcedDetail = "The domain accepts plaintext HTTP, so queries and results can be read off the wire. Turn on Require HTTPS in the domain's endpoint options."
	opensearchN2NOffDetail         = "Traffic between the domain's own nodes is unencrypted. Node-to-node encryption can only be enabled on a domain that already has it configured at creation — recreate the domain if this data is sensitive."
)
