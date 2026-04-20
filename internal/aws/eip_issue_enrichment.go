// eip_issue_enrichment.go — Wave 2 = None for the eip resource type.
package aws

func init() {
	registerIssueEnricher("eip", NoOpIssueEnricher, 100)
}
