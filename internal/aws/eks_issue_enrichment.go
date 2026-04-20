// eks_issue_enrichment.go — Wave 2 = None (in-fetcher) for the eks resource type.
package aws

func init() {
	// eks uses NoOpIssueEnricher because its fetcher already performs the
	// per-resource DescribeCluster call and populates the health_issues_count
	// and health_issues Wave 2 fields at fetch time. The Color func reads those
	// fields. This is a pragmatic in-fetcher Wave 2; the registry entry exists
	// for contract conformance (TestAttentionSignalsDoc enforces every documented
	// Wave 2 row has a registry entry).
	registerIssueEnricher("eks", NoOpIssueEnricher, 100)
}
