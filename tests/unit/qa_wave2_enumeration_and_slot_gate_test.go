// qa_wave2_enumeration_and_slot_gate_test.go — four rules the registrations
// and the docs hold by convention today and nothing checks.
//
// ROW 7. Wave 2 enumeration covers parents and children. A child type
// registers its enricher on the same field a parent does, and every consumer
// of that registration — the dispatch queue, the declared-reads gate, the
// detail bench's cache list — asks one enumeration for the set. An
// enumeration that answers for half the catalog reads like a general one at
// the call site, and the half it drops is silent: no dispatch, no findings,
// no error.
//
// ROW 8. The Wave 2 registration matches the docs generator's own input. A
// type whose signals table carries a wave2 row is a type something has to run
// on a second pass; a type with no wave2 row registering an enricher is
// wiring nobody can read a reason for. Both directions, because either one
// alone lets the pair drift.
//
// ROW 9. The colour rules in docs/design/ are generated from the catalog, the
// way the resource docs and the signals table already are. A hand-typed
// colour rule is a second declaration of a severity, and it went stale
// exactly as a second declaration does.
//
// ROW 10. No status cell carries a raw SDK error code. An error string AWS
// returns is `Code: sentence`, and pasting it whole into a phrase slot puts
// the code in front of the operator instead of the cause. The style gate
// misses it because its whole-cell check exempts anything containing a colon
// as an identifier, and its token check only matches all-caps tokens, so a
// CamelCase error code inside prose passes both.
package unit_test

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// ---------------------------------------------------------------------------
// ROW 7 — one enumeration over parents and children
// ---------------------------------------------------------------------------

// wave2ChildProbeName is a sentinel child type registered only for the
// duration of the pin below. Named so a failure elsewhere points back here.
const wave2ChildProbeName = "qa-wave2-child-probe"

// wave2ChildProbeReads is what the probe child declares it scans. The value is
// asserted on the enumerated entry, because the declared-reads gate and the
// detail bench both read Reads off whatever the enumeration hands them — a
// child that reaches the enumeration without its metadata is only half
// enumerated.
var wave2ChildProbeReads = []string{"ec2", "s3"}

// TestAChildTypesWave2EnricherIsEnumeratedAndDispatched registers a child type
// carrying a Wave 2 enricher through the child-type test registry, and holds
// the two read-side entry points to it. Wave2EnricherFor is what the dispatch
// path calls per type (core/runtime/probes.go ProbeEnrichment); AllWave2 is
// what builds the queue and what the declared-reads gate and the detail bench
// iterate. A registration only one of them can see is a registration that
// runs and reports nothing, or reports without running.
func TestAChildTypesWave2EnricherIsEnumeratedAndDispatched(t *testing.T) {
	called := false
	fn := func(_ context.Context, _ *awsclient.ServiceClients, _ []resource.Resource, _ resource.ResourceCache) (awsclient.IssueEnricherResult, error) {
		called = true
		return awsclient.IssueEnricherResult{
			Findings:         map[string][]domain.Finding{},
			AttentionDetails: map[string]map[domain.FindingCode]domain.AttentionDetail{},
			TruncatedIDs:     map[string]string{},
			FieldUpdates:     map[string]map[string]string{},
		}, nil
	}

	resource.SetChildTypeForTest(resource.ResourceTypeDef{
		Name:      "QA Wave2 Child Probe",
		ShortName: wave2ChildProbeName,
		Category:  "COMPUTE",
		Wave2: awsclient.IssueEnricher{
			Fn:       fn,
			Priority: 100,
			Reads:    wave2ChildProbeReads,
		},
	})
	t.Cleanup(func() { resource.CleanupChildTypeForTest(wave2ChildProbeName) })

	e, ok := awsclient.Wave2EnricherFor(wave2ChildProbeName)
	if !ok || e.Fn == nil {
		t.Fatalf("Wave2EnricherFor(%q) found nothing; a child type's enricher is "+
			"never dispatched, so the type is enriched by nobody", wave2ChildProbeName)
	}
	if _, err := e.Fn(context.Background(), nil, nil, nil); err != nil {
		t.Fatalf("probe enricher returned %v", err)
	}
	if !called {
		t.Error("Wave2EnricherFor returned an entry whose Fn is not the registered one")
	}

	var entry awsclient.Wave2Entry
	var found bool
	for _, w := range awsclient.AllWave2() {
		if w.ShortName == wave2ChildProbeName {
			entry, found = w, true
			break
		}
	}
	if !found {
		names := make([]string, 0, len(awsclient.AllWave2()))
		for _, w := range awsclient.AllWave2() {
			names = append(names, w.ShortName)
		}
		t.Fatalf("AllWave2() does not enumerate the child type %q (%d entries: %s). "+
			"The queue, the declared-reads gate and the detail bench all read this "+
			"one enumeration, so the child's enricher is invisible to every one of them",
			wave2ChildProbeName, len(names), strings.Join(names, ", "))
	}
	if !reflect.DeepEqual(entry.Enricher.Reads, wave2ChildProbeReads) {
		t.Errorf("AllWave2 entry for %q has Reads=%v, want %v — the enumeration "+
			"dropped the registration's metadata, so the bench loads the wrong caches",
			wave2ChildProbeName, entry.Enricher.Reads, wave2ChildProbeReads)
	}
}

// ---------------------------------------------------------------------------
// ROW 8 — the registration and the docs' Wave 2 column agree, both ways
// ---------------------------------------------------------------------------

// declaresWave2Signal reports whether the docs generator would print a wave2
// row for this type. FindingDef.Source is the generator's input for the Wave
// column of the signals table (cmd/catalogen), so it is the same fact the
// docs show, read from the same place rather than parsed back out of them.
func declaresWave2Signal(td catalog.ResourceTypeDef) bool {
	for _, f := range td.Findings {
		if f.Source == "wave2" {
			return true
		}
	}
	return false
}

func TestTheWave2RegistrationMatchesTheDocsWave2Column(t *testing.T) {
	sentinel := reflect.ValueOf(awsclient.InFetcherWave2Sentinel).Pointer()

	var missingRegistration, unexplainedRegistration []string
	for _, td := range append(catalog.All(), catalog.AllChildren()...) {
		declared := declaresWave2Signal(td)
		e, registered := awsclient.Wave2EnricherFor(td.ShortName)
		switch {
		case declared && !registered:
			missingRegistration = append(missingRegistration, fmt.Sprintf(
				"%s declares wave2-sourced findings and registers no Wave2 enricher, "+
					"so nothing ever emits them", td.ShortName))
		case !declared && registered:
			kind := "a real enricher"
			if e.Fn != nil && reflect.ValueOf(e.Fn).Pointer() == sentinel {
				kind = "InFetcherWave2Sentinel"
			}
			unexplainedRegistration = append(unexplainedRegistration, fmt.Sprintf(
				"%s registers %s and declares no wave2-sourced finding, so the docs "+
					"say this type has no Wave 2 signals while the catalog says it is "+
					"Wave-2-covered", td.ShortName, kind))
		}
	}

	sort.Strings(missingRegistration)
	sort.Strings(unexplainedRegistration)
	if len(missingRegistration) > 0 {
		t.Errorf("%d type(s) promise a Wave 2 signal nothing runs:\n  %s",
			len(missingRegistration), strings.Join(missingRegistration, "\n  "))
	}
	if len(unexplainedRegistration) > 0 {
		t.Errorf("%d type(s) are wired Wave-2-covered with nothing in the docs to "+
			"read that from. Either the in-fetcher findings carry Source \"wave2\" so "+
			"the signals table shows them, or the registration goes:\n  %s",
			len(unexplainedRegistration), strings.Join(unexplainedRegistration, "\n  "))
	}
}

// ---------------------------------------------------------------------------
// ROW 9 — the design docs' colour rules are generated from the catalog
// ---------------------------------------------------------------------------

const (
	designColorsBeginMarker = "<!-- BEGIN GENERATED: colors -->"
	designColorsEndMarker   = "<!-- END GENERATED: colors -->"
)

var designColorDocs = []string{
	"../../docs/design/design.md",
	"../../docs/design/ec2-status-checks.md",
}

// designColorWord matches a colour named as a word rather than as a hex or a
// style-token identifier. A palette table listing `#f7768e` against
// `ColStopped` is naming a swatch, not stating which rows are red.
var designColorWord = regexp.MustCompile(`(?i)\b(red|yellow|green|dim|gray|grey)\b`)

// stoppedByAWSQualifier marks a line that is talking about the instance AWS
// stopped itself rather than the one an operator stopped. The catalog splits
// those two into separate codes at separate severities, so a colour claim is
// only readable once you know which of the two the line means.
var stoppedByAWSQualifier = regexp.MustCompile(`(?i)(by AWS|Server\.|itself)`)

func TestTheDesignDocsColourRulesComeFromAGeneratedBlock(t *testing.T) {
	// The catalog is the one place these colours are decided. Reading them here
	// rather than typing them keeps this gate from becoming the third copy.
	stoppedSev := catalog.Severity("ec2.state.stopped")
	serverStoppedSev := catalog.Severity("ec2.state.stopped.server")
	terminatedSev := catalog.Severity("ec2.state.terminated")
	if stoppedSev != domain.SevWarn || serverStoppedSev != domain.SevBroken || terminatedSev != domain.SevDim {
		t.Fatalf("catalog severities moved (stopped=%v, stopped.server=%v, terminated=%v); "+
			"this gate's colour expectations are derived from them and need re-reading",
			stoppedSev, serverStoppedSev, terminatedSev)
	}

	for _, path := range designColorDocs {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		body := string(raw)

		begin := strings.Index(body, designColorsBeginMarker)
		end := strings.Index(body, designColorsEndMarker)
		if begin < 0 || end < begin {
			t.Errorf("%s carries no %s / %s block, so its colour rules are hand-typed "+
				"— a second declaration of a severity the catalog already owns, and "+
				"no check is red when the two disagree",
				path, designColorsBeginMarker, designColorsEndMarker)
		}

		for i, line := range strings.Split(body, "\n") {
			if !designColorWord.MatchString(line) {
				continue
			}
			lower := strings.ToLower(line)
			where := fmt.Sprintf("%s:%d: %s", path, i+1, strings.TrimSpace(line))

			// An operator-stopped instance is Warn. A line that says "stopped"
			// without naming the AWS-initiated case and calls it red is claiming
			// the severity of the other code.
			if strings.Contains(lower, "stopped") && !stoppedByAWSQualifier.MatchString(line) {
				if regexp.MustCompile(`(?i)\bred\b`).MatchString(line) {
					t.Errorf("%s\n    calls a `stopped` row red; the catalog ships "+
						"ec2.state.stopped at %v and only ec2.state.stopped.server at %v",
						where, stoppedSev, serverStoppedSev)
				}
			}
			// A terminated instance is Dim. Any other colour on a line naming it
			// contradicts the catalog.
			if strings.Contains(lower, "terminated") {
				if regexp.MustCompile(`(?i)\b(red|yellow|green)\b`).MatchString(line) {
					t.Errorf("%s\n    gives a `terminated` row a colour other than dim; "+
						"the catalog ships ec2.state.terminated at %v", where, terminatedSev)
				}
			}
		}
	}
}

// ---------------------------------------------------------------------------
// ROW 10 — no status cell carries a raw SDK error code
// ---------------------------------------------------------------------------

// sdkErrorCodePrefix matches an AWS error code introducing the sentence after
// it: two or more words run together, each Capitalised or an acronym, then a
// colon and a space. It is the same shape core/aws's awsErrorCodeShape
// accepts, deliberately, so the gate cannot go looking for a narrower set of
// codes than the humaniser strips — a code the humaniser handles and the gate
// misses is a rule with no check, and a code the gate flags and the humaniser
// leaves is a failure nobody can fix.
//
// The acronym alternative carries the whole rule: "KMSKeyNotFound" and
// "SNSInvalidParameter" are the shape half of these codes take, and a
// Capitalised-words-only pattern reads "AccessDenied" and walks straight past
// them.
//
// Two words minimum, and the colon must be followed by a space, so a
// sentence's own punctuation survives: "Note: …" is one word, and "PORTS:22"
// has no space.
//
// The style gate's own checks cannot see any of this — its whole-cell rule
// exempts any value containing a colon as an identifier, and its token rule
// only matches tokens with no lowercase letters at all.
var sdkErrorCodePrefix = regexp.MustCompile(`\b(?:[A-Z]+[a-z0-9]*){2,}: `)

// TestTheSDKErrorCodePatternCatchesTheShapesAWSReturns pins which codes the
// sweep above is looking for. The sweep is only as good as this pattern, and
// its demo bench holds one offending row: a pattern narrowed by accident would
// leave the sweep green and say nothing, which is how the acronym-led half of
// these codes went unseen in the first place.
func TestTheSDKErrorCodePatternCatchesTheShapesAWSReturns(t *testing.T) {
	cases := []struct {
		cell string
		want bool
		why  string
	}{
		{"delivery error: AccessDenied: The S3 bucket policy denies CloudTrail writes", true,
			"two Capitalised words, the shape the demo trail returns"},
		{"delivery error: KMSKeyNotFound: The KMS key for this trail no longer exists", true,
			"acronym-led, the half a Capitalised-words-only pattern walks past"},
		{"delivery error: SNSInvalidParameter: The topic ARN is not valid", true,
			"acronym-led with the acronym mid-code"},
		{"delivery error: ThrottlingException: Rate exceeded", true,
			"the Exception suffix is just another run-together word"},

		{"delivery error: the S3 bucket policy denies CloudTrail writes", false,
			"the humaniser has stripped the code; the sentence keeps its own capitals"},
		{"unhealthy targets: 2 of 5", false,
			"a prose label before a colon is one word, not a code"},
		{"risk: PORTS:22 open to the internet", false,
			"no space after the colon, so it is an identifier rather than a code"},
		{"stopping: instance is shutting down", false,
			"a lowercase label keeps its colon"},
	}

	for _, tc := range cases {
		if got := sdkErrorCodePrefix.MatchString(tc.cell); got != tc.want {
			t.Errorf("MatchString(%q) = %v, want %v — %s", tc.cell, got, tc.want, tc.why)
		}
	}
}

func TestNoStatusCellCarriesAnSDKErrorCode(t *testing.T) {
	byType, cache := buildVisibilityTypeCache(t)
	clients := demo.NewServiceClients()

	var offenders []string
	for _, td := range resource.AllResourceTypes() {
		rows := byType[td.ShortName]
		if len(rows) == 0 {
			continue
		}
		merged := mergeWave2Findings(t, td, rows, cache, clients)
		for _, r := range merged {
			cell, ok := listStatusCellFor(t, td, merged, r.ID)
			if !ok || cell == "" {
				continue
			}
			if code := sdkErrorCodePrefix.FindString(cell); code != "" {
				offenders = append(offenders, fmt.Sprintf(
					"%s %s: status cell %q carries the SDK error code %q",
					td.ShortName, r.ID, cell, strings.TrimSuffix(code, ": ")))
			}
		}
	}

	if len(offenders) > 0 {
		sort.Strings(offenders)
		t.Errorf("%d status cell(s) paste an AWS error string into a phrase slot "+
			"whole. The operator gets the code before the cause, and it reaches the "+
			"cell unseen because a value with a colon is exempt from the whole-cell "+
			"check. Route the error through the humaniser: strip the code, keep the "+
			"sentence:\n  %s", len(offenders), strings.Join(offenders, "\n  "))
	}
}

// demoTrailDeliveryErrorID is the demo trail whose GetTrailStatus reports
// LatestDeliveryError. The fixture's error is the realistic AWS shape, code
// then sentence, so the row exercises the strip rather than a value that would
// pass through untouched.
const demoTrailDeliveryErrorID = "data-events-trail"

// TestTheDemoTrailDeliveryErrorReadsAsWords holds the other half of row 10:
// stripping the code must not throw the cause away with it. A cell reading
// "delivery error" alone tells the operator less than the raw string did.
func TestTheDemoTrailDeliveryErrorReadsAsWords(t *testing.T) {
	byType, cache := buildVisibilityTypeCache(t)
	td := catalog.FindAny("trail")
	if td == nil {
		t.Fatal("trail is not a registered type")
	}
	merged := mergeWave2Findings(t, *td, byType["trail"], cache, demo.NewServiceClients())

	cell, ok := listStatusCellFor(t, *td, merged, demoTrailDeliveryErrorID)
	if !ok {
		t.Fatalf("no status cell for demo trail %q", demoTrailDeliveryErrorID)
	}
	if !strings.HasPrefix(cell, "delivery error: ") {
		t.Fatalf("demo trail %q status cell = %q; the delivery-error finding is not "+
			"the one this pin is reading", demoTrailDeliveryErrorID, cell)
	}
	slot := strings.TrimPrefix(cell, "delivery error: ")

	if strings.Contains(slot, "AccessDenied") {
		t.Errorf("the slot reads %q — AccessDenied is the SDK's code for the error, "+
			"not what went wrong", slot)
	}
	if !strings.Contains(strings.ToLower(slot), "bucket policy") {
		t.Errorf("the slot reads %q, and the cause the fixture reports is the S3 "+
			"bucket policy denying CloudTrail writes; stripping the code must keep "+
			"the sentence", slot)
	}
}
