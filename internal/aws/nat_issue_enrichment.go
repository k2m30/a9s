// nat_issue_enrichment.go — Wave 2 = None for the nat resource type.
package aws

func init() {
	registerIssueEnricher("nat", NoOpIssueEnricher, 100)
}
