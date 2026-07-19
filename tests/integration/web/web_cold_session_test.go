//go:build integration

// web_cold_session_test.go — the cold-session gate for the real web server.
//
// Every prior web-lane test in this package boots in demo mode where the
// availability sweep is synchronous and instant, and unit-level cold-boot
// coverage (tests/unit/app_web_live_cold_boot_test.go) exercises the
// controller directly, never the HTTP surface. Both lanes mask a class of
// regression where the menu's availability counts and issue badges only ever
// appear because a warm disk cache seeded them — a session with NO disk cache
// would show a permanently empty menu (badges never appearing, wave-2 never
// dispatching) and no existing gate would go red.
//
// This gate boots the real web server with an ISOLATED empty config/cache dir
// (A9S_CONFIG_FOLDER → fresh t.TempDir(); honored by core/config
// GetConfigDir and core/cache's dir resolution) and asserts, over the real
// HTTP surface with a cookie-preserving client, that the menu populates from
// in-session work ALONE:
//
//	Step 1 — availability counts for the core types (s3, ec2, dbi) appear on
//	         the menu without any list ever being opened. The only requests
//	         issued before the assertion are GET / (session handshake inside
//	         startServer) and read-only GET /state polls, so the counts can
//	         only come from the session's own sweep
//	         (AvailabilityPrefetched/AvailabilityChecked), never from a
//	         navigation side effect.
//	Step 2 — at least one menu entry carries a non-zero issue badge from the
//	         sweep alone (demo fixtures seed issue-carrying resources for
//	         many types).
//	Step 3 — opening the s3 list renders the wave-2 public-access-block
//	         phrase in a row, and after navigating back the s3 menu entry's
//	         issue badge equals the canonical fixture issue count.
//
// Polling, not fixed sleeps: demo converges in milliseconds when the lane
// works, but the poll budget is deliberately generous so a slow CI machine
// never produces a false red; a genuine architecture gap (the web lane never
// running the sweep) exhausts the budget and fails with a precise dump of
// what /state showed.
package webintegration

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/k2m30/a9s/v3/core/app"
)

const (
	coldSessionPollBudget   = 30 * time.Second
	coldSessionPollInterval = 100 * time.Millisecond

	// coldS3Wave2Phrase is the stable wave-2 status text EnrichS3PublicAccessBlock
	// writes into the s3 list row's status field (core/aws/s3_issue_enrichment.go,
	// FieldUpdates["status"]); pinned as the spec §4 phrase in
	// tests/integration/scenario_s3_visual_test.go (s3S4Phrase).
	coldS3Wave2Phrase = "public access block incomplete"

	// coldS3ExpectedIssueCount is the canonical s3 demo-fixture badge count.
	// The 4 PAB-finding buckets in core/demo/fixtures/s3.go — a9s-demo-nopab
	// (nil → NoSuchPublicAccessBlockConfiguration), a9s-demo-partial-pab
	// (BlockPublicAcls=false), a9s-demo-multifail-pab (two flags false),
	// a9s-demo-nilcfg (nil inner config) — all carry `~` SevWarn findings,
	// and the S1 badge counts only `!`-severity findings plus wave-1
	// issue-colored rows, so the s3 badge is 0. Pinned as
	// s3ExpectedIssueBkt=0 in tests/integration/scenario_s3_visual_test.go.
	coldS3ExpectedIssueCount = 0
)

// coldSessionCoreTypes are the resource types whose availability counts must
// appear from the in-session sweep alone. All three are rich demo types
// (fixtures always yield >0 resources), so AvailKnown && Availability>0 is
// the correct converged state for each.
var coldSessionCoreTypes = []string{"s3", "ec2", "dbi"} //nolint:gochecknoglobals // immutable test data

// coldMenuEntry returns the menu entry for shortName, or nil when the state
// is not a populated menu or the type is missing from it.
func coldMenuEntry(vs app.ViewState, shortName string) *app.MenuEntry {
	if vs.Body.Kind != app.BodyKindMenu || vs.Body.Menu == nil {
		return nil
	}
	for i := range vs.Body.Menu.Entries {
		if vs.Body.Menu.Entries[i].ShortName == shortName {
			return &vs.Body.Menu.Entries[i]
		}
	}
	return nil
}

// coldMenuDump renders the /state evidence used in failure messages: overall
// menu progress plus the exact availability/badge fields for the core types,
// so a red run reports precisely what the cold session showed.
func coldMenuDump(vs app.ViewState) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Body.Kind=%q", vs.Body.Kind)
	if vs.Body.Menu == nil {
		b.WriteString(" (no menu body)")
		return b.String()
	}
	m := vs.Body.Menu
	availKnown, badged := 0, 0
	for i := range m.Entries {
		if m.Entries[i].AvailKnown {
			availKnown++
		}
		if m.Entries[i].IssueBadge.Count > 0 {
			badged++
		}
	}
	fmt.Fprintf(&b, "; entries=%d avail_known=%d badge>0=%d progress=%q refreshing=%v",
		len(m.Entries), availKnown, badged, m.Progress, m.Refreshing)
	for _, sn := range coldSessionCoreTypes {
		e := coldMenuEntry(vs, sn)
		if e == nil {
			fmt.Fprintf(&b, "; %s=<absent>", sn)
			continue
		}
		fmt.Fprintf(&b, "; %s={availability=%d avail_known=%v issue_badge=%d origin=%q}",
			sn, e.Availability, e.AvailKnown, e.IssueBadge.Count, e.Origin)
	}
	return b.String()
}

// coldPollState polls GET /state until pred is satisfied or the budget is
// exhausted. Returns the converged state; on timeout it fails the test with
// the step label and the final /state evidence.
func coldPollState(t *testing.T, c *client, step string, pred func(app.ViewState) bool) app.ViewState {
	t.Helper()
	deadline := time.Now().Add(coldSessionPollBudget)
	var vs app.ViewState
	for {
		vs = c.state(t)
		if pred(vs) {
			return vs
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s: not converged within %v — %s", step, coldSessionPollBudget, coldMenuDump(vs))
		}
		time.Sleep(coldSessionPollInterval)
	}
}

// TestWebColdSession_MenuPopulatesFromInSessionSweepAlone is the cold-session
// gate described in the file header. The three steps share one server and one
// session because step 3's badge assertion must observe the same in-session
// state steps 1-2 converged on; a failed step aborts the remainder since the
// later assertions would only cascade the same root cause.
func TestWebColdSession_MenuPopulatesFromInSessionSweepAlone(t *testing.T) {
	// Isolated, empty config+cache dir: t.TempDir is empty by construction,
	// so no availability/type cache file can pre-exist for any profile pair.
	// Must be set before startServer — the server resolves the dir at
	// session construction.
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())

	c, cleanup := startServer(t)
	defer cleanup()

	// ---------------------------------------------------------------
	// Step 1 — core-type availability counts from the sweep alone.
	// ---------------------------------------------------------------
	step1 := t.Run("Step1_CoreTypeAvailability_NoListVisit", func(t *testing.T) {
		vs := coldPollState(t, c, "step 1 (menu availability for s3/ec2/dbi)", func(vs app.ViewState) bool {
			for _, sn := range coldSessionCoreTypes {
				e := coldMenuEntry(vs, sn)
				if e == nil || !e.AvailKnown || e.Availability == 0 {
					return false
				}
			}
			return true
		})
		t.Logf("step 1 converged: %s", coldMenuDump(vs))
	})
	if !step1 {
		t.Fatal("step 1 red: the in-session sweep never delivered availability counts to the menu — " +
			"see Step1 subtest output for the exact /state evidence; steps 2-3 aborted (same root cause)")
	}

	// ---------------------------------------------------------------
	// Step 2 — at least one non-zero issue badge from the sweep alone.
	// ---------------------------------------------------------------
	step2 := t.Run("Step2_IssueBadgeFromSweepAlone", func(t *testing.T) {
		vs := coldPollState(t, c, "step 2 (any non-zero issue badge)", func(vs app.ViewState) bool {
			if vs.Body.Kind != app.BodyKindMenu || vs.Body.Menu == nil {
				return false
			}
			for i := range vs.Body.Menu.Entries {
				if vs.Body.Menu.Entries[i].IssueBadge.Count > 0 {
					return true
				}
			}
			return false
		})
		t.Logf("step 2 converged: %s", coldMenuDump(vs))
	})
	if !step2 {
		t.Fatal("step 2 red: availability landed but no issue badge ever appeared — " +
			"wave-2 enrichment is not reaching the web menu; see Step2 subtest output; step 3 aborted")
	}

	// ---------------------------------------------------------------
	// Step 3 — open s3, see the wave-2 phrase render, go back, and the
	// s3 badge equals the canonical fixture count.
	// ---------------------------------------------------------------
	t.Run("Step3_S3Wave2RowAndExactBadge", func(t *testing.T) {
		c.action(t, app.ActionCommand, "s3")

		deadline := time.Now().Add(coldSessionPollBudget)
		for {
			status, html := c.bodyRaw(t, c.token)
			if status == 200 && strings.Contains(html, coldS3Wave2Phrase) {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("step 3 (s3 list wave-2 row): %q never rendered in /body within %v — "+
					"last status=%d, body length=%d; in-session wave-2 on the open list is not reaching the web renderer",
					coldS3Wave2Phrase, coldSessionPollBudget, status, len(html))
			}
			time.Sleep(coldSessionPollInterval)
		}

		c.action(t, app.ActionBack, "")
		vs := coldPollState(t, c, "step 3 (menu after back from s3)", func(vs app.ViewState) bool {
			return vs.Body.Kind == app.BodyKindMenu && vs.Body.Menu != nil
		})

		e := coldMenuEntry(vs, "s3")
		if e == nil {
			t.Fatalf("step 3: s3 entry missing from menu after back — %s", coldMenuDump(vs))
		}
		if e.IssueBadge.Count != coldS3ExpectedIssueCount {
			t.Errorf("step 3: s3 issue_badge=%d, want %d (PAB findings are `~` SevWarn and never bump the badge; "+
				"see coldS3ExpectedIssueCount citation) — %s",
				e.IssueBadge.Count, coldS3ExpectedIssueCount, coldMenuDump(vs))
		}
	})
}
