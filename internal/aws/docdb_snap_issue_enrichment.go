// docdb_snap_issue_enrichment.go — Wave 2 = None for the docdb-snap resource type.
package aws

func init() {
	registerIssueEnricher("docdb-snap", NoOpIssueEnricher, 100)
}
