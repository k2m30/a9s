package ctevent

import "github.com/k2m30/a9s/v3/internal/domain"

// Project implements domain.DetailProjector for CloudTrail event resources.
// It parses the raw event JSON from r.RawStruct (expected to be a *Event),
// builds the ctevent sections, and converts them to []domain.Section.
//
// If r.RawStruct is not a *Event, Project falls back to parsing r.Fields["raw"]
// as a JSON string. If that also fails it returns nil so the caller can fall
// back to the generic projector.
func Project(r domain.Resource) []domain.Section {
	event := parseResource(r)
	if event == nil {
		return nil
	}
	return convertSections(BuildSections(event))
}

// parseResource extracts a *Event from the domain.Resource.
// It handles two cases:
//  1. r.RawStruct is already a *Event (demo fixtures).
//  2. r.Fields["raw"] contains the raw CloudTrail JSON string (live path).
func parseResource(r domain.Resource) *Event {
	if r.RawStruct != nil {
		if ev, ok := r.RawStruct.(*Event); ok {
			return ev
		}
	}
	raw := r.Fields["raw"]
	if raw == "" {
		return nil
	}
	ev, err := Parse(raw)
	if err != nil {
		return nil
	}
	return ev
}

// convertSections adapts []Section (ctevent-local) to []domain.Section.
// Adapter note: ctevent.Row.Severity is a tier string ("ct-info"|"ct-attention"|
// "ct-danger"|""). domain.Item uses Tier for that string and Severity for the
// enum. We map the tier string to Tier and derive Severity from it.
func convertSections(sections []Section) []domain.Section {
	out := make([]domain.Section, 0, len(sections))
	for _, s := range sections {
		ds := domain.Section{
			Title: s.Name,
			Items: make([]domain.Item, 0, len(s.Rows)),
		}
		for _, row := range s.Rows {
			ds.Items = append(ds.Items, convertRow(row))
		}
		out = append(out, ds)
	}
	return out
}

// convertRow maps a ctevent Row to a domain Item.
func convertRow(r Row) domain.Item {
	return domain.Item{
		Kind:       domain.ItemField,
		Label:      r.Key,
		Value:      r.Value,
		Tier:       r.Severity, // ctevent Severity string → domain Tier string
		Severity:   tierToSeverity(r.Severity),
		Navigable:  r.IsNavigable,
		TargetType: r.TargetType,
	}
}

// tierToSeverity maps the ctevent tier string to a domain.Severity value.
// "ct-danger" → SevBroken, "ct-attention" → SevWarn, everything else → SevOK.
func tierToSeverity(tier string) domain.Severity {
	switch tier {
	case "ct-danger":
		return domain.SevBroken
	case "ct-attention":
		return domain.SevWarn
	default:
		return domain.SevOK
	}
}
