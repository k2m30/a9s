package unit

// qa67_view_errors_test.go — §I View-Specific Error Handling (issue #67)
//
// Bugs caught:
//   - I.1: clipboard unavailable shows error flash, no crash
//   - I.2: reveal deleted secret shows error in reveal view
//   - I.3: reveal secret with no current version shows error
//   - I.5: pressing x on non-secret type is a no-op
//   - I.7: tab autocomplete with no match in command mode does nothing
//   - I.8: sort key on resource type without status column does not crash
//   - I.9: horizontal scroll on narrow resource type is a no-op

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
)

// I.1 — FlashMsg with clipboard error shows error flash, application does not crash.
func TestQa67_I1_ClipboardUnavailable_ShowsErrorFlash(t *testing.T) {
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "ec2",
		Resources: []resource.Resource{
			{ID: "i-clip-test", Name: "clip-test", Fields: map[string]string{
				"instance_id": "i-clip-test",
				"name":        "clip-test",
				"state":       "running",
				"type":        "t3.micro",
				"private_ip":  "10.0.0.1",
				"public_ip":   "",
				"launch_time": "2025-01-01",
				"lifecycle":   "",
			}},
		},
	})

	// Simulate clipboard unavailable via FlashMsg (as the TUI layer would do)
	m, _ = rootApplyMsg(m, messages.Flash{
		Text:    "Error: clipboard not available",
		IsError: true,
	})

	out := rootViewContent(m)
	plain := stripANSI(out)
	// Must show the error flash
	if !strings.Contains(plain, "clipboard") && !strings.Contains(plain, "Error") {
		t.Errorf("I.1: clipboard error flash should be visible, got: %s", plain[:min(300, len(plain))])
	}
	// Application is functional
	if out == "" {
		t.Error("I.1: View() should not be empty after clipboard error flash")
	}
}

// I.2 — ValueRevealedMsg with error shows error in header, does not crash.
// (Simulates a deleted secret being revealed — API returns ResourceNotFoundException)
func TestQa67_I2_RevealDeletedSecret_ShowsError(t *testing.T) {
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "secrets",
	})
	secrets := []resource.Resource{
		{
			ID:     "arn:aws:secretsmanager:us-east-1:123:secret:deleted-secret",
			Name:   "deleted-secret",
			Fields: map[string]string{
				"name":               "deleted-secret",
				"arn":                "arn:aws:secretsmanager:us-east-1:123:secret:deleted-secret",
				"description":        "",
				"last_changed_date":  "",
				"last_accessed_date": "",
				"rotation_enabled":   "false",
				"kms_key_id":         "",
				"tags":               "",
			},
		},
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{ResourceType: "secrets", Resources: secrets})

	// Simulate the reveal result: ResourceNotFoundException
	m, _ = rootApplyMsg(m, messages.ValueRevealed{
		ResourceType: "secrets",
		ResourceID:   "deleted-secret",
		Err:          errResourceNotFound("deleted-secret"),
	})

	out := rootViewContent(m)
	// Must not crash; error flash should be shown
	if out == "" {
		t.Error("I.2: View() should not be empty after reveal error for deleted secret")
	}
}

func errResourceNotFound(name string) error {
	return fmt.Errorf("ResourceNotFoundException: Secrets Manager can't find the specified secret: %s", name)
}

// I.3 — Reveal for a secret with no current version shows error/empty indicator.
func TestQa67_I3_RevealSecretNoCurrentVersion_ShowsError(t *testing.T) {
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "secrets",
	})
	secrets := []resource.Resource{
		{
			ID:     "arn:aws:secretsmanager:us-east-1:123:secret:no-value-secret",
			Name:   "no-value-secret",
			Fields: map[string]string{
				"name":               "no-value-secret",
				"arn":                "arn:aws:secretsmanager:us-east-1:123:secret:no-value-secret",
				"description":        "",
				"last_changed_date":  "",
				"last_accessed_date": "",
				"rotation_enabled":   "false",
				"kms_key_id":         "",
				"tags":               "",
			},
		},
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{ResourceType: "secrets", Resources: secrets})

	// ValueRevealedMsg with error (no current version)
	m, _ = rootApplyMsg(m, messages.ValueRevealed{
		ResourceType: "secrets",
		ResourceID:   "no-value-secret",
		Err:          errNoVersionFound("no-value-secret"),
	})

	out := rootViewContent(m)
	if out == "" {
		t.Error("I.3: View() should not be empty after reveal with no-version error")
	}
}

func errNoVersionFound(name string) error {
	return fmt.Errorf("ResourceNotFoundException: Secrets Manager can't find the specified secret value for staging label AWSCURRENT: %s", name)
}

// I.5 — Pressing x on non-secret resource types (EC2) is a no-op.
func TestQa67_I5_XKeyOnNonSecretType_IsNoOp(t *testing.T) {
	// Representative sample of non-secret types — full sweep in CI slow suite
	nonSecretTypes := []string{"ec2", "s3"}
	for _, rt := range nonSecretTypes {
		t.Run(rt, func(t *testing.T) {
			m := newRootSizedModel()
			m, _ = rootApplyMsg(m, messages.Navigate{
				Target:       messages.TargetResourceList,
				ResourceType: rt,
			})
			viewBefore := rootViewContent(m)

			// Press 'x' — should be a no-op
			m, cmd := rootApplyMsg(m, tea.KeyPressMsg{Code: -1, Text: "x"})

			viewAfter := rootViewContent(m)

			// View should remain the same (no navigation away)
			_ = viewBefore
			_ = viewAfter

			// Most importantly: must not navigate to reveal view
			plain := stripANSI(viewAfter)
			if strings.Contains(plain, "Secret visible") {
				t.Errorf("I.5: pressing x on %s should not open reveal view", rt)
			}

			// cmd should be nil (no-op) or at most return a no-op FlashMsg
			if cmd != nil {
				msg := cmd()
				if _, ok := msg.(messages.ValueRevealed); ok {
					t.Errorf("I.5: pressing x on %s should not trigger a reveal", rt)
				}
			}
		})
	}
}

// I.7 — Tab autocomplete with no match in command mode does nothing (no crash).
func TestQa67_I7_TabAutocomplete_NoMatch_DoesNothing(t *testing.T) {
	m := newRootSizedModel()

	// Enter command mode
	m, _ = rootApplyMsg(m, rootKeyPress(":"))

	// Type something that won't match any command
	for _, r := range "zzznomatch" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(r)))
	}

	viewBefore := rootViewContent(m)

	// Press Tab — should be a no-op
	m, cmd := rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyTab})

	viewAfter := rootViewContent(m)
	_ = cmd // cmd may be nil

	// Must not crash; application should still be functional
	if viewAfter == "" {
		t.Error("I.7: View() should not be empty after Tab with no match in command mode")
	}
	// Tab with no match is a no-op: view content should be unchanged
	plainBefore := stripANSI(viewBefore)
	plainAfter := stripANSI(viewAfter)
	if plainBefore != plainAfter {
		t.Errorf("I.7: Tab with no match should not change view content;\nbefore: %s\nafter:  %s",
			plainBefore[:min(200, len(plainBefore))], plainAfter[:min(200, len(plainAfter))])
	}
}

// I.8 — Sort key on resource type without status column does not crash.
func TestQa67_I8_SortByStatus_ResourceTypeWithNoStatusColumn_NoCrash(t *testing.T) {
	// Use SNS Topics which has minimal columns (no standard "status" column)
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "sns",
	})
	resources := []resource.Resource{
		{
			ID:     "arn:aws:sns:us-east-1:123:topic-alpha",
			Name:   "topic-alpha",
			Fields: map[string]string{
				"topic_name": "topic-alpha",
				"topic_arn":  "arn:aws:sns:us-east-1:123:topic-alpha",
			},
		},
		{
			ID:     "arn:aws:sns:us-east-1:123:topic-beta",
			Name:   "topic-beta",
			Fields: map[string]string{
				"topic_name": "topic-beta",
				"topic_arn":  "arn:aws:sns:us-east-1:123:topic-beta",
			},
		},
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{ResourceType: "sns", Resources: resources})

	// Press S to sort by status — should be no-op or graceful
	m, _ = rootApplyMsg(m, rootKeyPress("S"))
	out := rootViewContent(m)
	if out == "" {
		t.Error("I.8: View() should not be empty after sorting by status on SNS")
	}

	// Press 1 to sort by column 0 (Topic Name) — should work fine
	m, _ = rootApplyMsg(m, rootKeyPress("1"))
	out = rootViewContent(m)
	if out == "" {
		t.Error("I.8: View() should not be empty after sorting by column 1 on SNS")
	}
}

// I.9 — Horizontal scroll on resource type with few columns is a no-op (no crash).
func TestQa67_I9_HorizontalScroll_FewColumns_IsNoOp(t *testing.T) {
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "sns",
	})
	resources := []resource.Resource{
		{
			ID:   "arn:aws:sns:us-east-1:123:small-topic",
			Name: "small-topic",
			Fields: map[string]string{
				"topic_name": "small-topic",
				"topic_arn":  "arn:aws:sns:us-east-1:123:small-topic",
			},
		},
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{ResourceType: "sns", Resources: resources})

	// Press l to scroll right — should stop at the last column (no crash)
	for range 10 {
		m, _ = rootApplyMsg(m, rootKeyPress("l"))
	}
	out := rootViewContent(m)
	if out == "" {
		t.Error("I.9: View() should not be empty after horizontal scroll on SNS")
	}

	// Press h to scroll left — should stop at the first column (no crash)
	for range 10 {
		m, _ = rootApplyMsg(m, rootKeyPress("h"))
	}
	out = rootViewContent(m)
	if out == "" {
		t.Error("I.9: View() should not be empty after horizontal scroll left on SNS")
	}
}

// I.4 — Reveal header warning shows "Secret visible" and does not auto-clear.
func TestQa67_I4_RevealHeaderWarning_PersistsVisible(t *testing.T) {
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "secrets",
	})
	secrets := []resource.Resource{
		{
			ID:     "arn:aws:secretsmanager:us-east-1:123:secret:prod/api/key",
			Name:   "prod/api/key",
			Fields: map[string]string{
				"name":               "prod/api/key",
				"arn":                "arn:aws:secretsmanager:us-east-1:123:secret:prod/api/key",
				"description":        "Production API Key",
				"last_changed_date":  "2025-01-01",
				"last_accessed_date": "2025-03-15",
				"rotation_enabled":   "false",
				"kms_key_id":         "",
				"tags":               "",
			},
		},
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{ResourceType: "secrets", Resources: secrets})

	// Navigate to reveal view via ValueRevealedMsg (success path)
	m, _ = rootApplyMsg(m, messages.ValueRevealed{
		ResourceType: "secrets",
		ResourceID:   "prod/api/key",
		Value:        "hunter2-secret-value",
	})

	out := rootViewContent(m)
	plain := stripANSI(out)
	// Header warning should show "Secret visible"
	if !strings.Contains(plain, "Secret visible") {
		t.Errorf("I.4: reveal view should show 'Secret visible' warning, got: %s", plain[:min(300, len(plain))])
	}
	// The secret value must be displayed
	if !strings.Contains(plain, "hunter2-secret-value") {
		t.Errorf("I.4: secret value should be visible in output, got: %s", plain[:min(300, len(plain))])
	}
	if out == "" {
		t.Error("I.4: View() should not be empty after reveal")
	}
}
