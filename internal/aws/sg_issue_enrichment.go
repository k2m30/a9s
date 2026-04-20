// sg_issue_enrichment.go — Wave 2 = None (in-fetcher) for the sg resource type.
package aws

func init() {
	// sg uses NoOpIssueEnricher because sg.go's fetcher already computes
	// dangerous_open_count and wide_open at fetch time. The Color func reads
	// those fields. Pragmatic in-fetcher Wave 2; the registry entry exists for
	// contract conformance (TestAttentionSignalsDoc enforces every documented
	// Wave 2 row has a registry entry).
	registerIssueEnricher("sg", NoOpIssueEnricher, 100)
}
