// dbc_issue_enrichment.go — Wave 2 issue enrichment for the dbc resource type.
package aws

func init() {
	registerIssueEnricher("dbc", EnrichRDSDocDBMaintenance, 100)
}
