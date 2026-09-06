package unit

// prowler_w5_secretscan_json_key_test.go — the shared secret scanner gained
// an optional quote before the separator so a credential written as a JSON
// object key matches. That change is in the one engine every caller shares,
// not at the Step Functions call site, so its blast radius is every caller.
//
// The scanner's own comment states the rule the widening could break: a
// value that resolves a secret elsewhere is good practice, not a leak, and
// reporting the indirection tells an operator their correct code is wrong.
// A Secrets Manager reference in JSON is now one optional quote closer to
// matching than it was, so the cases below are deliberately NOT state
// machine definitions.

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/secretscan"
)

// w5HasKeywordHit reports whether any hit is a keyword-class hit.
func w5HasKeywordHit(hits []secretscan.Hit) bool {
	for _, h := range hits {
		if h.Kind == "keyword" || h.Kind == "high-entropy" {
			return true
		}
	}
	return false
}

// The shape the widening exists to catch, stated once outside Step
// Functions: a credential inlined as a JSON object key. A task definition,
// a template parameter block and a plain config file all take this form.
func TestW5SecretScan_JSONObjectKeyIsAHit(t *testing.T) {
	cases := map[string]string{
		"task definition environment": `{
  "containerDefinitions": [
    {
      "name": "acme-api",
      "environment": [
        {"name": "PORT", "value": "8080"}
      ],
      "DB_PASSWORD": "hunter2-correct-horse-battery"
    }
  ]
}`,
		"single-quoted key":        `{'API_KEY': 'ak-live-9f8e7d6c5b4a32109876'}`,
		"no space after the colon": `{"client_secret":"cs-7f3a9b2e4d6c8a1f0e5b"}`,
	}

	for name, text := range cases {
		t.Run(name, func(t *testing.T) {
			hits := secretscan.ScanText(text)
			if !w5HasKeywordHit(hits) {
				t.Errorf("no keyword hit; a credential written as a JSON object key is the shape this pattern was widened for. got %+v", hits)
			}
			for _, h := range hits {
				if strings.Contains(h.Where, "hunter2") || strings.Contains(h.Where, "ak-live") {
					t.Errorf("Where = %q carries the value; it must carry the location only", h.Where)
				}
			}
		})
	}
}

// The regression the widening caused. Both of these were silent under the
// previous pattern and report a keyword hit under the new one, verified by
// running the two patterns side by side rather than inferred: the keyword
// used to have to be followed immediately by the separator, so a quote
// between them ended the match, and now the quote is consumed and the scan
// runs on to the next plausible value.
//
// Both RESOLVE a secret from somewhere else or are not a value at all, which
// is exactly the practice the scanner exists to encourage.
func TestW5SecretScan_ReferenceShapesAreStillNotHits(t *testing.T) {
	cases := map[string]string{
		"parameter store reference":        `{"DB_PASSWORD": "{{resolve:ssm-secure:/acme/db/password:1}}"}`,
		"cloudformation dynamic reference": `{"client_secret": "{{resolve:secretsmanager:acme/oauth:SecretString:client_secret}}"}`,
		"terraform interpolation":          `{"api_key": "${var.acme_api_key}"}`,
		"environment indirection":          `{"API_KEY": "$ACME_API_KEY"}`,
		"redacted in a log line":           `{"password": "REDACTED"}`,
		"masked value":                     `{"access_key": "****************"}`,
		"placeholder":                      `{"client_secret": "CHANGEME"}`,
	}

	for name, text := range cases {
		t.Run(name, func(t *testing.T) {
			if hits := secretscan.ScanText(text); w5HasKeywordHit(hits) {
				t.Errorf("reported %+v; this value resolves the secret elsewhere, which is the practice the scanner exists to encourage — reporting it tells an operator their correct code is a leak", hits)
			}
		})
	}
}

// ScanKV is the other entry point and the one every environment-variable
// caller uses. The widening touched a pattern ScanKV reaches through
// scanValue, so the same reference shapes must stay silent there.
func TestW5SecretScan_ScanKVReferenceShapesAreStillNotHits(t *testing.T) {
	kv := map[string]string{
		"DB_PASSWORD_ARN":  "arn:aws:secretsmanager:us-east-1:123456789012:secret:acme/db-AbCdEf",
		"API_KEY_SSM_PATH": "/acme/prod/api-key",
		"PORT":             "8080",
		"LOG_LEVEL":        "info",
	}
	for _, h := range secretscan.ScanKV(kv) {
		t.Errorf("ScanKV reported %+v; every value here names where the secret lives rather than carrying one", h)
	}
}

// A credential-named key whose value really is a credential must still be a
// hit through ScanKV — the negative test above passes trivially if the
// scanner has stopped reporting anything at all.
func TestW5SecretScan_ScanKVStillReportsARealValue(t *testing.T) {
	kv := map[string]string{"DB_PASSWORD": "hunter2-correct-horse-battery"}
	hits := secretscan.ScanKV(kv)
	if !w5HasKeywordHit(hits) {
		t.Fatalf("no keyword hit for a real inline credential; got %+v", hits)
	}
	for _, h := range hits {
		if h.Where != "DB_PASSWORD" {
			t.Errorf("Where = %q, want the key name", h.Where)
		}
		if strings.Contains(h.Where, "hunter2") {
			t.Errorf("Where = %q carries the value", h.Where)
		}
	}
}

// An ordinary sentence that happens to contain a credential word followed
// by punctuation and a long word must not become a hit. Prose reaches the
// scanner through log lines and description fields, and the widened pattern
// now accepts a quote between the word and the separator.
func TestW5SecretScan_ProseIsNotAHit(t *testing.T) {
	cases := map[string]string{
		"quoted word then colon": `The "password": rotate it every ninety days per policy.`,
		"key name in prose":      `Set the "api_key" header on every request to the partner endpoint.`,
	}
	for name, text := range cases {
		t.Run(name, func(t *testing.T) {
			if hits := secretscan.ScanText(text); w5HasKeywordHit(hits) {
				t.Errorf("prose reported %+v; a sentence mentioning a credential word is not a leak", hits)
			}
		})
	}
}

// A pre-existing false positive, kept separate so it is not read as this
// batch's doing: BOTH the previous pattern and the widened one report this,
// verified by running them side by side. A Secrets Manager ARN contains the
// literal "secret:" followed by the secret's own name, so the match lands
// INSIDE the ARN and isRealValue only ever sees the tail ("acme/db-AbCdEf")
// rather than the reference it came from.
//
// The scanner's own doc comment names this exact string as the shape that
// must not be reported, so the intent is settled and only the code disagrees.
func TestW5SecretScan_SecretsManagerARNIsNotALeak(t *testing.T) {
	const line = `{"SecretId": "arn:aws:secretsmanager:us-east-1:123456789012:secret:acme/db-AbCdEf"}`
	if hits := secretscan.ScanText(line); w5HasKeywordHit(hits) {
		t.Errorf("reported %+v; an ARN naming where the secret lives is the fix, not the leak — the match lands inside the ARN, so the reference guard never sees it", hits)
	}
}
