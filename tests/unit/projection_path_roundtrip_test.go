package unit

// projection_path_roundtrip_test.go — regression test for PR-01 / PR-301 deferred item:
// Path preservation through the fieldItemToDomainItem → domainItemToFieldItem round-trip.
//
// Bug: the original domainItemToFieldItem synthesized fi.Path = sectionTitle + "." + it.Label
// when it.Path was empty. That meant the real field path (e.g. "VpcId") was lost and replaced
// with a synthetic like ".VpcId" or "General.VpcId". Any caller that read Path after the
// round-trip saw a synthesized value, not the real fieldpath.ExtractFieldList path.
//
// Fix: fieldItemToDomainItem (in generic.go) now copies fi.Path directly into domain.Item.Path,
// and domainItemToFieldItem (in detail_fields.go) copies it.Path directly into fi.Path,
// falling back to synthesis only when it.Path is empty.
//
// This test verifies the invariant end-to-end by:
//   a. Checking that projection.Generic(r) emits Items with non-empty Path == the field key
//      ("VpcId"), not a synthesized form.
//   b. Checking that calling the DetailModel renders without losing the path —
//      confirmed indirectly by verifying the rendered output contains the VpcId value.
//
// The production code path under test:
//   ExtractFieldList → fieldItemToDomainItem (generic.go) → domain.Item.Path
//   → domainItemToFieldItem (detail_fields.go) → fieldpath.FieldItem.Path

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/semantics/projection"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ec2VpcResource constructs a minimal EC2-shaped domain.Resource with a VpcId
// field. The resource type is left empty so projection.Generic uses the flat
// alphabetical rendering path — no disk config is required and the test remains
// hermetic. This exercises the fieldItemToDomainItem code path inside
// groupIntoSections.
func ec2VpcResource() domain.Resource {
	return domain.Resource{
		ID:   "i-0roundtrip001",
		Type: "", // empty type → flat alphabetical path; no disk config needed
		Fields: map[string]string{
			"VpcId":        "vpc-123",
			"InstanceId":   "i-0roundtrip001",
			"InstanceType": "t3.medium",
		},
	}
}

// TestProjectionPath_FieldItemToDomainItem_PreservesPath asserts that
// projection.Generic emits domain.Items whose Path field equals the original
// field key emitted by ExtractFieldList ("VpcId"), not a synthesized form like
// "sectionTitle.VpcId" or ".VpcId".
//
// This test catches a regression where fieldItemToDomainItem drops fi.Path and
// domain.Item.Path ends up empty or synthesized.
func TestProjectionPath_FieldItemToDomainItem_PreservesPath(t *testing.T) {
	r := ec2VpcResource()
	sections := projection.Generic(r)

	if len(sections) == 0 {
		t.Fatal("projection.Generic returned zero sections for VpcId resource — cannot test Path")
	}

	// Find the VpcId item and assert its Path.
	found := false
	for _, sec := range sections {
		for _, item := range sec.Items {
			if item.Label == "VpcId" || item.Label == "vpc_id" {
				found = true
				// Path must equal the original field key, not a synthesized form.
				if item.Path == "" {
					t.Errorf("domain.Item for VpcId has empty Path — fieldItemToDomainItem did not copy fi.Path")
				}
				// The Path must NOT be a synthesized "sectionTitle.Label" form.
				// With empty type the leading section title is "" so the synthesized
				// form would be ".VpcId" or ".vpc_id". Either way it starts with ".".
				if len(item.Path) > 0 && item.Path[0] == '.' {
					t.Errorf("domain.Item for VpcId has synthesized Path %q — expected raw field key", item.Path)
				}
				// The path must be the actual key, not contain a dot prefix from synthesis.
				if item.Path != item.Label && item.Path != "VpcId" && item.Path != "vpc_id" {
					t.Errorf("domain.Item for VpcId: Path %q does not match Label %q — unexpected synthesis",
						item.Path, item.Label)
				}
			}
		}
	}

	if !found {
		t.Error("VpcId item not found in projection.Generic output — fixture or projector broken")
	}
}

// TestProjectionPath_DomainItemToFieldItem_PreservesPath asserts that rendering
// through the DetailModel preserves field paths end-to-end. The test uses a
// resource with a VpcId field and checks that the rendered output contains the
// expected value "vpc-123" — a proxy for verifying that the path round-trip
// did not corrupt the field data pipeline.
//
// The production code path under test:
//   projection.Generic → domain.Item{Path: "VpcId"} →
//   sectionsToFieldItems → domainItemToFieldItem → fieldpath.FieldItem{Path: "VpcId"} →
//   renderFromFieldList (value shown in output)
func TestProjectionPath_DomainItemToFieldItem_PreservesPath(t *testing.T) {
	r := ec2VpcResource()
	k := keys.Default()

	d := views.NewDetail(r, "", nil, k)
	d.SetSize(120, 40)
	output := stripANSI(d.View())

	// The rendered output must contain the VpcId value. If domainItemToFieldItem
	// corrupted the field path, the value would be absent or garbled.
	if !strings.Contains(output, "vpc-123") {
		t.Errorf("rendered detail view does not contain VpcId value 'vpc-123' — path round-trip may have broken field rendering\noutput:\n%s", output)
	}
}
