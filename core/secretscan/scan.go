// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// Package secretscan is the single source of truth for detecting plaintext
// credentials in the places AWS lets operators paste them: environment-style
// key/value maps and free text such as user data, templates and state-machine
// definitions. Hits name where a secret sits, never the secret itself.
package secretscan

import (
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// The kinds a Hit can carry, strongest first: a structural match names the
// credential format it recognised, and everything else is the keyword rule.
// One place names a secret once, by the reason that identifies it.
const (
	KindAWSAccessKey = "aws-access-key"
	KindPrivateKey   = "private-key"
	KindJWT          = "jwt"
	KindKeyword      = "keyword"
)

var kinds = []string{KindAWSAccessKey, KindPrivateKey, KindJWT, KindKeyword}

// IsKind reports whether v is one of the kinds a Hit carries. Callers that
// need the set read it here rather than mirroring it.
func IsKind(v string) bool { return slices.Contains(kinds, v) }

// Hit is one detected secret. Each scanned key, and each scanned line,
// yields at most one.
type Hit struct {
	Kind  string // one of the Kind constants above
	Where string // the key name (ScanKV) or "line <n>" (ScanText); never the value
}

var (
	awsKeyRe     = regexp.MustCompile(`\b(AKIA|ASIA)[0-9A-Z]{16}\b`)
	privateKeyRe = regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`)
	jwtRe        = regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\b`)
	// keywordRe: a credential word, optionally suffixed (SECRET_KEY, DB_PASSWORD_V2),
	// followed by = or : and a value of 6+ characters. Word boundaries would
	// reject the underscore-joined names that are the common case. Either
	// side may be quoted, independently: `"password": hunter2` is valid YAML
	// and a leak an operator reading the file would see.
	//
	// What keeps prose out is where the key sits, not how the value is
	// written. A quoted key is a key when it opens a line or follows a
	// structural character — `{`, `[`, `,`, a list dash — and is an English
	// quotation anywhere else, which is why `The "password": rotate it every
	// ninety days` is a sentence rather than a mapping.
	keywordRe = regexp.MustCompile(`(?i)(?:(?:^|[{\[,\-])\s*["']?|[^a-z"'])(?:password|passwd|pwd|secret|token|api[_-]?key|apikey|private[_-]?key|access[_-]?key|client[_-]?secret)(?:[_-][a-z0-9_-]*)?["']?\s*[=:]\s*["']?([^\s"',;]{6,})`)
	// arnRe matches any ARN. A Secrets Manager ARN carries the literal
	// "secret:" followed by the secret's own name, so a keyword match lands
	// INSIDE the ARN and the reference guard below only ever sees the tail.
	// Blanking ARNs before the keyword pass is what lets isRealValue see the
	// reference rather than its last segment.
	arnRe = regexp.MustCompile(`arn:aws[a-z-]*:[^\s"',;]+`)
	// envRefRe matches a bare environment reference and nothing else: a
	// dollar followed by an identifier, all the way to the end. Every hash
	// format whose own syntax opens with a dollar — bcrypt's $2y$, crypt's
	// $6$, $argon2id$ — fails it on the first character or the first
	// separator, and those are the credential itself, not a name for one.
	envRefRe = regexp.MustCompile(`^\$[A-Za-z_]\w*$`)
	// userinfoRe: scheme://user:password@host — the password is group 1.
	userinfoRe = regexp.MustCompile(`[A-Za-z][A-Za-z0-9+.-]*://[^\s/:@]+:([^\s/@]+)@`)
	kvKeyRe    = regexp.MustCompile(`(?i)(secret|passw(or)?d|passwd|token|api[_-]?key|apikey|private[_-]?key|access[_-]?key|client[_-]?secret|credential|auth[_-]?token|db[_-]?pass)`)
)

// ScanKV inspects a name→value map. A credential-named key with a real value
// is a keyword hit; every value is also scanned as text. Hits come back
// sorted by key, one per key.
//
// A value that is wholly a reference or a placeholder is skipped outright.
// Scanning inside one is what turns the RECOMMENDED shape into a false
// positive: "arn:aws:secretsmanager:…:secret:acme/db-AbCdEf" contains the
// literal "secret:" followed by a plausible value, so a per-line keyword
// scan reports the very indirection that fixes the leak.
func ScanKV(kv map[string]string) []Hit {
	var hits []Hit
	for key, value := range kv {
		if !isRealValue(value) {
			continue
		}
		kind := scanValue(value)
		if kind == "" && kvKeyRe.MatchString(key) {
			kind = KindKeyword
		}
		if kind != "" {
			hits = append(hits, Hit{Kind: kind, Where: key})
		}
	}
	slices.SortFunc(hits, func(a, b Hit) int { return strings.Compare(a.Where, b.Where) })
	return hits
}

// ScanText inspects free text line by line, one hit per line, in line order.
func ScanText(text string) []Hit {
	var hits []Hit
	for i, line := range strings.Split(text, "\n") {
		if kind := scanValue(line); kind != "" {
			hits = append(hits, Hit{Kind: kind, Where: "line " + strconv.Itoa(i+1)})
		}
	}
	return hits
}

// scanValue returns the kind found in one value or line, or "" for none. A
// structured credential (access key, PEM, JWT) beats the keyword match on the
// same text, so a secret is named by its most specific kind and named once.
func scanValue(s string) string {
	switch {
	case awsKeyRe.MatchString(s):
		return KindAWSAccessKey
	case privateKeyRe.MatchString(s):
		return KindPrivateKey
	case jwtRe.MatchString(s):
		return KindJWT
	}
	// An ARN is a reference, never a value, and a keyword match landing
	// inside one would be judged on its last segment alone.
	for _, m := range keywordRe.FindAllStringSubmatch(arnRe.ReplaceAllString(s, " "), -1) {
		if isRealValue(m[1]) {
			return KindKeyword
		}
	}
	for _, m := range userinfoRe.FindAllStringSubmatch(s, -1) {
		if isRealValue(m[1]) {
			return KindKeyword
		}
	}
	return ""
}

// isRealValue rejects placeholders and references: anything that resolves
// a secret elsewhere is a good practice, not a leak.
func isRealValue(v string) bool {
	v = strings.TrimSpace(v)
	if len(v) < 6 {
		return false
	}
	switch strings.ToLower(v) {
	case "true", "false", "null", "none", "changeme", "redacted":
		return false
	}
	if strings.Trim(v, "*xX#-") == "" {
		return false
	}
	lower := strings.ToLower(v)
	if envRefRe.MatchString(v) {
		return false
	}
	for _, p := range []string{"arn:", "${", "{{", "/", "ssm:", "secretsmanager:"} {
		if strings.HasPrefix(lower, p) {
			return false
		}
	}
	if strings.Contains(lower, "{{resolve:") {
		return false
	}
	if u, err := url.Parse(v); err == nil && u.Scheme != "" && u.Host != "" && u.User == nil {
		return false
	}
	return true
}

// Redact masks a value for display: the first and last two characters
// around an ellipsis, or just the ellipsis for values under 8 characters.
func Redact(value string) string {
	r := []rune(value)
	if len(r) < 8 {
		return "…"
	}
	return string(r[:2]) + "…" + string(r[len(r)-2:])
}
