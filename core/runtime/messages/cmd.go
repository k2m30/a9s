// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package messages

import "github.com/k2m30/a9s/v3/core/resource"

// ViewTarget identifies a destination view for Navigate.
type ViewTarget int

const (
	TargetMainMenu ViewTarget = iota
	TargetResourceList
	TargetDetail
	TargetYAML
	TargetJSON
	TargetReveal
	TargetProfile
	TargetRegion
	TargetTheme
	TargetHelp
	TargetCosts
)

// Navigate requests a view transition. The adapter handles push/pop.
type Navigate struct {
	Target         ViewTarget
	ResourceType   string
	Resource       *resource.Resource
	ReplaceCurrent bool // when true, pop current view before pushing target (used by auto-open flows)
}

func (Navigate) isCmd() {}

// PopView requests popping the current view from the stack.
type PopView struct{}

func (PopView) isCmd() {}

// LoadMore triggers loading the next page of a paginated resource list.
type LoadMore struct {
	ResourceType      string
	ContinuationToken string
	ParentContext     map[string]string // non-nil for child views
	FetchFilter       map[string]string
}

func (LoadMore) isCmd() {}

// ProfileSelected is sent when the user confirms a profile selection.
type ProfileSelected struct {
	Profile string
}

func (ProfileSelected) isCmd() {}

// RegionSelected is sent when the user confirms a region selection.
type RegionSelected struct {
	Region string
}

func (RegionSelected) isCmd() {}

// ThemeSelected is sent when the user confirms a theme selection.
type ThemeSelected struct {
	Theme string
}

func (ThemeSelected) isCmd() {}

// InitConnect triggers the initial AWS session setup.
type InitConnect struct {
	Profile string
	Region  string
}

func (InitConnect) isCmd() {}

// EnterChildView signals that the user has triggered a child view navigation.
// The adapter uses ChildType to look up the child type definition and fetcher,
// ParentContext to provide parameters to the child fetcher, and DisplayName
// for the child view's frame title.
type EnterChildView struct {
	ChildType     string
	ParentContext map[string]string
	DisplayName   string
}

func (EnterChildView) isCmd() {}

// LoadResources triggers an async fetch of resources for a given type.
type LoadResources struct {
	ResourceType  string
	ParentContext map[string]string
}

func (LoadResources) isCmd() {}

// RelatedNavigate requests navigation to a related resource type.
// Emitted by: (a) detail view when Enter pressed on navigable field,
// (b) rightColumnModel when Enter pressed on selected row.
// Handled by: app core (handleRelatedNavigate).
type RelatedNavigate struct {
	TargetType     string            // resource short name to navigate to (e.g., "vpc")
	SourceResource resource.Resource // the resource being viewed
	SourceType     string            // source resource short name (e.g., "ec2")
	TargetID       string            // specific ID for navigable field case (e.g., "vpc-0abc")
	RelatedIDs     []string          // IDs from checker for right-column case
	FetchFilter    map[string]string
	// Truncated is the source row's truncation flag — true for both "(0+)" and
	// "(N+)". Carried so the adapter routes them through the identical scoped
	// reverse-scan path; the zero count is never special-cased.
	Truncated bool
	// Checker is the originating RelatedDef.Checker. Carried forward so
	// each subsequent page of the target type (m-loads-more) can re-run
	// the predicate and extend the visible ID set — essential for
	// truncated pivots whose initial count is a lower bound.
	Checker resource.RelatedChecker
	// DirectDetail is true only when this event originates from a detail
	// view's navigable-field Enter handler (internal/tui/app_stack.go). It
	// means the user asked to see the TARGET resource itself — navigation
	// must land on the target's own detail view, even when the target type
	// registers a Children[Key="enter"] child view (e.g. "role" →
	// role_policies). This is deliberately narrower than the related-PANEL
	// Count=1 pivot (rightcolumn.go, DirectDetail left false), which must
	// keep mirroring "press Enter in the target's list view" — including
	// drilling into that child view — per the pinned 2026-04-24 invariant
	// (tests/unit/related_navigate_cache_enter_child_test.go).
	DirectDetail bool
}

func (RelatedNavigate) isCmd() {}

// AllCmdSamples returns exactly one minimal-but-valid instance of every
// concrete type implementing Cmd, in this file's declaration order — the
// enumerable registry the cross-renderer routing contract test iterates.
// ADDING A NEW COMMAND TYPE WITHOUT A SAMPLE HERE MUST FAIL THE CONTRACT
// TEST's count check, so keep this adjacent to the type definitions above.
//
// Fields are left at their zero value except where a non-zero value avoids
// a nil-deref/panic on the routing path or exercises the type's real branch
// (a registered ResourceType/TargetType, or a Resource carrying a Type/ID,
// instead of an unregistered empty-string no-op). Target: zero exclusions —
// every concrete Cmd type below has a sample.
func AllCmdSamples() []Cmd {
	return []Cmd{
		Navigate{Target: TargetMainMenu},
		PopView{},
		LoadMore{ResourceType: "ec2"},
		ProfileSelected{Profile: "sample"},
		RegionSelected{Region: "us-east-1"},
		ThemeSelected{Theme: "tokyo-night"},
		InitConnect{Profile: "sample", Region: "us-east-1"},
		EnterChildView{ChildType: "sample", DisplayName: "sample"},
		LoadResources{ResourceType: "ec2"},
		RelatedNavigate{TargetType: "vpc", SourceResource: resource.Resource{Type: "ec2", ID: "i-sample"}, SourceType: "ec2", TargetID: "vpc-sample"},
	}
}
