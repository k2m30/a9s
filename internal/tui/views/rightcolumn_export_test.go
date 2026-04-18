package views

// IsActionableRowForTest is a test-only entry point for isActionableRow.
func IsActionableRowForTest(count int, approximate bool, fetchFilter map[string]string, loading bool, err error, targetType string) bool {
	return isActionableRow(rightColumnRow{
		targetType:  targetType,
		count:       count,
		approximate: approximate,
		fetchFilter: fetchFilter,
		loading:     loading,
		err:         err,
	})
}
