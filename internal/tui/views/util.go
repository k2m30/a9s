package views

import (
	"fmt"
	"strconv"
	"strings"
)

// itoa is a shorthand for strconv.Itoa used in FrameTitle methods.
func itoa(n int) string {
	return strconv.Itoa(n)
}

// buildTextSearchMatchesForInfo counts case-insensitive occurrences of query
// across lines, returning one entry per match. Used only for match-count
// display in SearchInfo when the controller owns the search state.
func buildTextSearchMatchesForInfo(lines []string, query string) []struct{} {
	if query == "" {
		return nil
	}
	q := strings.ToLower(query)
	var count []struct{}
	for _, line := range lines {
		lower := strings.ToLower(line)
		start := 0
		for {
			idx := strings.Index(lower[start:], q)
			if idx < 0 {
				break
			}
			count = append(count, struct{}{})
			start += idx + len(q)
		}
	}
	return count
}

// formatSearchInfo returns "N/M matches" for the header.
func formatSearchInfo(current, total int) string {
	return fmt.Sprintf("%d/%d matches", current, total)
}
