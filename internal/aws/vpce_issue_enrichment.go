// vpce_issue_enrichment.go — Wave 2 = None for the vpce resource type.
package aws

func init() {
	registerIssueEnricher("vpce", NoOpIssueEnricher, 100)
}
