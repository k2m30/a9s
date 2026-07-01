package app

import (
	"strings"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
)

// buildListFooterHints builds the footer key hints for a resource-list screen,
// ported faithfully from ResourceListModel.BottomHints() so the controller is
// the single source of truth for all renderers. Callers must hold c.mu.
func (c *Controller) buildListFooterHints(ctx runtime.ScreenContext, ls *ListState) []KeyHint {
	var hints []KeyHint

	if ls.EscPops {
		hints = append(hints, KeyHint{Key: "esc", Help: "Back"})
	}

	td := resource.FindResourceType(ctx.ResourceType)
	if td == nil {
		if fv, ok := c.fallbackTypeDefs[ctx.ResourceType]; ok {
			td = &fv
		}
	}
	if td != nil {
		var enterChild *resource.ChildViewDef
		for i := range td.Children {
			if td.Children[i].Key == "enter" {
				enterChild = &td.Children[i]
				break
			}
		}
		if enterChild != nil {
			showEnterChild := true
			if enterChild.DrillCondition != nil {
				sel, ok := c.listSelected()
				showEnterChild = ok && enterChild.DrillCondition(sel)
			}
			if showEnterChild {
				desc := enterChild.ChildType
				if ct := resource.GetChildType(enterChild.ChildType); ct != nil {
					desc = ct.Name
				}
				hints = append(hints, KeyHint{Key: "enter", Help: desc})
				hints = append(hints, KeyHint{Key: "d", Help: "Detail"})
			}
		}

		if resource.HasRevealFetcher(td.ShortName) {
			hints = append(hints, KeyHint{Key: "x", Help: "Reveal"})
		}
	}

	hints = append(hints, KeyHint{Key: "y", Help: "YAML"})
	hints = append(hints, KeyHint{Key: "J", Help: "JSON"})

	if td != nil {
		for _, child := range td.Children {
			if child.Key == "enter" {
				continue
			}
			desc := child.ChildType
			if ct := resource.GetChildType(child.ChildType); ct != nil {
				desc = ct.Name
			}
			hints = append(hints, KeyHint{Key: child.Key, Help: desc})
		}

		if td.CloudTrailKey != "" && ls.ParentContext == nil {
			hints = append(hints, KeyHint{Key: "t", Help: "CloudTrail"})
		}
	}

	hints = append(hints, KeyHint{Key: "ctrl+r", Help: "Refresh"})
	hints = append(hints, KeyHint{Key: "ctrl+z", Help: "Only !"})

	if ls.HasPagination {
		hints = append(hints, KeyHint{Key: "m", Help: "More"})
	}

	return hints
}

// buildTextFooterHints builds the footer key hints for a YAML or JSON text
// screen. The CloudTrail hint appears only when the resource type has a
// CloudTrailKey and the resource can be found in the cache. Callers must hold c.mu.
func (c *Controller) buildTextFooterHints(screenID runtime.ScreenID, ctx runtime.ScreenContext) []KeyHint {
	hints := []KeyHint{
		{Key: "w", Help: "Wrap"},
		{Key: "c", Help: "Copy"},
	}
	// CloudTrail hint: needs a resource that has a CloudTrailKey.
	// Skip when no resource type is set (raw-text / reveal YAML path).
	if ctx.ResourceType != "" && ctx.ResourceID != "" {
		// Find the resource in the cache to call BuildCloudTrailFilter.
		for _, r := range c.resourceCache[ctx.ResourceType] {
			if r.ID == ctx.ResourceID {
				if resource.BuildCloudTrailFilter(r, ctx.ResourceType) != nil {
					hints = append(hints, KeyHint{Key: "t", Help: "CloudTrail"})
				}
				break
			}
		}
	}
	return hints
}

// buildDetailFooterHints builds the footer key hints for a detail screen.
// Ported faithfully from DetailModel.BottomHints() so the controller is
// the single source of truth for all renderers. Uses the controller-owned
// FieldCursor and RelatedRows/RelatedFocus state in ds. Callers must hold c.mu.
func (c *Controller) buildDetailFooterHints(ds *DetailState) []KeyHint {
	var hints []KeyHint

	// Right column focused: show Enter-on-selected-type + tab + y + J + t + ctrl+r.
	if ds.RelatedFocus && ds.RelatedVisible {
		// Find the selected row's target type.
		query := strings.TrimSpace(strings.ToLower(ds.RelatedFilter))
		var visibleRows []DetailRelatedRow
		for _, row := range ds.RelatedRows {
			if isSelfPivotZeroDetailRow(row, ds.ResourceType) {
				continue
			}
			if query != "" && !strings.Contains(strings.ToLower(row.DisplayName), query) {
				continue
			}
			visibleRows = append(visibleRows, row)
		}
		cursor := ds.RelatedCursor
		if cursor >= 0 && cursor < len(visibleRows) {
			selected := visibleRows[cursor]
			if isActionableDetailRow(selected) {
				displayName := selected.TargetType
				if rt := resource.FindResourceType(selected.TargetType); rt != nil {
					displayName = rt.Name
				} else if ct := resource.GetChildType(selected.TargetType); ct != nil {
					displayName = ct.Name
				}
				hints = append(hints, KeyHint{Key: "enter", Help: displayName})
			}
		}
		hints = append(hints, KeyHint{Key: "tab", Help: "Fields"})
		hints = append(hints, KeyHint{Key: "y", Help: "YAML"})
		hints = append(hints, KeyHint{Key: "J", Help: "JSON"})
		if resource.BuildCloudTrailFilter(ds.Resource, ds.ResourceType) != nil {
			hints = append(hints, KeyHint{Key: "t", Help: "CloudTrail"})
		}
		hints = append(hints, KeyHint{Key: "ctrl+r", Help: "Refresh"})
		return hints
	}

	// Left column focused: check navigable field under cursor.
	items := buildDetailFieldItems(ds, c.viewConfig)
	fc := ds.FieldCursor
	if fc >= 0 && fc < len(items) {
		item := items[fc]
		if item.IsNavigable && item.TargetType != "" {
			displayName := item.TargetType
			if rt := resource.FindResourceType(item.TargetType); rt != nil {
				displayName = rt.Name
			} else if ct := resource.GetChildType(item.TargetType); ct != nil {
				displayName = ct.Name
			}
			hints = append(hints, KeyHint{Key: "enter", Help: displayName})
		}
	}

	hints = append(hints, KeyHint{Key: "y", Help: "YAML"})
	hints = append(hints, KeyHint{Key: "J", Help: "JSON"})
	if resource.BuildCloudTrailFilter(ds.Resource, ds.ResourceType) != nil {
		hints = append(hints, KeyHint{Key: "t", Help: "CloudTrail"})
	}

	// Related panel hints.
	if related := resource.GetRelated(ds.ResourceType); len(related) > 0 {
		hints = append(hints, KeyHint{Key: "r", Help: "Related"})
		// "tab: Cols" only when user explicitly toggled the panel on —
		// mirrors DetailModel.BottomHints which checks m.rightColVisible (not auto-shown).
		if ds.RelatedUserVisible {
			hints = append(hints, KeyHint{Key: "tab", Help: "Cols"})
		}
	}

	hints = append(hints, KeyHint{Key: "ctrl+r", Help: "Refresh"})
	hints = append(hints, KeyHint{Key: "w", Help: "Wrap"})

	return hints
}
