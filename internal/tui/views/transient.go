// transient.go — zero-storage transient constructors and accessor methods
// used by the renderer-side free render functions (renderer.go). These allow
// the renderer to create short-lived view model instances from rendererState
// dimensions + controller body data, call Render*, and discard the model —
// never storing it across frames.
package views

import (
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/viewport"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// ── MainMenuModel ─────────────────────────────────────────────────────────────

// SetScrollOffset sets the menu's scroll offset. This is renderer-owned state:
// the controller handles cursor/selection; the renderer owns the viewport offset.
func (m *MainMenuModel) SetScrollOffset(n int) {
	m.scrollOffset = n
}

// GetScrollOffset returns the current scroll offset so callers can persist
// it after calling Update (e.g. in updateActiveRS).
func (m MainMenuModel) GetScrollOffset() int {
	return m.scrollOffset
}

// ── ResourceListModel ─────────────────────────────────────────────────────────

// NewTransientResourceList creates a ResourceListModel that holds only the fields
// that RenderList reads: typeDef (for column/sort cross-reference), width, height,
// and a fresh spinner. It has no ctrl — the caller must pass body from the
// controller snapshot directly to RenderList.
//
// The returned model is transient: it should be used only to call RenderList,
// then discarded. Its ctrl field is nil so any method that reads from ctrl will
// panic; callers must not call Update or key-handling methods on it.
func NewTransientResourceList(td resource.ResourceTypeDef, w, h int) ResourceListModel {
	sp := spinner.New()
	return ResourceListModel{
		typeDef: td,
		width:   w,
		height:  h,
		spinner: sp,
	}
}

// ── DetailModel ───────────────────────────────────────────────────────────────

// NewTransientDetail creates a DetailModel whose viewport is pre-initialised
// from vp. ready is set to true so RenderDetail does not return early with
// "Initializing...". The caller is responsible for propagating the mutated
// viewport back from Viewport() after each RenderDetail call.
func NewTransientDetail(w, h int, vp viewport.Model) DetailModel {
	return DetailModel{
		width:         w,
		height:        h,
		viewport:      vp,
		ready:         true,
		rightColWidth: 32,
	}
}

// Viewport returns the current viewport.Model so the caller can persist
// scroll state across renders.
func (m DetailModel) Viewport() viewport.Model {
	return m.viewport
}

// ── YAMLModel ─────────────────────────────────────────────────────────────────

// NewTransientYAML creates a YAMLModel whose viewport is pre-initialised from
// vp. ready is set to true so RenderText does not return early.
func NewTransientYAML(w, h int, vp viewport.Model) YAMLModel {
	return YAMLModel{
		width:    w,
		height:   h,
		viewport: vp,
		ready:    true,
	}
}

// Viewport returns the current viewport.Model so the caller can persist
// scroll state across renders.
func (m YAMLModel) Viewport() viewport.Model {
	return m.viewport
}

// ── SelectorModel ─────────────────────────────────────────────────────────────

// NewTransientSelector creates a SelectorModel with only width/height set.
// RenderSelector reads only m.height and m.width, so a nil ctrl is safe here.
// The caller must pass body from the controller snapshot to RenderSelector.
func NewTransientSelector(w, h int) SelectorModel {
	return SelectorModel{
		width:  w,
		height: h,
	}
}

// ── DetailModel helpers ────────────────────────────────────────────────────────

// RawYAMLFromResource converts a resource.Resource to a YAML string for
// clipboard copy. Exported for use by the tui package's handleCopy dispatcher
// which can no longer call DetailModel.RawYAML() because no DetailModel is
// stored on the rendererState stack.
func RawYAMLFromResource(res resource.Resource) string {
	m := DetailModel{res: res}
	return m.RawYAML()
}

// ── RightColumnModel ──────────────────────────────────────────────────────────

// NewRightColumn creates a RightColumnModel from related definitions and a
// parent resource. Exported so the tui package can reset the right-column
// widget on a rendererState (e.g. on Ctrl+R while a detail view is active)
// without an indirect round-trip through DetailModel.ResetRightColumn.
func NewRightColumn(defs []resource.RelatedDef, parentRes resource.Resource, sourceType string) RightColumnModel {
	return newRightColumn(defs, parentRes, sourceType)
}

// ── RevealModel ───────────────────────────────────────────────────────────────

// SetWrap sets the word-wrap flag on the reveal viewport without toggling it.
// Used by the free render function to restore wrap state from rendererState.
func (m *RevealModel) SetWrap(v bool) {
	m.wrap = v
}

// SetViewport replaces the reveal's internal viewport and marks the model ready.
// Used by the free render function to restore scroll state from rendererState.
func (m *RevealModel) SetViewport(vp viewport.Model) {
	m.viewport = vp
	m.ready = true
}

// GetViewport returns the current viewport so the caller can persist scroll
// state across renders.
func (m RevealModel) GetViewport() viewport.Model {
	return m.viewport
}
