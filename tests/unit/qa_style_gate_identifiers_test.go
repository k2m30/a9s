package unit_test

// qa_style_gate_identifiers_test.go — the raw-enum gate's blind spot.
//
// TestIssueTextStyleGate_RenderedSurfacesNeverRawEnum only recognises a
// violation shaped like SCREAMING_SNAKE (`^[A-Z][A-Z0-9_]+$`). That misses the
// two identifier shapes an author is far more likely to paste in: the SDK
// struct field (`MapPublicIpOnLaunch`, `SslPolicy`) and the API attribute key
// (`drop_invalid_header_fields`, `desync_mitigation_mode`). Both read to an
// operator as machine text, and neither is something they can act on — the
// point of a supporting row is to name the setting in the words the console
// uses, not the words the SDK uses.
//
// Scope: AttentionDetail row LABELS and Finding.Detail sentences. Row VALUES
// are exempt — a value legitimately IS an identifier (`ELBSecurityPolicy-2016-08`,
// `10.0.0.0/8`, `sg-0abc123`), and forcing prose there would destroy the
// information the row exists to carry.
//
// This gate has no allowlist on purpose. It is a purge, not a burn-down: the
// violations are hand-written literals in the authoring layer, every one is a
// one-line reword, and a green transition must require zero edits to this
// file. A pre-existing hit on a type outside the current batch is a finding to
// route, not an entry to add here.

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// camelCaseIdentifierPattern matches an SDK-style struct field or enum member
// written in CamelCase or lowerCamelCase: two or more capitalised segments run
// together with no space (`MapPublicIpOnLaunch`, `SslPolicy`,
// `AutoAcceptSharedAttachments`). Ordinary prose never produces this shape,
// because English puts a space between words.
var camelCaseIdentifierPattern = regexp.MustCompile(`^[A-Za-z][a-z0-9]*(?:[A-Z][a-z0-9]+)+$`)

// snakeCaseIdentifierPattern matches an API attribute key
// (`drop_invalid_header_fields`, `desync_mitigation_mode`,
// `routing.http.desync_mitigation_mode` once split on the dots).
var snakeCaseIdentifierPattern = regexp.MustCompile(`^[a-z0-9]+(?:_[a-z0-9]+)+$`)

// mixedCaseAcronymPattern matches the plural-acronym shape (`ENIs`, `VPCs`):
// a run of two or more capitals with a lowercase tail. It slips past the
// SCREAMING_SNAKE gate (not all-caps) and past CamelCase (no capitalised
// second segment). Labels only — a Detail sentence may say "over HTTPS", and
// a fully-uppercase label token like "ID" is the existing gate's business.
var mixedCaseAcronymPattern = regexp.MustCompile(`^[A-Z]{2,}[a-z]`)

// awsProductNames are AWS product names that are English proper nouns in this
// domain, not SDK identifiers. "CloudTrail recorded ..." is how an operator
// says it; there is no plainer word to reword them to.
var awsProductNames = map[string]bool{
	"CloudTrail": true, "CloudWatch": true, "CloudFront": true,
	"CloudFormation": true, "DynamoDB": true, "OpenSearch": true,
	"ElastiCache": true, "GuardDuty": true, "SecurityHub": true,
	"DocumentDB": true, "SageMaker": true, "CodePipeline": true,
	"CodeBuild": true, "EventBridge": true, "StepFunctions": true,
}

// parentheticalPattern matches a "(...)" aside. This codebase reports the
// runtime value a sentence is about inside parentheses ("recorded a
// destructive call (TerminateInstances)"). That is the row's data, and the
// batch contract exempts values — so parentheticals are stripped before
// scanning, the same way the raw-enum gate strips "<...>" placeholders.
var parentheticalPattern = regexp.MustCompile(`\([^()]*\)`)

// identifierTokens returns the tokens of text shaped like a machine
// identifier. Dots and slashes separate, so a dotted attribute key is judged
// by its segments. A colon does NOT separate: `sts:AssumeRole` and
// `aws:SourceAccount` are namespaced IAM identifiers an operator must type
// verbatim, and naming them is the only actionable advice a Detail sentence
// can give.
func identifierTokens(text string, labelRules bool) []string {
	if !labelRules {
		text = parentheticalPattern.ReplaceAllString(text, "")
	}
	fields := strings.FieldsFunc(text, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == ',' || r == ';' ||
			r == '/' || r == '.' || r == '(' || r == ')' ||
			r == '"' || r == '\'' || r == '`'
	})
	var out []string
	for _, tok := range fields {
		tok = strings.Trim(tok, ".,;\"'`")
		if strings.Contains(tok, ":") || awsProductNames[tok] {
			continue
		}
		switch {
		case snakeCaseIdentifierPattern.MatchString(tok):
		case camelCaseIdentifierPattern.MatchString(tok) && !knownAcronymExemptions[strings.ToUpper(tok)]:
		case labelRules && mixedCaseAcronymPattern.MatchString(tok):
		default:
			continue
		}
		out = append(out, tok)
	}
	return out
}

// styleGateIdentifierSurface is one hand-authored string plus enough context
// to tell the author which literal to edit.
type styleGateIdentifierSurface struct {
	key  string
	text string
	// label surfaces get the extra plural-acronym rule; Detail sentences do not.
	isLabel bool
}

// collectAuthoredSurfaces gathers every AttentionDetail label and Detail
// sentence a type can render, from the row the app actually builds: fixtures
// folded through runtime.ApplyWave2ToRow by mergeWave2Findings.
//
// Reading the enricher result directly instead would make the gate blind:
// the fold decides which wave-2 rows survive onto the row and under which
// code, so a gate that skips it scans surfaces the operator never sees and
// misses ones they do.
func collectAuthoredSurfaces(t *testing.T, td resource.ResourceTypeDef, fixtures []resource.Resource, cache resource.ResourceCache, clients *awsclient.ServiceClients) []styleGateIdentifierSurface {
	t.Helper()
	var out []styleGateIdentifierSurface

	add := func(shortName, resID, kind string, code domain.FindingCode, text string, isLabel bool) {
		if strings.TrimSpace(text) == "" {
			return
		}
		out = append(out, styleGateIdentifierSurface{
			key:     fmt.Sprintf("%s:%s:%s:%s", shortName, resID, code, kind),
			text:    text,
			isLabel: isLabel,
		})
	}

	for _, res := range mergeWave2Findings(t, td, fixtures, cache, clients) {
		for _, f := range res.Findings {
			add(td.ShortName, res.ID, "detail", f.Code, f.Detail, false)
		}
		for code, ad := range res.AttentionDetails {
			for _, row := range ad.Rows {
				add(td.ShortName, res.ID, "label", code, row.Label, true)
			}
		}
	}
	return out
}

// TestIssueTextStyleGate_LabelsAndDetailsNeverSDKIdentifiers sweeps the demo
// bench and fails on every AttentionDetail label or Detail sentence containing
// an SDK field name or API attribute key. Fixing one means rewording the
// literal in core/aws — there is nothing to route through a humanizer, because
// these strings are authored by hand at the point of emission.
func TestIssueTextStyleGate_LabelsAndDetailsNeverSDKIdentifiers(t *testing.T) {
	byType, cache := buildVisibilityTypeCache(t)
	demoClients := demo.NewServiceClients()

	types := resource.AllResourceTypes()
	sort.Slice(types, func(i, j int) bool { return types[i].ShortName < types[j].ShortName })

	seen := make(map[string]bool)
	var violations []string

	for _, td := range types {
		fixtures := byType[td.ShortName]
		if len(fixtures) == 0 {
			continue
		}
		for _, s := range collectAuthoredSurfaces(t, td, fixtures, cache, demoClients) {
			toks := identifierTokens(s.text, s.isLabel)
			if len(toks) == 0 {
				continue
			}
			line := fmt.Sprintf("%s = %q contains %v", s.key, s.text, toks)
			if seen[line] {
				continue
			}
			seen[line] = true
			violations = append(violations, line)
		}
	}

	if len(violations) == 0 {
		return
	}
	sort.Strings(violations)
	t.Errorf("%d rendered label/Detail surface(s) carry an SDK field name or API attribute key. "+
		"Reword the literal in core/aws: a label is plain words (\"Public address assignment\", "+
		"\"Security policy\", \"Network interfaces\"), a Detail sentence says what to change in words, "+
		"not the attribute name. Row VALUES are exempt and are not scanned. Do not add an allowlist:\n  %s",
		len(violations), strings.Join(violations, "\n  "))
}
