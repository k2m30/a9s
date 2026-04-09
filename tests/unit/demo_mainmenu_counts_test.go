package unit

import (
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

// countSuffixRE matches count suffixes rendered by the main menu for a known
// resource count, e.g. "(5)", "(25)", "(3+)".
var countSuffixRE = regexp.MustCompile(`\(\d+\+?\)`)

// TestDemoMainMenu_ShowsResourceCounts asserts that after ClientsReadyMsg is
// delivered in demo mode, the main menu view shows resource counts for at least
// 3 distinct resource types without requiring the user to navigate to each list
// first. This is the "warm menu" behavior that demo mode must provide.
//
// Expected to FAIL until the coder implements a demo-mode prefetch path that
// calls SetAvailability on the main menu for all registered types.
func TestDemoMainMenu_ShowsResourceCounts(t *testing.T) {
	m := newDemoColdCacheApp(t)

	// Size the model so View() renders a realistic menu.
	*m, _ = rootApplyMsg(*m, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Inject ClientsReadyMsg to wire the fake clients.
	// Gen=0 matches connectGen zero-value so the stale-result guard passes.
	clients := demo.NewServiceClients()
	var initCmd tea.Cmd
	*m, initCmd = rootApplyMsg(*m, messages.ClientsReadyMsg{Clients: clients, Gen: 0})

	// Drain any commands returned by handleClientsReady (identity probe, avail
	// cache load). In demo mode with noCache=true these are expected to be nil
	// or return empty results — but a coder-added demo prefetch would appear here.
	if initCmd != nil {
		msg := initCmd()
		if msg != nil {
			*m, _ = rootApplyMsg(*m, msg)
		}
	}

	// Render the main menu.
	plain := stripANSI(rootViewContent(*m))

	// The main menu must be visible (not a list or detail view).
	if !strings.Contains(plain, "EC2 Instances") && !strings.Contains(plain, "S3 Buckets") {
		t.Fatalf("expected to be on main menu after ClientsReadyMsg, got:\n%s", plain)
	}

	// Count how many distinct "(N)" or "(N+)" suffixes appear in the rendered menu.
	matches := countSuffixRE.FindAllString(plain, -1)
	distinctCounts := make(map[string]bool)
	for _, m := range matches {
		distinctCounts[m] = true
	}

	// At minimum, EC2, S3, and RDS (all have fixture data) must show counts.
	const minTypesWithCounts = 3
	if len(distinctCounts) < minTypesWithCounts {
		t.Errorf(
			"main menu shows resource counts for only %d type(s) after ClientsReadyMsg — "+
				"expected at least %d; demo prefetch not implemented yet.\n"+
				"Count suffixes found: %v\n"+
				"Menu view:\n%s",
			len(distinctCounts), minTypesWithCounts, mapKeys(distinctCounts), plain,
		)
	}
}

// TestDemoMainMenu_EC2CountNonZero asserts that the EC2 count shown on the
// main menu after ClientsReadyMsg is greater than zero (fixture has instances).
//
// Expected to FAIL until the coder implements demo-mode prefetch.
func TestDemoMainMenu_EC2CountNonZero(t *testing.T) {
	m := newDemoColdCacheApp(t)
	*m, _ = rootApplyMsg(*m, tea.WindowSizeMsg{Width: 120, Height: 40})

	clients := demo.NewServiceClients()
	var initCmd tea.Cmd
	*m, initCmd = rootApplyMsg(*m, messages.ClientsReadyMsg{Clients: clients, Gen: 0})

	if initCmd != nil {
		msg := initCmd()
		if msg != nil {
			*m, _ = rootApplyMsg(*m, msg)
		}
	}

	plain := stripANSI(rootViewContent(*m))

	// EC2 Instances row must appear with a non-zero count suffix.
	ec2LineRE := regexp.MustCompile(`EC2 Instances.*\(([1-9]\d*)\+?\)`)
	if !ec2LineRE.MatchString(plain) {
		t.Errorf(
			"EC2 Instances row does not show a non-zero count on the main menu — "+
				"demo prefetch not implemented.\nMenu view:\n%s",
			plain,
		)
	}
}

// mapKeys returns the key set of a map as a slice (for error messages).
func mapKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
