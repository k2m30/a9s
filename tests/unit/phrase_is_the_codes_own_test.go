// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit_test

// phrase_is_the_codes_own_test.go — a finding's phrase, severity and detail
// belong to its code, not to the item that happened to trigger it.
//
// A code is declared once, in a catalog.FindingDef that carries the phrase the
// reader sees, the severity that colours the row and the operator sentence
// under it. An emitter that builds its own wording from the first offending
// item makes the phrase a property of the item instead: two stages of one
// pipeline, two containers of one task and two keys of one user each have a
// different claim to the same code, and setWave2Finding's same-code collapse
// silently keeps whichever emission arrived first. The reader is then told
// about one item with nothing to say the others were inspected, and the
// generated signals page carries a phrase no row ever renders.
//
// Severity has the same shape: the code declares one, and every call site
// passes its own glyph. Nothing before this gate compared the two.
//
// Scope is every demo row of every registered type, wave 1 read off the row
// the real fetcher produced and wave 2 observed at the emission point through
// awsclient.Wave2EmissionObserver — the emissions the collapse discards are
// visible nowhere else, and they are exactly the ones that disagree.

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// phraseMatchesCatalog reports whether an emitted phrase is the catalog's
// registered phrase for its code with the placeholders filled in. A "<…>" span
// in a registered phrase stands for a value the emitter supplies — "<N> jobs
// failed in last 24h" is one phrase, not one per N — so it matches any
// non-empty run of characters in that position and the literal text around it
// must match exactly. A registered phrase with no placeholder must be
// reproduced verbatim.
func phraseMatchesCatalog(registered, emitted string) bool {
	if !strings.Contains(registered, "<") {
		return registered == emitted
	}
	var pattern strings.Builder
	pattern.WriteString("^")
	rest := registered
	for {
		open := strings.Index(rest, "<")
		if open < 0 {
			break
		}
		closeAt := strings.Index(rest[open:], ">")
		if closeAt < 0 {
			break
		}
		pattern.WriteString(regexp.QuoteMeta(rest[:open]))
		pattern.WriteString(".+")
		rest = rest[open+closeAt+1:]
	}
	pattern.WriteString(regexp.QuoteMeta(rest))
	pattern.WriteString("$")
	re, err := regexp.Compile(pattern.String())
	if err != nil {
		return false
	}
	return re.MatchString(emitted)
}

// phraseGateEmission is one Finding as its emitter built it, before any
// collapse or fold, with the resource it was emitted for.
type phraseGateEmission struct {
	resourceID string
	f          domain.Finding
}

// observeWave2Emissions runs enricher against rows with an observer installed
// and returns every Finding it built, including the ones setWave2Finding's
// same-code collapse drops. The observer is called from ForEachParallel
// workers that do not all hold a lock, so the append is guarded here.
func observeWave2Emissions(
	t *testing.T,
	shortName string,
	fn awsclient.IssueEnricherFunc,
	clients *awsclient.ServiceClients,
	rows []resource.Resource,
	cache resource.ResourceCache,
) []phraseGateEmission {
	t.Helper()
	var mu sync.Mutex
	var seen []phraseGateEmission
	awsclient.Wave2EmissionObserver = func(resourceID string, f domain.Finding) {
		mu.Lock()
		defer mu.Unlock()
		seen = append(seen, phraseGateEmission{resourceID: resourceID, f: f})
	}
	defer func() { awsclient.Wave2EmissionObserver = nil }()

	if _, err := fn(context.Background(), clients, rows, cache); err != nil {
		t.Fatalf("%s: Wave-2 enricher returned error: %v", shortName, err)
	}
	mu.Lock()
	defer mu.Unlock()
	return append([]phraseGateEmission(nil), seen...)
}

func TestEveryEmittedFindingCarriesItsCodesPhraseAndSeverity(t *testing.T) {
	defs := map[domain.FindingCode]catalog.FindingDef{}
	for _, td := range append(catalog.All(), catalog.AllChildren()...) {
		for _, f := range td.Findings {
			defs[f.Code] = f
		}
	}
	if len(defs) < 300 {
		t.Fatalf("only %d declared finding codes; the gate is not seeing the catalog", len(defs))
	}

	clients := demo.NewServiceClients()
	byType, cache := buildVisibilityTypeCache(t)

	types := resource.AllResourceTypes()
	sort.Slice(types, func(i, j int) bool { return types[i].ShortName < types[j].ShortName })

	// One line per (type, code, kind, offending value): a type with thirty demo
	// rows would otherwise print the same defect thirty times.
	reported := map[string]bool{}
	var phraseOffenders, severityOffenders, disagreements []string
	report := func(bucket *[]string, line string) {
		if reported[line] {
			return
		}
		reported[line] = true
		*bucket = append(*bucket, line)
	}

	check := func(shortName, resourceID string, f domain.Finding) {
		def, declared := defs[f.Code]
		if !declared {
			// TestFindingCodesAreDeclared owns the undeclared-code case; this
			// gate has nothing to compare against.
			return
		}
		if !phraseMatchesCatalog(def.Phrase, f.Phrase) {
			report(&phraseOffenders, fmt.Sprintf(
				"%s %s: emitted %q, catalog declares %q (first seen on %s)",
				shortName, f.Code, f.Phrase, def.Phrase, resourceID))
		}
		if f.Severity != def.Severity {
			report(&severityOffenders, fmt.Sprintf(
				"%s %s: emitted severity %v, catalog declares %v (first seen on %s)",
				shortName, f.Code, f.Severity, def.Severity, resourceID))
		}
	}

	// What a (resource, code) pair says the first time it is emitted; a later
	// emission of the same pair must say the same three things, because the
	// collapse keeps only this one.
	type saying struct {
		phrase   string
		severity domain.Severity
		detail   string
	}

	for _, td := range types {
		rows := byType[td.ShortName]
		if len(rows) == 0 {
			continue
		}

		for _, r := range rows {
			for _, f := range r.Findings {
				check(td.ShortName, r.ID, f)
			}
		}

		enricher, ok := awsclient.Wave2EnricherFor(td.ShortName)
		if !ok || enricher.Fn == nil {
			continue
		}
		first := map[string]saying{}
		for _, e := range observeWave2Emissions(t, td.ShortName, enricher.Fn, clients, rows, cache) {
			check(td.ShortName, e.resourceID, e.f)

			key := e.resourceID + "\x00" + string(e.f.Code)
			said := saying{phrase: e.f.Phrase, severity: e.f.Severity, detail: e.f.Detail}
			prior, seen := first[key]
			if !seen {
				first[key] = said
				continue
			}
			if prior != said {
				report(&disagreements, fmt.Sprintf(
					"%s %s on %s: first emission said (%q, %v, %q), a later one said (%q, %v, %q)",
					td.ShortName, e.f.Code, e.resourceID,
					prior.phrase, prior.severity, prior.detail,
					said.phrase, said.severity, said.detail))
			}
		}
	}

	sort.Strings(phraseOffenders)
	sort.Strings(severityOffenders)
	sort.Strings(disagreements)

	for _, o := range phraseOffenders {
		t.Errorf("phrase is not the code's: %s — register the wording the reader sees on the "+
			"code's catalog.FindingDef and pass that phrase; the item that triggered it belongs "+
			"in a supporting row", o)
	}
	for _, o := range severityOffenders {
		t.Errorf("severity is not the code's: %s — the glyph the call site passes must be the "+
			"severity the catalog declares, or the code needs splitting into one code per "+
			"severity", o)
	}
	for _, o := range disagreements {
		t.Errorf("two emissions of one (resource, code) disagree: %s — setWave2Finding keeps the "+
			"first and discards the rest, so the reader is told about one item only; the "+
			"emissions must agree and the items must be rows", o)
	}
}
