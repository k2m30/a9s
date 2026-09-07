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

// w5HasKeywordHit reports whether any hit came from the keyword rule, as
// opposed to a structural match on the value itself.
func w5HasKeywordHit(hits []secretscan.Hit) bool {
	for _, h := range hits {
		if h.Kind == secretscan.KindKeyword {
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

// A Secrets Manager ARN contains the literal "secret:" followed by the
// secret's own name, so a keyword match lands INSIDE the ARN and would be
// judged on its tail ("acme/db-AbCdEf") rather than on the reference it came
// from. The scanner blanks every ARN before the keyword pass for exactly
// this reason, and an ARN naming where a secret lives is the fix rather than
// the leak.
func TestW5SecretScan_SecretsManagerARNIsNotALeak(t *testing.T) {
	const line = `{"SecretId": "arn:aws:secretsmanager:us-east-1:123456789012:secret:acme/db-AbCdEf"}`
	if hits := secretscan.ScanText(line); w5HasKeywordHit(hits) {
		t.Errorf("reported %+v; an ARN naming where the secret lives is the fix, not the leak — the match lands inside the ARN, so the reference guard never sees it", hits)
	}
}

// A value beginning with "$" is now treated as an environment reference and
// never reported. That is right for $ACME_API_KEY and wrong for every hash
// format whose own syntax starts with "$": bcrypt writes $2y$, $2a$, $2b$,
// and crypt writes $6$ and $argon2id$. Those are credentials, not
// references, and the plain KEY=value form they arrive in is the shape the
// scanner's own doc comment names as its primary case.
//
// This is a regression, not a gap. All three lines below were reported by
// the pattern that predates this batch entirely, verified by running the
// original, the widened and the current pattern side by side with each
// version's own reference list. A scanner that stops reporting a leak is
// worse than one that reports a reference: the false positive is argued
// with, the false negative is never seen.
func TestW5SecretScan_DollarLeadingValuesAreStillLeaks(t *testing.T) {
	cases := map[string]string{
		"bcrypt hash in an environment variable":  `DB_PASSWORD=$2y$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy`,
		"crypt sha-512 hash":                      `password=$6$rounds=656000$YQ8kEpHkQ1nT2mBv$abcdefghijklmnopqrstuvwx`,
		"literal value that starts with a dollar": `api_key=$LITERAL-SECRET-9f8e7d6c5b`,
	}

	for name, text := range cases {
		t.Run(name, func(t *testing.T) {
			if hits := secretscan.ScanText(text); !w5HasKeywordHit(hits) {
				t.Errorf("no keyword hit for %q; a leading dollar makes this a reference only when what follows is an identifier — a hash whose own syntax starts with $ is the credential itself", text)
			}
		})
	}
}

// The counterpart that must keep working, so the fix above cannot be made by
// simply reverting the reference rule.
func TestW5SecretScan_EnvironmentReferencesStayQuiet(t *testing.T) {
	cases := map[string]string{
		"bare environment reference":   `{"API_KEY": "$ACME_API_KEY"}`,
		"braced reference":             `{"api_key": "${var.acme_api_key}"}`,
		"shell-style braced reference": `API_KEY=${ACME_API_KEY}`,
	}
	for name, text := range cases {
		t.Run(name, func(t *testing.T) {
			if hits := secretscan.ScanText(text); w5HasKeywordHit(hits) {
				t.Errorf("reported %+v; this names where the secret lives", hits)
			}
		})
	}
}

// The ARN blanking must not swallow a real credential that shares a line
// with an ARN. Blanking runs before the keyword pass, so anything the ARN
// pattern over-matches is invisible to the scan that follows.
func TestW5SecretScan_ARNAdjacentCredentialIsStillFound(t *testing.T) {
	cases := map[string]string{
		"reference and inline secret on one line": `{"SecretId": "arn:aws:secretsmanager:us-east-1:123456789012:secret:acme/db-AbCdEf", "DB_PASSWORD": "hunter2-correct-horse-battery"}`,
		"secret before the arn":                   `{"DB_PASSWORD": "hunter2-correct-horse-battery", "role": "arn:aws:iam::123456789012:role/acme-task"}`,
	}
	for name, text := range cases {
		t.Run(name, func(t *testing.T) {
			if hits := secretscan.ScanText(text); !w5HasKeywordHit(hits) {
				t.Errorf("no keyword hit; the inline credential shares a line with an ARN, and blanking the ARN must not reach past it. got %+v", hits)
			}
		})
	}
}

// An AWS access key sharing a line with an ARN is matched by its own
// structural pattern, which runs over the untouched line. Pinned because the
// blanking is one edit away from being applied to every pattern.
func TestW5SecretScan_ARNAdjacentAccessKeyIsStillFound(t *testing.T) {
	const line = `{"role": "arn:aws:iam::123456789012:role/acme-task", "key": "AKIAIOSFODNN7EXAMPLE"}`
	var found bool
	for _, h := range secretscan.ScanText(line) {
		if h.Kind == "aws-access-key" {
			found = true
		}
	}
	if !found {
		t.Errorf("no aws-access-key hit; the structural patterns run over the line before any ARN is blanked. got %+v", secretscan.ScanText(line))
	}
}

// The reference rule's boundary, both sides of it in one table. A bare
// dollar value names where a secret lives only when what follows is an
// identifier and nothing else; anything that continues past the identifier,
// or does not start one, is the credential itself.
//
// The cases are chosen so the rule cannot be satisfied by a prefix test, by
// a first-character test, or by a list of known hash formats: an identifier
// followed by a hyphen starts valid and fails later, a digit fails
// immediately, and argon2 starts with letters and fails at its separator.
func TestW5SecretScan_DollarReferenceBoundary(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantHit bool
		why     string
	}{
		{"identifier", "$ACME_API_KEY", false, "a name for a secret held elsewhere"},
		{"identifier with a leading underscore", "$_ACME_KEY", false, "still an identifier"},
		{"identifier with digits", "$ACME_KEY_V2", false, "still an identifier"},
		{"identifier then a hyphen", "$LITERAL-SECRET-9f8e7d6c5b", true, "starts as a name and keeps going, so it is a value"},
		{"digit straight after the dollar", "$2y$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad", true, "bcrypt, a credential"},
		{"argon2 identifier then a separator", "$argon2id$v=19$m=65536,t=3,p=4$c29tZXNhbHQ", true, "argon2, a credential"},
		{"braced reference", "${ACME_API_KEY}", false, "shell interpolation"},
		{"braced reference with a default", "${ACME_API_KEY:-fallback}", false, "shell interpolation with a default"},
		{"braced terraform reference", "${var.acme_api_key}", false, "terraform interpolation"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			line := "DB_PASSWORD=" + tc.value
			got := w5HasKeywordHit(secretscan.ScanText(line))
			if got != tc.wantHit {
				t.Errorf("ScanText(%q) reported=%v, want %v — %s", line, got, tc.wantHit, tc.why)
			}
		})
	}
}
