package unit

// ec2_augment_placement_test.go — regression test for PR-01 Bug 2.
//
// Bug: augmentEC2StatusChecks searches for a domain.Section with Title == "State"
// to find the insertion point. However, projection.Generic does NOT produce a
// section titled "State" — State appears as an ItemHeader (domain.ItemHeader)
// inside an unnamed leading section. The Title-based search never matches, so
// Status Checks is always appended at the end (after Tags), not immediately
// after the State block.
//
// Fix: augmentEC2StatusChecks must instead locate the State block by scanning
// for an ItemHeader whose Label == "State" within the leading unnamed section,
// then split and insert after that header+subfields cluster.
//
// This test builds a synthetic []domain.Section that mirrors what
// projection.Generic produces for a running EC2 instance:
//
//	Section{Title: ""}  — unnamed leading section
//	  Items:
//	    ItemField{Label:"InstanceId", Value:"i-xxx"}
//	    ItemHeader{Label:"State"}
//	    ItemSubfield{Label:"Name", Value:"running"}
//	    ItemSubfield{Label:"Code", Value:"16"}
//	    ItemField{Label:"InstanceType", Value:"t3.large"}
//	Section{Title: "Tags"}
//	  Items:
//	    ItemField{Label:"env", Value:"prod"}
//
// It calls the EC2 type's Augment hook (resource.FindResourceType("ec2").Augment)
// and verifies that Status Checks appears BEFORE Tags in the result.

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// buildEC2SyntheticSections returns a []domain.Section that mirrors what
// projection.Generic emits for a running EC2 instance. Crucially, State is an
// ItemHeader inside the unnamed leading section — NOT a Section with Title=="State".
func buildEC2SyntheticSections() []domain.Section {
	return []domain.Section{
		{
			Title: "", // unnamed leading section — no sec.Title == "State"
			Items: []domain.Item{
				{Kind: domain.ItemField, Label: "InstanceId", Value: "i-0test000000000001"},
				{Kind: domain.ItemHeader, Label: "State"}, // State is a header, not a section title
				{Kind: domain.ItemSubfield, Label: "Name", Value: "running", IndentLevel: 1},
				{Kind: domain.ItemSubfield, Label: "Code", Value: "16", IndentLevel: 1},
				{Kind: domain.ItemField, Label: "InstanceType", Value: "t3.large"},
			},
		},
		{
			Title: "Tags",
			Items: []domain.Item{
				{Kind: domain.ItemField, Label: "env", Value: "prod"},
			},
		},
	}
}

// sectionTitles returns the Title of every section in order.
func sectionTitles(sections []domain.Section) []string {
	titles := make([]string, len(sections))
	for i, s := range sections {
		titles[i] = s.Title
	}
	return titles
}

// sectionIndexByTitle returns the index of the first section with the given
// Title, or -1 if not found.
func sectionIndexByTitle(sections []domain.Section, title string) int {
	for i, s := range sections {
		if s.Title == title {
			return i
		}
	}
	return -1
}

// TestAugmentEC2StatusChecks_PlacementAfterState asserts that when the EC2
// augmenter injects "Status Checks" it appears BEFORE the "Tags" section in
// the output.
//
// Today the augmenter searches for sec.Title == "State" — which never matches
// because projection.Generic places State as an ItemHeader inside an unnamed
// section. So Status Checks is appended at the very end (after Tags).
//
// The test FAILS today because Status Checks index > Tags index.
// After the fix, Status Checks index < Tags index.
func TestAugmentEC2StatusChecks_PlacementAfterState(t *testing.T) {
	td := resource.FindResourceType("ec2")
	if td == nil {
		t.Fatal("resource type 'ec2' not registered — cannot access Augment hook")
	}
	if td.Augment == nil {
		t.Fatal("ec2 ResourceTypeDef has no Augment hook — nothing to test")
	}

	r := domain.Resource{
		ID:   "i-0test000000000001",
		Type: "ec2",
		Fields: map[string]string{
			"state":           "running",
			"system_status":   "impaired",
			"instance_status": "ok",
		},
	}

	sections := buildEC2SyntheticSections()
	result := td.Augment(r, sections)

	statusChecksIdx := sectionIndexByTitle(result, "Status Checks")
	tagsIdx := sectionIndexByTitle(result, "Tags")

	if statusChecksIdx == -1 {
		t.Fatalf("augmenter did not inject 'Status Checks' section; got sections: %v", sectionTitles(result))
	}
	if tagsIdx == -1 {
		t.Fatalf("'Tags' section missing from augmented output; got sections: %v", sectionTitles(result))
	}

	// The key assertion: Status Checks must appear BEFORE Tags.
	// Today this fails because Status Checks is appended at end (index > Tags index).
	if statusChecksIdx > tagsIdx {
		t.Errorf(
			"Status Checks placed at index %d (after Tags at index %d) but expected immediately after State block — "+
				"augmentEC2StatusChecks did not find 'State' section title because projection.Generic emits State as ItemHeader not a titled section",
			statusChecksIdx, tagsIdx,
		)
	}
}

// TestAugmentEC2StatusChecks_HealthyNoInjection verifies that when both status
// checks are "ok" the augmenter returns sections unchanged (no Status Checks
// section injected).
func TestAugmentEC2StatusChecks_HealthyNoInjection(t *testing.T) {
	td := resource.FindResourceType("ec2")
	if td == nil {
		t.Fatal("resource type 'ec2' not registered")
	}
	if td.Augment == nil {
		t.Fatal("ec2 ResourceTypeDef has no Augment hook")
	}

	r := domain.Resource{
		ID:   "i-0healthy000000001",
		Type: "ec2",
		Fields: map[string]string{
			"state":           "running",
			"system_status":   "ok",
			"instance_status": "ok",
		},
	}

	sections := buildEC2SyntheticSections()
	result := td.Augment(r, sections)

	if idx := sectionIndexByTitle(result, "Status Checks"); idx != -1 {
		t.Errorf("healthy instance (both ok) should NOT have Status Checks injected; found at index %d", idx)
	}
	if len(result) != len(sections) {
		t.Errorf("augmenter modified section count for healthy instance: got %d sections, want %d", len(result), len(sections))
	}
}

// TestAugmentEC2StatusChecks_NonRunningNoInjection verifies that a stopped
// instance does not get Status Checks injected even when status fields are set.
func TestAugmentEC2StatusChecks_NonRunningNoInjection(t *testing.T) {
	td := resource.FindResourceType("ec2")
	if td == nil {
		t.Fatal("resource type 'ec2' not registered")
	}
	if td.Augment == nil {
		t.Fatal("ec2 ResourceTypeDef has no Augment hook")
	}

	r := domain.Resource{
		ID:   "i-0stopped000000001",
		Type: "ec2",
		Fields: map[string]string{
			"state":           "stopped",
			"system_status":   "impaired",
			"instance_status": "impaired",
		},
	}

	sections := buildEC2SyntheticSections()
	result := td.Augment(r, sections)

	if idx := sectionIndexByTitle(result, "Status Checks"); idx != -1 {
		t.Errorf("stopped instance should NOT have Status Checks injected; found at index %d", idx)
	}
}

// TestAugmentEC2StatusChecks_TolerateSpacerInStateBlock asserts that an
// ItemSpacer between the State header and its subfields does not cause
// Status Checks to land mid-block. The augmenter must advance through both
// subfields and spacers when finding endOfState, so Status Checks still lands
// AFTER the entire State cluster (header + spacer + subfields), not mid-block.
//
// This test FAILS on pre-fix code (augmenter stops at the ItemSpacer and splits
// the State block, placing Status Checks between the spacer and the subfields)
// and PASSES after the coder's fix to advance through ItemSpacer as well.
func TestAugmentEC2StatusChecks_TolerateSpacerInStateBlock(t *testing.T) {
	td := resource.FindResourceType("ec2")
	if td == nil {
		t.Fatal("resource type 'ec2' not registered — cannot access Augment hook")
	}
	if td.Augment == nil {
		t.Fatal("ec2 ResourceTypeDef has no Augment hook — nothing to test")
	}

	sections := []domain.Section{
		{
			Title: "", // unnamed leading section
			Items: []domain.Item{
				{Kind: domain.ItemField, Label: "InstanceId", Value: "i-spacer000000001"},
				{Kind: domain.ItemHeader, Label: "State"},
				{Kind: domain.ItemSpacer},                                     // spacer between header and subfields
				{Kind: domain.ItemSubfield, Label: "Name", Value: "running"},
				{Kind: domain.ItemSubfield, Label: "Code", Value: "16"},
				{Kind: domain.ItemField, Label: "InstanceType", Value: "t3.medium"},
			},
		},
	}

	r := domain.Resource{
		ID:   "i-spacer000000001",
		Type: "ec2",
		Fields: map[string]string{
			"state":           "running",
			"system_status":   "impaired",
			"instance_status": "ok",
		},
	}

	result := td.Augment(r, sections)

	statusChecksIdx := sectionIndexByTitle(result, "Status Checks")
	if statusChecksIdx == -1 {
		t.Fatalf("augmenter did not inject 'Status Checks' section; got sections: %v", sectionTitles(result))
	}

	// Expect 3 sections: leading (with InstanceId, State, Spacer, Name, Code)
	// + Status Checks + tail (with InstanceType).
	if len(result) != 3 {
		t.Fatalf("expected 3 sections (leading + status checks + tail), got %d: %v",
			len(result), sectionTitles(result))
	}

	// Section 0: the leading unnamed section — must contain both the State
	// header+spacer+subfields AND the InstanceId field.
	// Section 1: Status Checks — injected after the full State cluster.
	// Section 2: tail — must start with InstanceType (not cut inside State block).
	if result[1].Title != "Status Checks" {
		t.Errorf("expected section[1].Title == 'Status Checks', got %q", result[1].Title)
	}

	tailItems := result[2].Items
	if len(tailItems) == 0 {
		t.Fatal("tail section (result[2]) has no items — InstanceType was lost")
	}
	if tailItems[0].Label != "InstanceType" {
		t.Errorf("tail section[0].Label = %q, want 'InstanceType' — Status Checks may have split the State block mid-way",
			tailItems[0].Label)
	}

	// The leading section must still contain the State header and its subfields.
	leadItems := result[0].Items
	hasStateHeader := false
	hasNameSubfield := false
	hasCodeSubfield := false
	for _, it := range leadItems {
		switch {
		case it.Kind == domain.ItemHeader && it.Label == "State":
			hasStateHeader = true
		case it.Kind == domain.ItemSubfield && it.Label == "Name":
			hasNameSubfield = true
		case it.Kind == domain.ItemSubfield && it.Label == "Code":
			hasCodeSubfield = true
		}
	}
	if !hasStateHeader {
		t.Error("leading section lost the State header after augmentation")
	}
	if !hasNameSubfield {
		t.Error("leading section lost the State.Name subfield after augmentation — spacer caused premature split")
	}
	if !hasCodeSubfield {
		t.Error("leading section lost the State.Code subfield after augmentation — spacer caused premature split")
	}
}
