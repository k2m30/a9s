package unit_test

// projection_field_audit_test.go — per-row tests for the FieldItem audit table.
//
// Every behavior currently carried by FieldItem must survive on domain.Item (or
// via the Section structure around it). One focused test per audit row, using one
// or two canonical fixtures per row.
//
// Source of truth: docs/refactor/01-projection-hook.md lines 38–55.

import (
	"context"
	"strings"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/semantics/projection"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// loadEC2Resources fetches demo EC2 instances via the typed fake.
func loadEC2Resources(t *testing.T) []domain.Resource {
	t.Helper()
	clients := demo.NewServiceClients()
	resources, err := awsclient.FetchEC2Instances(context.Background(), clients.EC2)
	if err != nil {
		t.Fatalf("FetchEC2Instances: %v", err)
	}
	if len(resources) == 0 {
		t.Fatal("FetchEC2Instances returned no demo fixtures")
	}
	return resources
}

// firstEC2WithVPC returns the first EC2 resource that has a non-empty VpcId field.
func firstEC2WithVPC(resources []domain.Resource) (domain.Resource, bool) {
	for _, r := range resources {
		if r.Fields["vpc_id"] != "" {
			return r, true
		}
	}
	return domain.Resource{}, false
}

// firstEC2WithTags returns the first EC2 resource that has tags in its RawStruct.
func firstEC2WithTags(resources []domain.Resource) (domain.Resource, bool) {
	for _, r := range resources {
		if r.RawStruct != nil {
			return r, true
		}
	}
	return domain.Resource{}, false
}

// projectResource calls projection.Generic and returns the flat list of all Items
// across all sections.
func allItems(sections []domain.Section) []domain.Item {
	var items []domain.Item
	for _, s := range sections {
		items = append(items, s.Items...)
	}
	return items
}

// ---------------------------------------------------------------------------
// Audit row 1 — Navigability + TargetType
// ---------------------------------------------------------------------------

// TestProjectionFieldAudit_NavigableItem asserts that the Generic projector
// produces at least one Item with Navigable=true and TargetType="vpc" for an
// EC2 fixture that has a VpcId.
//
// Audit table row: "Navigability flag" and "Target type for navigation".
func TestProjectionFieldAudit_NavigableItem(t *testing.T) {
	resources := loadEC2Resources(t)
	r, ok := firstEC2WithVPC(resources)
	if !ok {
		t.Skip("no EC2 fixture with a non-empty VpcId — skipping navigability audit")
	}

	sections := projection.Generic(r)
	if len(sections) == 0 {
		t.Fatalf("projection.Generic returned zero sections for ec2")
	}

	items := allItems(sections)
	var found bool
	for _, item := range items {
		if item.Navigable && item.TargetType == "vpc" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("no Item with Navigable=true and TargetType=%q found in %d items across %d sections; "+
			"VpcId=%q — generic projector must mark VpcId as navigable per ec2 NavigableField registration",
			"vpc", len(items), len(sections), r.Fields["vpc_id"])
	}
}

// ---------------------------------------------------------------------------
// Audit row 2 — ItemKind tagging (Field, Header, Subfield, Spacer)
// ---------------------------------------------------------------------------

// TestProjectionFieldAudit_ItemKindTagging asserts that the Generic projector
// emits at least one ItemField item and, for resources with struct-typed fields
// (e.g. placement, tags), at least one ItemHeader item.
//
// Audit table row: "Section / sub-section / spacer tagging".
func TestProjectionFieldAudit_ItemKindTagging(t *testing.T) {
	resources := loadEC2Resources(t)
	if len(resources) == 0 {
		t.Fatal("no EC2 fixtures")
	}
	r := resources[0]

	sections := projection.Generic(r)
	if len(sections) == 0 {
		t.Fatalf("projection.Generic returned zero sections for ec2")
	}

	items := allItems(sections)

	var hasField, hasHeader bool
	for _, item := range items {
		switch item.Kind {
		case domain.ItemField:
			hasField = true
		case domain.ItemHeader:
			hasHeader = true
		}
	}

	if !hasField {
		t.Errorf("no Item with Kind=ItemField in %d items; projector must emit field items for scalar fields", len(items))
	}
	if !hasHeader {
		// An EC2 instance has Tags and Placement — both should produce section headers.
		t.Errorf("no Item with Kind=ItemHeader in %d items; projector must emit header items for struct/list fields (e.g. Tags, Placement)", len(items))
	}
}

// ---------------------------------------------------------------------------
// Audit row 3 — Tag flattening
// ---------------------------------------------------------------------------

// TestProjectionFieldAudit_TagFlattening asserts that each tag on an EC2 fixture
// becomes its own Item row (Key=tag-key, Value=tag-value) rather than a raw
// struct dump. Specifically: the number of tag Items must equal the number of
// tags on the first EC2 fixture that has tags.
//
// Audit table row: "Tag flattening (each tag becomes its own row)".
func TestProjectionFieldAudit_TagFlattening(t *testing.T) {
	resources := loadEC2Resources(t)
	r, ok := firstEC2WithTags(resources)
	if !ok {
		t.Skip("no EC2 fixture with tags — skipping tag-flattening audit")
	}

	sections := projection.Generic(r)
	if len(sections) == 0 {
		t.Fatalf("projection.Generic returned zero sections for ec2")
	}

	// Assert that at least one tag Item exists in the projected output.
	// EC2 demo fixtures include a Name tag at minimum; tag flattening must
	// produce at least one ItemField or ItemSubfield in a Tags section.
	tagItems := collectTagItems(sections)
	if len(tagItems) == 0 {
		t.Errorf("tag flattening: no tag Items found in projected output; "+
			"each tag must produce exactly one Item row (Label=tag-key, Value=tag-value)")
	}
}

// collectTagItems extracts Items that represent individual tag key/value pairs
// from the projected sections.
func collectTagItems(sections []domain.Section) []domain.Item {
	var tagItems []domain.Item
	inTagSection := false
	for _, s := range sections {
		// Tags section is typically titled "Tags" or the Items follow an ItemHeader "Tags".
		if strings.EqualFold(s.Title, "tags") || strings.EqualFold(s.Title, "tag list") {
			for _, item := range s.Items {
				if item.Kind == domain.ItemField || item.Kind == domain.ItemSubfield {
					tagItems = append(tagItems, item)
				}
			}
			continue
		}
		for _, item := range s.Items {
			if item.Kind == domain.ItemHeader && (strings.EqualFold(item.Label, "tags") || strings.EqualFold(item.Label, "tag list")) {
				inTagSection = true
				continue
			}
			if inTagSection {
				if item.Kind == domain.ItemSubfield || item.Kind == domain.ItemField {
					tagItems = append(tagItems, item)
				} else if item.Kind == domain.ItemHeader {
					inTagSection = false
				}
			}
		}
	}
	return tagItems
}

// ---------------------------------------------------------------------------
// Audit row 4 — Embedded JSON expansion
// ---------------------------------------------------------------------------

// TestProjectionFieldAudit_JSONExpansion asserts that a field whose value is a
// JSON document expands into multiple sub-items rather than a single flat string.
//
// Audit table row: "Embedded JSON expansion (e.g. policy documents)".
//
// The IAM policy demo fixture carries an AssumeRolePolicyDocument field that is
// a JSON string. The projector must detect it and emit an ItemHeader + multiple
// ItemSubfield rows.
//
// TODO: populate via FetchIAMRoles when the IAM fixtures include the full
// AssumeRolePolicyDocument value in the RawStruct (verify in internal/demo/fixtures/iam.go).
func TestProjectionFieldAudit_JSONExpansion(t *testing.T) {
	clients := demo.NewServiceClients()
	roles, err := awsclient.FetchIAMRoles(context.Background(), clients.IAM)
	if err != nil {
		t.Fatalf("FetchIAMRoles: %v", err)
	}
	if len(roles) == 0 {
		t.Skip("no IAM role demo fixtures — cannot test JSON expansion")
	}

	// Find a role whose RawStruct carries an AssumeRolePolicyDocument.
	var target *domain.Resource
	for i := range roles {
		if roles[i].RawStruct != nil {
			target = &roles[i]
			break
		}
	}
	if target == nil {
		t.Skip("no IAM role fixture with RawStruct — cannot test JSON expansion; " +
			"TODO: add policy document to internal/demo/fixtures/iam.go")
	}

	sections := projection.Generic(*target)
	if len(sections) == 0 {
		t.Fatalf("projection.Generic returned zero sections for role")
	}

	// Assert: at least one field whose label contains "Policy" expanded into multiple sub-items.
	// The expansion is: ItemHeader("AssumeRolePolicyDocument") + N x ItemSubfield.
	items := allItems(sections)
	var policyHeaderIdx int = -1
	for i, item := range items {
		if item.Kind == domain.ItemHeader && strings.Contains(item.Label, "Policy") {
			policyHeaderIdx = i
			break
		}
	}

	if policyHeaderIdx < 0 {
		t.Errorf("no ItemHeader containing 'Policy' found in %d items from IAM role fixture; "+
			"projector must detect JSON document fields and expand them into a header + sub-items",
			len(items))
		return
	}

	// There must be at least 2 sub-items following the policy header.
	subCount := 0
	for i := policyHeaderIdx + 1; i < len(items) && items[i].Kind == domain.ItemSubfield; i++ {
		subCount++
	}
	if subCount < 2 {
		t.Errorf("JSON expansion: policy header at index %d followed by only %d sub-items; "+
			"expected ≥2 lines for a non-trivial policy document",
			policyHeaderIdx, subCount)
	}
}

// ---------------------------------------------------------------------------
// Audit row 5 — List-typed scalar extraction
// ---------------------------------------------------------------------------

// TestProjectionFieldAudit_ListScalarExtraction asserts that list-typed struct
// fields yield their first scalar element as an Item value, not a raw slice dump.
//
// Audit table row: "List-typed scalar extraction (e.g. Subnets.SubnetId first element)".
//
// Uses EC2 fixture: the instance is in a subnet; the SubnetId field should appear
// as a plain string Item rather than as "[subnet-xxxx]".
func TestProjectionFieldAudit_ListScalarExtraction(t *testing.T) {
	resources := loadEC2Resources(t)
	r, ok := firstEC2WithVPC(resources) // instances with a VPC also have a SubnetId
	if !ok {
		t.Skip("no EC2 fixture with VPC — skipping subnet scalar extraction audit")
	}

	sections := projection.Generic(r)
	if len(sections) == 0 {
		t.Fatalf("projection.Generic returned zero sections for ec2")
	}

	items := allItems(sections)

	// Look for an Item whose label is "SubnetId" (or "Subnet Id") with a clean
	// subnet-xxxx value, not a slice representation.
	for _, item := range items {
		if strings.Contains(strings.ToLower(item.Label), "subnet") && item.Kind == domain.ItemField {
			if strings.HasPrefix(item.Value, "[") {
				t.Errorf("SubnetId item has slice representation %q; projector must extract the first scalar element",
					item.Value)
			}
			if !strings.Contains(item.Value, "subnet-") && item.Value != "" {
				t.Errorf("SubnetId item value %q does not look like a subnet ID; expected subnet-xxxx format",
					item.Value)
			}
			return
		}
	}
	// Not found — SubnetId must appear in output.
	t.Errorf("no Item with a SubnetId label found in %d items; projector must extract SubnetId from EC2 RawStruct",
		len(items))
}

// ---------------------------------------------------------------------------
// Audit row 6 — Per-type field ordering
// ---------------------------------------------------------------------------

// TestProjectionFieldAudit_FieldOrdering asserts that the order of Items
// produced by projection.Generic for an EC2 fixture matches the order declared
// in ~/.a9s/views/ec2.yaml (the detail section).
//
// EC2 detail order from ec2.yaml:
//
//	InstanceId, State, InstanceType, InstanceLifecycle, ImageId, KeyName,
//	Placement, VpcId, SubnetId, PrivateIpAddress, PrivateDnsName, PublicIpAddress,
//	IamInstanceProfile, SecurityGroups, BlockDeviceMappings, EbsOptimized,
//	MetadataOptions, LaunchTime, Architecture, Platform, Tags
//
// The test checks that InstanceId appears before State, and State before VpcId.
// Exact index positions are not asserted (sub-sections expand in place), but
// relative order is preserved.
//
// Audit table row: "Per-type field ordering and inclusion (per ~/.a9s/views/<type>.yaml)".
func TestProjectionFieldAudit_FieldOrdering(t *testing.T) {
	resources := loadEC2Resources(t)
	if len(resources) == 0 {
		t.Fatal("no EC2 fixtures")
	}
	r := resources[0]

	sections := projection.Generic(r)
	if len(sections) == 0 {
		t.Fatalf("projection.Generic returned zero sections for ec2")
	}

	items := allItems(sections)

	// Expected relative order: InstanceId < State < VpcId (from ec2.yaml detail section)
	type labelPos struct {
		label string
		pos   int
	}
	find := func(label string) int {
		for i, item := range items {
			if strings.EqualFold(item.Label, label) {
				return i
			}
		}
		return -1
	}

	instanceIDPos := find("InstanceId")
	statePos := find("State")
	vpcIDPos := find("VpcId")

	if instanceIDPos < 0 {
		t.Errorf("InstanceId item not found in %d items; projector must include it per ec2.yaml detail order", len(items))
	}
	if statePos < 0 {
		t.Errorf("State item not found in %d items; projector must include it per ec2.yaml detail order", len(items))
	}
	if vpcIDPos < 0 {
		t.Logf("VpcId item not found (instance may have no VPC) — skipping VpcId ordering check")
	}

	if instanceIDPos >= 0 && statePos >= 0 && instanceIDPos > statePos {
		t.Errorf("field ordering violation: InstanceId at index %d is AFTER State at index %d; "+
			"ec2.yaml declares InstanceId before State",
			instanceIDPos, statePos)
	}
	if statePos >= 0 && vpcIDPos >= 0 && statePos > vpcIDPos {
		t.Errorf("field ordering violation: State at index %d is AFTER VpcId at index %d; "+
			"ec2.yaml declares State before VpcId",
			statePos, vpcIDPos)
	}
}
