// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package catalog

import (
	"slices"
	"strings"

	"github.com/k2m30/a9s/v3/core/domain"
)

// LifecycleClass is what a lifecycle value AWS reports means for the row:
// whether the resource is serving, on its way to a steady state, broken but
// still there, deliberately idle, or leaving. The class decides whether the
// row's posture findings stand beside its state finding and whether Wave 2
// reads it; the state FindingDef carries the class, so a value is classified
// in one place.
type LifecycleClass uint8

const (
	// LifecycleNormal is a value with no finding: the resource is serving.
	LifecycleNormal LifecycleClass = iota
	// LifecycleTransitional is an operation in flight that ends on its own.
	LifecycleTransitional
	// LifecycleFailedLive is a failure on a resource that still exists and
	// still holds its data and configuration.
	LifecycleFailedLive
	// LifecyclePaused is a resource an operator stopped or disabled.
	LifecyclePaused
	LifecycleTearingDown
	// LifecycleEnded is a resource AWS reports as deleted, terminated or
	// inactive.
	LifecycleEnded
	// LifecyclePendingDeletion is a deletion AWS has scheduled and can still
	// cancel (a KMS key's waiting period).
	LifecyclePendingDeletion
)

// DefaultSeverity is the severity a state FindingDef of this class carries
// unless it declares SeverityDeviates.
func (c LifecycleClass) DefaultSeverity() domain.Severity {
	switch c {
	case LifecycleTransitional, LifecycleTearingDown, LifecyclePendingDeletion:
		return domain.SevWarn
	case LifecycleFailedLive:
		return domain.SevBroken
	case LifecyclePaused, LifecycleEnded:
		return domain.SevDim
	}
	return domain.SevOK
}

// Live reports whether a resource in this class still has posture to judge:
// its posture findings stand and Wave 2 reads it.
func (c LifecycleClass) Live() bool {
	return c != LifecycleTearingDown && c != LifecycleEnded && c != LifecyclePendingDeletion
}

// Lifecycle maps a lifecycle value of td to its class and the state finding
// it raises; code is "" for a normal value. A value td's table does not
// declare raises td.UnknownState and keeps the row live, because nothing says
// the resource is leaving. A type without a table, or a row with no value,
// is Normal.
func Lifecycle(td ResourceTypeDef, raw string) (LifecycleClass, domain.FindingCode) {
	if len(td.StateEnum) == 0 || raw == "" || slices.Contains(td.NormalStates, raw) {
		return LifecycleNormal, ""
	}
	for _, def := range td.Findings {
		if slices.Contains(def.States, raw) {
			return def.Class, def.Code
		}
	}
	return LifecycleTransitional, td.UnknownState
}

var (
	stateTables     map[string]ResourceTypeDef              //nolint:gochecknoglobals // process-scope catalog index, set with the registry
	appliesIn       map[domain.FindingCode][]LifecycleClass //nolint:gochecknoglobals // process-scope catalog index, set with the registry
	appliesInStates map[domain.FindingCode][]string         //nolint:gochecknoglobals // process-scope catalog index, set with the registry
)

func indexStateTables(types []ResourceTypeDef) {
	if stateTables == nil {
		stateTables = map[string]ResourceTypeDef{}
	}
	if appliesIn == nil {
		appliesIn = map[domain.FindingCode][]LifecycleClass{}
	}
	if appliesInStates == nil {
		appliesInStates = map[domain.FindingCode][]string{}
	}
	for _, t := range types {
		if len(t.StateEnum) > 0 {
			stateTables[strings.ToLower(t.ShortName)] = t
		}
		for _, f := range t.Findings {
			if len(f.AppliesIn) > 0 {
				appliesIn[f.Code] = f.AppliesIn
			}
			if len(f.AppliesInStates) > 0 {
				appliesInStates[f.Code] = f.AppliesInStates
			}
		}
	}
}

// StateTable is indexed at install, so a fetcher asking per row does not
// scan the registry.
func StateTable(shortName string) (ResourceTypeDef, bool) {
	td, ok := stateTables[strings.ToLower(shortName)]
	return td, ok
}

// LifecycleOf is the by-name entry for fetchers, which hold a raw value and
// not their catalog literal.
func LifecycleOf(shortName, raw string) (LifecycleClass, domain.FindingCode) {
	td, ok := StateTable(shortName)
	if !ok {
		return LifecycleNormal, ""
	}
	return Lifecycle(td, raw)
}

// RowLifecycle is the class of a row of td, read from the one field that
// holds the lifecycle value AWS reported.
func RowLifecycle(td ResourceTypeDef, r domain.Resource) LifecycleClass {
	class, _ := Lifecycle(td, r.Fields[td.StatusKey()])
	return class
}

// Applies is the one rule for whether posture stands in a lifecycle class, so
// no fetcher returns early on a status string.
func Applies(code domain.FindingCode, class LifecycleClass, raw string) bool {
	if only, narrowed := appliesIn[code]; narrowed {
		if !slices.Contains(only, class) {
			return false
		}
	} else if !class.Live() {
		return false
	}
	only, narrowed := appliesInStates[code]
	return !narrowed || slices.Contains(only, raw)
}
