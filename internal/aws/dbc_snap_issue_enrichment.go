// docdb_snap_issue_enrichment.go — Wave 2 = None for the dbc-snap resource type.
package aws

func init() {
	registerIssueEnricher("dbc-snap", NoOpIssueEnricher, 100)
}
