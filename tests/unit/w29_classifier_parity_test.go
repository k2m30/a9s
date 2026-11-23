package unit

import (
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
)

// For every type in w29Types a fetched row is coloured by its own findings, so
// the classifier and the fetcher cannot disagree about the same row. That the
// worst finding wins is pinned for every registered type by
// TestColorTakesWorstSeverity_EveryType.

// w29Types are the classifiers under test, by the short name each is
// registered under, grouped so a category runs on its own:
// -run 'TestW29_.*/compute'.
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

// TestW29_PendingEKSClusterIsAWarning: docs/resources/eks.md and
// docs/attention-signals.md say a queued create or update reads as a warning
// while EKS has not started the work. The code name follows the four siblings
// in eks_codes.go, which are eks.state.<status> without exception.
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

// A key DescribeKey answered for always has a state, so every demo kms row
// but the ones whose describe was refused or failed carries it. Those carry
// the finding for the failed read instead.
func TestW29_EveryDemoKMSRowHasAState(t *testing.T) {
	rows, _ := w4bBench(t, "kms")
	degraded := 0
	for _, r := range rows {
		if r.Fields[awsclient.DegradedFindingField] != "" || hasCode(r.Findings, awsclient.CodeKMSAccessDenied) {
			degraded++
			continue
		}
		if r.Fields["status"] == "" {
			t.Errorf("demo kms row %q has no status", r.ID)
		}
	}
	if degraded == len(rows) {
		t.Fatalf("all %d demo kms rows are degraded; the check is vacuous", len(rows))
	}
}

// TestW29_ACMTimedOutCertificateReadsAsWords: the failed-status finding takes
// its phrase from the status word the fetcher derives, so a certificate whose
// validation ran out of time says so in words rather than in the SDK's
// spelling, as docs/resources/acm.md promises. The phrase is passed through,
// not a FindingDef literal.
func TestW29_ACMTimedOutCertificateReadsAsWords(t *testing.T) {
	rows, _ := w4bBench(t, "acm")

	var found bool
	for _, r := range rows {
		if r.Fields["status"] != "VALIDATION_TIMED_OUT" {
			continue
		}
		found = true
		if len(r.Findings) != 1 {
			t.Errorf("row %q carries %+v, want the one failed-status finding", r.ID, r.Findings)
			continue
		}
		f := r.Findings[0]
		if f.Phrase != "validation timed out" {
			t.Errorf("row %q: Phrase = %q, want %q", r.ID, f.Phrase, "validation timed out")
		}
		if f.Severity != domain.SevBroken {
			t.Errorf("row %q: Severity = %v, want SevBroken", r.ID, f.Severity)
		}
	}
	if !found {
		t.Error("no demo certificate has a timed-out validation; the wording is unwitnessed")
	}
}
