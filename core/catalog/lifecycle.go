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

// declKey names one type's declaration of a code. One code can be declared
// by two types with different rules (a top-level type and its child view), so
// a finding is judged by the declaration of the type whose row carries it.
type declKey struct {
	shortName string
	code      domain.FindingCode
}

var (
	stateTables map[string]ResourceTypeDef //nolint:gochecknoglobals // process-scope catalog index, set with the registry
	rowTypes    map[string]ResourceTypeDef //nolint:gochecknoglobals // process-scope catalog index, set with the registry
	decls       map[declKey]FindingDef     //nolint:gochecknoglobals // process-scope catalog index, set with the registry
)

func indexStateTables(types []ResourceTypeDef) {
	if stateTables == nil {
		stateTables = map[string]ResourceTypeDef{}
	}
	if rowTypes == nil {
		rowTypes = map[string]ResourceTypeDef{}
	}
	if decls == nil {
		decls = map[declKey]FindingDef{}
	}
	for _, t := range types {
		name := strings.ToLower(t.ShortName)
		rowTypes[name] = t
		if len(t.StateEnum) > 0 {
			stateTables[name] = t
		}
		for _, f := range t.Findings {
			decls[declKey{name, f.Code}] = f
			if (len(f.OnlyKinds)+len(f.NotKinds) > 0 || len(f.Rows) > 0) && t.Kind == nil {
				panic("catalog: " + string(f.Code) + " declares kinds and " + t.ShortName + " declares no Kind classifier")
			}
			if len(f.Rows) > 0 && f.Source != "wave2" {
				panic("catalog: " + string(f.Code) + " declares rows by kind, which only the Wave 2 door judges")
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

// Applies is the one rule for whether a finding stands on a row of the type
// shortName: the lifecycle class and value the row is in, and the kind its
// type's classifier names it, against what that type declares for code. No
// fetcher returns early on a status string and no enricher tests a kind field
// itself. kind is "" when the caller holds no row to classify; a finding
// narrowed by OnlyKinds does not stand on a kind nobody read.
func Applies(shortName string, code domain.FindingCode, class LifecycleClass, raw, kind string) bool {
	def := decls[declKey{strings.ToLower(shortName), code}]
	if len(def.AppliesIn) > 0 {
		if !slices.Contains(def.AppliesIn, class) {
			return false
		}
	} else if !class.Live() {
		return false
	}
	if len(def.AppliesInStates) > 0 && !slices.Contains(def.AppliesInStates, raw) {
		return false
	}
	return kindAllowed(def.OnlyKinds, def.NotKinds, kind)
}

func kindAllowed(only, not []string, kind string) bool {
	return (len(only) == 0 || slices.Contains(only, kind)) && !slices.Contains(not, kind)
}

// RowApplies reports whether the supporting row labelled label of the
// finding code stands on r, a row of the type shortName, by the kind the
// finding's Rows declare for it. A row the finding does not declare stands.
func RowApplies(shortName string, code domain.FindingCode, label string, r domain.Resource) bool {
	name := strings.ToLower(shortName)
	def := decls[declKey{name, code}]
	i := slices.IndexFunc(def.Rows, func(d RowDecl) bool { return d.Label == label })
	if i < 0 {
		return true
	}
	return kindAllowed(def.Rows[i].OnlyKinds, def.Rows[i].NotKinds, rowTypes[name].Kind(r))
}

// Declares answers for installed types only: a type a test registers at run
// time declares nothing here.
func Declares(shortName string, code domain.FindingCode) bool {
	_, ok := decls[declKey{strings.ToLower(shortName), code}]
	return ok
}

// Judge is the rule both emit doors apply to a finding raised on a row of the
// type shortName: AppliesToRow, for a code the type declares. A code an
// installed type does not declare has no rule to stand by, so raising it is a
// defect in the code that raised it and Judge panics. A type the catalog does
// not hold declares nothing to judge by.
func Judge(shortName string, code domain.FindingCode, r domain.Resource) bool {
	name := strings.ToLower(shortName)
	if _, installed := rowTypes[name]; installed {
		if _, declared := decls[declKey{name, code}]; !declared {
			panic("catalog: " + shortName + " raises " + string(code) + ", which it does not declare")
		}
	}
	return AppliesToRow(shortName, code, r)
}

// AppliesToRow is Applies for a row of the type shortName, in the lifecycle
// its status field holds. The state finding that value raises stands whatever
// the class, since it is the lifecycle value itself; a code the type does not
// declare stands.
func AppliesToRow(shortName string, code domain.FindingCode, r domain.Resource) bool {
	name := strings.ToLower(shortName)
	if _, declared := decls[declKey{name, code}]; !declared {
		return true
	}
	td := rowTypes[name]
	raw := r.Fields[td.StatusKey()]
	class, state := Lifecycle(td, raw)
	kind := ""
	if td.Kind != nil {
		kind = td.Kind(r)
	}
	return code == state || Applies(shortName, code, class, raw, kind)
}
