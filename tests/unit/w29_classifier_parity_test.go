package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// w29_classifier_parity_test.go — the net under the twenty-one conversions.
//
// Each of these types answers a row's colour twice today: from the findings the
// fetcher put on it, and from a raw-field switch for rows that carry none. The
// task deletes the second copy and routes it through the type's own predicate.
// Nothing about the rendered demo may move while that happens.
//
// Two properties hold before the conversion and must still hold after. A
// fetched row is coloured by its own findings, so the classifier and the
// fetcher cannot disagree about the same row. And the same row stripped of its
// findings gets the same colour, so the fallback path answers what the findings
// path would have.
//
// The third property row 1 promises — that the worst finding wins rather than
// the first — is already pinned for every registered type with no exclusions by
// TestColorTakesWorstSeverity_EveryType, so it is not repeated here.

// w29Types are the twenty-one classifiers the burn-down lists, by the short name
// each is registered under, grouped so a category can be run on its own after
// dev converts it: -run 'TestW29_.*/compute'.
var w29Types = map[string][]string{ //nolint:gochecknoglobals // test-only table
	"compute":    {"ecs-task", "eb", "ebs"},
	"networking": {"elb", "vpc", "subnet", "nat", "igw", "vpce", "tgw", "eni"},
	"secrets":    {"secrets", "kms"},
	"containers": {"eks", "ng"},
	"data":       {"athena"},
	"dns_cdn":    {"cf", "r53", "acm"},
	"monitoring": {"logs"},
	"cicd":       {"cfn"},
}

func w29EachType(t *testing.T, check func(t *testing.T, short string)) {
	t.Helper()
	for category, shorts := range w29Types {
		t.Run(category, func(t *testing.T) {
			for _, short := range shorts {
				t.Run(short, func(t *testing.T) { check(t, short) })
			}
		})
	}
}

// TestW29_FetchedRowColourComesFromItsFindings pins that the classifier agrees
// with the fetcher about the row in front of it. A fetched row in a state worth
// reporting carries the finding for it, so a classifier reaching past that
// finding to read a raw field is answering a question already answered.
func TestW29_FetchedRowColourComesFromItsFindings(t *testing.T) {
	w29EachType(t, func(t *testing.T, short string) {
		rows, td := w4bBench(t, short)
		flagged := 0
		for _, r := range rows {
			if len(r.Findings) > 0 {
				flagged++
			}
			if got, want := td.ResolveColor(r), d1ColorOf(r.Findings); got != want {
				t.Errorf("row %q colour = %v, want %v from its findings %+v; fields = %v",
					r.ID, got, want, r.Findings, r.Fields)
			}
		}
		// Every row healthy would pass the loop above without exercising it.
		if flagged == 0 {
			t.Errorf("no demo %s row carries a finding; this pin would be vacuous", short)
		}
	})
}

// TestW29_StrippedRowFallsBackToTheSameColour pins the path the conversion
// rewrites. Removing a row's findings sends the classifier down its fallback,
// which must reach the same verdict — that is what makes deleting the raw
// switch a deletion rather than a behaviour change.
//
// Only rows whose findings are all wave 1 can hold that. A wave-2 finding is a
// fact the fetcher never wrote into Fields — a disabled key rotation, an open
// key policy, a zone nobody queries — so no predicate over Fields can recover
// it and no fallback ever could, before this task or after.
func TestW29_StrippedRowFallsBackToTheSameColour(t *testing.T) {
	w29EachType(t, func(t *testing.T, short string) {
		rows, td := w4bBench(t, short)
		checked := 0
		for _, r := range rows {
			if len(r.Findings) == 0 || !w29FallbackCanAnswer(r) {
				continue
			}
			checked++
			stripped := resource.Resource{ID: r.ID, Name: r.Name, Fields: r.Fields, RawStruct: r.RawStruct}
			if got, want := td.ResolveColor(stripped), td.ResolveColor(r); got != want {
				t.Errorf("row %q: with findings %v, stripped %v — the fallback and the findings path disagree; fields = %v",
					r.ID, want, got, r.Fields)
			}
		}
		if checked == 0 {
			t.Skipf("no demo %s row carries recoverable wave-1 findings alone; nothing here for the fallback to answer", short)
		}
	})
}

// w29FallbackCanAnswer reports whether the fields on r hold everything its
// findings were derived from, which is the only case where a fallback over
// Fields can reach the same verdict.
//
// Only a wave-2 finding fails that now: it is a fact the fetcher went and
// looked up, like a disabled key rotation or a zone nobody queries, and no
// predicate over Fields can recover it. The two wave-1 exceptions this pin
// carried are gone — the subnet and endpoint fetchers surface auto_public_ip
// and policy_exposure, so their findings are reachable like every other.
func w29FallbackCanAnswer(r resource.Resource) bool {
	for _, f := range r.Findings {
		if f.Source != "wave1" {
			return false
		}
	}
	return true
}

// TestW29_PendingEKSClusterIsAWarning is the ruling folded into row 1. Both
// docs/resources/eks.md §4 and docs/attention-signals.md promise a queued
// create or update reads as a warning; nothing emits it, so the row renders
// green while EKS has not started the work. The code name follows the four
// siblings in eks_codes.go, which are eks.state.<status> without exception.
func TestW29_PendingEKSClusterIsAWarning(t *testing.T) {
	rows, _ := w4bBench(t, "eks")

	var carriers []string
	for _, r := range rows {
		for _, f := range r.Findings {
			if f.Code != domain.FindingCode("eks.state.pending") {
				continue
			}
			carriers = append(carriers, r.ID)
			if f.Phrase != "pending" {
				t.Errorf("row %q: Phrase = %q, want %q", r.ID, f.Phrase, "pending")
			}
			if f.Severity != domain.SevWarn {
				t.Errorf("row %q: Severity = %v, want SevWarn", r.ID, f.Severity)
			}
			if f.Source != "wave1" {
				t.Errorf("row %q: Source = %q, want %q", r.ID, f.Source, "wave1")
			}
		}
	}
	if len(carriers) != 1 {
		t.Errorf("%d demo eks rows carry eks.state.pending, want exactly 1: %v", len(carriers), carriers)
	}
}

// TestW29_TransitGatewayAutoAcceptReachesTheFallback covers the third field the
// conversion surfaced. subnet's auto_public_ip and vpce's policy_exposure each
// have a demo row, so the bench pins above already exercise them; no demo
// gateway accepts shared attachments, so this one has to be built.
//
// A gateway that accepts any attachment offered to it is the finding, and a
// gateway on its way out is not: an attachment setting on a resource being
// deleted is not something anyone will act on.
func TestW29_TransitGatewayAutoAcceptReachesTheFallback(t *testing.T) {
	td := resource.FindResourceType("tgw")
	if td == nil {
		t.Fatal("tgw not registered")
	}

	cases := []struct {
		name  string
		state string
		auto  string
		want  resource.Color
	}{
		{"available_auto_accept_on", "available", "yes", resource.ColorWarning},
		{"available_auto_accept_off", "available", "no", resource.ColorHealthy},
		{"available_auto_accept_absent", "available", "", resource.ColorHealthy},
		// The deleted row is the one that proves the suppression: its state is
		// Dim, so an auto-accept finding firing anyway would show as Warning.
		// The deleting row cannot tell the two apart on colour alone and is
		// here for the second arm of the same condition.
		{"deleting_state_only", "deleting", "yes", resource.ColorWarning},
		{"deleted_suppresses_it", "deleted", "yes", resource.ColorDim},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := td.ResolveColor(resource.Resource{
				ID:     "tgw-probe",
				Fields: map[string]string{"state": tc.state, "auto_accept": tc.auto},
			})
			if got != tc.want {
				t.Errorf("state=%q auto_accept=%q → %v, want %v", tc.state, tc.auto, got, tc.want)
			}
		})
	}
}

// TestW29_KMSKeyStateReachesTheFallback covers the one converted type whose
// demo rows are all wave-2, so the bench pins above skip it and its fallback is
// otherwise untested. The predicate is shared with the fetcher, and its default
// arm reports an unrecognised state as broken — right for a fetched row, where
// the state is always one AWS returned, and wrong for a row built without one,
// which is the regression the empty-state arm exists to stop.
func TestW29_KMSKeyStateReachesTheFallback(t *testing.T) {
	td := resource.FindResourceType("kms")
	if td == nil {
		t.Fatal("kms not registered")
	}

	cases := []struct {
		status string
		want   resource.Color
	}{
		{"Enabled", resource.ColorHealthy},
		{"", resource.ColorHealthy},
		{"Disabled", resource.ColorWarning},
		{"PendingDeletion", resource.ColorBroken},
		{"PendingImport", resource.ColorBroken},
		{"PendingReplicaDeletion", resource.ColorBroken},
		{"Unavailable", resource.ColorBroken},
	}

	for _, tc := range cases {
		name := tc.status
		if name == "" {
			name = "absent"
		}
		t.Run(name, func(t *testing.T) {
			got := td.ResolveColor(resource.Resource{
				ID:     "key-probe",
				Fields: map[string]string{"status": tc.status},
			})
			if got != tc.want {
				t.Errorf("status=%q → %v, want %v", tc.status, got, tc.want)
			}
		})
	}
}

// The fetcher is the predicate's other caller, and a real key always has a
// state, so the arm that made a stateless row healthy must not have made a
// stateless row possible on the bench.
func TestW29_EveryDemoKMSRowHasAState(t *testing.T) {
	rows, _ := w4bBench(t, "kms")
	for _, r := range rows {
		if r.Fields["status"] == "" {
			t.Errorf("demo kms row %q has no status; the predicate would report nothing for it", r.ID)
		}
	}
}
