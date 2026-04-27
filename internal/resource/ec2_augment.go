package resource

import "github.com/k2m30/a9s/v3/internal/domain"

// augmentEC2StatusChecks is the Augmenter for the EC2 resource type.
// It injects a "Status Checks" section after the projector's output when
// the instance is running and has non-trivial (non-ok) status checks.
//
// The section is omitted entirely for:
//   - Instances that are not in the "running" state.
//   - Instances where both system_status and instance_status are empty.
//   - Instances where both are "ok" (fully healthy — no noise).
func augmentEC2StatusChecks(r domain.Resource, sections []domain.Section) []domain.Section {
	// Only inject when instance is running.
	state := r.Fields["state"]
	if state != "running" {
		return sections
	}
	sysStatus := r.Fields["system_status"]
	instStatus := r.Fields["instance_status"]

	// Omit when both fields are empty.
	if sysStatus == "" && instStatus == "" {
		return sections
	}
	// Omit when both are "ok" (healthy — no noise).
	if sysStatus == "ok" && instStatus == "ok" {
		return sections
	}

	sysVal := sysStatus
	if sysVal == "" {
		sysVal = "—"
	}
	instVal := instStatus
	if instVal == "" {
		instVal = "—"
	}

	statusSection := domain.Section{
		Title: "Status Checks",
		Items: []domain.Item{
			{
				Kind:        domain.ItemSubfield,
				Label:       "System",
				Value:       sysVal,
				Tier:        ec2StatusCheckTier(sysStatus),
				IndentLevel: 1,
			},
			{
				Kind:        domain.ItemSubfield,
				Label:       "Instance",
				Value:       instVal,
				Tier:        ec2StatusCheckTier(instStatus),
				IndentLevel: 1,
			},
		},
	}

	// Find the insertion point: after the "State" section.
	for i, sec := range sections {
		if sec.Title == "State" {
			result := make([]domain.Section, 0, len(sections)+1)
			result = append(result, sections[:i+1]...)
			result = append(result, statusSection)
			result = append(result, sections[i+1:]...)
			return result
		}
	}

	// State section not found — append at end.
	return append(sections, statusSection)
}

// ec2StatusCheckTier maps an EC2 status check value to a tier string
// for styling in the detail view.
func ec2StatusCheckTier(status string) string {
	switch status {
	case "ok":
		return "ok"
	case "impaired":
		return "impaired"
	case "initializing":
		return "initializing"
	default:
		return ""
	}
}
