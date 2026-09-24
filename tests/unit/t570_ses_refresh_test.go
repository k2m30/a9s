package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
)

// Refreshing the SES list replaces the session's receipt-rule-set store and
// rewires the clients to it, on every lane: a web R is the same refresh as a
// TUI Ctrl+R, so the SES Lambda and S3 pivots read the current rule set after
// either.
func TestT570_SESRefresh_ResetsTheRuleSetStoreOnTheHeadlessLane(t *testing.T) {
	core := newExecutorCore(t)
	ctrl := newBlessedController(t, core)
	ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: "ses"})

	before := core.Session().RuleSets
	ctrl.Apply(app.Action{Kind: app.ActionRefresh})
	after := core.Session().RuleSets

	if after == before {
		t.Fatal("refresh on the ses list kept the receipt-rule-set store")
	}
	if core.Session().Clients != nil && core.Session().Clients.RuleSets() != after {
		t.Error("refresh swapped the session store but the clients still read the old one")
	}
}
