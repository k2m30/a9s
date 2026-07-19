// SPDX-License-Identifier: GPL-3.0-or-later

// coverage_live_gaps_whitebox_test.go — white-box tests for live,
// production-reachable functions that carried low coverage (primaryWave2Finding,
// saveThemeConfigCmd). Both are unexported, so they are tested directly from
// package tui rather than through the full renderer stack.
package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// ---------------------------------------------------------------------------
// primaryWave2Finding (app_enrich_fold.go)
// ---------------------------------------------------------------------------

func TestLiveGap_PrimaryWave2Finding_NoWave2Findings_ReturnsNilNil(t *testing.T) {
	r := resource.Resource{
		Findings: []domain.Finding{
			{Code: "ec2.state", Phrase: "running", Severity: domain.SevOK, Source: "wave1"},
		},
	}
	f, ad := primaryWave2Finding(r)
	if f != nil || ad != nil {
		t.Errorf("primaryWave2Finding() = (%v, %v), want (nil, nil) when Findings has no wave2 entries", f, ad)
	}
}

func TestLiveGap_PrimaryWave2Finding_PicksWorstSeverityAmongMultipleWave2(t *testing.T) {
	r := resource.Resource{
		Findings: []domain.Finding{
			{Code: "ec2.state", Phrase: "running", Severity: domain.SevOK, Source: "wave1"},
			{Code: "ec2.warn", Phrase: "degraded", Severity: domain.SevWarn, Source: "wave2:ec2"},
			{Code: "ec2.broken", Phrase: "impaired", Severity: domain.SevBroken, Source: "wave2:ec2"},
		},
		AttentionDetails: map[domain.FindingCode]domain.AttentionDetail{
			"ec2.broken": {Rows: []domain.DetailRow{{Label: "Action", Value: "reboot"}}},
		},
	}
	f, ad := primaryWave2Finding(r)
	if f == nil {
		t.Fatal("primaryWave2Finding() finding = nil, want the worst-severity wave2 finding")
	}
	if f.Code != "ec2.broken" {
		t.Errorf("primaryWave2Finding() finding.Code = %q, want %q (SevBroken beats SevWarn)", f.Code, "ec2.broken")
	}
	if ad == nil || len(ad.Rows) != 1 || ad.Rows[0].Value != "reboot" {
		t.Errorf("primaryWave2Finding() AttentionDetail = %+v, want a single row with Value=%q", ad, "reboot")
	}
}

func TestLiveGap_PrimaryWave2Finding_AttentionDetailWithEmptyRows_ReturnsNilDetail(t *testing.T) {
	r := resource.Resource{
		Findings: []domain.Finding{
			{Code: "ec2.x", Phrase: "impaired", Severity: domain.SevBroken, Source: "wave2:ec2"},
		},
		AttentionDetails: map[domain.FindingCode]domain.AttentionDetail{
			"ec2.x": {Rows: nil},
		},
	}
	f, ad := primaryWave2Finding(r)
	if f == nil {
		t.Fatal("primaryWave2Finding() finding = nil, want the wave2 finding")
	}
	if ad != nil {
		t.Errorf("primaryWave2Finding() AttentionDetail = %+v, want nil when the matching AttentionDetail has no Rows", ad)
	}
}

// ---------------------------------------------------------------------------
// saveThemeConfigCmd (screens.go)
// ---------------------------------------------------------------------------

func TestLiveGap_SaveThemeConfigCmd_WritesThemeToConfigYAML(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", dir)

	cmd := saveThemeConfigCmd(runtime.SaveThemeConfigPayload{Theme: "dracula.yaml"})
	if cmd == nil {
		t.Fatal("saveThemeConfigCmd(...) returned a nil tea.Cmd")
	}
	if msg := cmd(); msg != nil {
		t.Fatalf("saveThemeConfigCmd(...)() on success = %#v, want nil (success is silent)", msg)
	}

	data, err := os.ReadFile(filepath.Join(dir, "config.yaml"))
	if err != nil {
		t.Fatalf("reading config.yaml after saveThemeConfigCmd: %v", err)
	}
	if !strings.Contains(string(data), "dracula.yaml") {
		t.Errorf("config.yaml after saveThemeConfigCmd(Theme=%q) = %q, want it to contain the theme name", "dracula.yaml", data)
	}
}

func TestLiveGap_SaveThemeConfigCmd_ErrorWhenConfigDirEmpty_EmitsErrorFlash(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("APPDATA", "")
	t.Setenv("A9S_CONFIG_FOLDER", "")

	cmd := saveThemeConfigCmd(runtime.SaveThemeConfigPayload{Theme: "dracula.yaml"})
	if cmd == nil {
		t.Fatal("saveThemeConfigCmd(...) returned a nil tea.Cmd")
	}
	msg := cmd()
	fl, ok := msg.(messages.Flash)
	if !ok {
		t.Fatalf("saveThemeConfigCmd(...)() on error = %T (%#v), want messages.Flash", msg, msg)
	}
	if !fl.IsError {
		t.Error("saveThemeConfigCmd(...)() on error: Flash.IsError = false, want true")
	}
	if fl.Text == "" {
		t.Error("saveThemeConfigCmd(...)() on error: Flash.Text is empty, want an error description")
	}
}
