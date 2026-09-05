// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// Package secretscan is the single source of truth for detecting plaintext
// credentials in the places AWS lets operators paste them: environment-style
// key/value maps and free text such as user data, templates and state-machine
// definitions. Hits name where a secret sits, never the secret itself.
package secretscan

import (
	"math"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Hit is one detected secret.
type Hit struct {
	Kind  string // "aws-access-key" | "private-key" | "jwt" | "keyword" | "high-entropy"
	Where string // the key name (ScanKV) or "line <n>" (ScanText); never the value
}

var (
	awsKeyRe     = regexp.MustCompile(`\b(AKIA|ASIA)[0-9A-Z]{16}\b`)
	privateKeyRe = regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`)
	jwtRe        = regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\b`)
	// keywordRe: a credential word, optionally suffixed (SECRET_KEY, DB_PASSWORD_V2),
	// followed by = or : and a value of 6+ characters. Word boundaries would
	// reject the underscore-joined names that are the common case.
	keywordRe = regexp.MustCompile(`(?i)(?:^|[^a-z])(?:password|passwd|pwd|secret|token|api[_-]?key|apikey|private[_-]?key|access[_-]?key|client[_-]?secret)(?:[_-][a-z0-9_-]*)?\s*[=:]\s*["']?([^\s"',;]{6,})`)
	// userinfoRe: scheme://user:password@host — the password is group 1.
	userinfoRe = regexp.MustCompile(`[A-Za-z][A-Za-z0-9+.-]*://[^\s/:@]+:([^\s/@]+)@`)
	kvKeyRe    = regexp.MustCompile(`(?i)(secret|passw(or)?d|passwd|token|api[_-]?key|apikey|private[_-]?key|access[_-]?key|client[_-]?secret|credential|auth[_-]?token|db[_-]?pass)`)
)

// ScanKV inspects a name→value map. A credential-named key with a real value
// is a keyword hit; every value is also scanned as text.
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
		kinds := scanValue(value)
		if len(kinds) == 0 && kvKeyRe.MatchString(key) {
			kinds = append(kinds, "keyword")
			if isHighEntropy(value) {
				kinds = append(kinds, "high-entropy")
			}
		}
		for _, k := range kinds {
			hits = append(hits, Hit{Kind: k, Where: key})
		}
	}
	return finish(hits, func(h Hit) string { return h.Where })
}

// ScanText inspects free text line by line.
func ScanText(text string) []Hit {
	var hits []Hit
	for i, line := range strings.Split(text, "\n") {
		for _, k := range scanValue(line) {
			hits = append(hits, Hit{Kind: k, Where: "line " + strconv.Itoa(i+1)})
		}
	}
	return finish(hits, func(h Hit) string {
		n, _ := strconv.Atoi(strings.TrimPrefix(h.Where, "line "))
		return strconv.Itoa(n + 1_000_000)
	})
}

// scanValue returns the de-duplicated kinds found in one value or line.
// A structured credential (access key, PEM, JWT) beats the keyword match on
// the same value so a hit is reported once, by its most specific kind.
func scanValue(s string) []string {
	var kinds []string
	if awsKeyRe.MatchString(s) {
		kinds = append(kinds, "aws-access-key")
	}
	if privateKeyRe.MatchString(s) {
		kinds = append(kinds, "private-key")
	}
	if jwtRe.MatchString(s) {
		kinds = append(kinds, "jwt")
	}
	if len(kinds) > 0 {
		return kinds
	}
	for _, m := range keywordRe.FindAllStringSubmatch(s, -1) {
		if isRealValue(m[1]) {
			kinds = append(kinds, "keyword")
			if isHighEntropy(m[1]) {
				kinds = append(kinds, "high-entropy")
			}
		}
	}
	for _, m := range userinfoRe.FindAllStringSubmatch(s, -1) {
		if isRealValue(m[1]) {
			kinds = append(kinds, "keyword")
		}
	}
	slices.Sort(kinds)
	return slices.Compact(kinds)
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

// isHighEntropy flags a long base64/hex-looking value with Shannon entropy
// above 3.5 bits per character, the usual signature of a generated token.
func isHighEntropy(v string) bool {
	if len(v) < 20 {
		return false
	}
	freq := map[rune]float64{}
	for _, r := range v {
		switch {
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '+', r == '/', r == '=', r == '_', r == '-':
			freq[r]++
		default:
			return false
		}
	}
	n := float64(len(v))
	var h float64
	for _, c := range freq {
		p := c / n
		h -= p * math.Log2(p)
	}
	return h > 3.5
}

func finish(hits []Hit, order func(Hit) string) []Hit {
	if len(hits) == 0 {
		return nil
	}
	slices.SortFunc(hits, func(a, b Hit) int {
		if c := strings.Compare(order(a), order(b)); c != 0 {
			return c
		}
		return strings.Compare(a.Kind, b.Kind)
	})
	return slices.Compact(hits)
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
