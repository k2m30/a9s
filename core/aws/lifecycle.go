// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"slices"

	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
)

func enumValues[T interface {
	~string
	Values() []T
}]() []string {
	var zero T
	vs := zero.Values()
	out := make([]string, len(vs))
	for i, v := range vs {
		out[i] = string(v)
	}
	return out
}

// LifecycleClassOf answers Normal for a type that declares no state table, so
// a caller holding a child kind or an untabled type needs no special case.
func LifecycleClassOf(shortName, status string) catalog.LifecycleClass {
	class, _ := catalog.LifecycleOf(shortName, status)
	return class
}

// StateFinding is the one Wave-1 emitter of a state finding. slots default
// to AWS's word for raw, so a value the table does not know is shown as AWS
// reported it.
func StateFinding(shortName, raw string, slots ...string) (domain.Finding, bool) {
	_, code := catalog.LifecycleOf(shortName, raw)
	if code == "" {
		return domain.Finding{}, false
	}
	if len(slots) == 0 {
		slots = []string{domain.HumanizeStatusPhrase(raw)}
	}
	return wave1Finding(code, slots...), true
}

// appliesAny is catalog.Applies for a pass that reads once for several
// codes: the read is worth making when any of them can stand on the row.
func appliesAny(shortName, raw string, codes ...domain.FindingCode) bool {
	class := LifecycleClassOf(shortName, raw)
	return slices.ContainsFunc(codes, func(c domain.FindingCode) bool { return catalog.Applies(c, class, raw) })
}

// withState is the composer every fetcher with a state table returns
// through, so whether posture stands in a state is decided by the class and
// never by where a check sits in a fetcher.
func withState(shortName, raw string, posture []domain.Finding, slots ...string) []domain.Finding {
	class, _ := catalog.LifecycleOf(shortName, raw)
	var out []domain.Finding
	if f, ok := StateFinding(shortName, raw, slots...); ok {
		out = append(out, f)
	}
	for _, f := range posture {
		if catalog.Applies(f.Code, class, raw) {
			out = append(out, f)
		}
	}
	return out
}
