package app

import (
	"strings"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
)

// detailPageSize is the default scroll jump for PageUp/PageDown on a detail screen
// when the renderer does not supply a viewport size via Action.N.
const detailPageSize = 10

// detailPageSizeFor returns the page size for a PageUp/PageDown action on a
// detail screen.
func detailPageSizeFor(a Action) int {
	if a.N > 0 {
		return a.N
	}
	return detailPageSize
}

// applyDetailActions handles detail-screen-specific action kinds within
// applyLocked. Returns (snapshot, tasks, handled). If handled is false the
// caller should continue to the next action group.
func (c *Controller) applyDetailActions(a Action) (ViewState, []runtime.TaskRequest, bool) {
	ds := c.topDetailState()
	if ds == nil {
		return ViewState{}, nil, false
	}

	switch a.Kind {
	case ActionMoveUp:
		// Field cursor moves up in the left column; scroll up when right-focused.
		if !ds.RelatedFocus {
			if ds.FieldCursor > 0 {
				ds.FieldCursor--
				// Skip section headers and spacers — mirrors the TUI legacy path.
				items := buildDetailFieldItems(ds, c.viewConfig)
				for ds.FieldCursor > 0 && ds.FieldCursor < len(items) &&
					(items[ds.FieldCursor].IsSection || items[ds.FieldCursor].IsSpacer) {
					ds.FieldCursor--
				}
			}
		} else {
			if ds.RelatedCursor > 0 {
				ds.RelatedCursor--
			}
		}
		return c.snapshot(), nil, true

	case ActionMoveDown:
		if !ds.RelatedFocus {
			items := buildDetailFieldItems(ds, c.viewConfig)
			fieldCount := len(items)
			if ds.FieldCursor < fieldCount-1 {
				ds.FieldCursor++
				// Skip section headers and spacers — mirrors the TUI legacy path.
				for ds.FieldCursor < fieldCount-1 &&
					(items[ds.FieldCursor].IsSection || items[ds.FieldCursor].IsSpacer) {
					ds.FieldCursor++
				}
			}
		} else {
			relatedCount := c.detailRelatedVisibleCount(ds)
			if ds.RelatedCursor < relatedCount-1 {
				ds.RelatedCursor++
			}
		}
		return c.snapshot(), nil, true

	case ActionMoveTop:
		if !ds.RelatedFocus {
			ds.FieldCursor = 0
			ds.ScrollY = 0
		} else {
			ds.RelatedCursor = 0
			ds.RelatedScroll = 0
		}
		return c.snapshot(), nil, true

	case ActionMoveBottom:
		if !ds.RelatedFocus {
			fieldCount := c.detailFieldCount(ds)
			if fieldCount > 0 {
				ds.FieldCursor = fieldCount - 1
			}
		} else {
			relatedCount := c.detailRelatedVisibleCount(ds)
			if relatedCount > 0 {
				ds.RelatedCursor = relatedCount - 1
			}
		}
		return c.snapshot(), nil, true

	case ActionPageUp:
		if !ds.RelatedFocus {
			ds.ScrollY -= detailPageSizeFor(a)
			if ds.ScrollY < 0 {
				ds.ScrollY = 0
			}
		} else {
			ds.RelatedScroll -= detailPageSizeFor(a)
			if ds.RelatedScroll < 0 {
				ds.RelatedScroll = 0
			}
		}
		return c.snapshot(), nil, true

	case ActionPageDown:
		if !ds.RelatedFocus {
			ds.ScrollY += detailPageSizeFor(a)
		} else {
			relatedCount := c.detailRelatedVisibleCount(ds)
			ds.RelatedScroll += detailPageSizeFor(a)
			if ds.RelatedScroll >= relatedCount {
				ds.RelatedScroll = max(relatedCount-1, 0)
			}
		}
		return c.snapshot(), nil, true

	case ActionToggleWrap:
		ds.Wrap = !ds.Wrap
		return c.snapshot(), nil, true

	case ActionSearch:
		ds.SearchQuery = a.Arg
		ds.SearchCursor = 0
		return c.snapshot(), nil, true

	case ActionSearchNext:
		if ds.SearchQuery != "" {
			ds.SearchCursor++
		}
		return c.snapshot(), nil, true

	case ActionSearchPrev:
		if ds.SearchQuery != "" && ds.SearchCursor > 0 {
			ds.SearchCursor--
		}
		return c.snapshot(), nil, true

	case ActionSearchClear:
		ds.SearchQuery = ""
		ds.SearchCursor = 0
		return c.snapshot(), nil, true

	case ActionToggleRelated:
		ds.RelatedVisible = !ds.RelatedVisible
		// Once the user toggles, RelatedHidden gates auto-show so the panel
		// stays hidden across resizes when the user has turned it off.
		ds.RelatedHidden = true
		if !ds.RelatedVisible {
			ds.RelatedFocus = false
		}
		return c.snapshot(), nil, true

	case ActionToggleFocus:
		// Tab: toggle focus between left (field) column and right (related) column.
		// Only effective when the related panel is visible and has actionable rows.
		if ds.RelatedVisible {
			ds.RelatedFocus = !ds.RelatedFocus
		}
		return c.snapshot(), nil, true

	case ActionSetFilter:
		// Related-panel filter. The renderer only emits ActionSetFilter for the
		// related panel while it is focused, so we trust that intent rather than
		// re-checking ds.RelatedFocus (which can lag the renderer's focus state).
		if ds.RelatedVisible {
			ds.RelatedFilter = a.Arg
			ds.RelatedFilterActive = a.Arg != ""
			ds.RelatedCursor = 0
			ds.RelatedScroll = 0
		}
		return c.snapshot(), nil, true
	}

	return ViewState{}, nil, false
}

// detailFieldCount returns the number of field items for the given DetailState's
// resource by running the projector pipeline. Used by cursor clamping in
// applyDetailActions without building a full body.
func (c *Controller) detailFieldCount(ds *DetailState) int {
	return len(buildDetailFieldItems(ds, c.viewConfig))
}

// detailRelatedVisibleCount returns the number of visible related rows after
// applying the current filter.
func (c *Controller) detailRelatedVisibleCount(ds *DetailState) int {
	query := strings.TrimSpace(strings.ToLower(ds.RelatedFilter))
	count := 0
	for _, row := range ds.RelatedRows {
		if isSelfPivotZeroDetailRow(row, ds.ResourceType) {
			continue
		}
		if query == "" || strings.Contains(strings.ToLower(row.DisplayName), query) {
			count++
		}
	}
	return count
}

// isSelfPivotZeroDetailRow mirrors rightColumnModel.isSelfPivotZeroRow for
// DetailRelatedRow values.
func isSelfPivotZeroDetailRow(row DetailRelatedRow, sourceType string) bool {
	return !row.Loading &&
		row.Err == "" &&
		row.Count == 0 &&
		sourceType != "" &&
		row.TargetType == sourceType
}

// isActionableDetailRow delegates to the single shared predicate
// resource.IsRelatedActionable so the actionability rule is defined once.
func isActionableDetailRow(row DetailRelatedRow) bool {
	return resource.IsRelatedActionable(row.Count, row.Approximate, len(row.FetchFilter) > 0, row.Loading, row.Err != "")
}
