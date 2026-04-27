package resource

import "github.com/k2m30/a9s/v3/internal/domain"

// augmentEC2StatusChecks inserts a Status Checks section immediately after the
// State block. The State block is the State header item plus all consecutive
// ItemSubfield and ItemSpacer items that follow it. ItemField or another
// ItemHeader terminates the State block.
//
// It injects the section when the instance is running and has non-trivial
// (non-ok) status checks.
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

	// Find the insertion point: scan for an ItemHeader{Label:"State"} within any
	// section.  projection.Generic does not produce a Section with Title=="State";
	// instead, State is emitted as an ItemHeader inside the unnamed leading section.
	for i, sec := range sections {
		for j, item := range sec.Items {
			if item.Kind != domain.ItemHeader || item.Label != "State" {
				continue
			}
			// Find end of State block: scan forward through consecutive
			// ItemSubfield and ItemSpacer items. A future projector may
			// insert spacers between the State header and its sub-fields;
			// accepting both kinds ensures the insertion point is correct.
			// ItemField or another ItemHeader terminates the block.
			endOfState := j + 1
			for endOfState < len(sec.Items) &&
				(sec.Items[endOfState].Kind == domain.ItemSubfield ||
					sec.Items[endOfState].Kind == domain.ItemSpacer) {
				endOfState++
			}
			// Split the matched section into [leading+state block] and optional [tail].
			leading := domain.Section{
				Title: sec.Title,
				Items: sec.Items[:endOfState],
			}
			var tail *domain.Section
			if endOfState < len(sec.Items) {
				tail = &domain.Section{
					Title: sec.Title, // forward-safe: preserve title in case future projector emits State header inside a titled section
					Items: sec.Items[endOfState:],
				}
			}
			result := make([]domain.Section, 0, len(sections)+2)
			result = append(result, sections[:i]...)
			result = append(result, leading)
			result = append(result, statusSection)
			if tail != nil {
				result = append(result, *tail)
			}
			result = append(result, sections[i+1:]...)
			return result
		}
	}

	// State header not found — append at end.
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
