// igw_issue_enrichment.go — Wave 2 = None for the igw resource type.
package aws

func init() {
	registerIssueEnricher("igw", NoOpIssueEnricher, 100)
}
