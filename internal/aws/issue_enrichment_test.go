package aws

import "testing"

// TestRegisterIssueEnricher_PanicsOnDuplicate pins the hard guard on the
// unexported registerIssueEnricher helper. With per-resource init() blocks
// registering every short name, a typo that reuses an existing key would
// silently overwrite the prior entry without this panic.
func TestRegisterIssueEnricher_PanicsOnDuplicate(t *testing.T) {
	orig, ok := IssueEnricherRegistry["ec2"]
	if !ok {
		t.Fatal("precondition: ec2 must be registered in IssueEnricherRegistry after package init")
	}
	defer func() {
		IssueEnricherRegistry["ec2"] = orig
		if r := recover(); r == nil {
			t.Fatal("registerIssueEnricher must panic on duplicate short name")
		}
	}()
	registerIssueEnricher("ec2", NoOpIssueEnricher, orig.Priority)
}
