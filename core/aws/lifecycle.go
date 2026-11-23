// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"slices"

	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
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

// appliesAny is catalog.Applies for a pass that reads one object for several
// codes, in the lifecycle the read itself reported: the read is worth making
// when any of them can stand. The types it serves declare no kind.
func appliesAny(shortName, raw string, codes ...domain.FindingCode) bool {
	class := LifecycleClassOf(shortName, raw)
	return slices.ContainsFunc(codes, func(c domain.FindingCode) bool { return catalog.Applies(shortName, c, class, raw, "") })
}

// rowAppliesAny is catalog.AppliesToRow for a per-row read that answers
// several codes: the read is made, and the row marked when it fails, only
// when one of them can stand on the row's lifecycle and kind.
func rowAppliesAny(shortName string, r resource.Resource, codes ...domain.FindingCode) bool {
	return slices.ContainsFunc(codes, func(c domain.FindingCode) bool { return catalog.AppliesToRow(shortName, c, r) })
}

// withState composes a row's state finding with its posture findings, for
// the emit door to keep the ones the row's lifecycle and kind allow. A
// posture finding of the state finding's own code is the state already
// stated.
func withState(shortName, raw string, posture []domain.Finding, slots ...string) []domain.Finding {
	state, ok := StateFinding(shortName, raw, slots...)
	if !ok {
		return posture
	}
	out := []domain.Finding{state}
	for _, f := range posture {
		if f.Code != state.Code {
			out = append(out, f)
		}
	}
	return out
}
