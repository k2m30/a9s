// eni_issue_enrichment.go — Wave 2 = None for the eni resource type.
package aws

func init() {
	registerIssueEnricher("eni", NoOpIssueEnricher, 100)
}
