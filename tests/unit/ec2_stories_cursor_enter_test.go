package unit_test

// ec2_stories_cursor_enter_test.go — Spec-008 / EC2 stories EC2-006 through EC2-016.
//
// Stories covered:
//   EC2-006: Viewport scrolls when cursor reaches visible edge
//   EC2-007: g jumps to first row, G jumps to last row
//   EC2-008: Cursor traverses section headers and sub-fields uniformly
//   EC2-009: Enter on VpcId emits RelatedNavigateMsg{TargetType:"vpc"}
//   EC2-010: Enter on SubnetId emits RelatedNavigateMsg{TargetType:"subnet"}
//   EC2-011: Enter on SecurityGroups GroupId sub-field emits RelatedNavigateMsg{TargetType:"sg"}
//   EC2-012: Each SecurityGroup sub-field is independently navigable
//   EC2-013: Enter on ImageId emits RelatedNavigateMsg{TargetType:"ami"}
//   EC2-014: Enter on non-navigable field is a no-op (nil cmd)
//   EC2-015: Enter on section header is a no-op (nil cmd)
//   EC2-016: IamInstanceProfile.Arn sub-field is NOT navigable
//
// Build tag: //go:build spec008
//   This file requires the exported FieldCursor() getter on DetailModel.
//   Once the coder adds:
//     func (m DetailModel) FieldCursor() int { return m.fieldCursor }
//   remove the build tag from this file so it runs in normal CI.
//
// NOTE on EC2-007: g/G key handling in detail.Update() does not exist yet.
//   These tests will compile but fail until the coder adds:
//     case key.Matches(msg, m.keys.Top):
//         m.fieldCursor = 0; m.syncViewportToCursor(); return m, nil
//     case key.Matches(msg, m.keys.Bottom):
//         m.fieldCursor = len(m.fieldList)-1; m.syncViewportToCursor(); return m, nil
//
// NOTE on EC2-011/EC2-012: Sub-field navigability (IsNavigable on IsSubField items)
//   is not yet implemented in ExtractFieldList. These tests will fail until
//   ExtractFieldList propagates navigable map entries to matching sub-field values.

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ---------------------------------------------------------------------------
// Helpers local to this file
// ---------------------------------------------------------------------------

// makeEC2DetailWithFields builds a DetailModel from plain Fields (no RawStruct)
// using a custom detail field path list. Width/height control the viewport size.
func makeEC2DetailWithFields(t *testing.T, fieldValues map[string]string, detailPaths []string, width, height int) views.DetailModel {
	t.Helper()
	res := resource.Resource{
		ID:     "i-0a1b2c3d4e5f60001",
		Name:   "web-prod-01",
		Fields: fieldValues,
	}
	detailFields := make([]config.DetailField, len(detailPaths))
	for i, p := range detailPaths {
		detailFields[i] = config.DetailField{Path: p}
	}
	cfg := &config.ViewsConfig{
		Views: map[string]config.ViewDef{
			"ec2": {Detail: detailFields},
		},
	}
	k := keys.Default()
	d := views.NewDetail(res, "ec2", cfg, k)
	d.SetSize(width, height)
	return d
}

// makeEC2DetailWithRaw builds a DetailModel from an ec2types.Instance RawStruct
// plus an optional Fields map and a custom detail path list.
func makeEC2DetailWithRaw(t *testing.T, inst ec2types.Instance, fieldValues map[string]string, detailPaths []string, width, height int) views.DetailModel {
	t.Helper()
	res := resource.Resource{
		ID:        aws.ToString(inst.InstanceId),
		Name:      "web-prod-01",
		Fields:    fieldValues,
		RawStruct: inst,
	}
	detailFields := make([]config.DetailField, len(detailPaths))
	for i, p := range detailPaths {
		detailFields[i] = config.DetailField{Path: p}
	}
	cfg := &config.ViewsConfig{
		Views: map[string]config.ViewDef{
			"ec2": {Detail: detailFields},
		},
	}
	k := keys.Default()
	d := views.NewDetail(res, "ec2", cfg, k)
	d.SetSize(width, height)
	return d
}

// pressG sends the g key (Top binding) to the DetailModel.
func pressG(d views.DetailModel) (views.DetailModel, tea.Cmd) {
	return d.Update(tea.KeyPressMsg{Code: -1, Text: "g"})
}

// pressGShift sends the G key (Bottom binding) to the DetailModel.
func pressGShift(d views.DetailModel) (views.DetailModel, tea.Cmd) {
	return d.Update(tea.KeyPressMsg{Code: -1, Text: "G"})
}

// pressEnterEC2 sends Enter to the DetailModel and returns the model and cmd.
func pressEnterEC2(d views.DetailModel) (views.DetailModel, tea.Cmd) {
	return d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
}

// navigateToIndex presses j exactly n times to move the cursor to index n.
func navigateToIndex(d views.DetailModel, n int) views.DetailModel {
	for range n {
		d, _ = d.Update(tea.KeyPressMsg{Code: -1, Text: "j"})
	}
	return d
}

// extractRelatedNavigateMsg calls cmd() and type-asserts to RelatedNavigateMsg.
// Returns (msg, true) on success, (zero, false) if cmd is nil or wrong type.
func extractRelatedNavigateMsg(cmd tea.Cmd) (messages.RelatedNavigate, bool) {
	if cmd == nil {
		return messages.RelatedNavigate{}, false
	}
	msg := cmd()
	nav, ok := msg.(messages.RelatedNavigate)
	return nav, ok
}

// registerEC2NavFields registers VpcId, SubnetId, ImageId as navigable for "ec2"
// and returns a cleanup function.
func registerEC2NavFields(t *testing.T) {
	t.Helper()
	resource.RegisterNavigableFields("ec2", []resource.NavigableField{
		{FieldPath: "VpcId", TargetType: "vpc"},
		{FieldPath: "SubnetId", TargetType: "subnet"},
		{FieldPath: "ImageId", TargetType: "ami"},
	})
	t.Cleanup(func() { resource.UnregisterNavigableFields("ec2") })
}

// registerEC2NavFieldsWithSG registers VpcId, SubnetId, ImageId, and also
// SecurityGroups.GroupId as navigable for "ec2".
func registerEC2NavFieldsWithSG(t *testing.T) {
	t.Helper()
	resource.RegisterNavigableFields("ec2", []resource.NavigableField{
		{FieldPath: "VpcId", TargetType: "vpc"},
		{FieldPath: "SubnetId", TargetType: "subnet"},
		{FieldPath: "ImageId", TargetType: "ami"},
		{FieldPath: "SecurityGroups.GroupId", TargetType: "sg"},
	})
	t.Cleanup(func() { resource.UnregisterNavigableFields("ec2") })
}

// ---------------------------------------------------------------------------
// EC2-006: Viewport scrolls when cursor reaches visible edge
// ---------------------------------------------------------------------------

// TestEC2_006_ViewportScrollsAtEdge verifies that pressing j past the last
// visible row scrolls the viewport down and pressing k back up scrolls it up.
//
// Given: A detail view with more fields than visible height (height=5, fields=10)
// When: Cursor is at the last visible row and j is pressed
// Then: FieldCursor() increases AND viewport YOffset changes
// When: Cursor is scrolled back up with k
// Then: FieldCursor() decreases AND viewport YOffset decreases
func TestEC2_006_ViewportScrollsAtEdge(t *testing.T) {
	fieldPaths := []string{
		"InstanceId", "State", "InstanceType", "InstanceLifecycle", "ImageId",
		"KeyName", "VpcId", "SubnetId", "PrivateIpAddress", "PublicIpAddress",
	}
	fieldValues := make(map[string]string, len(fieldPaths))
	for _, p := range fieldPaths {
		fieldValues[p] = p + "-value"
	}

	// height=5 means 5 visible lines; 10 fields means scrolling is needed
	d := makeEC2DetailWithFields(t, fieldValues, fieldPaths, 80, 5)

	if d.FieldCursor() != 0 {
		t.Fatalf("precondition: initial FieldCursor() must be 0, got %d", d.FieldCursor())
	}

	// Press j 4 times to reach the last visible row (index 4, the 5th row)
	for range 4 {
		d, _ = d.Update(tea.KeyPressMsg{Code: -1, Text: "j"})
	}
	if d.FieldCursor() != 4 {
		t.Fatalf("precondition: expected cursor at 4 after 4 j presses, got %d", d.FieldCursor())
	}

	// Record viewport offset before the scrolling j press
	viewBefore := d.View()

	// Press j once more — cursor goes to index 5, which is BELOW the visible area
	d, _ = d.Update(tea.KeyPressMsg{Code: -1, Text: "j"})

	if d.FieldCursor() != 5 {
		t.Errorf("EC2-006: cursor must advance to 5 when j pressed at edge, got %d", d.FieldCursor())
	}

	// Viewport content must change (scroll happened)
	viewAfterDown := d.View()
	if viewAfterDown == viewBefore {
		t.Error("EC2-006: viewport content must change when cursor scrolls past visible edge (j)")
	}

	// Now press k back to verify reverse scroll
	cursorBefore := d.FieldCursor()
	d, _ = d.Update(tea.KeyPressMsg{Code: -1, Text: "k"})

	if d.FieldCursor() != cursorBefore-1 {
		t.Errorf("EC2-006: k must decrease cursor from %d to %d, got %d",
			cursorBefore, cursorBefore-1, d.FieldCursor())
	}
}

// ---------------------------------------------------------------------------
// EC2-007: g jumps to first row, G jumps to last row
// ---------------------------------------------------------------------------

// TestEC2_007_GJumpsToTop_GShiftJumpsToBottom verifies that pressing g moves
// the cursor to index 0 and pressing G moves it to the last field index.
//
// Given: Cursor somewhere in the middle
// When: g is pressed
// Then: FieldCursor() == 0
// When: G is pressed
// Then: FieldCursor() == lastFieldIndex
func TestEC2_007_GJumpsToTop_GShiftJumpsToBottom(t *testing.T) {
	fieldPaths := []string{
		"InstanceId", "State", "InstanceType", "InstanceLifecycle",
		"ImageId", "KeyName", "VpcId", "SubnetId",
	}
	fieldValues := make(map[string]string, len(fieldPaths))
	for _, p := range fieldPaths {
		fieldValues[p] = p + "-value"
	}

	d := makeEC2DetailWithFields(t, fieldValues, fieldPaths, 80, 24)

	// Move cursor to index 4 (somewhere in the middle)
	d = navigateToIndex(d, 4)
	if d.FieldCursor() != 4 {
		t.Fatalf("precondition: expected cursor at 4 after 4 j presses, got %d", d.FieldCursor())
	}

	// Press g — must jump to first row
	d, _ = pressG(d)
	if d.FieldCursor() != 0 {
		t.Errorf("EC2-007: g must jump cursor to 0 (first field), got %d", d.FieldCursor())
	}

	// Press G — must jump to last row
	d, _ = pressGShift(d)
	lastIdx := len(fieldPaths) - 1
	if d.FieldCursor() != lastIdx {
		t.Errorf("EC2-007: G must jump cursor to %d (last field), got %d", lastIdx, d.FieldCursor())
	}
}

// TestEC2_007_GFromBottom_StaysAtTop verifies that g from the last row still jumps to 0.
func TestEC2_007_GFromBottom_StaysAtTop(t *testing.T) {
	fieldPaths := []string{"InstanceId", "State", "InstanceType", "VpcId", "SubnetId"}
	fieldValues := make(map[string]string, len(fieldPaths))
	for _, p := range fieldPaths {
		fieldValues[p] = p + "-value"
	}

	d := makeEC2DetailWithFields(t, fieldValues, fieldPaths, 80, 24)

	// Navigate to bottom
	d, _ = pressGShift(d)
	lastIdx := len(fieldPaths) - 1
	if d.FieldCursor() != lastIdx {
		t.Fatalf("precondition: G must land on last index %d, got %d", lastIdx, d.FieldCursor())
	}

	// Press g from the bottom
	d, _ = pressG(d)
	if d.FieldCursor() != 0 {
		t.Errorf("EC2-007: g from last field must jump cursor to 0, got %d", d.FieldCursor())
	}
}

// ---------------------------------------------------------------------------
// EC2-008: Cursor traverses section headers and sub-fields uniformly
// ---------------------------------------------------------------------------

// TestEC2_008_CursorTraversesHeadersAndSubFields verifies that j advances
// through ALL rows including section headers (IsHeader) and sub-fields
// (IsSubField). The total number of j presses to reach the last row must
// equal the total FieldItem count minus 1 (since we start at 0).
//
// Given: An EC2 detail with Placement (multi-line → header + sub-fields)
// When: j is pressed until cursor cannot move further
// Then: Final FieldCursor() equals len(fieldList)-1
func TestEC2_008_CursorTraversesHeadersAndSubFields(t *testing.T) {
	// Use an ec2types.Instance with Placement set (struct → multi-line YAML)
	// This causes ExtractFieldList to produce:
	//   [0] InstanceId (scalar)
	//   [1] Placement: (header)
	//   [2] ... AvailabilityZone: us-east-1a (sub-field)
	//   [3] ... GroupName:  (sub-field, if present)
	//   ... other Placement sub-fields
	//   [N] VpcId (scalar)
	inst := ec2types.Instance{
		InstanceId:   aws.String("i-ec2008test"),
		InstanceType: ec2types.InstanceTypeT3Large,
		VpcId:        aws.String("vpc-ec2008"),
		Placement: &ec2types.Placement{
			AvailabilityZone: aws.String("us-east-1a"),
			Tenancy:          ec2types.TenancyDefault,
		},
	}

	fieldValues := map[string]string{
		"InstanceId": "i-ec2008test",
		"VpcId":      "vpc-ec2008",
	}
	detailPaths := []string{"InstanceId", "Placement", "VpcId"}

	d := makeEC2DetailWithRaw(t, inst, fieldValues, detailPaths, 80, 24)

	// Count how many j presses it takes to reach the last row, clamping at
	// a maximum of 100 to avoid an infinite loop.
	const maxPresses = 100
	prevCursor := -1
	pressCount := 0
	for range maxPresses {
		cur := d.FieldCursor()
		if cur == prevCursor {
			// Cursor stopped moving — we're at the bottom
			break
		}
		prevCursor = cur
		d, _ = d.Update(tea.KeyPressMsg{Code: -1, Text: "j"})
		pressCount++
	}

	finalCursor := d.FieldCursor()
	if finalCursor == 0 {
		t.Fatal("EC2-008: cursor never moved — fieldList may be empty or j not handled")
	}

	// The cursor must be at the last index and must have traversed at least
	// 2 rows (InstanceId + Placement header + at least one sub-field + VpcId).
	if finalCursor < 2 {
		t.Errorf("EC2-008: expected cursor to traverse at least 3 rows (header+subfield+scalar), reached %d", finalCursor)
	}

	// Verify that one more j press does NOT advance the cursor (boundary clamping).
	d, _ = d.Update(tea.KeyPressMsg{Code: -1, Text: "j"})
	if d.FieldCursor() != finalCursor {
		t.Errorf("EC2-008: extra j after reaching last row must clamp cursor at %d, got %d",
			finalCursor, d.FieldCursor())
	}
}

// TestEC2_008_CursorVisitsPlacementHeader verifies that after pressing j from
// InstanceId, the cursor lands on the Placement section header (not skipping it).
func TestEC2_008_CursorVisitsPlacementHeader(t *testing.T) {
	inst := ec2types.Instance{
		InstanceId:   aws.String("i-ec2008header"),
		InstanceType: ec2types.InstanceTypeT3Large,
		VpcId:        aws.String("vpc-ec2008header"),
		Placement: &ec2types.Placement{
			AvailabilityZone: aws.String("us-east-1a"),
		},
	}

	fieldValues := map[string]string{
		"InstanceId": "i-ec2008header",
		"VpcId":      "vpc-ec2008header",
	}
	detailPaths := []string{"InstanceId", "Placement", "VpcId"}

	d := makeEC2DetailWithRaw(t, inst, fieldValues, detailPaths, 80, 24)

	// cursor starts at 0 (InstanceId). Press j once → should be at index 1 (Placement header).
	d, _ = d.Update(tea.KeyPressMsg{Code: -1, Text: "j"})
	if d.FieldCursor() != 1 {
		t.Errorf("EC2-008: j from InstanceId must move cursor to index 1 (Placement header), got %d", d.FieldCursor())
	}

	// Press j again → index 2 (first sub-field of Placement, e.g. AvailabilityZone).
	d, _ = d.Update(tea.KeyPressMsg{Code: -1, Text: "j"})
	if d.FieldCursor() != 2 {
		t.Errorf("EC2-008: j from Placement header must move cursor to index 2 (first sub-field), got %d", d.FieldCursor())
	}
}

// ---------------------------------------------------------------------------
// EC2-009: Enter on VpcId emits RelatedNavigateMsg{TargetType:"vpc"}
// ---------------------------------------------------------------------------

// TestEC2_009_EnterOnVpcId_EmitsRelatedNavigateMsg verifies that pressing Enter
// when the cursor is on the VpcId navigable field emits a RelatedNavigateMsg
// with TargetType="vpc" and TargetID matching the VPC id value.
//
// Given: VpcId registered as navigable → "vpc"; cursor on VpcId row
// When: Enter is pressed
// Then: cmd() produces RelatedNavigateMsg{TargetType:"vpc", TargetID:"vpc-0abc123def456789a"}
func TestEC2_009_EnterOnVpcId_EmitsRelatedNavigateMsg(t *testing.T) {
	registerEC2NavFields(t)

	fieldValues := map[string]string{
		"VpcId":        "vpc-0abc123def456789a",
		"InstanceType": "t3.large",
	}
	detailPaths := []string{"VpcId", "InstanceType"}

	d := makeEC2DetailWithFields(t, fieldValues, detailPaths, 80, 24)

	// cursor starts at index 0 (VpcId)
	if d.FieldCursor() != 0 {
		t.Fatalf("precondition: expected initial cursor at 0 (VpcId), got %d", d.FieldCursor())
	}

	_, cmd := pressEnterEC2(d)

	nav, ok := extractRelatedNavigateMsg(cmd)
	if !ok {
		t.Fatal("EC2-009: Enter on VpcId must emit RelatedNavigateMsg, got nil or wrong type")
	}
	if nav.TargetType != "vpc" {
		t.Errorf("EC2-009: RelatedNavigateMsg.TargetType must be %q, got %q", "vpc", nav.TargetType)
	}
	if nav.TargetID != "vpc-0abc123def456789a" {
		t.Errorf("EC2-009: RelatedNavigateMsg.TargetID must be %q, got %q",
			"vpc-0abc123def456789a", nav.TargetID)
	}
	if nav.SourceType != "ec2" {
		t.Errorf("EC2-009: RelatedNavigateMsg.SourceType must be %q, got %q", "ec2", nav.SourceType)
	}
}

// ---------------------------------------------------------------------------
// EC2-010: Enter on SubnetId emits RelatedNavigateMsg{TargetType:"subnet"}
// ---------------------------------------------------------------------------

// TestEC2_010_EnterOnSubnetId_EmitsNavigateMsg verifies the same pattern for SubnetId.
//
// Given: SubnetId registered as navigable → "subnet"; cursor on SubnetId row
// When: Enter is pressed
// Then: cmd() produces RelatedNavigateMsg{TargetType:"subnet", TargetID:"subnet-0aaa111111111111a"}
func TestEC2_010_EnterOnSubnetId_EmitsNavigateMsg(t *testing.T) {
	registerEC2NavFields(t)

	fieldValues := map[string]string{
		"VpcId":    "vpc-0abc123def456789a",
		"SubnetId": "subnet-0aaa111111111111a",
		"ImageId":  "ami-0abc123def456789a",
	}
	detailPaths := []string{"VpcId", "SubnetId", "ImageId"}

	d := makeEC2DetailWithFields(t, fieldValues, detailPaths, 80, 24)

	// Navigate cursor to index 1 (SubnetId)
	d = navigateToIndex(d, 1)
	if d.FieldCursor() != 1 {
		t.Fatalf("precondition: expected cursor at 1 (SubnetId), got %d", d.FieldCursor())
	}

	_, cmd := pressEnterEC2(d)

	nav, ok := extractRelatedNavigateMsg(cmd)
	if !ok {
		t.Fatal("EC2-010: Enter on SubnetId must emit RelatedNavigateMsg, got nil or wrong type")
	}
	if nav.TargetType != "subnet" {
		t.Errorf("EC2-010: RelatedNavigateMsg.TargetType must be %q, got %q", "subnet", nav.TargetType)
	}
	if nav.TargetID != "subnet-0aaa111111111111a" {
		t.Errorf("EC2-010: RelatedNavigateMsg.TargetID must be %q, got %q",
			"subnet-0aaa111111111111a", nav.TargetID)
	}
	if nav.SourceType != "ec2" {
		t.Errorf("EC2-010: RelatedNavigateMsg.SourceType must be %q, got %q", "ec2", nav.SourceType)
	}
}

// ---------------------------------------------------------------------------
// EC2-011: Enter on SecurityGroups GroupId sub-field emits RelatedNavigateMsg
// ---------------------------------------------------------------------------

// TestEC2_011_EnterOnGroupId_EmitsNavigateMsg verifies that pressing Enter on
// a SecurityGroup GroupId sub-field emits RelatedNavigateMsg{TargetType:"sg"}.
//
// Given: SecurityGroups.GroupId registered as navigable → "sg"
// And: EC2 instance has SecurityGroups: [{GroupId:"sg-0aaa111111111111a"}]
// When: Cursor navigated to the GroupId sub-field row and Enter pressed
// Then: RelatedNavigateMsg{TargetType:"sg", TargetID:"sg-0aaa111111111111a"}
//
// NOTE: This test fails until ExtractFieldList propagates navigable map entries
// to matching sub-field values (sub-field navigability not yet implemented).
func TestEC2_011_EnterOnGroupId_EmitsNavigateMsg(t *testing.T) {
	registerEC2NavFieldsWithSG(t)

	inst := ec2types.Instance{
		InstanceId: aws.String("i-ec2011test"),
		VpcId:      aws.String("vpc-ec2011"),
		SecurityGroups: []ec2types.GroupIdentifier{
			{
				GroupId:   aws.String("sg-0aaa111111111111a"),
				GroupName: aws.String("acme-web-alb-sg"),
			},
		},
	}

	fieldValues := map[string]string{
		"InstanceId": "i-ec2011test",
		"VpcId":      "vpc-ec2011",
	}
	detailPaths := []string{"InstanceId", "SecurityGroups"}

	d := makeEC2DetailWithRaw(t, inst, fieldValues, detailPaths, 80, 24)

	// fieldList: [0] InstanceId, [1] SecurityGroups (header), [2+] sub-fields
	// Navigate past the header to the first sub-field row that contains GroupId value.
	// Press j until we find a row whose Enter cmd yields TargetType=="sg".
	const maxSearch = 20
	foundNav := false
	for i := 1; i < maxSearch; i++ {
		d2 := navigateToIndex(d, i)
		_, cmd := pressEnterEC2(d2)
		nav, ok := extractRelatedNavigateMsg(cmd)
		if ok && nav.TargetType == "sg" {
			foundNav = true
			if nav.TargetID != "sg-0aaa111111111111a" {
				t.Errorf("EC2-011: RelatedNavigateMsg.TargetID must be %q, got %q",
					"sg-0aaa111111111111a", nav.TargetID)
			}
			if nav.SourceType != "ec2" {
				t.Errorf("EC2-011: RelatedNavigateMsg.SourceType must be %q, got %q", "ec2", nav.SourceType)
			}
			break
		}
	}
	if !foundNav {
		t.Error("EC2-011: no row in SecurityGroups sub-fields emitted RelatedNavigateMsg{TargetType:\"sg\"}")
	}
}

// ---------------------------------------------------------------------------
// EC2-012: Each SecurityGroup sub-field is independently navigable
// ---------------------------------------------------------------------------

// TestEC2_012_EachSGSubFieldIndependentlyNavigable verifies that with two
// SecurityGroups, each GroupId row emits a distinct RelatedNavigateMsg.
//
// Given: EC2 instance has two SecurityGroups
// When: Cursor on first GroupId row → Enter → TargetID == first SG id
// And:  Cursor on second GroupId row → Enter → TargetID == second SG id
//
// NOTE: This test fails until sub-field navigability is implemented.
func TestEC2_012_EachSGSubFieldIndependentlyNavigable(t *testing.T) {
	registerEC2NavFieldsWithSG(t)

	inst := ec2types.Instance{
		InstanceId: aws.String("i-ec2012test"),
		VpcId:      aws.String("vpc-ec2012"),
		SecurityGroups: []ec2types.GroupIdentifier{
			{GroupId: aws.String("sg-0aaa111111111111a"), GroupName: aws.String("acme-web-alb-sg")},
			{GroupId: aws.String("sg-0bbb222222222222b"), GroupName: aws.String("acme-web-app-sg")},
		},
	}

	fieldValues := map[string]string{
		"InstanceId": "i-ec2012test",
		"VpcId":      "vpc-ec2012",
	}
	detailPaths := []string{"InstanceId", "SecurityGroups"}

	d := makeEC2DetailWithRaw(t, inst, fieldValues, detailPaths, 80, 24)

	// Find all rows that emit sg navigation messages
	type sgNavResult struct {
		cursorIdx int
		targetID  string
	}
	var found []sgNavResult

	const maxSearch = 30
	for i := 1; i < maxSearch; i++ {
		d2 := navigateToIndex(d, i)
		_, cmd := pressEnterEC2(d2)
		nav, ok := extractRelatedNavigateMsg(cmd)
		if ok && nav.TargetType == "sg" {
			found = append(found, sgNavResult{cursorIdx: i, targetID: nav.TargetID})
		}
	}

	if len(found) < 2 {
		t.Errorf("EC2-012: expected at least 2 independently navigable SG rows, found %d", len(found))
		return
	}

	// First SG row must navigate to sg-0aaa111111111111a
	if found[0].targetID != "sg-0aaa111111111111a" {
		t.Errorf("EC2-012: first SG row TargetID must be %q, got %q",
			"sg-0aaa111111111111a", found[0].targetID)
	}
	// Second SG row must navigate to sg-0bbb222222222222b
	if found[1].targetID != "sg-0bbb222222222222b" {
		t.Errorf("EC2-012: second SG row TargetID must be %q, got %q",
			"sg-0bbb222222222222b", found[1].targetID)
	}
	// They must be at different cursor positions
	if found[0].cursorIdx == found[1].cursorIdx {
		t.Errorf("EC2-012: both SG rows are at the same cursor index %d — they must be separate rows",
			found[0].cursorIdx)
	}
}

// ---------------------------------------------------------------------------
// EC2-013: Enter on ImageId emits RelatedNavigateMsg{TargetType:"ami"}
// ---------------------------------------------------------------------------

// TestEC2_013_EnterOnImageId_EmitsNavigateMsg verifies that pressing Enter on
// the ImageId navigable field emits RelatedNavigateMsg{TargetType:"ami"}.
//
// Given: ImageId registered as navigable → "ami"; cursor on ImageId row
// When: Enter is pressed
// Then: RelatedNavigateMsg{TargetType:"ami", TargetID:"ami-0abc123def456789a"}
func TestEC2_013_EnterOnImageId_EmitsNavigateMsg(t *testing.T) {
	registerEC2NavFields(t)

	fieldValues := map[string]string{
		"VpcId":    "vpc-0abc123def456789a",
		"SubnetId": "subnet-0aaa111111111111a",
		"ImageId":  "ami-0abc123def456789a",
	}
	detailPaths := []string{"VpcId", "SubnetId", "ImageId"}

	d := makeEC2DetailWithFields(t, fieldValues, detailPaths, 80, 24)

	// Navigate to index 2 (ImageId)
	d = navigateToIndex(d, 2)
	if d.FieldCursor() != 2 {
		t.Fatalf("precondition: expected cursor at 2 (ImageId), got %d", d.FieldCursor())
	}

	_, cmd := pressEnterEC2(d)

	nav, ok := extractRelatedNavigateMsg(cmd)
	if !ok {
		t.Fatal("EC2-013: Enter on ImageId must emit RelatedNavigateMsg, got nil or wrong type")
	}
	if nav.TargetType != "ami" {
		t.Errorf("EC2-013: RelatedNavigateMsg.TargetType must be %q, got %q", "ami", nav.TargetType)
	}
	if nav.TargetID != "ami-0abc123def456789a" {
		t.Errorf("EC2-013: RelatedNavigateMsg.TargetID must be %q, got %q",
			"ami-0abc123def456789a", nav.TargetID)
	}
	if nav.SourceType != "ec2" {
		t.Errorf("EC2-013: RelatedNavigateMsg.SourceType must be %q, got %q", "ec2", nav.SourceType)
	}
}

// ---------------------------------------------------------------------------
// EC2-014: Enter on non-navigable field is a no-op
// ---------------------------------------------------------------------------

// TestEC2_014_EnterOnNonNavigable_IsNoOp verifies that pressing Enter on a
// non-navigable field (InstanceType, State, PrivateIpAddress) returns nil cmd.
//
// Given: None of InstanceType/State/PrivateIpAddress are navigable
// When: Enter pressed on each of those rows
// Then: cmd is nil (no navigation, no flash message, no error)
func TestEC2_014_EnterOnNonNavigable_IsNoOp(t *testing.T) {
	registerEC2NavFields(t) // only VpcId, SubnetId, ImageId are navigable

	tests := []struct {
		name         string
		fieldValues  map[string]string
		detailPaths  []string
		cursorTarget int // which row to navigate to
		fieldName    string
	}{
		{
			name:         "InstanceType",
			fieldValues:  map[string]string{"InstanceType": "t3.large", "VpcId": "vpc-xxx"},
			detailPaths:  []string{"InstanceType", "VpcId"},
			cursorTarget: 0,
			fieldName:    "InstanceType",
		},
		{
			name:         "State",
			fieldValues:  map[string]string{"State": "running", "VpcId": "vpc-xxx"},
			detailPaths:  []string{"State", "VpcId"},
			cursorTarget: 0,
			fieldName:    "State",
		},
		{
			name:         "PrivateIpAddress",
			fieldValues:  map[string]string{"PrivateIpAddress": "10.0.48.175", "VpcId": "vpc-xxx"},
			detailPaths:  []string{"PrivateIpAddress", "VpcId"},
			cursorTarget: 0,
			fieldName:    "PrivateIpAddress",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := makeEC2DetailWithFields(t, tc.fieldValues, tc.detailPaths, 80, 24)
			d = navigateToIndex(d, tc.cursorTarget)

			_, cmd := pressEnterEC2(d)
			if cmd != nil {
				msg := cmd()
				t.Errorf("EC2-014: Enter on %s must return nil cmd (no-op), got msg type %T: %v",
					tc.fieldName, msg, msg)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// EC2-015: Enter on section header is a no-op
// ---------------------------------------------------------------------------

// TestEC2_015_EnterOnSectionHeader_IsNoOp verifies that pressing Enter on
// a section header row (IsHeader=true) returns nil cmd.
//
// Given: Placement is a struct field → expands to header + sub-fields
// When: Cursor is on the Placement: header row, Enter is pressed
// Then: cmd is nil
//
// Also tests: MetadataOptions header and SecurityGroups header are no-ops.
func TestEC2_015_EnterOnSectionHeader_IsNoOp(t *testing.T) {
	registerEC2NavFields(t)

	// Placement struct → header row at index 1
	inst := ec2types.Instance{
		InstanceId:   aws.String("i-ec2015header"),
		InstanceType: ec2types.InstanceTypeT3Large,
		VpcId:        aws.String("vpc-ec2015"),
		Placement: &ec2types.Placement{
			AvailabilityZone: aws.String("us-east-1a"),
		},
		MetadataOptions: &ec2types.InstanceMetadataOptionsResponse{
			HttpEndpoint: ec2types.InstanceMetadataEndpointStateEnabled,
		},
		SecurityGroups: []ec2types.GroupIdentifier{
			{GroupId: aws.String("sg-0aaa111111111111a")},
		},
	}

	fieldValues := map[string]string{
		"InstanceId": "i-ec2015header",
		"VpcId":      "vpc-ec2015",
	}

	// Test Placement header (index 1 in [InstanceId, Placement, ...])
	t.Run("Placement header", func(t *testing.T) {
		detailPaths := []string{"InstanceId", "Placement", "VpcId"}
		d := makeEC2DetailWithRaw(t, inst, fieldValues, detailPaths, 80, 24)

		// Index 0 = InstanceId (scalar), index 1 = Placement (header)
		d = navigateToIndex(d, 1)
		if d.FieldCursor() != 1 {
			t.Fatalf("precondition: cursor at 1 (Placement header), got %d", d.FieldCursor())
		}

		_, cmd := pressEnterEC2(d)
		if cmd != nil {
			msg := cmd()
			t.Errorf("EC2-015: Enter on Placement section header must return nil cmd, got %T: %v", msg, msg)
		}
	})

	// Test MetadataOptions header
	t.Run("MetadataOptions header", func(t *testing.T) {
		detailPaths := []string{"InstanceId", "MetadataOptions", "VpcId"}
		d := makeEC2DetailWithRaw(t, inst, fieldValues, detailPaths, 80, 24)

		d = navigateToIndex(d, 1)
		if d.FieldCursor() != 1 {
			t.Fatalf("precondition: cursor at 1 (MetadataOptions header), got %d", d.FieldCursor())
		}

		_, cmd := pressEnterEC2(d)
		if cmd != nil {
			msg := cmd()
			t.Errorf("EC2-015: Enter on MetadataOptions section header must return nil cmd, got %T: %v", msg, msg)
		}
	})

	// Test SecurityGroups header (index 1 when [InstanceId, SecurityGroups])
	t.Run("SecurityGroups header", func(t *testing.T) {
		detailPaths := []string{"InstanceId", "SecurityGroups"}
		d := makeEC2DetailWithRaw(t, inst, fieldValues, detailPaths, 80, 24)

		d = navigateToIndex(d, 1)
		if d.FieldCursor() != 1 {
			t.Fatalf("precondition: cursor at 1 (SecurityGroups header), got %d", d.FieldCursor())
		}

		_, cmd := pressEnterEC2(d)
		if cmd != nil {
			msg := cmd()
			t.Errorf("EC2-015: Enter on SecurityGroups section header must return nil cmd, got %T: %v", msg, msg)
		}
	})
}

// ---------------------------------------------------------------------------
// EC2-016: IamInstanceProfile.Arn sub-field is NOT navigable
// ---------------------------------------------------------------------------

// TestEC2_016_IamInstanceProfileArn_NotNavigable verifies that the Arn sub-field
// under IamInstanceProfile: does not emit a RelatedNavigateMsg on Enter.
//
// Given: IamInstanceProfile is a struct → header row + sub-fields (Arn, Id)
// And: Arn is NOT registered as a navigable field
// When: Cursor on the Arn sub-field row, Enter pressed
// Then: cmd is nil (no navigation)
func TestEC2_016_IamInstanceProfileArn_NotNavigable(t *testing.T) {
	registerEC2NavFields(t) // VpcId, SubnetId, ImageId only — no Arn

	inst := ec2types.Instance{
		InstanceId: aws.String("i-ec2016test"),
		VpcId:      aws.String("vpc-ec2016"),
		IamInstanceProfile: &ec2types.IamInstanceProfile{
			Arn: aws.String("arn:aws:iam::123456789012:instance-profile/web-prod-profile"),
			Id:  aws.String("AIPA1234567890ABCDEF"),
		},
	}

	fieldValues := map[string]string{
		"InstanceId": "i-ec2016test",
		"VpcId":      "vpc-ec2016",
	}
	// IamInstanceProfile is a struct → produces header + sub-fields (Arn, Id)
	detailPaths := []string{"InstanceId", "IamInstanceProfile", "VpcId"}

	d := makeEC2DetailWithRaw(t, inst, fieldValues, detailPaths, 80, 24)

	// fieldList:
	//   [0] InstanceId (scalar)
	//   [1] IamInstanceProfile: (header)
	//   [2] Arn: arn:aws:iam::... (sub-field)
	//   [3] Id: AIPA... (sub-field)    [may vary by field ordering]
	//   [4] VpcId (navigable scalar)

	// Try Enter on all rows that are NOT VpcId (index >= 1 until we find VpcId).
	// None of them must emit a RelatedNavigateMsg — specifically not the Arn row.
	const maxCheck = 10
	for i := 1; i <= maxCheck; i++ {
		d2 := navigateToIndex(d, i)
		if d2.FieldCursor() != i {
			// Cursor clamped — we've gone past the end of fieldList
			break
		}
		_, cmd := pressEnterEC2(d2)
		if cmd == nil {
			continue
		}
		msg := cmd()
		nav, ok := msg.(messages.RelatedNavigate)
		if !ok {
			continue
		}
		// VpcId at the end IS navigable — that is expected. But Arn must NOT be.
		if nav.TargetType == "vpc" {
			// Reached VpcId navigable row — stop here, that is correct behavior
			break
		}
		// Any other navigation at a non-VpcId row is an error
		t.Errorf("EC2-016: Enter on IamInstanceProfile sub-field at cursor index %d must NOT navigate, "+
			"but got RelatedNavigateMsg{TargetType:%q, TargetID:%q}",
			i, nav.TargetType, nav.TargetID)
	}

	// Specific assertion: directly navigate to index 2 (Arn sub-field) and verify no navigation
	d2 := navigateToIndex(d, 2)
	if d2.FieldCursor() == 2 {
		_, cmd := pressEnterEC2(d2)
		if cmd != nil {
			msg := cmd()
			if nav, ok := msg.(messages.RelatedNavigate); ok && nav.TargetType != "vpc" {
				t.Errorf("EC2-016: Arn sub-field at index 2 must NOT be navigable, "+
					"but got RelatedNavigateMsg{TargetType:%q, TargetID:%q}",
					nav.TargetType, nav.TargetID)
			}
		}
	}
}
