package unit_test

import (
	"context"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	sqssvc "github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"

	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/iampolicy"
	"github.com/k2m30/a9s/v3/core/resource"
)

var policyBenchTypes = []string{ //nolint:gochecknoglobals // test-only list
	"ddb", "sqs", "sns", "ecr", "efs", "kms", "secrets", "codeartifact", "apigw", "lambda", "opensearch", "vpce",
}

func policyBench(t *testing.T) map[string][]resource.Resource {
	t.Helper()
	clients := demo.NewServiceClients()
	byType, cache := buildVisibilityTypeCache(t)
	out := map[string][]resource.Resource{}
	for _, short := range policyBenchTypes {
		td := resource.FindResourceType(short)
		if td == nil {
			t.Fatalf("%s not registered", short)
		}
		out[short] = mergeWave2Findings(t, *td, byType[short], cache, clients)
	}
	return out
}

// A queue whose wildcard grant is fenced by an explicit Deny is not open to
// anyone; a queue open to everyone reads "queue policy open to anyone".
func TestDemoQueueFencedByExplicitDenyShowsNoPublicPolicy(t *testing.T) {
	td := resource.FindResourceType("sqs")
	if td == nil {
		t.Fatal("sqs not registered")
	}
	rows := policyBench(t)["sqs"]
	clients := demo.NewServiceClients()

	var fenced []resource.Resource
	for _, r := range rows {
		out, err := clients.SQS.GetQueueAttributes(context.Background(), &sqssvc.GetQueueAttributesInput{
			QueueUrl:       aws.String(r.Fields["queue_url"]),
			AttributeNames: []sqstypes.QueueAttributeName{sqstypes.QueueAttributeNamePolicy},
		})
		if err != nil || out.Attributes["Policy"] == "" {
			continue
		}
		doc, err := iampolicy.Parse(out.Attributes["Policy"])
		if err != nil {
			t.Fatalf("demo queue %s carries a policy that does not parse: %v", r.ID, err)
		}
		var wildcardAllow, deny bool
		for _, st := range doc.Statement {
			wildcardAllow = wildcardAllow || (st.Effect == "Allow" &&
				iampolicy.Evaluate(iampolicy.Document{Statement: []iampolicy.Statement{st}}, "").Public)
			deny = deny || st.Effect == "Deny"
		}
		if wildcardAllow && deny {
			fenced = append(fenced, r)
		}
	}
	if len(fenced) == 0 {
		t.Fatal("no demo queue grants Allow * alongside a matching explicit Deny; the rule has no row where it can be seen to hold against the naive reading")
	}

	for _, r := range fenced {
		for _, f := range r.Findings {
			if f.Code == "sqs.public-policy" {
				t.Errorf("demo queue %s: Allow * fenced by an explicit Deny still carries sqs.public-policy", r.ID)
			}
		}
		if cell, ok := listStatusCellFor(t, *td, rows, r.ID); ok && strings.Contains(cell, "queue policy open to anyone") {
			t.Errorf("demo queue %s: Status cell %q still says the policy is open", r.ID, cell)
		}
	}

	var witness *resource.Resource
	for i := range rows {
		if rows[i].ID == fixtures.SQSPublicPolicy {
			witness = &rows[i]
		}
	}
	if witness == nil {
		t.Fatalf("witness queue %s missing from the demo", fixtures.SQSPublicPolicy)
	}
	top, ok := domain.TopFinding(witness.Findings)
	if !ok || top.Code != "sqs.public-policy" {
		t.Fatalf("witness %s top finding = %q, want sqs.public-policy", witness.ID, top.Code)
	}
	cell, ok := listStatusCellFor(t, *td, rows, witness.ID)
	if !ok {
		t.Fatalf("no list row for witness %s", witness.ID)
	}
	if got, _, _ := strings.Cut(cell, " (+"); got != "queue policy open to anyone" {
		t.Errorf("witness %s Status cell = %q, want %q", witness.ID, cell, "queue policy open to anyone")
	}
	if got, want := td.ResolveColor(*witness), w5ColorOfSeverity(top.Severity); got != want {
		t.Errorf("witness %s colour = %v, want %v (the colour of its top finding)", witness.ID, got, want)
	}
}

// Row colour and Status cell both come from the row's top finding.
func TestPolicyTypesColourAndStatusSelectTheSameFinding(t *testing.T) {
	multi := 0
	for short, rows := range policyBench(t) {
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
				t.Errorf("%s/%s: status cell names %q, colour's top finding is %q", short, r.ID, got, top.Phrase)
			}
		}
	}
	if multi == 0 {
		t.Error("no policy-reading demo row carries more than one finding; the selector check above cannot fail")
	}
}

// A supporting row adds what its phrase does not say, and says it in words:
// no row normalises to the phrase, and no value is a Go bool literal or SDK
// enum casing (a trailing parenthesised aside is stripped first).
func TestPolicyTypesAttentionRowsAddWordsNotRepeats(t *testing.T) {
	for short, rows := range policyBench(t) {
		for _, r := range rows {
			for _, f := range r.Findings {
				ad, ok := r.AttentionDetails[f.Code]
				if !ok {
					continue
				}
				phrase := w5Normalize(f.Phrase)
				for _, row := range ad.Rows {
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
}
