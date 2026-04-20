// ami_issue_enrichment.go — Wave 2 = None for the ami resource type.
package aws

func init() {
	registerIssueEnricher("ami", NoOpIssueEnricher, 100)
}
