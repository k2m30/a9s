// SPDX-License-Identifier: GPL-3.0-or-later

package views

import (
	"strings"

	"charm.land/bubbles/v2/viewport"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/config"
	"github.com/k2m30/a9s/v3/core/fieldpath"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
)

// DetailModel renders the key-value describe view using bubbles/viewport for scroll.
// ctrl is non-nil when the model is constructed by the TUI navigator via
// NewDetailWithCtrl; in that case View() delegates to
// RenderDetail(ctrl.Snapshot().Body.Detail) and key actions route to ctrl.Apply.
// Unit tests and isolated callers leave ctrl nil; View() builds a live body and
// delegates to RenderDetail so parity tests remain unaffected.
type DetailModel struct {
	ctrl                   *app.Controller // non-nil = controller-backed TUI path
	res                    resource.Resource
	resourceType           string // e.g. "ec2", "s3", "rds" — used to look up correct ViewDef
	viewConfig             *config.ViewsConfig
	navProvider            func(string) []resource.NavigableField // returns navigable fields for a resource type; defaults to GetActiveNavigableFields
	viewport               viewport.Model
	ready                  bool
	width                  int
	height                 int
	keys                   keys.Map
	rightCol               RightColumnModel
	rightColVisible        bool                  // true when explicitly toggled on
	rightColAutoShown      bool                  // true when right column was auto-shown on SetSize (wide terminal + registered defs)
	rightColUserToggled    bool                  // true after user explicitly toggles related visibility
	rightColWidth          int                   // width of right column panel (default 32)
	pendingRelatedDispatch bool                  // true when a narrow→wide resize should dispatch RelatedCheckStartedMsg
	fieldList              []fieldpath.FieldItem // structured field data; nil = not yet computed
	fieldCursor            int                   // index into fieldList for navigable cursor
}

// NewDetailWithCtrl creates a DetailModel backed by the given controller.
// The controller stack must already have ScreenDetail pushed and EnsureDetailState
// called before the first View() so that Snapshot().Body.Detail is non-nil from
// the first render.
//
// Key actions in Update route to ctrl.Apply. Enrichment findings and related
// results must be applied via ctrl.ApplyDetailFinding / ctrl.ApplyDetailRelated
// (not via SetEnrichmentFinding / ApplyRelatedResults) so the controller
// remains the single source of truth.
func NewDetailWithCtrl(res resource.Resource, resourceType string, viewConfig *config.ViewsConfig, k keys.Map, ctrl *app.Controller) DetailModel {
	if resourceType == "" {
		resourceType = inferDetailResourceType(res)
	}
	return DetailModel{
		ctrl:          ctrl,
		resourceType:  resourceType,
		res:           res,
		viewConfig:    viewConfig,
		navProvider:   resource.GetActiveNavigableFields,
		keys:          k,
		rightColWidth: 32,
	}
}

// SetNavProvider overrides the nav field provider used by the projector
// (projection.buildItems, core/semantics/projection/generic.go).
// TUI construction paths call this with resource.GetNavigableFields (merged
// ACTIVE+DEFAULT) so that prod code sees all registered navigable fields.
// Test-direct paths retain the ACTIVE-only default to stay isolated from
// init-time registrations.
func (m *DetailModel) SetNavProvider(p func(string) []resource.NavigableField) {
	m.navProvider = p
}

// inferDetailResourceType provides a conservative fallback for routes that
// navigate to detail without an explicit type. This prevents losing related
// and navigable behavior for common top-level resources.
func inferDetailResourceType(res resource.Resource) string {
	has := func(k string) bool {
		v, ok := res.Fields[k]
		return ok && strings.TrimSpace(v) != ""
	}
	// EC2 signature: infer only from EC2-shaped key sets.
	// Anchor on instance id key plus at least one EC2-specific companion field.
	// Fetchers write snake_case, and that is the only spelling a Fields map
	// carries: a path-cased key here would name a fact under a second
	// spelling nothing writes.
	hasInstanceID := has("instance_id")
	hasEC2Companion := has("image_id") ||
		has("vpc_id") ||
		has("subnet_id") ||
		has("private_ip") ||
		has("public_ip") ||
		has("key_name") ||
		has("lifecycle") ||
		has("launch_time") ||
		has("iam_instance_profile") ||
		has("security_groups")
	if hasInstanceID && hasEC2Companion {
		return "ec2"
	}
	return ""
}
