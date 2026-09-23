package unit_test

import (
	"context"
	"strings"
	"testing"
	"unicode"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/lambda"

	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

var t561BenchTypes = []string{"vpce", "lambda", "ecr"} //nolint:gochecknoglobals // test-only list

func t561Bench(t *testing.T) map[string][]resource.Resource {
	t.Helper()
	clients := demo.NewServiceClients()
	byType, cache := buildVisibilityTypeCache(t)
	out := map[string][]resource.Resource{}
	for _, short := range t561BenchTypes {
		td := resource.FindResourceType(short)
		if td == nil {
			t.Fatalf("%s not registered", short)
		}
		out[short] = mergeWave2Findings(t, *td, byType[short], cache, clients)
	}
	return out
}

// The demo answers in the shapes AWS uses, so a check that reads a field AWS
// does not send fails on the demo as it does on a real account: endpoint state
// in the service's spelling, no lifecycle fields on ListFunctions, no scan
// summary on DescribeImages.
func TestT561_DemoFakesAnswerInAWSShapes(t *testing.T) {
	clients := demo.NewServiceClients()
	bench := t561Bench(t)

	for _, r := range bench["vpce"] {
		if s := r.Fields["state"]; s == "" || !unicode.IsLower(rune(s[0])) {
			t.Errorf("vpce %s: state %q is the SDK constant, AWS sends it lower-camel", r.ID, s)
		}
	}

	fns, err := clients.Lambda.ListFunctions(context.Background(), &lambda.ListFunctionsInput{})
	if err != nil {
		t.Fatalf("demo ListFunctions: %v", err)
	}
	for _, fn := range fns.Functions {
		if fn.State != "" || fn.StateReasonCode != "" || fn.LastUpdateStatus != "" {
			t.Errorf("demo ListFunctions %s carries State=%q StateReasonCode=%q LastUpdateStatus=%q; ListFunctions returns none of them",
				aws.ToString(fn.FunctionName), fn.State, fn.StateReasonCode, fn.LastUpdateStatus)
		}
	}

	for _, repo := range bench["ecr"] {
		out, err := clients.ECR.DescribeImages(context.Background(), &ecr.DescribeImagesInput{RepositoryName: aws.String(repo.Name)})
		if err != nil {
			t.Fatalf("demo DescribeImages %s: %v", repo.Name, err)
		}
		for _, img := range out.ImageDetails {
			if img.ImageScanFindingsSummary != nil || img.ImageScanStatus != nil {
				t.Errorf("demo DescribeImages %s@%s carries a scan summary or status; Basic Scanning leaves both empty", repo.Name, aws.ToString(img.ImageDigest))
			}
		}
	}
}

// The witnesses the demo shows for these checks: a failed and a
// pending-acceptance endpoint, a failed function, a function on an end-of-life
// runtime and a repository with its critical count. Each shows its finding in
// the Status cell and in the row colour.
func TestT561_DemoWitnessesShowTheirFinding(t *testing.T) {
	bench := t561Bench(t)
	witnesses := []struct {
		short string
		code  domain.FindingCode
		match func(resource.Resource) bool
	}{
		{"vpce", "vpce.state.failed", func(r resource.Resource) bool { return r.Fields["state"] == "failed" }},
		{"vpce", "vpce.state.pending_acceptance", func(r resource.Resource) bool { return r.Fields["state"] == "pendingAcceptance" }},
		{"lambda", "lambda.state.failed", func(resource.Resource) bool { return true }},
		{"lambda", "lambda.runtime.deprecated", func(resource.Resource) bool { return true }},
		{"ecr", "ecr.vulnerabilities", func(resource.Resource) bool { return true }},
	}
	for _, w := range witnesses {
		td := resource.FindResourceType(w.short)
		rows := bench[w.short]
		found := false
		for _, r := range rows {
			if !w.match(r) || !t561Has(r.Findings, w.code) {
				continue
			}
			found = true
			top, _ := domain.TopFinding(r.Findings)
			if top.Code != w.code {
				t.Errorf("%s %s: top finding %s outranks %s; the witness does not show it", w.short, r.ID, top.Code, w.code)
				continue
			}
			cell, ok := listStatusCellFor(t, *td, rows, r.ID)
			if !ok {
				t.Fatalf("%s %s: no list row", w.short, r.ID)
			}
			if got, _, _ := strings.Cut(cell, " (+"); got != top.Phrase {
				t.Errorf("%s %s: Status cell = %q, want %q", w.short, r.ID, cell, top.Phrase)
			}
			if got, want := td.ResolveColor(r), w5ColorOfSeverity(top.Severity); got != want {
				t.Errorf("%s %s: colour = %v, want %v", w.short, r.ID, got, want)
			}
		}
		if !found {
			t.Errorf("no demo %s row carries %s", w.short, w.code)
		}
	}
}

// Row colour and Status cell select the same finding, and a supporting row
// adds words: it neither restates its phrase nor prints a Go bool literal or
// SDK enum casing (a trailing parenthesised aside is stripped first).
func TestT561_BenchRowsSelectOneFindingAndAddWords(t *testing.T) {
	multi := 0
	for short, rows := range t561Bench(t) {
		td := resource.FindResourceType(short)
		for _, r := range rows {
			top, ok := domain.TopFinding(r.Findings)
			if !ok {
				continue
			}
			if len(r.Findings) > 1 {
				multi++
			}
			if got, want := td.ResolveColor(r), w5ColorOfSeverity(top.Severity); got != want {
				t.Errorf("%s/%s: colour %v, top finding %q is %v", short, r.ID, got, top.Code, want)
			}
			if got, _, _ := strings.Cut(domain.StatusPhrase(r.Findings), " (+"); got != top.Phrase {
				t.Errorf("%s/%s: status names %q, colour's top finding is %q", short, r.ID, got, top.Phrase)
			}
			for _, f := range r.Findings {
				phrase := w5Normalize(f.Phrase)
				for _, row := range r.AttentionDetails[f.Code].Rows {
					if w5Normalize(row.Label+" "+row.Value) == phrase || w5Normalize(row.Value) == phrase {
						t.Errorf("%s/%s %s: row %q=%q restates the phrase %q", short, r.ID, f.Code, row.Label, row.Value, f.Phrase)
					}
					v := strings.TrimSpace(row.Value)
					if i := strings.LastIndex(v, " ("); i > 0 && strings.HasSuffix(v, ")") {
						v = strings.TrimSpace(v[:i])
					}
					switch {
					case v == "true" || v == "false":
						t.Errorf("%s/%s %s row %q = %q: a Go bool literal", short, r.ID, f.Code, row.Label, row.Value)
					case v != "" && v != "*" && v != strings.ToLower(v) && v == strings.ToUpper(v) && !strings.ContainsAny(v, "0123456789"):
						t.Errorf("%s/%s %s row %q = %q: SDK enum casing", short, r.ID, f.Code, row.Label, row.Value)
					}
				}
			}
		}
	}
	if multi == 0 {
		t.Error("no bench row carries more than one finding; the selector check cannot fail")
	}
}
