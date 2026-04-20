// redshift_issue_enrichment.go — Wave 2 = None for the redshift resource type.
package aws

func init() {
	registerIssueEnricher("redshift", NoOpIssueEnricher, 100)
}
