package unit_test

// ctdetail_demo_rightcol_nav_test.go — right-column navigation dispatch tests.
//
// For each of the 9 demo ct-events fixtures (Cases A–I), this file tests that:
//   - pressing Tab focuses the right column (when actionable rows exist)
//   - pressing Enter on the correct row dispatches messages.RelatedNavigate
//   - the RelatedNavigateMsg.TargetType matches the expected group
//   - each RelatedID in the message resolves to a real demo fixture
//
// All tests here need rewrite onto the cold-cache harness (T047-T049, Phase 5 rewrite).

import (
	"testing"

	_ "github.com/k2m30/a9s/v3/internal/aws"
)

// Case A: e-a1b2c3d4 — Karpenter DescribeInstances (role only)
func TestCtEventsRightColumnNav_CaseA(t *testing.T) {
	t.Skip("needs rewrite onto cold-cache harness (T047-T049)")
}

// Case B: e-b2c3d4e5 — SSO TerminateInstances (role, ec2)
func TestCtEventsRightColumnNav_CaseB(t *testing.T) {
	t.Skip("needs rewrite onto cold-cache harness (T047-T049)")
}

// Case C: e-c3d4e5f6 — IAMUser PutObject AccessDenied (iam-user, s3, s3_objects)
func TestCtEventsRightColumnNav_CaseC(t *testing.T) {
	t.Skip("needs rewrite onto cold-cache harness (T047-T049)")
}

// Case D: e-d4e5f6a7 — KMS RotateKey AwsServiceEvent (kms only)
func TestCtEventsRightColumnNav_CaseD(t *testing.T) {
	t.Skip("needs rewrite onto cold-cache harness (T047-T049)")
}

// Case E: e-e5f6a7b8 — Root PutBucketPolicy (s3 only)
func TestCtEventsRightColumnNav_CaseE(t *testing.T) {
	t.Skip("needs rewrite onto cold-cache harness (T047-T049)")
}

// Case F: e-f6a7b8c9 — IRSA GetObject (role, s3, s3_objects, vpce)
func TestCtEventsRightColumnNav_CaseF(t *testing.T) {
	t.Skip("needs rewrite onto cold-cache harness (T047-T049)")
}

// Case G: e-a7b8c9d0 — CrossAccount PutObject (role, s3, s3_objects)
func TestCtEventsRightColumnNav_CaseG(t *testing.T) {
	t.Skip("needs rewrite onto cold-cache harness (T047-T049)")
}

// Case H: e-b8c9d0e1 — Insight RunInstances (no actionable typed-group rows)
func TestCtEventsRightColumnNav_CaseH(t *testing.T) {
	t.Skip("needs rewrite onto cold-cache harness (T047-T049)")
}

// Case I: e-c9d0e1f2 — NetworkActivity VPCE deny (role, s3, s3_objects, vpce)
func TestCtEventsRightColumnNav_CaseI(t *testing.T) {
	t.Skip("needs rewrite onto cold-cache harness (T047-T049)")
}
