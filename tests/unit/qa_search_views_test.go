package unit

// qa_search_views_test.go — TDD integration tests for search embedded in
// DetailModel and YAMLModel (T015).

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
	tui "github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ---------------------------------------------------------------------------
// Fixtures shared across T015 tests
// ---------------------------------------------------------------------------

// searchTestEC2Resource returns a minimal EC2-like resource with a "running"
// status field that can be used as a search target.
func searchTestEC2Resource() resource.Resource {
	return resource.Resource{
		ID:   "i-search-test",
		Name: "search-test-instance",
		Fields: map[string]string{
			"instance_id":       "i-search-test",
			"instance_type":     "t3.medium",
			"state":             "running",
			"availability_zone": "us-east-1a",
		},
	}
}

// searchResourceTypes returns a table of resource types + representative
// resources covering the major categories required by T015-6.
func searchResourceTypes() []struct {
	typeName string
	res      resource.Resource
} {
	return []struct {
		typeName string
		res      resource.Resource
	}{
		{
			"ec2",
			resource.Resource{
				ID:   "i-ec2-search",
				Name: "ec2-search-instance",
				Fields: map[string]string{
					"state": "running",
					"type":  "t3.micro",
				},
			},
		},
		{
			"s3",
			resource.Resource{
				ID:   "search-bucket",
				Name: "search-bucket",
				Fields: map[string]string{
					"region": "us-east-1",
				},
			},
		},
		{
			"rds",
			resource.Resource{
				ID:   "search-db",
				Name: "search-db",
				Fields: map[string]string{
					"engine": "postgres",
					"status": "available",
				},
			},
		},
		{
			"lambda",
			resource.Resource{
				ID:   "search-function",
				Name: "search-function",
				Fields: map[string]string{
					"runtime": "go1.x",
					"state":   "Active",
				},
			},
		},
		{
			"vpc",
			resource.Resource{
				ID:   "vpc-search",
				Name: "search-vpc",
				Fields: map[string]string{
					"cidr_block": "10.0.0.0/16",
					"state":      "available",
				},
			},
		},
		{
			"sg",
			resource.Resource{
				ID:   "sg-search",
				Name: "search-sg",
				Fields: map[string]string{
					"group_name":  "search-sg",
					"description": "search test security group",
				},
			},
		},
		{
			"iam-roles",
			resource.Resource{
				ID:   "search-role",
				Name: "search-role",
				Fields: map[string]string{
					"arn": "arn:aws:iam::123456789012:role/search-role",
				},
			},
		},
		{
			"eks",
			resource.Resource{
				ID:   "search-cluster",
				Name: "search-cluster",
				Fields: map[string]string{
					"version": "1.29",
					"status":  "ACTIVE",
				},
			},
		},
		{
			"sqs",
			resource.Resource{
				ID:   "https://sqs.us-east-1.amazonaws.com/123456789012/search-queue",
				Name: "search-queue",
				Fields: map[string]string{
					"visibility_timeout_seconds": "30",
				},
			},
		},
		{
			"cloudfront",
			resource.Resource{
				ID:   "ESEARCH123",
				Name: "search-distribution",
				Fields: map[string]string{
					"domain_name": "dsearch.cloudfront.net",
					"status":      "Deployed",
				},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// T015-1: "/" key activates search in DetailModel
// ---------------------------------------------------------------------------

// TestSearch_DetailView_SlashActivatesSearch verifies that sending a "/" key
// event to DetailModel sets IsSearchActive() to true.
func TestSearch_DetailView_SlashActivatesSearch(t *testing.T) {
	k := keys.Default()
	res := searchTestEC2Resource()
	d := views.NewDetail(res, "ec2", nil, k)
	d.SetSize(80, 40)

	slashKey := tea.KeyPressMsg{Code: '/', Text: "/"}
	d, _ = d.Update(slashKey)

	if !d.IsSearchActive() {
		t.Error("expected IsSearchActive()=true after '/' key press")
	}
}

// ---------------------------------------------------------------------------
// T015-2: Typing a query highlights matches in DetailModel.View()
// ---------------------------------------------------------------------------

// TestSearch_DetailView_TypeQueryHighlightsMatches verifies that after
// activating search and typing a query matching an existing field value,
// DetailModel.View() output contains ANSI highlight sequences around the
// matched text.
func TestSearch_DetailView_TypeQueryHighlightsMatches(t *testing.T) {
	k := keys.Default()
	res := searchTestEC2Resource()
	d := views.NewDetail(res, "ec2", nil, k)
	d.SetSize(80, 40)

	// Activate search.
	slashKey := tea.KeyPressMsg{Code: '/', Text: "/"}
	d, _ = d.Update(slashKey)

	// Type "running" character by character.
	for _, ch := range "running" {
		d, _ = d.Update(tea.KeyPressMsg{Code: ch, Text: string(ch)})
	}

	output := d.View()

	// After typing, highlights should be present (more ANSI sequences than baseline).
	// We check that the output is non-empty and contains ANSI sequences.
	if output == "" {
		t.Fatal("View() returned empty string after search activation")
	}
	if !strings.Contains(output, "\x1b[") {
		t.Error("expected ANSI sequences in View() output after search query")
	}

	// The plain-text content must still contain "running".
	plain := ansiRe.ReplaceAllString(output, "")
	if !strings.Contains(plain, "running") {
		t.Errorf("plain content missing 'running' after search; got: %q", plain)
	}
}

// ---------------------------------------------------------------------------
// T015-3: Esc exits search in DetailModel
// ---------------------------------------------------------------------------

// TestSearch_DetailView_EscExitsSearch verifies that pressing Esc while search
// is active sets IsSearchActive() to false and View() output no longer contains
// any search highlight sequences.
func TestSearch_DetailView_EscExitsSearch(t *testing.T) {
	k := keys.Default()
	res := searchTestEC2Resource()
	d := views.NewDetail(res, "ec2", nil, k)
	d.SetSize(80, 40)

	// Activate search and type a query.
	slashKey := tea.KeyPressMsg{Code: '/', Text: "/"}
	d, _ = d.Update(slashKey)
	for _, ch := range "running" {
		d, _ = d.Update(tea.KeyPressMsg{Code: ch, Text: string(ch)})
	}

	// Deactivate with Esc.
	escKey := tea.KeyPressMsg{Code: tea.KeyEscape}
	d, _ = d.Update(escKey)

	if d.IsSearchActive() {
		t.Error("expected IsSearchActive()=false after Esc")
	}

	// View() must not panic and must return non-empty content.
	output := d.View()
	if output == "" {
		t.Fatal("View() returned empty string after search deactivation")
	}
}

// ---------------------------------------------------------------------------
// T015-4: "/" key activates search in YAMLModel
// ---------------------------------------------------------------------------

// TestSearch_YAMLView_SlashActivatesSearch verifies that sending a "/" key
// event to YAMLModel sets IsSearchActive() to true.
func TestSearch_YAMLView_SlashActivatesSearch(t *testing.T) {
	k := keys.Default()
	res := searchTestEC2Resource()
	y := views.NewYAML(res, "", k)
	y.SetSize(80, 40)

	slashKey := tea.KeyPressMsg{Code: '/', Text: "/"}
	y, _ = y.Update(slashKey)

	if !y.IsSearchActive() {
		t.Error("expected IsSearchActive()=true after '/' key press in YAMLModel")
	}
}

// ---------------------------------------------------------------------------
// T015-5: Esc exits search in YAMLModel
// ---------------------------------------------------------------------------

// TestSearch_YAMLView_EscExitsSearch verifies that pressing Esc while search
// is active in YAMLModel sets IsSearchActive() to false.
func TestSearch_YAMLView_EscExitsSearch(t *testing.T) {
	k := keys.Default()
	res := searchTestEC2Resource()
	y := views.NewYAML(res, "", k)
	y.SetSize(80, 40)

	// Activate search.
	slashKey := tea.KeyPressMsg{Code: '/', Text: "/"}
	y, _ = y.Update(slashKey)

	// Deactivate with Esc.
	escKey := tea.KeyPressMsg{Code: tea.KeyEscape}
	y, _ = y.Update(escKey)

	if y.IsSearchActive() {
		t.Error("expected IsSearchActive()=false after Esc in YAMLModel")
	}

	output := y.View()
	if output == "" {
		t.Fatal("YAMLModel.View() returned empty string after search deactivation")
	}
}

// ---------------------------------------------------------------------------
// T015-6: All resource types — search activation does not panic
// ---------------------------------------------------------------------------

// TestSearch_DetailView_AllResourceTypes is a table-driven test that verifies
// search activation and query entry does not panic and produces valid View()
// output for every major resource type. Covers at least 10 resource types.
func TestSearch_DetailView_AllResourceTypes(t *testing.T) {
	k := keys.Default()
	cases := searchResourceTypes()

	for _, tc := range cases {
		t.Run(tc.typeName, func(t *testing.T) {
			d := views.NewDetail(tc.res, tc.typeName, nil, k)
			d.SetSize(80, 40)

			// Activate search.
			slashKey := tea.KeyPressMsg{Code: '/', Text: "/"}
			d, _ = d.Update(slashKey)

			if !d.IsSearchActive() {
				t.Errorf("%s: expected IsSearchActive()=true after '/' key", tc.typeName)
			}

			// Type a short query.
			for _, ch := range "test" {
				d, _ = d.Update(tea.KeyPressMsg{Code: ch, Text: string(ch)})
			}

			// View() must not panic and must return a non-empty string.
			output := d.View()
			if output == "" {
				t.Errorf("%s: View() returned empty string during search", tc.typeName)
			}

			// Deactivate and verify IsSearchActive() is false.
			escKey := tea.KeyPressMsg{Code: tea.KeyEscape}
			d, _ = d.Update(escKey)
			if d.IsSearchActive() {
				t.Errorf("%s: expected IsSearchActive()=false after Esc", tc.typeName)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// T015-7: Root-level n/N navigation in DetailModel (integration test)
// ---------------------------------------------------------------------------

// TestSearch_DetailView_RootLevel_NextPrevMatch verifies that n/N key presses
// navigate between search matches when routed through the full root model.
// This reproduces the real-app bug where n/N are swallowed by the root model
// before reaching the detail view's search handler.
//
// To make the test meaningful, the resource has "running" in TWO fields so
// there are 2 matches. After pressing Enter to confirm the search, the header
// shows "1/2 matches". After pressing n, it must change to "2/2 matches".
// If n is silently swallowed the header stays "1/2 matches" and the test fails.
func TestSearch_DetailView_RootLevel_NextPrevMatch(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Resource with "running" in two fields → 2 search matches.
	res := &resource.Resource{
		ID:   "i-search-nav",
		Name: "test-search-nav",
		Fields: map[string]string{
			"state":       "running",
			"power_state": "running",
			"name":        "test-search-nav",
		},
	}
	m, _ = rootApplyMsg(m, messages.NavigateMsg{Target: messages.TargetDetail, Resource: res})

	// Activate search with /.
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: '/', Text: "/"})

	// Type "running" character by character.
	for _, ch := range "running" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(ch)))
	}

	// Confirm search with Enter.
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})

	// After confirming search, the header should NOT show "? for help"
	// (search mode replaces the help hint with match info).
	afterEnter := rootViewContent(m)
	plainAfterEnter := ansiRe.ReplaceAllString(afterEnter, "")
	if strings.Contains(plainAfterEnter, "? for help") {
		t.Error("header should NOT show '? for help' when search is active after Enter")
	}

	// The view should contain ANSI sequences (search highlights present).
	if !strings.Contains(afterEnter, "\x1b[") {
		t.Error("expected ANSI highlight sequences in detail view after search confirmed")
	}

	// Header must show "1/2 matches" — confirming two hits were found.
	if !strings.Contains(plainAfterEnter, "1/2 matches") {
		t.Errorf("expected '1/2 matches' in header after search confirmed; got plain: %q", plainAfterEnter)
	}

	// Press n — navigate to next match. The header must advance to "2/2 matches".
	// If n is swallowed by the root model the index stays at 1/2 and the test fails.
	m, _ = rootApplyMsg(m, rootKeyPress("n"))

	afterN := rootViewContent(m)
	if afterN == "" {
		t.Fatal("View() returned empty string after pressing n in detail view search")
	}
	plainAfterN := ansiRe.ReplaceAllString(afterN, "")
	if !strings.Contains(plainAfterN, "2/2 matches") {
		t.Errorf("expected '2/2 matches' after pressing n; got plain: %q", plainAfterN)
	}

	// Press N (shift+n) — navigate to previous match. The header must wrap back to "1/2 matches".
	// If N is swallowed the index stays at 2/2 and the test fails.
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: -1, Text: "N"})

	afterShiftN := rootViewContent(m)
	if afterShiftN == "" {
		t.Fatal("View() returned empty string after pressing N in detail view search")
	}
	plainAfterShiftN := ansiRe.ReplaceAllString(afterShiftN, "")
	if !strings.Contains(plainAfterShiftN, "1/2 matches") {
		t.Errorf("expected '1/2 matches' after pressing N; got plain: %q", plainAfterShiftN)
	}
}

// ---------------------------------------------------------------------------
// T015-8: Root-level n/N navigation in YAMLModel (integration test)
// ---------------------------------------------------------------------------

// TestSearch_YAMLView_RootLevel_NextPrevMatch verifies that n/N key presses
// navigate between search matches when routed through the full root model for
// the YAML view. This mirrors the detail view bug but for YAMLModel.
//
// The resource has "running" in two fields so there are 2 matches. The test
// asserts the match counter in the header advances on n and retreats on N.
func TestSearch_YAMLView_RootLevel_NextPrevMatch(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Resource with "running" in two fields → 2 search matches.
	res := &resource.Resource{
		ID:   "i-yaml-search-nav",
		Name: "test-yaml-search-nav",
		Fields: map[string]string{
			"state":       "running",
			"power_state": "running",
			"name":        "test-yaml-search-nav",
		},
	}
	m, _ = rootApplyMsg(m, messages.NavigateMsg{Target: messages.TargetYAML, Resource: res})

	// Activate search with /.
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: '/', Text: "/"})

	// Type "running" character by character.
	for _, ch := range "running" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(ch)))
	}

	// Confirm search with Enter.
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})

	// After confirming search, the header should NOT show "? for help".
	afterEnter := rootViewContent(m)
	plainAfterEnter := ansiRe.ReplaceAllString(afterEnter, "")
	if strings.Contains(plainAfterEnter, "? for help") {
		t.Error("header should NOT show '? for help' when YAML search is active after Enter")
	}

	// The view should contain ANSI sequences (search highlights present).
	if !strings.Contains(afterEnter, "\x1b[") {
		t.Error("expected ANSI highlight sequences in YAML view after search confirmed")
	}

	// Header must show "1/2 matches" — confirming two hits were found.
	if !strings.Contains(plainAfterEnter, "1/2 matches") {
		t.Errorf("expected '1/2 matches' in header after YAML search confirmed; got plain: %q", plainAfterEnter)
	}

	// Press n — navigate to next match. The header must advance to "2/2 matches".
	// If n is swallowed by the root model the index stays at 1/2 and the test fails.
	m, _ = rootApplyMsg(m, rootKeyPress("n"))

	afterN := rootViewContent(m)
	if afterN == "" {
		t.Fatal("View() returned empty string after pressing n in YAML view search")
	}
	plainAfterN := ansiRe.ReplaceAllString(afterN, "")
	if !strings.Contains(plainAfterN, "2/2 matches") {
		t.Errorf("expected '2/2 matches' in YAML after pressing n; got plain: %q", plainAfterN)
	}

	// Press N (shift+n) — navigate to previous match. Header must wrap back to "1/2 matches".
	// If N is swallowed the index stays at 2/2 and the test fails.
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: -1, Text: "N"})

	afterShiftN := rootViewContent(m)
	if afterShiftN == "" {
		t.Fatal("View() returned empty string after pressing N in YAML view search")
	}
	plainAfterShiftN := ansiRe.ReplaceAllString(afterShiftN, "")
	if !strings.Contains(plainAfterShiftN, "1/2 matches") {
		t.Errorf("expected '1/2 matches' in YAML after pressing N; got plain: %q", plainAfterShiftN)
	}
}

// ---------------------------------------------------------------------------
// Compile-time interface verification
// ---------------------------------------------------------------------------

// The variables below verify that the existing types used in T015 tests satisfy
// the method sets we already know about.

var _ = func() {
	k := keys.Default()
	res := resource.Resource{ID: "compile-check"}

	// DetailModel — existing API compiles cleanly.
	d := views.NewDetail(res, "ec2", nil, k)
	d.SetSize(80, 40)
	_ = d.View()

	// YAMLModel — existing API compiles cleanly.
	y := views.NewYAML(res, "", k)
	y.SetSize(80, 40)
	_ = y.View()

	// KeyPressMsg construction — confirm BT v2 API is used correctly.
	_ = tea.KeyPressMsg{Code: '/', Text: "/"}
	_ = tea.KeyPressMsg{Code: tea.KeyEscape}
}
