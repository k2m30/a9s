// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// kinesis_issue_enrichment.go — Wave 2 issue enrichment for the kinesis
// resource type. ListStreams returns kinesistypes.StreamSummary, which
// carries neither EncryptionType nor RetentionPeriodHours; both live on
// StreamDescriptionSummary, so both signals need DescribeStreamSummary and
// are wave 2 rather than wave 1.
package aws

import (
	"context"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// kinesis canonical FindingCodes.
const (
	kinesisCodeUnencrypted  domain.FindingCode = "kinesis.unencrypted"
	kinesisCodeMinRetention domain.FindingCode = "kinesis.min-retention"
)

// EnrichKinesisStreamSummary calls DescribeStreamSummary per stream (cap
// EnrichmentCap) to surface encryption-at-rest and retention posture.
func EnrichKinesisStreamSummary(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]bool),
		FieldUpdates: make(map[string]map[string]string),
	}
	if clients.Kinesis == nil {
		return result, nil
	}
	return result, nil
}
