// ct_events_issue_enrichment.go — Wave 2 = None for the ct-events resource type.
package aws

func init() {
	registerIssueEnricher("ct-events", NoOpIssueEnricher, 100)
}
