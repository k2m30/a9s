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

// Principal is the parsed Principal (or NotPrincipal) block of a statement.
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

// Parse decodes a policy document. IAM APIs return some documents
// URL-encoded, so a failed decode is retried after unescaping when the text
// contains a '%'. Unrecognised shapes inside the document are skipped, never
// fatal.
func Parse(doc string) (Document, error) {
	doc = strings.TrimSpace(doc)
	if doc == "" {
		return Document{}, errors.New("iampolicy: empty document")
	}
	var raw map[string]any
	err := json.Unmarshal([]byte(doc), &raw)
	if err != nil && strings.Contains(doc, "%") {
		if decoded, uerr := url.QueryUnescape(doc); uerr == nil {
			err = json.Unmarshal([]byte(decoded), &raw)
		}
	}
	if err != nil {
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
	if np, ok := s["NotPrincipal"]; ok {
		st.NotPrincipal = true
		st.Principal = parsePrincipal(np)
	} else {
		st.Principal = parsePrincipal(s["Principal"])
	}
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

// isAnyoneARN mirrors Prowler's has_public_principal: "arn:aws:iam::*:root"
// is any account's root, which is as public as "*".
func isAnyoneARN(s string) bool {
	return s == "*" || s == "arn:aws:iam::*:root"
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
