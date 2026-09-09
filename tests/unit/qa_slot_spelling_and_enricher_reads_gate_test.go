package unit

// qa_slot_spelling_and_enricher_reads_gate_test.go — two standing rules the
// catalog's declarations have to hold.
//
// RULE 4 (one placeholder spelling). A phrase's "<…>" slot is read by a person
// in the generated signal tables, where the token is printed verbatim because
// there is no resource to substitute into it. A slot spelled as an SDK field
// name or in capitals is the same fact in a second spelling: the reader has to
// decide whether "<STATUS>" and "<status>" are one thing, and whether
// "<LatestDeliveryError>" is a value or a Go identifier they should go and
// grep. Slots are lowercase prose. The only exceptions are the substitution
// tokens fillSlot itself reads — N, M and LIST — which are machinery, not
// wording, and are declared here rather than discovered.
//
// RULE 5 (an enricher declares the caches it reads). A Wave 2 enricher that
// scans another type's ResourceCache produces nothing when that cache is not
// loaded, and produces it silently: no error, no skip, just an absent finding.
// Anything that has to arrange those caches — the demo bench below, a future
// prefetch — needs to know which they are, and the only place that knowledge
// can live without going stale is beside the enricher's own registration.
// This gate does not read that declaration to decide what is true; it runs the
// enricher against the full cache and against a cache holding only what it
// declares, and fails when the two disagree. The set of scanners is therefore
// derived from the code every time, not from a list somebody has to maintain.

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/resource"
)

// phraseSlot matches one "<…>" substitution slot inside a declared phrase.
var phraseSlot = regexp.MustCompile(`<([^<>]*)>`)

// slotMachineryTokens are the words fillSlot (core/aws/wave1_rows.go) reads as
// instructions rather than as wording: the count tokens and the list token.
// They are capitals because they are not prose, and they are the only capitals
// a slot may hold.
var slotMachineryTokens = map[string]bool{"LIST": true, "N": true, "M": true}

// slotWord splits a slot's content into the words the case rule judges. Whole
// words, not substrings: stripping the machinery tokens as substrings let a
// slot spelled "NM" strip itself to nothing and pass, and would have let
// "MONTH" through as "OTH" had that not still read as capitals by luck.
var slotWord = regexp.MustCompile(`[A-Za-z]+`)

func TestEveryPhraseSlotIsLowercaseProse(t *testing.T) {
	var wrong []string
	for _, td := range append(catalog.All(), catalog.AllChildren()...) {
		for _, def := range td.Findings {
			for _, slot := range phraseSlot.FindAllStringSubmatch(def.Phrase, -1) {
				for _, word := range slotWord.FindAllString(slot[1], -1) {
					if slotMachineryTokens[word] || strings.ToLower(word) == word {
						continue
					}
					wrong = append(wrong, fmt.Sprintf("%s %s phrase=%q slot=%q word=%q",
						td.ShortName, def.Code, def.Phrase, slot[0], word))
				}
			}
		}
	}
	if len(wrong) > 0 {
		sort.Strings(wrong)
		t.Errorf("%d phrase slot(s) are not lowercase prose, so the generated signal "+
			"tables print two spellings of one kind of placeholder:\n  %s",
			len(wrong), strings.Join(wrong, "\n  "))
	}
}

// enricherFindingFingerprint runs one type's Wave 2 enricher against cache and
// returns its findings as a stable string. "" means the type registers none.
func enricherFindingFingerprint(t *testing.T, shortName string, clients *awsclient.ServiceClients, cache resource.ResourceCache) string {
	t.Helper()
	td := resource.FindResourceType(shortName)
	if td == nil {
		t.Fatalf("%s: not a registered type", shortName)
	}
	rows, ok := DrainFixtures(t, *td, clients)
	if !ok {
		return ""
	}
	e, ok := awsclient.Wave2EnricherFor(shortName)
	if !ok || e.Fn == nil {
		return ""
	}
	res, err := e.Fn(context.Background(), clients, rows, cache)
	if err != nil {
		t.Fatalf("demo %s Wave2Enricher: %v", shortName, err)
	}
	var lines []string
	for id, fs := range res.Findings {
		for _, f := range fs {
			lines = append(lines, id+"/"+string(f.Code)+"/"+f.Phrase)
		}
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

func TestEveryCrossRefEnricherDeclaresTheCachesItReads(t *testing.T) {
	clients := demo.NewServiceClients()
	full := resource.ResourceCache{}
	for _, td := range resource.AllResourceTypes() {
		if rows, ok := DrainFixtures(t, td, clients); ok {
			full[td.ShortName] = resource.ResourceCacheEntry{Resources: rows}
		}
	}
	if len(full) < 30 {
		t.Fatalf("only %d types drained; the bench is not seeing the demo fixtures", len(full))
	}

	var undeclared []string
	for _, entry := range awsclient.AllWave2() {
		declared := resource.ResourceCache{}
		for _, name := range entry.Enricher.Reads {
			if e, ok := full[name]; ok {
				declared[name] = e
			}
		}
		withAll := enricherFindingFingerprint(t, entry.ShortName, clients, full)
		withDeclared := enricherFindingFingerprint(t, entry.ShortName, clients, declared)
		if withAll == withDeclared {
			continue
		}
		undeclared = append(undeclared, fmt.Sprintf(
			"%s declares Reads=%v, and running it against only those caches changes "+
				"its findings, so it scans a cache it does not name", entry.ShortName, entry.Enricher.Reads))
	}
	if len(undeclared) > 0 {
		sort.Strings(undeclared)
		t.Errorf("%d Wave 2 enricher(s) read a resource cache they do not declare. An "+
			"undeclared read is silent: whatever arranges the caches cannot know to load "+
			"it, and the finding simply never appears. Add the cache to Reads on the "+
			"catalog's Wave2 registration:\n  %s",
			len(undeclared), strings.Join(undeclared, "\n  "))
	}
}

// TestStripAWSErrorCodeKeepsTheSentence pins the strip against the shapes AWS
// actually returns. The acronym-led codes are the ones that matter: a
// CamelCase-only rule reads "AccessDenied" and misses "KMSKeyNotFound", which
// is the more common shape for exactly the errors a trail reports.
func TestStripAWSErrorCodeKeepsTheSentence(t *testing.T) {
	cases := []struct{ in, want string }{
		{"AccessDenied: The S3 bucket policy denies CloudTrail writes", "The S3 bucket policy denies CloudTrail writes"},
		{"KMSKeyNotFound: the key is gone", "the key is gone"},
		{"AccessDeniedException: nope", "nope"},
		{"InvalidParameterValue: a: b", "a: b"},
		// Not codes: one word, so the colon belongs to the sentence.
		{"Error: something", "Error: something"},
		{"Note: the bucket policy denies writes", "Note: the bucket policy denies writes"},
		{"S3: short", "S3: short"},
		{"lowercase: still a sentence", "lowercase: still a sentence"},
		// Nothing after the code is nothing to keep.
		{"AccessDenied:", "AccessDenied:"},
		{"AccessDenied: ", "AccessDenied: "},
		{"no colon at all", "no colon at all"},
		{"", ""},
	}
	for _, c := range cases {
		if got := awsclient.StripAWSErrorCode(c.in); got != c.want {
			t.Errorf("StripAWSErrorCode(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
