package unit_test

// ctdetail_demo_nav_test.go — navigation tests for ct-events detail view.
//
// Case H: Insight RunInstances ApiCallRateInsight — ZERO navigable fields.
// Uses demo fixture "e-b8c9d0e1" from internal/demo/fixtures_monitoring.go.
//
// Insight events have no userIdentity, no resources[], and null requestParameters,
// so the detail view must not produce any IsNavigable FieldItem. Pressing Enter
// at every cursor position must return nil or a non-RelatedNavigateMsg cmd.

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// assertTargetResolves verifies that nav.TargetID is present in the demo
// fixture set for nav.TargetType. It calls t.Errorf (not t.Fatalf) so the
// caller subtest can still report other failures. An empty fixture set is a
// hard failure (t.Fatalf) because that indicates a missing demo fixture, not
// just a bad ID.
func assertTargetResolves(t *testing.T, nav messages.RelatedNavigateMsg, subtestName string) {
	t.Helper()
	targets, ok := demo.GetResources(nav.TargetType)
	if !ok || len(targets) == 0 {
		t.Fatalf("%s: demo.GetResources(%q) returned zero resources — fixture missing for target type",
			subtestName, nav.TargetType)
	}
	var found bool
	for _, r := range targets {
		if r.ID == nav.TargetID {
			found = true
			break
		}
	}
	if !found {
		ids := make([]string, 0, len(targets))
		for _, r := range targets {
			ids = append(ids, r.ID)
		}
		t.Errorf("%s: TargetID %q not found in demo.GetResources(%q). Available IDs: %v",
			subtestName, nav.TargetID, nav.TargetType, ids)
	}
}

// TestCTDetailDemoNav_CaseH walks every cursor position in the Insight
// "e-b8c9d0e1" detail view and asserts that Enter never emits a
// RelatedNavigateMsg. If any position produces a navigation message, the
// Insight fixture has an unexpected navigable field — a design violation.
func TestCTDetailDemoNav_CaseH(t *testing.T) {
	ensureNoColor(t)

	// Load the demo fixture for ct-events.
	resources, ok := demo.GetResources("ct-events")
	if !ok || len(resources) == 0 {
		t.Fatal("demo.GetResources(\"ct-events\") returned no fixtures")
	}

	// Find fixture "e-b8c9d0e1" (Case H — Insight ApiCallRateInsight).
	var caseHIdx int = -1
	for i, r := range resources {
		if r.ID == "e-b8c9d0e1" {
			caseHIdx = i
			break
		}
	}
	if caseHIdx == -1 {
		t.Fatal("demo fixture \"e-b8c9d0e1\" not found in ct-events fixtures")
	}
	res := resources[caseHIdx]

	// Build the detail model at a standard size.
	// Width 80 is intentionally below the right-column auto-show threshold (<100)
	// so Enter falls through to the navigable-field handler, not right-column focus.
	cfg := configForType("ct-events")
	m := newDetailModel(res, "ct-events", cfg)
	m.SetSize(80, 40)

	// Determine field count by moving down until the cursor stops advancing.
	// We drive the model purely through Update() — no direct fieldList access.
	const maxFields = 50 // defensive upper bound; Insight events have ~6-10 rows
	fieldCount := countFields(t, m)
	if fieldCount == 0 {
		t.Fatal("fieldList appears empty for Insight fixture — ct-events branch may not be active (T027 required)")
	}
	t.Logf("Case H: fieldList has %d entries", fieldCount)

	// Walk every cursor position and press Enter; assert no RelatedNavigateMsg.
	m2 := newDetailModel(res, "ct-events", cfg)
	m2.SetSize(80, 40)

	for pos := 0; pos < fieldCount; pos++ {
		// Press Enter at the current cursor position.
		_, cmd := m2.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

		if cmd != nil {
			msg := cmd()
			if _, isNav := msg.(messages.RelatedNavigateMsg); isNav {
				t.Errorf("cursor position %d: Enter emitted RelatedNavigateMsg — "+
					"Insight fixture must have ZERO navigable fields (design violation)", pos)
			}
		}

		// Advance the cursor to the next row.
		m2, _ = m2.Update(tea.KeyPressMsg{Code: -1, Text: "j"})
	}
}

// TestCTDetailDemoNav_CaseG verifies that the three navigable fields in the
// Case G fixture ("e-a7b8c9d0", cross-account PutObject) each dispatch a
// RelatedNavigateMsg with the correct TargetType, TargetID, and SourceType.
//
// FieldList layout for Case G (AssumedRole, Management/AwsApiCall, cross-account):
//
//	idx 0 — ACTOR      (IsSection)
//	idx 1 — Principal  (AssumedRole ARN, IsNavigable=true, TargetType="role")
//	idx 2 — Access key
//	idx 3 — User agent
//	idx 4 — ACTION     (IsSection)
//	idx 5 — Event      (s3:PutObject, severity=ct-attention)
//	idx 6 — TARGET     (IsSection)
//	idx 7 — Bucket     (IsNavigable=true, TargetType="s3", Value="shared-artifacts")
//	idx 8 — Object     (IsNavigable=true, TargetType="s3", Value="shared-artifacts/build-4821.tar.gz")
//
// Cursor navigation (section headers are auto-skipped by 'j'):
//
//	Principal → j×1
//	Bucket    → j×5  (skips ACTION header at 4, TARGET header at 6)
//	Object    → j×6
func TestCTDetailDemoNav_CaseG(t *testing.T) {
	ensureNoColor(t)

	// Load demo fixtures for ct-events.
	resources, ok := demo.GetResources("ct-events")
	if !ok || len(resources) == 0 {
		t.Fatal("demo.GetResources(\"ct-events\") returned no fixtures")
	}

	// Locate fixture "e-a7b8c9d0" (Case G — cross-account PutObject).
	var caseGIdx int = -1
	for i, r := range resources {
		if r.ID == "e-a7b8c9d0" {
			caseGIdx = i
			break
		}
	}
	if caseGIdx == -1 {
		t.Fatal("demo fixture \"e-a7b8c9d0\" not found in ct-events fixtures")
	}
	res := resources[caseGIdx]

	cfg := configForType("ct-events")
	base := newDetailModel(res, "ct-events", cfg)

	// Guard: the TARGET section must be present (ctdetail branch active).
	view := stripAnsi(base.View())
	if !strings.Contains(view, "TARGET") {
		t.Skipf("TARGET section not found — ctdetail branch not active; view:\n%s", view)
	}

	jPress := tea.KeyPressMsg{Code: -1, Text: "j"}
	enterPress := tea.KeyPressMsg{Code: tea.KeyEnter}

	// -----------------------------------------------------------------------
	// Subtest 1: Principal row → RelatedNavigateMsg{TargetType:"role", SourceType:"ct-events"}
	// Navigate j×1: ACTOR(section) → Principal
	// -----------------------------------------------------------------------
	t.Run("Principal dispatches role navigate", func(t *testing.T) {
		m := base
		m, _ = m.Update(jPress) // idx 0 → idx 1 (Principal)

		_, cmd := m.Update(enterPress)
		if cmd == nil {
			t.Fatal("Enter on Principal row returned nil cmd — Principal row is not navigable")
		}
		msg := cmd()
		navMsg, ok := msg.(messages.RelatedNavigateMsg)
		if !ok {
			t.Fatalf("Enter on Principal returned %T, want messages.RelatedNavigateMsg", msg)
		}

		if navMsg.TargetType != "role" {
			t.Errorf("Principal TargetType = %q, want %q", navMsg.TargetType, "role")
		}
		if navMsg.SourceType != "ct-events" {
			t.Errorf("Principal SourceType = %q, want %q", navMsg.SourceType, "ct-events")
		}
		// TargetID must be the bare role name extracted from the assumed-role ARN.
		// Full ARN: "arn:aws:sts::888888888888:assumed-role/CiBuildRole/build-4821"
		// Correct behavior: strip to "CiBuildRole" (role name segment, not full ARN).
		// BUG: production code currently passes the full ARN as TargetID — this test will FAIL.
		if navMsg.TargetID != "CiBuildRole" {
			t.Errorf("Principal TargetID = %q, want %q", navMsg.TargetID, "CiBuildRole")
		}
		assertTargetResolves(t, navMsg, "TestCTDetailDemoNav_CaseG/Principal dispatches role navigate")
	})

	// -----------------------------------------------------------------------
	// Subtest 2: Bucket row → RelatedNavigateMsg{TargetType:"s3", TargetID:"shared-artifacts"}
	// Navigate j×5: skips ACTION section header and TARGET section header.
	// -----------------------------------------------------------------------
	t.Run("Bucket dispatches s3 navigate", func(t *testing.T) {
		m := base
		for i := 0; i < 5; i++ {
			m, _ = m.Update(jPress)
		}

		_, cmd := m.Update(enterPress)
		if cmd == nil {
			t.Fatal("Enter on Bucket row returned nil cmd — Bucket row is not navigable")
		}
		msg := cmd()
		navMsg, ok := msg.(messages.RelatedNavigateMsg)
		if !ok {
			t.Fatalf("Enter on Bucket returned %T, want messages.RelatedNavigateMsg", msg)
		}

		if navMsg.TargetType != "s3" {
			t.Errorf("Bucket TargetType = %q, want %q", navMsg.TargetType, "s3")
		}
		if navMsg.SourceType != "ct-events" {
			t.Errorf("Bucket SourceType = %q, want %q", navMsg.SourceType, "ct-events")
		}
		if navMsg.TargetID != "shared-artifacts" {
			t.Errorf("Bucket TargetID = %q, want %q", navMsg.TargetID, "shared-artifacts")
		}
		assertTargetResolves(t, navMsg, "TestCTDetailDemoNav_CaseG/Bucket dispatches s3 navigate")
	})

	// -----------------------------------------------------------------------
	// Subtest 3: Object row → RelatedNavigateMsg{TargetType:"s3", TargetID:"shared-artifacts"}
	// Navigate j×6: one additional j from Bucket position (idx 7 → idx 8).
	// NavID is set to the bucket name so Enter on the Object row navigates to the
	// parent bucket, not the object path. The display Value retains the full key.
	// -----------------------------------------------------------------------
	t.Run("Object dispatches s3 navigate", func(t *testing.T) {
		m := base
		for i := 0; i < 6; i++ {
			m, _ = m.Update(jPress)
		}

		_, cmd := m.Update(enterPress)
		if cmd == nil {
			t.Fatal("Enter on Object row returned nil cmd — Object row is not navigable")
		}
		msg := cmd()
		navMsg, ok := msg.(messages.RelatedNavigateMsg)
		if !ok {
			t.Fatalf("Enter on Object returned %T, want messages.RelatedNavigateMsg", msg)
		}

		if navMsg.TargetType != "s3" {
			t.Errorf("Object TargetType = %q, want %q", navMsg.TargetType, "s3")
		}
		if navMsg.SourceType != "ct-events" {
			t.Errorf("Object SourceType = %q, want %q", navMsg.SourceType, "ct-events")
		}
		if navMsg.TargetID != "shared-artifacts" {
			t.Errorf("Object TargetID = %q, want %q", navMsg.TargetID, "shared-artifacts")
		}
		assertTargetResolves(t, navMsg, "TestCTDetailDemoNav_CaseG/Object dispatches s3 navigate")
	})
}

// TestCTDetailDemoNav_CaseA verifies that the Principal row of the Karpenter
// DescribeInstances demo fixture dispatches RelatedNavigateMsg{TargetType:"role",
// TargetID:<full assumed-role ARN>, SourceType:"ct-events"} when Enter is pressed.
//
// Fixture "e-a1b2c3d4" (Case A) — AssumedRole / KarpenterNodeRole, read-only,
// no error, no resources[], requestParameters has a filterSet (no instancesSet).
//
// FieldList layout (AssumedRole, Management/AwsApiCall, no MFA, has AccessKeyID,
// has UserAgent, requestParameters.filterSet present → Instances:(all)):
//
//	idx 0 — ACTOR      (IsSection, skipped by Down handler)
//	idx 1 — Principal  (AssumedRole ARN, IsNavigable=true, TargetType="role")
//	idx 2 — Access key
//	idx 3 — User agent
//	idx 4 — ACTION     (IsSection, skipped)
//	idx 5 — Event      (ec2:DescribeInstances, severity=ct-info)
//
// Cursor walk: one Down press from idx 0 (section header is auto-skipped)
// lands at idx 1 — the Principal row.
func TestCTDetailDemoNav_CaseA(t *testing.T) {
	ensureNoColor(t)

	// Load demo fixtures for ct-events.
	resources, ok := demo.GetResources("ct-events")
	if !ok || len(resources) == 0 {
		t.Fatal("demo.GetResources(\"ct-events\") returned no fixtures")
	}

	// Locate fixture "e-a1b2c3d4" (Case A — Karpenter DescribeInstances).
	var caseAIdx = -1
	for i, r := range resources {
		if r.ID == "e-a1b2c3d4" {
			caseAIdx = i
			break
		}
	}
	if caseAIdx == -1 {
		t.Fatal("demo fixture \"e-a1b2c3d4\" not found in ct-events fixtures")
	}
	res := resources[caseAIdx]

	// Build the detail model at the required viewport size.
	cfg := configForType("ct-events")
	m := newDetailModel(res, "ct-events", cfg)
	m.SetSize(180, 40)

	// Guard: the ACTOR section must be present — verifies ctdetail branch is active.
	view := stripAnsi(m.View())
	if !strings.Contains(view, "ACTOR") {
		t.Skipf("ACTOR section not found — ct-events branch not yet implemented; view:\n%s", view)
	}

	jPress := tea.KeyPressMsg{Code: -1, Text: "j"}
	enterPress := tea.KeyPressMsg{Code: tea.KeyEnter}

	// -----------------------------------------------------------------------
	// Step 1: Move cursor down once.
	// fieldCursor starts at 0 (ACTOR section header, IsSection=true).
	// Down handler increments then skips IsSection entries, landing at idx 1.
	// -----------------------------------------------------------------------
	m1, _ := m.Update(jPress)

	if m1.FieldCursor() != 1 {
		t.Fatalf("TestCTDetailDemoNav_CaseA: after one Down press expected fieldCursor=1, got %d",
			m1.FieldCursor())
	}

	// -----------------------------------------------------------------------
	// Step 2: Press Enter on the Principal row (IsNavigable=true, TargetType="role").
	// -----------------------------------------------------------------------
	_, cmd := m1.Update(enterPress)

	if cmd == nil {
		t.Fatal("TestCTDetailDemoNav_CaseA: Enter on Principal row returned nil cmd — navigate not triggered")
	}

	msg := cmd()
	navMsg, ok := msg.(messages.RelatedNavigateMsg)
	if !ok {
		t.Fatalf("TestCTDetailDemoNav_CaseA: Enter returned %T, want messages.RelatedNavigateMsg", msg)
	}

	// TargetType must be "role" — arnTargetType() maps assumed-role ARNs to "role".
	if navMsg.TargetType != "role" {
		t.Errorf("TestCTDetailDemoNav_CaseA: TargetType = %q, want \"role\"", navMsg.TargetType)
	}

	// SourceType must be "ct-events".
	if navMsg.SourceType != "ct-events" {
		t.Errorf("TestCTDetailDemoNav_CaseA: SourceType = %q, want \"ct-events\"", navMsg.SourceType)
	}

	// TargetID must be the bare role name extracted from the assumed-role ARN.
	// Case A JSON: userIdentity.arn = "arn:aws:sts::111111111111:assumed-role/KarpenterNodeRole/karpenter-1759"
	// Correct behavior: strip to "KarpenterNodeRole" (role name segment, not full ARN).
	// BUG: production code currently passes the full ARN as TargetID — this test will FAIL.
	const wantRoleName = "KarpenterNodeRole"
	if navMsg.TargetID != wantRoleName {
		t.Errorf("TestCTDetailDemoNav_CaseA: TargetID = %q, want %q", navMsg.TargetID, wantRoleName)
	}
	assertTargetResolves(t, navMsg, "TestCTDetailDemoNav_CaseA/Principal")

	// -----------------------------------------------------------------------
	// Step 3: Verify the adjacent rows in the ACTOR section are NOT role-navigable.
	// idx 2 = Access key — not navigable.
	// idx 3 = User agent — not navigable.
	// Pressing Enter on these must not dispatch a "role" RelatedNavigateMsg.
	// -----------------------------------------------------------------------
	m2, _ := m1.Update(jPress) // idx 1 → idx 2 (Access key)
	_, cmd2 := m2.Update(enterPress)
	if cmd2 != nil {
		msg2 := cmd2()
		if nav2, isNav := msg2.(messages.RelatedNavigateMsg); isNav && nav2.TargetType == "role" {
			t.Errorf("TestCTDetailDemoNav_CaseA: unexpected \"role\" navigate from Access key row (idx 2): %+v", nav2)
		}
	}

	m3, _ := m2.Update(jPress) // idx 2 → idx 3 (User agent)
	_, cmd3 := m3.Update(enterPress)
	if cmd3 != nil {
		msg3 := cmd3()
		if nav3, isNav := msg3.(messages.RelatedNavigateMsg); isNav && nav3.TargetType == "role" {
			t.Errorf("TestCTDetailDemoNav_CaseA: unexpected \"role\" navigate from User agent row (idx 3): %+v", nav3)
		}
	}
}

// TestCTDetailDemoNav_CaseD runs two subtests for the kms:RotateKey AwsServiceEvent
// fixture "e-d4e5f6a7":
//
//  1. Key_IsNavigable: pressing Enter on the KEY (TARGET) row dispatches
//     RelatedNavigateMsg{TargetType:"kms", TargetID:"2f7e9a5b-8c1d-4e3f-9a0b-1c2d3e4f5a6b",
//     SourceType:"ct-events"}.
//
//  2. Service_IsNotNavigable: pressing Enter on the SERVICE row in the ACTOR section
//     dispatches nil (AwsServiceEvent has no userIdentity ARN — not navigable).
//
// FieldList layout after RotateKey feature implementation:
//
//	idx 0 — ACTOR (IsSection, auto-skipped by 'j')
//	idx 1 — Service: kms.amazonaws.com (not navigable)
//	idx 2 — ACTION (IsSection, auto-skipped by 'j')
//	idx 3 — Event: kms:RotateKey
//	idx 4 — Category: Management / AwsServiceEvent
//	idx 5 — TARGET (IsSection, auto-skipped by 'j')
//	idx 6 — Key: 2f7e9a5b-8c1d-4e3f-9a0b-1c2d3e4f5a6b (IsNavigable=true, TargetType="kms")
//
// Cursor path to Key row:     j×4 (skips ACTOR header at 0, ACTION header at 2, TARGET header at 5).
// Cursor path to Service row: j×1.
func TestCTDetailDemoNav_CaseD(t *testing.T) {
	ensureNoColor(t)

	// Load the demo fixture for ct-events.
	resources, ok := demo.GetResources("ct-events")
	if !ok || len(resources) == 0 {
		t.Fatal("demo.GetResources(\"ct-events\") returned no fixtures")
	}

	// Find fixture "e-d4e5f6a7" (Case D — kms:RotateKey AwsServiceEvent).
	var caseDIdx int = -1
	for i, r := range resources {
		if r.ID == "e-d4e5f6a7" {
			caseDIdx = i
			break
		}
	}
	if caseDIdx == -1 {
		t.Fatal("demo fixture \"e-d4e5f6a7\" not found in ct-events fixtures")
	}
	res := resources[caseDIdx]

	// Build the detail model.
	// Width 80 is intentionally below the right-column auto-show threshold (<100)
	// so Enter falls through to the navigable-field handler, not right-column focus.
	cfg := configForType("ct-events")
	base := newDetailModel(res, "ct-events", cfg)
	base.SetSize(80, 40)

	// Guard: verify ct-events branch is active (ACTOR section must be present).
	view := stripAnsi(base.View())
	if !strings.Contains(view, "ACTOR") {
		t.Skipf("ACTOR section not found — ct-events branch not yet implemented; view:\n%s", view)
	}

	jPress := tea.KeyPressMsg{Code: -1, Text: "j"}
	enterPress := tea.KeyPressMsg{Code: tea.KeyEnter}

	// -----------------------------------------------------------------------
	// Subtest 1: Key row → RelatedNavigateMsg{TargetType:"kms", TargetID:"2f7e9a5b-..."}
	// Navigate j×4: skips ACTOR header (0), ACTION header (2), TARGET header (5).
	// -----------------------------------------------------------------------
	t.Run("Key_IsNavigable", func(t *testing.T) {
		m := base
		for i := 0; i < 4; i++ {
			m, _ = m.Update(jPress)
		}

		_, cmd := m.Update(enterPress)
		if cmd == nil {
			t.Fatal("TestCTDetailDemoNav_CaseD/Key_IsNavigable: Enter on Key row returned nil cmd — " +
				"Key row is not navigable (RotateKey case not yet added to extractByEventName)")
		}

		msg := cmd()
		navMsg, ok := msg.(messages.RelatedNavigateMsg)
		if !ok {
			t.Fatalf("TestCTDetailDemoNav_CaseD/Key_IsNavigable: Enter returned %T, want messages.RelatedNavigateMsg", msg)
		}

		const wantTargetType = "kms"
		if navMsg.TargetType != wantTargetType {
			t.Errorf("TestCTDetailDemoNav_CaseD/Key_IsNavigable: TargetType = %q, want %q", navMsg.TargetType, wantTargetType)
		}

		const wantSourceType = "ct-events"
		if navMsg.SourceType != wantSourceType {
			t.Errorf("TestCTDetailDemoNav_CaseD/Key_IsNavigable: SourceType = %q, want %q", navMsg.SourceType, wantSourceType)
		}

		// TargetID must be the bare KMS key UUID (no "key/" prefix, no full ARN).
		// KMS resource IDs in a9s are bare UUIDs as returned by ListKeys — the
		// navigator matches TargetID against Resource.ID, so "key/<uuid>" would miss.
		const wantTargetID = "2f7e9a5b-8c1d-4e3f-9a0b-1c2d3e4f5a6b"
		if navMsg.TargetID != wantTargetID {
			t.Errorf("TestCTDetailDemoNav_CaseD/Key_IsNavigable: TargetID = %q, want %q", navMsg.TargetID, wantTargetID)
		}
		assertTargetResolves(t, navMsg, "TestCTDetailDemoNav_CaseD/Key_IsNavigable")
	})

	// -----------------------------------------------------------------------
	// Subtest 2: Service row → no RelatedNavigateMsg (AwsServiceEvent, no ARN).
	// Navigate j×1: ACTOR(section, idx 0) → Service (idx 1).
	// -----------------------------------------------------------------------
	t.Run("Service_IsNotNavigable", func(t *testing.T) {
		m := base
		m, _ = m.Update(jPress) // idx 0 (ACTOR/IsSection) → idx 1 (Service: kms.amazonaws.com)

		_, cmd := m.Update(enterPress)
		if cmd != nil {
			msg := cmd()
			if _, isNav := msg.(messages.RelatedNavigateMsg); isNav {
				t.Fatal("TestCTDetailDemoNav_CaseD/Service_IsNotNavigable: Enter on Service row dispatched " +
					"RelatedNavigateMsg — AwsServiceEvent Service row must not be navigable (no userIdentity ARN)")
			}
		}
	})
}

// TestCTDetailDemoNav_CaseE verifies navigation behaviour for the ct-events
// detail view when opened on demo fixture "e-e5f6a7b8" (Root PutBucketPolicy).
//
// Subtest Bucket (positive): Enter on the TARGET Bucket row dispatches
// RelatedNavigateMsg{TargetType:"s3", TargetID:"prod-artifacts", SourceType:"ct-events"}.
// This is a TDD test — it will fail until s3:PutBucketPolicy is added to the
// ExtractTarget per-event-name lookup table so that requestParameters.bucketName
// is extracted as a navigable Bucket row (IsNavigable=true, TargetType="s3")
// instead of the current catch-all "Resource" row (IsNavigable=false).
//
// Subtest RootPrincipal (negative): Enter on the ACTOR Principal row
// (ARN "arn:aws:iam::555555555555:root") must not produce a useful navigation.
// a9s has no "root" resource type — if a RelatedNavigateMsg is dispatched,
// its TargetType must be empty.
//
// FieldList layout for Case E (Root, PutBucketPolicy, Management/AwsApiCall):
//
//	idx 0 — ACTOR        (IsSection — auto-skipped by 'j')
//	idx 1 — Principal    (arn:aws:iam::555555555555:root, IsNavigable=true, TargetType="")
//	idx 2 — User agent
//	idx 3 — ACTION       (IsSection — skipped)
//	idx 4 — Event        (s3:PutBucketPolicy, severity=ct-attention)
//	idx 5 — TARGET       (IsSection — skipped)
//	idx 6 — Bucket       (prod-artifacts, IsNavigable=true, TargetType="s3") ← post-impl
//	         currently:   Resource (prod-artifacts, IsNavigable=false)        ← catch-all
//
// Cursor navigation (j presses from initial position 0):
//
//	j×1 → Principal (idx 1)
//	j×4 → Bucket    (idx 6, skips ACTION header at 3, TARGET header at 5)
func TestCTDetailDemoNav_CaseE(t *testing.T) {
	ensureNoColor(t)

	// Load demo fixture for ct-events.
	resources, ok := demo.GetResources("ct-events")
	if !ok || len(resources) == 0 {
		t.Fatal("demo.GetResources(\"ct-events\") returned no fixtures")
	}

	// Find fixture "e-e5f6a7b8" (Case E — Root PutBucketPolicy).
	var caseEIdx int = -1
	for i, r := range resources {
		if r.ID == "e-e5f6a7b8" {
			caseEIdx = i
			break
		}
	}
	if caseEIdx == -1 {
		t.Fatal("demo fixture \"e-e5f6a7b8\" not found in ct-events fixtures")
	}
	res := resources[caseEIdx]

	// Width 80 is intentionally below the right-column auto-show threshold (<100)
	// so Enter falls through to the navigable-field handler, not right-column focus.
	cfg := configForType("ct-events")
	jPress := tea.KeyPressMsg{Code: -1, Text: "j"}
	enterPress := tea.KeyPressMsg{Code: tea.KeyEnter}

	// -----------------------------------------------------------------------
	// Subtest: Bucket (positive) — TARGET row → s3 navigate
	// j×4: skips ACTION header at idx 3 and TARGET header at idx 5.
	// -----------------------------------------------------------------------
	t.Run("Bucket", func(t *testing.T) {
		m := newDetailModel(res, "ct-events", cfg)
		m.SetSize(80, 30)

		for i := 0; i < 4; i++ {
			m, _ = m.Update(jPress)
		}

		_, cmd := m.Update(enterPress)
		if cmd == nil {
			t.Fatal("Enter on Bucket TARGET row returned nil cmd " +
				"(TDD: fails until PutBucketPolicy is added to extractByEventName in ctdetail/target.go)")
		}

		msg := cmd()
		navMsg, ok := msg.(messages.RelatedNavigateMsg)
		if !ok {
			t.Fatalf("Enter on Bucket returned %T, want messages.RelatedNavigateMsg", msg)
		}
		if navMsg.TargetType != "s3" {
			t.Errorf("Bucket TargetType = %q, want %q", navMsg.TargetType, "s3")
		}
		if navMsg.TargetID != "prod-artifacts" {
			t.Errorf("Bucket TargetID = %q, want %q", navMsg.TargetID, "prod-artifacts")
		}
		if navMsg.SourceType != "ct-events" {
			t.Errorf("Bucket SourceType = %q, want %q", navMsg.SourceType, "ct-events")
		}
		assertTargetResolves(t, navMsg, "TestCTDetailDemoNav_CaseE/Bucket")
	})

	// -----------------------------------------------------------------------
	// Subtest: RootPrincipal (negative) — ACTOR Principal row must not navigate
	// j×1: ACTOR section header (idx 0) → Principal row (idx 1).
	// arnTargetType("arn:aws:iam::555555555555:root") returns "" — no target type.
	// -----------------------------------------------------------------------
	t.Run("RootPrincipal", func(t *testing.T) {
		m := newDetailModel(res, "ct-events", cfg)
		m.SetSize(80, 30)

		m, _ = m.Update(jPress) // idx 0 (ACTOR section) → idx 1 (Principal)

		_, cmd := m.Update(enterPress)

		if cmd == nil {
			// Acceptable: Principal (Root) produces no cmd at all.
			return
		}

		msg := cmd()
		navMsg, ok := msg.(messages.RelatedNavigateMsg)
		if !ok {
			// Acceptable: a non-navigate message (e.g. CopiedMsg or no-op).
			return
		}

		// If a RelatedNavigateMsg was dispatched, TargetType must be empty.
		// The Root ARN has no registered a9s resource type — routing to "" is a no-op.
		if navMsg.TargetType != "" {
			t.Errorf("Root ARN must not navigate to a typed resource; TargetType = %q, want %q",
				navMsg.TargetType, "")
		}
	})
}

// TestCTDetailDemoNav_CaseC verifies that the three navigable fields in the
// Case C detail view (fixture "e-c3d4e5f6": IAMUser bob, s3:PutObject
// AccessDenied) each emit a RelatedNavigateMsg with the correct TargetType,
// TargetID, and SourceType when Enter is pressed.
//
// TargetID values verified against fixtures_monitoring.go "e-c3d4e5f6":
//   - Principal: userIdentity.arn          = "arn:aws:iam::333333333333:user/bob"
//   - Bucket:    requestParameters.bucketName = "prod-logs"
//   - Object:    requestParameters.key        = "prod-logs/2026/04/07/app.log"
//
// Field list layout (confirmed from golden testdata/golden/ctdetail_demo_case_c.txt
// and sections.go buildActorRows — IAMUser, has AccessKeyID and UserAgent, no MFA):
//
//	Index  Content
//	    0  ACTOR        (section header — initial cursor position)
//	    1  Principal    (navigable, iam-user, "arn:aws:iam::333333333333:user/bob")
//	    2  Access key
//	    3  User agent
//	    4  ACTION       (section header)
//	    5  Event        (s3:PutObject, ct-danger)
//	    6  TARGET       (section header)
//	    7  Bucket       (navigable, s3, "prod-logs")
//	    8  Object       (navigable, s3, "prod-logs/2026/04/07/app.log")
//	    9  CONTEXT      (section header)
//	   ...
//
// Down-key trace from initial position 0 (section headers are auto-skipped):
//
//	1 press: 0→1 (Principal — stop)
//	2 press: 1→2 (Access key)
//	3 press: 2→3 (User agent)
//	4 press: 3→4 (ACTION, IsSection) skip→5 (Event — stop)
//	5 press: 5→6 (TARGET, IsSection) skip→7 (Bucket — stop)
//	6 press: 7→8 (Object — stop)
func TestCTDetailDemoNav_CaseC(t *testing.T) {
	ensureNoColor(t)

	// Load demo fixtures for ct-events.
	resources, ok := demo.GetResources("ct-events")
	if !ok || len(resources) == 0 {
		t.Fatal("demo.GetResources(\"ct-events\") returned no fixtures")
	}

	// Locate fixture "e-c3d4e5f6" (Case C — IAMUser bob, s3:PutObject AccessDenied).
	var caseCIdx int = -1
	for i, r := range resources {
		if r.ID == "e-c3d4e5f6" {
			caseCIdx = i
			break
		}
	}
	if caseCIdx == -1 {
		t.Fatal("demo fixture \"e-c3d4e5f6\" not found in ct-events fixtures")
	}
	res := resources[caseCIdx]

	cfg := configForType("ct-events")
	// Width 80: below the right-column auto-show threshold, so Enter falls through
	// to navigable-field handling rather than the right-column focus path.
	base := newDetailModel(res, "ct-events", cfg)
	base.SetSize(80, 40)

	// Guard: TARGET section must be present — verifies ctdetail branch is active.
	view := stripAnsi(base.View())
	if !strings.Contains(view, "TARGET") {
		t.Skipf("TARGET section not found — ctdetail branch not active; view:\n%s", view)
	}

	jPress := tea.KeyPressMsg{Code: -1, Text: "j"}
	enterPress := tea.KeyPressMsg{Code: tea.KeyEnter}

	// -----------------------------------------------------------------------
	// Subtest 1: Principal row → iam-user navigate
	// 1 Down press: ACTOR(section, idx 0) → Principal (idx 1, IsNavigable=true).
	// TargetType="iam-user" via arnTargetType() on ":user/" ARN.
	// -----------------------------------------------------------------------
	t.Run("Principal navigates to iam-user", func(t *testing.T) {
		m := base
		m, _ = m.Update(jPress) // idx 0 → idx 1 (Principal)

		_, cmd := m.Update(enterPress)
		if cmd == nil {
			t.Fatal("Enter on Principal returned nil cmd — field not navigable or cursor misaligned")
		}
		msg := cmd()
		nav, ok := msg.(messages.RelatedNavigateMsg)
		if !ok {
			t.Fatalf("cmd() returned %T, want messages.RelatedNavigateMsg", msg)
		}
		if nav.TargetType != "iam-user" {
			t.Errorf("Principal: TargetType = %q, want %q", nav.TargetType, "iam-user")
		}
		// TargetID must be the bare user name extracted from the IAM user ARN.
		// Full ARN: "arn:aws:iam::333333333333:user/bob"
		// Correct behavior: strip to "bob" (user name segment, not full ARN).
		// BUG: production code currently passes the full ARN as TargetID — this test will FAIL.
		if nav.TargetID != "bob" {
			t.Errorf("Principal: TargetID = %q, want %q", nav.TargetID, "bob")
		}
		if nav.SourceType != "ct-events" {
			t.Errorf("Principal: SourceType = %q, want %q", nav.SourceType, "ct-events")
		}
		assertTargetResolves(t, nav, "TestCTDetailDemoNav_CaseC/Principal navigates to iam-user")
	})

	// -----------------------------------------------------------------------
	// Subtest 2: Bucket TARGET row → s3 navigate
	// 5 Down presses, skipping ACTION header at idx 4 and TARGET header at idx 6,
	// landing at idx 7 (Bucket, IsNavigable=true, TargetType="s3").
	// navFromLabel("Bucket") returns (true, "s3").
	// -----------------------------------------------------------------------
	t.Run("Bucket TARGET row navigates to s3", func(t *testing.T) {
		m := base
		for i := 0; i < 5; i++ {
			m, _ = m.Update(jPress)
		}

		_, cmd := m.Update(enterPress)
		if cmd == nil {
			t.Fatal("Enter on Bucket returned nil cmd — field not navigable or cursor misaligned")
		}
		msg := cmd()
		nav, ok := msg.(messages.RelatedNavigateMsg)
		if !ok {
			t.Fatalf("cmd() returned %T, want messages.RelatedNavigateMsg", msg)
		}
		if nav.TargetType != "s3" {
			t.Errorf("Bucket: TargetType = %q, want %q", nav.TargetType, "s3")
		}
		if nav.TargetID != "prod-logs" {
			t.Errorf("Bucket: TargetID = %q, want %q", nav.TargetID, "prod-logs")
		}
		if nav.SourceType != "ct-events" {
			t.Errorf("Bucket: SourceType = %q, want %q", nav.SourceType, "ct-events")
		}
		assertTargetResolves(t, nav, "TestCTDetailDemoNav_CaseC/Bucket TARGET row navigates to s3")
	})

	// -----------------------------------------------------------------------
	// Subtest 3: Object TARGET row → s3 navigate
	// 6 Down presses (one additional j from Bucket at idx 7) → idx 8
	// (Object, IsNavigable=true, TargetType="s3").
	// navFromLabel("Object") returns (true, "s3").
	// NavID = "prod-logs" (bucket name) — pressing Enter on the Object row navigates
	// to the parent bucket, not the object path. Display Value remains the full key.
	// -----------------------------------------------------------------------
	t.Run("Object TARGET row navigates to s3", func(t *testing.T) {
		m := base
		for i := 0; i < 6; i++ {
			m, _ = m.Update(jPress)
		}

		_, cmd := m.Update(enterPress)
		if cmd == nil {
			t.Fatal("Enter on Object returned nil cmd — field not navigable or cursor misaligned")
		}
		msg := cmd()
		nav, ok := msg.(messages.RelatedNavigateMsg)
		if !ok {
			t.Fatalf("cmd() returned %T, want messages.RelatedNavigateMsg", msg)
		}
		if nav.TargetType != "s3" {
			t.Errorf("Object: TargetType = %q, want %q", nav.TargetType, "s3")
		}
		if nav.TargetID != "prod-logs" {
			t.Errorf("Object: TargetID = %q, want %q", nav.TargetID, "prod-logs")
		}
		if nav.SourceType != "ct-events" {
			t.Errorf("Object: SourceType = %q, want %q", nav.SourceType, "ct-events")
		}
		assertTargetResolves(t, nav, "TestCTDetailDemoNav_CaseC/Object TARGET row navigates to s3")
	})
}

// TestCTDetailDemoNav_CaseB verifies that pressing Enter on each of the three
// navigable rows in the Case B fixture emits the expected RelatedNavigateMsg.
//
// Case B: SSO Console ec2:TerminateInstances (ct-danger, MFA=true).
// Fixture "e-b2c3d4e5" from internal/demo/fixtures_monitoring.go.
//
// FieldList layout for Case B (Management/AwsApiCall, AssumedRole via SSO):
//
//	idx 0 — ACTOR       (IsSection, auto-skipped by Down handler)
//	idx 1 — Principal   (SSO assumed-role ARN, IsNavigable=true, TargetType="role")
//	idx 2 — MFA         ("yes")
//	idx 3 — Access key
//	idx 4 — User agent
//	idx 5 — ACTION      (IsSection, auto-skipped)
//	idx 6 — Event       (ec2:TerminateInstances, severity=ct-danger)
//	idx 7 — TARGET      (IsSection, auto-skipped)
//	idx 8 — Instance    (IsNavigable=true, TargetType="ec2", Value="i-0a1b2c3d4e5f60001")
//	idx 9 — Instance    (IsNavigable=true, TargetType="ec2", Value="i-0a1b2c3d4e5f60002")
//
// Cursor navigation (section headers auto-skipped by 'j'):
//
//	Principal  → j×1  (idx 0 → idx 1)
//	Instance 1 → j×6  (skips ACTION header at 5, TARGET header at 7; net idx 8)
//	Instance 2 → j×7  (one additional j from Instance 1; idx 9)
func TestCTDetailDemoNav_CaseB(t *testing.T) {
	ensureNoColor(t)

	// Load demo fixtures for ct-events.
	resources, ok := demo.GetResources("ct-events")
	if !ok || len(resources) == 0 {
		t.Fatal("demo.GetResources(\"ct-events\") returned no fixtures")
	}

	// Locate fixture "e-b2c3d4e5" (Case B — SSO TerminateInstances).
	var caseBIdx int = -1
	for i, r := range resources {
		if r.ID == "e-b2c3d4e5" {
			caseBIdx = i
			break
		}
	}
	if caseBIdx == -1 {
		t.Fatal("demo fixture \"e-b2c3d4e5\" not found in ct-events fixtures")
	}
	res := resources[caseBIdx]

	cfg := configForType("ct-events")
	base := newDetailModel(res, "ct-events", cfg)

	// Guard: the TARGET section must be present (ctdetail branch active).
	view := stripAnsi(base.View())
	if !strings.Contains(view, "TARGET") {
		t.Skipf("TARGET section not found — ctdetail branch not active; view:\n%s", view)
	}

	jPress := tea.KeyPressMsg{Code: -1, Text: "j"}
	enterPress := tea.KeyPressMsg{Code: tea.KeyEnter}

	// -----------------------------------------------------------------------
	// Subtest 1: Principal row → RelatedNavigateMsg{TargetType:"role", SourceType:"ct-events"}
	// ARN: arn:aws:sts::222222222222:assumed-role/AWSReservedSSO_AdminAccess_3c4d5e6f7a8b9c0d/alice@corp
	// arnTargetType returns "role" for :assumed-role/ ARNs.
	// TargetID is the role name extracted from the assumed-role ARN segment.
	// Navigate j×1: ACTOR(section, idx 0) → Principal (idx 1).
	// -----------------------------------------------------------------------
	t.Run("Principal dispatches role navigate", func(t *testing.T) {
		m := base
		m, _ = m.Update(jPress) // idx 0 → idx 1 (Principal)

		_, cmd := m.Update(enterPress)
		if cmd == nil {
			t.Fatal("Enter on Principal row returned nil cmd — Principal row is not navigable")
		}
		msg := cmd()
		navMsg, ok := msg.(messages.RelatedNavigateMsg)
		if !ok {
			t.Fatalf("Enter on Principal returned %T, want messages.RelatedNavigateMsg", msg)
		}

		if navMsg.TargetType != "role" {
			t.Errorf("Principal TargetType = %q, want %q", navMsg.TargetType, "role")
		}
		if navMsg.SourceType != "ct-events" {
			t.Errorf("Principal SourceType = %q, want %q", navMsg.SourceType, "ct-events")
		}
		// TargetID must be the role name from the SSO assumed-role ARN.
		// Full ARN: arn:aws:sts::222222222222:assumed-role/AWSReservedSSO_AdminAccess_3c4d5e6f7a8b9c0d/alice@corp
		// Expected role identifier: the name segment after assumed-role/.
		const wantRoleName = "AWSReservedSSO_AdminAccess_3c4d5e6f7a8b9c0d"
		if navMsg.TargetID != wantRoleName {
			t.Errorf("Principal TargetID = %q, want %q", navMsg.TargetID, wantRoleName)
		}
		assertTargetResolves(t, navMsg, "TestCTDetailDemoNav_CaseB/Principal dispatches role navigate")
	})

	// -----------------------------------------------------------------------
	// Subtest 2: Instance 1 row → RelatedNavigateMsg{TargetType:"ec2", TargetID:"i-0a1b2c3d4e5f60001"}
	// Source: requestParameters.instancesSet.items[0].instanceId in CloudTrailEvent JSON.
	// Navigate j×6: skips ACTION section header (idx 5) and TARGET section header (idx 7).
	// -----------------------------------------------------------------------
	t.Run("Instance1 dispatches ec2 navigate", func(t *testing.T) {
		m := base
		for i := 0; i < 6; i++ {
			m, _ = m.Update(jPress)
		}

		_, cmd := m.Update(enterPress)
		if cmd == nil {
			t.Fatal("Enter on Instance 1 row returned nil cmd — Instance row is not navigable")
		}
		msg := cmd()
		navMsg, ok := msg.(messages.RelatedNavigateMsg)
		if !ok {
			t.Fatalf("Enter on Instance 1 returned %T, want messages.RelatedNavigateMsg", msg)
		}

		if navMsg.TargetType != "ec2" {
			t.Errorf("Instance 1 TargetType = %q, want %q", navMsg.TargetType, "ec2")
		}
		if navMsg.SourceType != "ct-events" {
			t.Errorf("Instance 1 SourceType = %q, want %q", navMsg.SourceType, "ct-events")
		}
		if navMsg.TargetID != "i-0a1b2c3d4e5f60001" {
			t.Errorf("Instance 1 TargetID = %q, want %q", navMsg.TargetID, "i-0a1b2c3d4e5f60001")
		}
		assertTargetResolves(t, navMsg, "TestCTDetailDemoNav_CaseB/Instance1 dispatches ec2 navigate")
	})

	// -----------------------------------------------------------------------
	// Subtest 3: Instance 2 row → RelatedNavigateMsg{TargetType:"ec2", TargetID:"i-0a1b2c3d4e5f60002"}
	// Source: requestParameters.instancesSet.items[1].instanceId in CloudTrailEvent JSON.
	// Navigate j×7: one additional j from Instance 1 position (idx 8 → idx 9).
	// -----------------------------------------------------------------------
	t.Run("Instance2 dispatches ec2 navigate", func(t *testing.T) {
		m := base
		for i := 0; i < 7; i++ {
			m, _ = m.Update(jPress)
		}

		_, cmd := m.Update(enterPress)
		if cmd == nil {
			t.Fatal("Enter on Instance 2 row returned nil cmd — Instance row is not navigable")
		}
		msg := cmd()
		navMsg, ok := msg.(messages.RelatedNavigateMsg)
		if !ok {
			t.Fatalf("Enter on Instance 2 returned %T, want messages.RelatedNavigateMsg", msg)
		}

		if navMsg.TargetType != "ec2" {
			t.Errorf("Instance 2 TargetType = %q, want %q", navMsg.TargetType, "ec2")
		}
		if navMsg.SourceType != "ct-events" {
			t.Errorf("Instance 2 SourceType = %q, want %q", navMsg.SourceType, "ct-events")
		}
		if navMsg.TargetID != "i-0a1b2c3d4e5f60002" {
			t.Errorf("Instance 2 TargetID = %q, want %q", navMsg.TargetID, "i-0a1b2c3d4e5f60002")
		}
		assertTargetResolves(t, navMsg, "TestCTDetailDemoNav_CaseB/Instance2 dispatches ec2 navigate")
	})
}

// TestCTDetailDemoNav_CaseF verifies that pressing Enter on each of the three
// navigable field rows in demo fixture "e-f6a7b8c9" dispatches a
// messages.RelatedNavigateMsg with the correct TargetType, TargetID, and
// SourceType.
//
// Fixture "e-f6a7b8c9" is an IRSA (WebIdentity) AssumedRole GetObject event:
//   - userIdentity.arn             = "arn:aws:sts::666666666666:assumed-role/eks-checkout-svc-sa/1717156821993453824"
//   - requestParameters.bucketName = "checkout-config"
//   - requestParameters.key        = "checkout-config/prod/config.json"
//
// The resources[] array in the CloudTrailEvent JSON is absent, so ExtractTarget
// falls through to the §2 per-event-name lookup (extractS3ObjectEvent) which
// produces navigable Bucket and Object rows from requestParameters.
//
// NOTE on Federation row: the CloudTrail JSON places "webIdFederationData" at the
// userIdentity level (not inside sessionContext). The rawUserIdentity parser maps
// it to rawSessionContext.WebIDFederationData only when it appears under
// sessionContext — so WebIDFederationData is nil after parsing and NO Federation
// row is emitted. Verified against tests/testdata/golden/ctdetail_demo_case_f.txt.
//
// FieldList layout after BuildSections (verified from golden file):
//
//	Index  Content
//	    0  ACTOR      (section header — initial cursor position, auto-skipped)
//	    1  Principal  (full assumed-role ARN, IsNavigable=true, TargetType="role")
//	    2  User agent (not navigable; no Federation row — see note above)
//	    3  ACTION     (section header, auto-skipped)
//	    4  Event      (s3:GetObject, severity=ct-info, not navigable)
//	    5  TARGET     (section header, auto-skipped)
//	    6  Bucket     (checkout-config, IsNavigable=true, TargetType="s3")
//	    7  Object     (checkout-config/prod/config.json, IsNavigable=true, TargetType="s3")
//	    8  CONTEXT    (section header)
//	   ...
//
// Down-key trace from initial position 0 (section headers auto-skipped by 'j'):
//
//	1 press: 0→1 (Principal — stop)
//	2 press: 1→2 (User agent)
//	3 press: 2→3 (ACTION, IsSection) skip→4 (Event — stop)
//	4 press: 4→5 (TARGET, IsSection) skip→6 (Bucket — stop)
//	5 press: 6→7 (Object — stop)
func TestCTDetailDemoNav_CaseF(t *testing.T) {
	ensureNoColor(t)

	// Load demo fixtures for ct-events.
	resources, ok := demo.GetResources("ct-events")
	if !ok || len(resources) == 0 {
		t.Fatal("demo.GetResources(\"ct-events\") returned no fixtures")
	}

	// Locate fixture "e-f6a7b8c9" (Case F — IRSA s3:GetObject, WebIdentityUser).
	var caseFIdx int = -1
	for i, r := range resources {
		if r.ID == "e-f6a7b8c9" {
			caseFIdx = i
			break
		}
	}
	if caseFIdx == -1 {
		t.Fatal("demo fixture \"e-f6a7b8c9\" not found in ct-events fixtures")
	}
	res := resources[caseFIdx]

	cfg := configForType("ct-events")
	base := newDetailModel(res, "ct-events", cfg)

	// Guard: TARGET section must be present — verifies ctdetail branch is active.
	view := stripAnsi(base.View())
	if !strings.Contains(view, "TARGET") {
		t.Skipf("TARGET section not found — ctdetail branch not active; view:\n%s", view)
	}

	jPress := tea.KeyPressMsg{Code: -1, Text: "j"}
	enterPress := tea.KeyPressMsg{Code: tea.KeyEnter}

	// -----------------------------------------------------------------------
	// Subtest 1: Principal (ACTOR) → role navigate
	// 1 Down press: ACTOR(section, idx 0) → Principal (idx 1, IsNavigable=true).
	// TargetType="role" via arnTargetType() detecting ":assumed-role/" in the ARN.
	// TargetID = full ARN (item.Value = userIdentity.arn verbatim from JSON).
	// -----------------------------------------------------------------------
	t.Run("Principal dispatches role navigate", func(t *testing.T) {
		m := base
		m, _ = m.Update(jPress) // idx 0 → idx 1 (Principal)

		_, cmd := m.Update(enterPress)
		if cmd == nil {
			t.Fatal("Enter on Principal row returned nil cmd — Principal row is not navigable")
		}
		msg := cmd()
		nav, ok := msg.(messages.RelatedNavigateMsg)
		if !ok {
			t.Fatalf("cmd() returned %T, want messages.RelatedNavigateMsg", msg)
		}
		if nav.TargetType != "role" {
			t.Errorf("Principal: TargetType = %q, want %q", nav.TargetType, "role")
		}
		// TargetID must be the bare role name extracted from the assumed-role ARN.
		// Full ARN: "arn:aws:sts::666666666666:assumed-role/eks-checkout-svc-sa/1717156821993453824"
		// Correct behavior: strip to "eks-checkout-svc-sa" (role name segment, not full ARN).
		// BUG: production code currently passes the full ARN as TargetID — this test will FAIL.
		if nav.TargetID != "eks-checkout-svc-sa" {
			t.Errorf("Principal: TargetID = %q, want %q", nav.TargetID, "eks-checkout-svc-sa")
		}
		if nav.SourceType != "ct-events" {
			t.Errorf("Principal: SourceType = %q, want %q", nav.SourceType, "ct-events")
		}
		assertTargetResolves(t, nav, "TestCTDetailDemoNav_CaseF/Principal dispatches role navigate")
	})

	// -----------------------------------------------------------------------
	// Subtest 2: Bucket (TARGET) → s3 navigate
	// 4 Down presses: idx 0→1→2→3(ACTION,IsSection,skip)→4(Event)→
	//                 5(TARGET,IsSection,skip)→6 (Bucket).
	// navFromLabel("Bucket") returns (true, "s3"). TargetID = requestParameters.bucketName.
	// -----------------------------------------------------------------------
	t.Run("Bucket dispatches s3 navigate", func(t *testing.T) {
		m := base
		for i := 0; i < 4; i++ {
			m, _ = m.Update(jPress)
		}

		_, cmd := m.Update(enterPress)
		if cmd == nil {
			t.Fatal("Enter on Bucket row returned nil cmd — Bucket row is not navigable " +
				"(verify extractS3ObjectEvent is called for GetObject and returns IsNavigable=true)")
		}
		msg := cmd()
		nav, ok := msg.(messages.RelatedNavigateMsg)
		if !ok {
			t.Fatalf("cmd() returned %T, want messages.RelatedNavigateMsg", msg)
		}
		if nav.TargetType != "s3" {
			t.Errorf("Bucket: TargetType = %q, want %q", nav.TargetType, "s3")
		}
		if nav.TargetID != "checkout-config" {
			t.Errorf("Bucket: TargetID = %q, want %q", nav.TargetID, "checkout-config")
		}
		if nav.SourceType != "ct-events" {
			t.Errorf("Bucket: SourceType = %q, want %q", nav.SourceType, "ct-events")
		}
		assertTargetResolves(t, nav, "TestCTDetailDemoNav_CaseF/Bucket dispatches s3 navigate")
	})

	// -----------------------------------------------------------------------
	// Subtest 3: Object (TARGET) → s3 navigate
	// 5 Down presses: one additional j from Bucket (idx 6) → idx 7 (Object).
	// navFromLabel("Object") returns (true, "s3").
	// NavID = "checkout-config" (bucket name) — pressing Enter on the Object row
	// navigates to the parent bucket, not the object path.
	// -----------------------------------------------------------------------
	t.Run("Object dispatches s3 navigate", func(t *testing.T) {
		m := base
		for i := 0; i < 5; i++ {
			m, _ = m.Update(jPress)
		}

		_, cmd := m.Update(enterPress)
		if cmd == nil {
			t.Fatal("Enter on Object row returned nil cmd — Object row is not navigable " +
				"(verify extractS3ObjectEvent returns an Object row for requestParameters.key)")
		}
		msg := cmd()
		nav, ok := msg.(messages.RelatedNavigateMsg)
		if !ok {
			t.Fatalf("cmd() returned %T, want messages.RelatedNavigateMsg", msg)
		}
		if nav.TargetType != "s3" {
			t.Errorf("Object: TargetType = %q, want %q", nav.TargetType, "s3")
		}
		if nav.TargetID != "checkout-config" {
			t.Errorf("Object: TargetID = %q, want %q", nav.TargetID, "checkout-config")
		}
		if nav.SourceType != "ct-events" {
			t.Errorf("Object: SourceType = %q, want %q", nav.SourceType, "ct-events")
		}
		assertTargetResolves(t, nav, "TestCTDetailDemoNav_CaseF/Object dispatches s3 navigate")
	})
}

// TestCTDetailDemoNav_CaseI verifies navigation for the Case I fixture
// ("e-c9d0e1f2": NetworkActivity s3:PutObject VPCE deny from DataPipelineRole).
//
// Fixture "e-c9d0e1f2" from internal/demo/fixtures_monitoring.go:
//   - userIdentity.arn             = "arn:aws:sts::111111111111:assumed-role/DataPipelineRole/dp-0719"
//   - requestParameters.bucketName = "prod-lake"
//   - requestParameters.key        = "prod-lake/landing/2026/04/07/batch-0719.parquet"
//
// FieldList layout (AssumedRole, NetworkActivity/AwsVpceEvent, has error section):
//
//	idx 0  ACTOR      (section header — initial cursor position, auto-skipped)
//	idx 1  Principal  (full assumed-role ARN, IsNavigable=true, TargetType="role")
//	idx 2  User agent
//	idx 3  ACTION     (section header, auto-skipped)
//	idx 4  Event      (s3:PutObject, severity=ct-danger)
//	idx 5  Category   (NetworkActivity / AwsVpceEvent)
//	idx 6  TARGET     (section header, auto-skipped)
//	idx 7  Bucket     (prod-lake, IsNavigable=true, TargetType="s3")
//	idx 8  Object     (prod-lake/landing/2026/04/07/batch-0719.parquet, IsNavigable=true, TargetType="s3")
//	idx 9  CONTEXT    (section header)
//	idx 10 Region
//	idx 11 Source IP
//	idx 12 Time
//	idx 13 ERROR      (section header)
//	idx 14 errorCode
//	idx 15 errorMessage
//
// Down-key trace from initial position 0 (section headers auto-skipped by 'j'):
//
//	j×1 → Principal (idx 1)
//	j×5 → Bucket    (skips ACTION header at 3 and TARGET header at 6)
//	j×6 → Object    (one additional j from Bucket)
//
// The Principal subtest expects "DataPipelineRole" (bare role name) — this will FAIL
// until the ARN-stripping bug is fixed. Bucket and Object subtests expect the full
// bucket/key values and should PASS.
func TestCTDetailDemoNav_CaseI(t *testing.T) {
	ensureNoColor(t)

	// Load demo fixtures for ct-events.
	resources, ok := demo.GetResources("ct-events")
	if !ok || len(resources) == 0 {
		t.Fatal("demo.GetResources(\"ct-events\") returned no fixtures")
	}

	// Locate fixture "e-c9d0e1f2" (Case I — NetworkActivity VPCE deny, DataPipelineRole).
	var caseIIdx int = -1
	for i, r := range resources {
		if r.ID == "e-c9d0e1f2" {
			caseIIdx = i
			break
		}
	}
	if caseIIdx == -1 {
		t.Fatal("demo fixture \"e-c9d0e1f2\" not found in ct-events fixtures")
	}
	res := resources[caseIIdx]

	// Width 80: below the right-column auto-show threshold, so Enter falls through
	// to navigable-field handling rather than the right-column focus path.
	cfg := configForType("ct-events")
	base := newDetailModel(res, "ct-events", cfg)
	base.SetSize(80, 40)

	// Guard: TARGET section must be present — verifies ctdetail branch is active.
	view := stripAnsi(base.View())
	if !strings.Contains(view, "TARGET") {
		t.Skipf("TARGET section not found — ctdetail branch not active; view:\n%s", view)
	}

	jPress := tea.KeyPressMsg{Code: -1, Text: "j"}
	enterPress := tea.KeyPressMsg{Code: tea.KeyEnter}

	// -----------------------------------------------------------------------
	// Subtest 1: Principal (ACTOR) → role navigate
	// j×1: ACTOR(section, idx 0) → Principal (idx 1, IsNavigable=true).
	// TargetType="role" via arnTargetType() detecting ":assumed-role/" in the ARN.
	// TargetID must be "DataPipelineRole" (bare role name).
	// BUG: production code passes the full ARN — this subtest will FAIL until fixed.
	// -----------------------------------------------------------------------
	t.Run("Principal dispatches role navigate", func(t *testing.T) {
		m := base
		m, _ = m.Update(jPress) // idx 0 (ACTOR section) → idx 1 (Principal)

		_, cmd := m.Update(enterPress)
		if cmd == nil {
			t.Fatal("Enter on Principal row returned nil cmd — Principal row is not navigable")
		}
		msg := cmd()
		nav, ok := msg.(messages.RelatedNavigateMsg)
		if !ok {
			t.Fatalf("cmd() returned %T, want messages.RelatedNavigateMsg", msg)
		}
		if nav.TargetType != "role" {
			t.Errorf("Principal: TargetType = %q, want %q", nav.TargetType, "role")
		}
		if nav.SourceType != "ct-events" {
			t.Errorf("Principal: SourceType = %q, want %q", nav.SourceType, "ct-events")
		}
		// TargetID must be the bare role name extracted from the assumed-role ARN.
		// Full ARN: "arn:aws:sts::111111111111:assumed-role/DataPipelineRole/dp-0719"
		// Correct behavior: strip to "DataPipelineRole" (role name segment, not full ARN).
		// BUG: production code currently passes the full ARN as TargetID — this test will FAIL.
		if nav.TargetID != "DataPipelineRole" {
			t.Errorf("Principal: TargetID = %q, want %q", nav.TargetID, "DataPipelineRole")
		}
		assertTargetResolves(t, nav, "TestCTDetailDemoNav_CaseI/Principal dispatches role navigate")
	})

	// -----------------------------------------------------------------------
	// Subtest 2: Bucket (TARGET) → s3 navigate
	// j×5: skips ACTION header at idx 3 and TARGET header at idx 6.
	// TargetID = requestParameters.bucketName = "prod-lake".
	// -----------------------------------------------------------------------
	t.Run("Bucket dispatches s3 navigate", func(t *testing.T) {
		m := base
		for i := 0; i < 5; i++ {
			m, _ = m.Update(jPress)
		}

		_, cmd := m.Update(enterPress)
		if cmd == nil {
			t.Fatal("Enter on Bucket row returned nil cmd — Bucket row is not navigable")
		}
		msg := cmd()
		nav, ok := msg.(messages.RelatedNavigateMsg)
		if !ok {
			t.Fatalf("cmd() returned %T, want messages.RelatedNavigateMsg", msg)
		}
		if nav.TargetType != "s3" {
			t.Errorf("Bucket: TargetType = %q, want %q", nav.TargetType, "s3")
		}
		if nav.TargetID != "prod-lake" {
			t.Errorf("Bucket: TargetID = %q, want %q", nav.TargetID, "prod-lake")
		}
		if nav.SourceType != "ct-events" {
			t.Errorf("Bucket: SourceType = %q, want %q", nav.SourceType, "ct-events")
		}
		assertTargetResolves(t, nav, "TestCTDetailDemoNav_CaseI/Bucket dispatches s3 navigate")
	})

	// -----------------------------------------------------------------------
	// Subtest 3: Object (TARGET) → s3 navigate
	// j×6: one additional j from Bucket (idx 7) → idx 8 (Object).
	// NavID = "prod-lake" (bucket name) — pressing Enter on the Object row navigates
	// to the parent bucket, not the object path. Display Value retains the full key.
	// -----------------------------------------------------------------------
	t.Run("Object dispatches s3 navigate", func(t *testing.T) {
		m := base
		for i := 0; i < 6; i++ {
			m, _ = m.Update(jPress)
		}

		_, cmd := m.Update(enterPress)
		if cmd == nil {
			t.Fatal("Enter on Object row returned nil cmd — Object row is not navigable")
		}
		msg := cmd()
		nav, ok := msg.(messages.RelatedNavigateMsg)
		if !ok {
			t.Fatalf("cmd() returned %T, want messages.RelatedNavigateMsg", msg)
		}
		if nav.TargetType != "s3" {
			t.Errorf("Object: TargetType = %q, want %q", nav.TargetType, "s3")
		}
		if nav.TargetID != "prod-lake" {
			t.Errorf("Object: TargetID = %q, want %q", nav.TargetID, "prod-lake")
		}
		if nav.SourceType != "ct-events" {
			t.Errorf("Object: SourceType = %q, want %q", nav.SourceType, "ct-events")
		}
		assertTargetResolves(t, nav, "TestCTDetailDemoNav_CaseI/Object dispatches s3 navigate")
	})
}

// countFields returns the number of rows in the fieldList for the given model
// by pressing "j" until FieldCursor() stops advancing.
// It rebuilds a fresh model to avoid perturbing the caller's model state.
func countFields(t *testing.T, seed views.DetailModel) int {
	t.Helper()
	// Re-create from the same resource so we have a clean cursor at 0.
	cfg := configForType("ct-events")
	m := newDetailModel(seed.SourceResource(), "ct-events", cfg)
	m.SetSize(80, 40)

	prev := -1
	count := 0
	for i := 0; i < 200; i++ { // hard cap to prevent infinite loop
		cur := m.FieldCursor()
		if cur == prev {
			// Cursor did not advance — we've hit the bottom.
			break
		}
		prev = cur
		count++
		m, _ = m.Update(tea.KeyPressMsg{Code: -1, Text: "j"})
	}
	return count
}
