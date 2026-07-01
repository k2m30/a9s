package main

import (
	"encoding/json"
)

// s3StatusIncomplete is the S4 Status-column text for a PAB-incomplete bucket
// (docs/resources/s3.md §4). Must match what the list row renders.
const s3StatusIncomplete = "public access block incomplete"

// mirror of cmd/snapshot's normalized s3 data (the data contract, redeclared so
// the checklist generator depends on no collector internals).
type s3Data struct {
	Buckets []struct {
		Name string `json:"name"`
		PAB  struct {
			Outcome               string `json:"outcome"`
			ErrorCode             string `json:"error_code"`
			BlockPublicAcls       bool   `json:"block_public_acls"`
			IgnorePublicAcls      bool   `json:"ignore_public_acls"`
			BlockPublicPolicy     bool   `json:"block_public_policy"`
			RestrictPublicBuckets bool   `json:"restrict_public_buckets"`
		} `json:"pab"`
	} `json:"buckets"`
}

// checklistS3 derives the expected s3 root+list checklist from raw bucket facts.
//
// docs/resources/s3.md rules applied here:
//   - §3.1 no Wave-1 signal → every row is Healthy (green), never warning/broken/dim.
//   - §3.2 Wave-2 PAB: a bucket with no PAB config, or any of the four flags
//     false, is flagged with a "!" glyph on its Healthy row + Status
//     "public access block incomplete". A per-bucket call error (e.g. a
//     cross-region PermanentRedirect) is data-incomplete ("?" marker), NOT a
//     finding.
func checklistS3(section json.RawMessage) (Checklist, error) {
	var d s3Data
	if err := json.Unmarshal(section, &d); err != nil {
		return Checklist{}, err
	}

	total := len(d.Buckets)
	shown := min(total, pageSize)

	colors := map[string]int{"healthy": 0, "warning": 0, "broken": 0, "dim": 0}
	decorators := map[string]int{"!": 0, "~": 0}
	var flagged []FlaggedRow

	for i := range shown {
		b := d.Buckets[i]
		colors["healthy"]++ // s3 has no state field → all rows green

		incomplete := false
		switch b.PAB.Outcome {
		case "none":
			incomplete = true
		case "configured":
			if !(b.PAB.BlockPublicAcls && b.PAB.IgnorePublicAcls && b.PAB.BlockPublicPolicy && b.PAB.RestrictPublicBuckets) {
				incomplete = true
			}
		case "error":
			// "?" row marker (cross-region / other). Not a finding, no decorator.
		}
		if incomplete {
			decorators["!"]++
			flagged = append(flagged, FlaggedRow{Name: b.Name, Decorator: "!", Status: s3StatusIncomplete})
		}
	}

	issues := len(flagged) // pageSize == enrichmentCap, so findings span exactly the shown page
	return Checklist{
		Type: "s3",
		Menu: ChecklistMenu{
			Display:         "S3 Buckets",
			Availability:    shown,
			AvailTruncated:  total > pageSize,
			Issues:          issues,
			IssuesTruncated: total > enrichmentCap,
		},
		List: ChecklistList{
			// impl-plan §4 gap table + U10: the Status column replaced the old
			// jargon "Public Access" column; these headers, no others.
			Columns:    []string{"Bucket Name", "Region", "Creation Date", "Status"},
			Jargon:     []string{"Public Access", "CIS", "Flags", "Policy", "Issues", "NOBKP", "UNENC", "PUB", "NOPROT"},
			Shown:      shown,
			Truncated:  total > pageSize,
			Colors:     colors,
			Decorators: decorators,
			Flagged:    flagged,
		},
	}, nil
}
