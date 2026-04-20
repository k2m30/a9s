// rtb_issue_enrichment.go — Wave 2 = None for the rtb resource type.
package aws

func init() {
	registerIssueEnricher("rtb", NoOpIssueEnricher, 100)
}
