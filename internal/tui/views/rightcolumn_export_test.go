package views

// IsActionableRowForTest is a test-only entry point for isActionableRow.
func IsActionableRowForTest(count int, truncated bool, fetchFilter map[string]string, loading bool, err error, targetType string) bool {
	return isActionableRow(rightColumnRow{
		targetType:  targetType,
		count:       count,
		truncated: truncated,
		fetchFilter: fetchFilter,
		loading:     loading,
		err:         err,
	})
}
