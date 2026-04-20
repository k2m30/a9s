// subnet_issue_enrichment.go — Wave 2 = None for the subnet resource type.
package aws

func init() {
	registerIssueEnricher("subnet", NoOpIssueEnricher, 100)
}
