// ng_issue_enrichment.go — Wave 2 = None (in-fetcher) for the ng resource type.
package aws

func init() {
	// ng uses NoOpIssueEnricher because its fetcher already performs the
	// per-resource DescribeNodegroup call and populates Wave 2 fields at fetch
	// time. The Color func reads those fields. This is a pragmatic in-fetcher
	// Wave 2; the registry entry exists for contract conformance
	// (TestAttentionSignalsDoc enforces every documented Wave 2 row has a
	// registry entry).
	registerIssueEnricher("ng", NoOpIssueEnricher, 100)
}
