// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// kinesis_issue_enrichment.go — Wave 2 issue enrichment for the kinesis
// resource type. ListStreams returns kinesistypes.StreamSummary, which
// carries neither EncryptionType nor RetentionPeriodHours; both live on
// StreamDescriptionSummary, so both signals need DescribeStreamSummary and
// are wave 2 rather than wave 1.
package aws

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	kinesissvc "github.com/aws/aws-sdk-go-v2/service/kinesis"
	kinesistypes "github.com/aws/aws-sdk-go-v2/service/kinesis/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// kinesis canonical FindingCodes.
const (
	kinesisCodeUnencrypted  domain.FindingCode = "kinesis.unencrypted"
	kinesisCodeMinRetention domain.FindingCode = "kinesis.min-retention"
)

// S5 operator sentences for the stream posture codes above.
// kinesisDefaultRetentionHours is the retention a stream is created with. At
// or below it, a day-long consumer outage is data loss.
const kinesisDefaultRetentionHours int32 = 24

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
	api, ok := clients.Kinesis.(KinesisDescribeStreamSummaryAPI)
	if !ok {
		return result, nil
	}
	var failures []string
	total := 0
	resources = capAtEnrichmentCap(&result, resources, resourceIDsOf)
	n := len(resources)
	var mu sync.Mutex
	_ = ForEachParallel(ctx, n, EnrichmentParallelism, func(i int) {
		r := resources[i]
		name := r.Fields["stream_name"]
		if name == "" {
			name = r.ID
		}
		if name == "" {
			return
		}
		// Rule 4: a stream being torn down has no posture worth reporting.
		if r.Fields["stream_status"] == string(kinesistypes.StreamStatusDeleting) {
			return
		}
		mu.Lock()
		total++
		mu.Unlock()
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*kinesissvc.DescribeStreamSummaryOutput, error) {
			return api.DescribeStreamSummary(ctx, &kinesissvc.DescribeStreamSummaryInput{
				StreamName: aws.String(name),
			})
		})
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			// A stream that vanished between the listing and this call is
			// gone, not unreadable — unknown either way, but not a failure
			// worth surfacing in the error log.
			if !isKinesisStreamGone(err) {
				failures = append(failures, fmt.Sprintf("%s: %v", r.ID, err))
			}
			result.TruncatedIDs[r.ID] = true
			return
		}
		sum := out.StreamDescriptionSummary
		if sum == nil {
			result.TruncatedIDs[r.ID] = true
			return
		}
		// AWS omits EncryptionType for an unencrypted stream and returns
		// NONE for others; both are the same fact.
		if sum.EncryptionType == "" || sum.EncryptionType == kinesistypes.EncryptionTypeNone {
			setWave2Finding(&result, r.ID, kinesisCodeUnencrypted, "not encrypted at rest", "~", "kinesis",
				[]domain.DetailRow{{Label: "Encryption key", Value: "none", Tier: "~"}})
		}
		if h := sum.RetentionPeriodHours; h != nil && *h <= kinesisDefaultRetentionHours {
			setWave2Finding(&result, r.ID, kinesisCodeMinRetention, "24h retention", "~", "kinesis",
				// The phrase names the default; a stream set BELOW it needs
				// the row to say what it is actually keeping.
				[]domain.DetailRow{{Label: "Records kept", Value: fmt.Sprintf("%dh", *h), Tier: "~"}})
		}
	})
	sort.Strings(failures)
	MarkInformationalOnly(&result)
	return result, AggregateFailures("kinesis-enrich: DescribeStreamSummary", failures, total)
}

// isKinesisStreamGone reports whether err is Kinesis saying the stream no
// longer exists — the expected race between listing and describing.
func isKinesisStreamGone(err error) bool {
	var notFound *kinesistypes.ResourceNotFoundException
	return errors.As(err, &notFound)
}
