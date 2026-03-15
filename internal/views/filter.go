package views

import (
	"strings"

	"github.com/k2m30/a9s/internal/resource"
)

// FilterResources returns the subset of resources that match the given query string.
// Matching is case-insensitive and checks the resource ID, Name, Status, and all Fields values.
// An empty query returns all resources unchanged.
func FilterResources(query string, resources []resource.Resource) []resource.Resource {
	if query == "" {
		return resources
	}
	q := strings.ToLower(query)
	result := make([]resource.Resource, 0)
	for _, r := range resources {
		if matchesFilter(q, r) {
			result = append(result, r)
		}
	}
	return result
}

// matchesFilter checks whether a single resource matches the lowercased query string.
func matchesFilter(query string, r resource.Resource) bool {
	if strings.Contains(strings.ToLower(r.ID), query) {
		return true
	}
	if strings.Contains(strings.ToLower(r.Name), query) {
		return true
	}
	if strings.Contains(strings.ToLower(r.Status), query) {
		return true
	}
	for _, v := range r.Fields {
		if strings.Contains(strings.ToLower(v), query) {
			return true
		}
	}
	return false
}
