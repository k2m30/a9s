package unit

// scan_secretscan_one_hit_test.go — the scanner reports each key once, and
// misses nothing a YAML reader would see.
//
// Two properties, both inside the one engine every caller shares:
//
//   - One key, one hit. A generated credential under a credential-named key
//     answers more than one of the scanner's rules at once, and it is still
//     one credential. Two hits for one key means every caller renders a
//     near-identical supporting row pair for a single leak.
//   - The key match does not depend on how the value is quoted. A quoted key
//     with a bare value is valid YAML and a real shape in task parameters
//     and CloudFormation templates; skipping it hides a leak an operator
//     reading the same file would see.
//
// The caller half is here too: a stage carrying two leaking variables is one
// finding with two supporting rows, not two findings. Emitting inside the
// per-hit loop appends the finding and its Stage row again for every hit, and
// the second call's AttentionDetail overwrites the first's, so the extra
// finding is paid for by losing a hit row.

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	apigwtypes "github.com/aws/aws-sdk-go-v2/service/apigateway/types"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/secretscan"
)

// A Lambda-style environment value that is credential-named and looks
// generated, so more than one of the scanner's rules answers for it.
const scanGeneratedSecret = "OhbVrpoiVgRV5IfLBcbfnoGMbJmTPS"

func TestScanKV_GeneratedValueUnderCredentialKey_IsOneHit(t *testing.T) {
	hits := secretscan.ScanKV(map[string]string{
		"DB_PASSWORD": scanGeneratedSecret,
		"AWS_REGION":  "eu-central-1",
	})
	want := []secretscan.Hit{{Kind: "keyword", Where: "DB_PASSWORD"}}
	if !reflect.DeepEqual(hits, want) {
		t.Errorf("ScanKV() = %v, want %v — one key is one hit, reported by the reason that names it", hits, want)
	}
}

// Two leaking keys are two leaks: the per-key collapse must not collapse
// across keys.
func TestScanKV_TwoCredentialKeys_AreTwoHits(t *testing.T) {
	hits := secretscan.ScanKV(map[string]string{
		"DB_PASSWORD": scanGeneratedSecret,
		"API_KEY":     "wRPbNzKcTdXmQvJhSyLgEuFaHi",
		"LOG_LEVEL":   "info",
		"DB_ENDPOINT": "acme-primary.cluster-abc123.eu-central-1.rds.amazonaws.com",
		"SECRET_ARN":  "arn:aws:secretsmanager:eu-central-1:123456789012:secret:acme/db-AbCdEf",
	})
	want := []secretscan.Hit{
		{Kind: "keyword", Where: "API_KEY"},
		{Kind: "keyword", Where: "DB_PASSWORD"},
	}
	if !reflect.DeepEqual(hits, want) {
		t.Errorf("ScanKV() = %v, want %v", hits, want)
	}
}

// scanQuotingDoc carries every shape a credential is written in inside the
// documents that reach ScanText — user data, a CloudFormation template, a
// task-definition fragment — followed by the reference shapes that are the
// fix rather than the leak.
const scanQuotingDoc = `# acme-api deployment parameters
DB_PASSWORD: Tr0ub4dor&3xample
"API_KEY": "s3cr3t&value!42"
"CLIENT_SECRET": Tr0ub4dor&3xample
DB_PASSWD: "Tr0ub4dor&3xample"
{"access_key": "Tr0ub4dor&3xample"}
- "password": Tr0ub4dor&3xample
SECRET_ARN: arn:aws:secretsmanager:eu-central-1:123456789012:secret:acme/db-AbCdEf
TOKEN_REF: $DEPLOY_TOKEN
EMPTY_PASSWORD: ""`

func TestScanText_EveryQuotingShapeIsReported(t *testing.T) {
	hits := secretscan.ScanText(scanQuotingDoc)
	want := []secretscan.Hit{
		{Kind: "keyword", Where: "line 2"}, // bare key, bare value
		{Kind: "keyword", Where: "line 3"}, // quoted key, quoted value
		{Kind: "keyword", Where: "line 4"}, // quoted key, bare value
		{Kind: "keyword", Where: "line 5"}, // bare key, quoted value
		{Kind: "keyword", Where: "line 6"}, // JSON object key
		{Kind: "keyword", Where: "line 7"}, // list item
	}
	if !reflect.DeepEqual(hits, want) {
		t.Errorf("ScanText(quoting doc) = %v, want %v\n"+
			"lines 2-7 are the four quoting shapes plus JSON and a list item, all leaks;\n"+
			"lines 8-10 are an ARN, an environment reference and a blank, all references or nothing",
			hits, want)
	}
}

func TestW6AAPIGWStageVariableSecret_TwoLeakingVariablesAreOneFinding(t *testing.T) {
	const apiID = "rst004secret"

	leaky := w6aV1Stage("prod")
	leaky.Variables = map[string]string{
		"backendStage": "prod",
		"DB_PASSWORD":  "hunter2-not-a-real-secret",
		"API_KEY":      "s3cr3t&value!42",
	}

	res := w6aEnrichAPIGW(t,
		&w6aAPIGWV1Fake{
			authorizers: []apigwtypes.Authorizer{{Id: aws.String("auth01")}},
			stages:      []apigwtypes.Stage{leaky},
		},
		&w6aAPIGWV2Fake{},
		w6aRESTRes(apiID, "acme-leaky-rest", "REGIONAL", ""),
	)

	var carried int
	for _, f := range res.Findings[apiID] {
		if f.Code == w6aAPIGWStageSecret {
			carried++
		}
	}
	if carried != 1 {
		t.Errorf("stage %q carries %d %q findings, want 1 — one stage is one finding however many of its variables leak",
			"prod", carried, w6aAPIGWStageSecret)
	}

	rows := w2Rows(t, res, apiID, w6aAPIGWStageSecret)
	var stageRows int
	seen := map[string]string{}
	for _, r := range rows {
		if r.Label == "Stage" {
			stageRows++
			continue
		}
		seen[r.Label] = r.Value
	}
	if stageRows != 1 {
		t.Errorf("rows %+v carry %d Stage rows, want 1", rows, stageRows)
	}
	for _, key := range []string{"API_KEY", "DB_PASSWORD"} {
		if got := seen[key]; got != "keyword" {
			t.Errorf("rows %+v: row for %q = %q, want %q — both leaking variables must reach the detail view", rows, key, got, "keyword")
		}
	}
	if len(rows) != 3 {
		t.Errorf("rows %+v = %d rows, want 3 (one Stage row and one row per leaking variable)", rows, len(rows))
	}
}

// The negative for the same stage: a single leaking variable is still one
// finding with one hit row, so the pin above cannot pass by emitting
// everything twice.
func TestW6AAPIGWStageVariableSecret_OneLeakingVariableIsOneRow(t *testing.T) {
	const apiID = "rst005secret"

	leaky := w6aV1Stage("prod")
	leaky.Variables = map[string]string{
		"backendStage": "prod",
		"DB_PASSWORD":  "hunter2-not-a-real-secret",
	}

	res := w6aEnrichAPIGW(t,
		&w6aAPIGWV1Fake{
			authorizers: []apigwtypes.Authorizer{{Id: aws.String("auth01")}},
			stages:      []apigwtypes.Stage{leaky},
		},
		&w6aAPIGWV2Fake{},
		w6aRESTRes(apiID, "acme-leaky-rest", "REGIONAL", ""),
	)

	rows := w2Rows(t, res, apiID, w6aAPIGWStageSecret)
	want := []domain.DetailRow{
		{Label: "Stage", Value: "prod", Tier: "!"},
		{Label: "DB_PASSWORD", Value: "keyword", Tier: "!"},
	}
	if len(rows) != len(want) {
		t.Fatalf("rows = %+v, want %+v", rows, want)
	}
	for i := range want {
		if rows[i] != want[i] {
			t.Errorf("rows[%d] = %+v, want %+v", i, rows[i], want[i])
		}
	}
}

// The four callers that copied the scan-and-emit shape must reach the
// scanner through the shared helper instead. A caller holding its own
// secretscan call is free to build the finding its own way, which is how
// apigw came to emit per hit and cfn came to scan the same outputs twice.
func TestSecretScanEmissionLivesInOnePlace(t *testing.T) {
	for _, path := range []string{
		"../../core/aws/lambda.go",
		"../../core/aws/ecs_task_issue_enrichment.go",
		"../../core/aws/cfn.go",
		"../../core/aws/apigw_issue_enrichment.go",
	} {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if n := strings.Count(string(src), "secretscan."); n > 0 {
			t.Errorf("%s references secretscan %d time(s); the scan and the finding-plus-rows emission belong to the shared helper, not to each caller", path, n)
		}
	}
}
