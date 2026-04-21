// dbi_issue_enrichment.go — Wave 2 issue enrichment for the dbi resource type.
package aws

func init() {
	registerIssueEnricher("dbi", EnrichDbiMaintenance, 10)
}
