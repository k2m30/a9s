package unit

// A Principal row is navigable only when it has somewhere to go: an ARN that
// ends at the type marker names a type but no principal, and a row that opens
// nothing must not offer to open.

import "testing"

func TestCTPrincipalRow_ARNWithNoNameIsNotNavigable(t *testing.T) {
	for _, arn := range []string{
		"arn:aws:iam::123456789012:role/",
		"arn:aws:iam::123456789012:user/",
		"arn:aws:sts::123456789012:assumed-role/",
	} {
		row := ct0916PrincipalRow(t, "Role", arn)
		if row.IsNavigable {
			t.Errorf("Principal.IsNavigable for %q = true, want false (NavID %q)", arn, row.NavID)
		}
		if row.Value != arn {
			t.Errorf("Principal.Value for %q = %q, want the full ARN", arn, row.Value)
		}
	}
}
