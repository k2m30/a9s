package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/fieldpath"
)

// TestToSnakeCase_CharacterizationPinnedBehavior pins the current PascalCase→snake_case
// conversion including its acronym edge cases. This is a CHARACTERIZATION test — its
// purpose is to make current behavior a conscious contract, not an accidental one.
//
// WARNING: If a future change "fixes" ACM → a_c_m, review both call sites first:
//   - internal/fieldpath/extract.go ExtractFieldList fallback lookup (line ~400)
//   - internal/tui/views/detail_render.go renderFromConfig fallback lookup (line ~140)
//
// Both only pass PascalCase paths like VpcId today, so the limitation never fires.
// If you need acronym-aware snake_case, add a new function — don't edit this one
// without auditing those call sites.
func TestToSnakeCase_CharacterizationPinnedBehavior(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		// Primary use case: PascalCase single-word fields — the happy path all callers hit.
		{"VpcId", "vpc_id"},
		{"InstanceId", "instance_id"},
		{"SecurityGroupId", "security_group_id"},
		{"DBInstanceIdentifier", "d_b_instance_identifier"}, // ← pinned acronym behavior
		// Acronym edge cases — documented limitation:
		{"ACM", "a_c_m"},
		{"EBSSnap", "e_b_s_snap"},
		{"ARN", "a_r_n"},
		// Lowercase and mixed
		{"id", "id"},
		{"vpcId", "vpc_id"},
		{"", ""},
		{"A", "a"},
		{"AB", "a_b"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got := fieldpath.ToSnakeCase(tt.in)
			if got != tt.want {
				t.Errorf("ToSnakeCase(%q) = %q, want %q (characterization — see test comment before changing)", tt.in, got, tt.want)
			}
		})
	}
}
