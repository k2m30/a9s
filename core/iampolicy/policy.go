// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// Package iampolicy is the single source of truth for reading an IAM or
// resource policy document and answering the questions the security views
// ask of it: is it public, is it scoped by a condition, which foreign
// accounts it trusts, which actions it effectively allows, whether it is an
// admin or privilege-escalation policy, and which service principals it
// trusts without confused-deputy scoping.
package iampolicy

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// Principal is the parsed Principal block of a statement. A NotPrincipal
// block names who is excluded, not who is granted, so its entries are never
// stored here; the statement only records that it used one.
type Principal struct {
	Wildcard      bool
	AWS           []string
	Service       []string
	Federated     []string
	CanonicalUser []string
}

// Statement is one entry of a policy's Statement block.
type Statement struct {
	Sid          string
	Effect       string
	Principal    Principal
	NotPrincipal bool
	Action       []string
	NotAction    []string
	Resource     []string
	NotResource  []string
	// Condition is operator -> key -> values. Keys keep their original case;
	// compare with strings.EqualFold.
	Condition map[string]map[string][]string
}

// Document is a parsed policy.
type Document struct {
	Version   string
	Statement []Statement
}

// Decode returns the text of a policy document as the IAM APIs hand it over.
// A document that already is JSON is used as it stands, so a literal '%'
// inside a string value survives; only a document that is not JSON is
// unescaped, once, path-style — a literal '+' inside a policy (a regex, a
// resource name) must survive, which query-style unescaping would turn into a
// space. A document that does not unescape comes back unchanged so the
// caller's JSON parse reports the real problem rather than this function
// inventing one.
func Decode(doc string) string {
	if json.Valid([]byte(doc)) || !strings.Contains(doc, "%") {
		return doc
	}
	if decoded, err := url.PathUnescape(doc); err == nil {
		return decoded
	}
	return doc
}

// Parse reads a policy document, encoded or not — see Decode for the one
// unescape rule. Unrecognised shapes inside the document are skipped, never
// fatal.
func Parse(doc string) (Document, error) {
	doc = strings.TrimSpace(doc)
	if doc == "" {
		return Document{}, errors.New("iampolicy: empty document")
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(Decode(doc)), &raw); err != nil {
		return Document{}, fmt.Errorf("iampolicy: %w", err)
	}
	d := Document{Version: str(raw["Version"])}
	for _, s := range objects(raw["Statement"]) {
		d.Statement = append(d.Statement, parseStatement(s))
	}
	return d, nil
}

func parseStatement(s map[string]any) Statement {
	st := Statement{
		Sid:         str(s["Sid"]),
		Effect:      str(s["Effect"]),
		Action:      strs(s["Action"]),
		NotAction:   strs(s["NotAction"]),
		Resource:    strs(s["Resource"]),
		NotResource: strs(s["NotResource"]),
		Condition:   parseCondition(s["Condition"]),
	}
	_, st.NotPrincipal = s["NotPrincipal"]
	st.Principal = parsePrincipal(s["Principal"])
	return st
}

func parsePrincipal(v any) Principal {
	var p Principal
	switch pv := v.(type) {
	case string:
		p.Wildcard = pv == "*"
	case map[string]any:
		for k, entries := range pv {
			for _, e := range strs(entries) {
				switch {
				case strings.EqualFold(k, "AWS"):
					if isAnyoneARN(e) {
						p.Wildcard = true
					} else {
						p.AWS = append(p.AWS, e)
					}
				case strings.EqualFold(k, "Service"):
					p.Service = append(p.Service, e)
				case strings.EqualFold(k, "Federated"):
					p.Federated = append(p.Federated, e)
				case strings.EqualFold(k, "CanonicalUser"):
					if e == "*" {
						p.Wildcard = true
					} else {
						p.CanonicalUser = append(p.CanonicalUser, e)
					}
				}
			}
		}
	}
	return p
}

// isAnyoneARN mirrors Prowler's has_public_principal: an IAM ARN whose
// account is "*" and whose resource is "root" is every account's root user,
// which is as public as "*". Every partition names its own ARNs, so the
// fields are parsed rather than the commercial spelling compared.
func isAnyoneARN(s string) bool {
	if s == "*" {
		return true
	}
	f, ok := arnFields(s)
	return ok && (f[1] == "aws" || strings.HasPrefix(f[1], "aws-")) &&
		f[2] == "iam" && f[4] == "*" && f[5] == "root"
}

// arnFields splits an ARN into its six fields — arn, partition, service,
// region, account, resource — reporting false for a value not shaped like
// one. The resource field keeps any colons of its own.
func arnFields(v string) ([]string, bool) {
	f := strings.SplitN(v, ":", 6)
	if len(f) != 6 || f[0] != "arn" {
		return nil, false
	}
	return f, true
}

func parseCondition(v any) map[string]map[string][]string {
	ops, ok := v.(map[string]any)
	if !ok || len(ops) == 0 {
		return nil
	}
	cond := make(map[string]map[string][]string, len(ops))
	for op, keys := range ops {
		km, ok := keys.(map[string]any)
		if !ok {
			continue
		}
		block := make(map[string][]string, len(km))
		for key, vals := range km {
			block[key] = scalars(vals)
		}
		cond[op] = block
	}
	return cond
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

// strs accepts a string or an array and keeps only the string elements.
func strs(v any) []string {
	switch t := v.(type) {
	case string:
		return []string{t}
	case []any:
		var out []string
		for _, e := range t {
			if s, ok := e.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// scalars is strs for condition values, where bools and numbers are legal
// and rendered as text.
func scalars(v any) []string {
	switch t := v.(type) {
	case []any:
		var out []string
		for _, e := range t {
			switch e.(type) {
			case string, bool, float64, json.Number:
				out = append(out, fmt.Sprintf("%v", e))
			}
		}
		return out
	case string, bool, float64, json.Number:
		return []string{fmt.Sprintf("%v", t)}
	}
	return nil
}

func objects(v any) []map[string]any {
	switch t := v.(type) {
	case map[string]any:
		return []map[string]any{t}
	case []any:
		var out []map[string]any
		for _, e := range t {
			if m, ok := e.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out
	}
	return nil
}
