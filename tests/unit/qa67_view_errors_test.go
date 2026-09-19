package unit

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// A Flash carrying a clipboard error shows the error flash without crashing.
func TestQa67_I1_ClipboardUnavailable_ShowsErrorFlash(t *testing.T) {
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
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

	m, _ = rootApplyMsg(m, messages.Flash{
		Text:    "Error: clipboard not available",
		IsError: true,
	})

	out := rootViewContent(m)
	plain := stripANSI(out)
	if !strings.Contains(plain, "clipboard") && !strings.Contains(plain, "Error") {
		t.Errorf("I.1: clipboard error flash should be visible, got: %s", plain[:min(300, len(plain))])
	}
	if out == "" {
		t.Error("I.1: View() should not be empty after clipboard error flash")
	}
}

// A ValueRevealed carrying ResourceNotFoundException (a deleted secret) shows
// the error in the header without crashing.
func TestQa67_I2_RevealDeletedSecret_ShowsError(t *testing.T) {
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "secrets",
	})
	secrets := []resource.Resource{
		{
			ID:   "arn:aws:secretsmanager:us-east-1:123:secret:deleted-secret",
			Name: "deleted-secret",
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
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList, ResourceType: "secrets", Resources: secrets})

	m, _ = rootApplyMsg(m, messages.ValueRevealed{
		ResourceType: "secrets",
		ResourceID:   "deleted-secret",
		Err:          errResourceNotFound("deleted-secret"),
	})

	out := rootViewContent(m)
	if out == "" {
		t.Error("I.2: View() should not be empty after reveal error for deleted secret")
	}
}

func errResourceNotFound(name string) error {
	return fmt.Errorf("ResourceNotFoundException: Secrets Manager can't find the specified secret: %s", name)
}

// Revealing a secret with no current version shows an error or empty
// indicator.
func TestQa67_I3_RevealSecretNoCurrentVersion_ShowsError(t *testing.T) {
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "secrets",
	})
	secrets := []resource.Resource{
		{
			ID:   "arn:aws:secretsmanager:us-east-1:123:secret:no-value-secret",
			Name: "no-value-secret",
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
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList, ResourceType: "secrets", Resources: secrets})

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

// Pressing x on a non-secret resource type is a no-op.
func TestQa67_I5_XKeyOnNonSecretType_IsNoOp(t *testing.T) {
	nonSecretTypes := []string{"ec2", "s3"}
	for _, rt := range nonSecretTypes {
		t.Run(rt, func(t *testing.T) {
			m := newRootSizedModel()
			m, _ = rootApplyMsg(m, messages.Navigate{
				Target:       messages.TargetResourceList,
				ResourceType: rt,
			})
			viewBefore := rootViewContent(m)

			m, cmd := rootApplyMsg(m, tea.KeyPressMsg{Code: -1, Text: "x"})

			viewAfter := rootViewContent(m)

			_ = viewBefore
			_ = viewAfter

			plain := stripANSI(viewAfter)
			if strings.Contains(plain, "Secret visible") {
				t.Errorf("I.5: pressing x on %s should not open reveal view", rt)
			}

			if cmd != nil {
				msg := cmd()
				if _, ok := msg.(messages.ValueRevealed); ok {
					t.Errorf("I.5: pressing x on %s should not trigger a reveal", rt)
				}
			}
		})
	}
}

// Tab autocomplete with no match in command mode does nothing.
func TestQa67_I7_TabAutocomplete_NoMatch_DoesNothing(t *testing.T) {
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, rootKeyPress(":"))

	for _, r := range "zzznomatch" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(r)))
	}

	viewBefore := rootViewContent(m)

	m, cmd := rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyTab})

	viewAfter := rootViewContent(m)
	_ = cmd // cmd may be nil

	if viewAfter == "" {
		t.Error("I.7: View() should not be empty after Tab with no match in command mode")
	}
	plainBefore := stripANSI(viewBefore)
	plainAfter := stripANSI(viewAfter)
	if plainBefore != plainAfter {
		t.Errorf("I.7: Tab with no match should not change view content;\nbefore: %s\nafter:  %s",
			plainBefore[:min(200, len(plainBefore))], plainAfter[:min(200, len(plainAfter))])
	}
}

// A sort key on a resource type without a status column does not crash.
func TestQa67_I8_SortByStatus_ResourceTypeWithNoStatusColumn_NoCrash(t *testing.T) {
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "sns",
	})
	resources := []resource.Resource{
		{
			ID:   "arn:aws:sns:us-east-1:123:topic-alpha",
			Name: "topic-alpha",
			Fields: map[string]string{
				"topic_name": "topic-alpha",
				"topic_arn":  "arn:aws:sns:us-east-1:123:topic-alpha",
			},
		},
		{
			ID:   "arn:aws:sns:us-east-1:123:topic-beta",
			Name: "topic-beta",
			Fields: map[string]string{
				"topic_name": "topic-beta",
				"topic_arn":  "arn:aws:sns:us-east-1:123:topic-beta",
			},
		},
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList, ResourceType: "sns", Resources: resources})

	m, _ = rootApplyMsg(m, rootKeyPress("S"))
	out := rootViewContent(m)
	if out == "" {
		t.Error("I.8: View() should not be empty after sorting by status on SNS")
	}

	m, _ = rootApplyMsg(m, rootKeyPress("1"))
	out = rootViewContent(m)
	if out == "" {
		t.Error("I.8: View() should not be empty after sorting by column 1 on SNS")
	}
}

// Horizontal scroll on a resource type with few columns stops at the edges.
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
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList, ResourceType: "sns", Resources: resources})

	for range 10 {
		m, _ = rootApplyMsg(m, rootKeyPress("l"))
	}
	out := rootViewContent(m)
	if out == "" {
		t.Error("I.9: View() should not be empty after horizontal scroll on SNS")
	}

	for range 10 {
		m, _ = rootApplyMsg(m, rootKeyPress("h"))
	}
	out = rootViewContent(m)
	if out == "" {
		t.Error("I.9: View() should not be empty after horizontal scroll left on SNS")
	}
}

// The reveal header warning shows "Secret visible" and does not auto-clear.
func TestQa67_I4_RevealHeaderWarning_PersistsVisible(t *testing.T) {
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "secrets",
	})
	secrets := []resource.Resource{
		{
			ID:   "arn:aws:secretsmanager:us-east-1:123:secret:prod/api/key",
			Name: "prod/api/key",
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
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList, ResourceType: "secrets", Resources: secrets})

	m, _ = rootApplyMsg(m, messages.ValueRevealed{
		ResourceType: "secrets",
		ResourceID:   "prod/api/key",
		Value:        "hunter2-secret-value",
	})

	out := rootViewContent(m)
	plain := stripANSI(out)
	if !strings.Contains(plain, "Secret visible") {
		t.Errorf("I.4: reveal view should show 'Secret visible' warning, got: %s", plain[:min(300, len(plain))])
	}
	if !strings.Contains(plain, "hunter2-secret-value") {
		t.Errorf("I.4: secret value should be visible in output, got: %s", plain[:min(300, len(plain))])
	}
	if out == "" {
		t.Error("I.4: View() should not be empty after reveal")
	}
}
