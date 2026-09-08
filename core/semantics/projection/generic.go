// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// Package projection provides DetailProjector implementations for resource types.
package projection

import (
	"sort"
	"strings"

	"github.com/k2m30/a9s/v3/core/config"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/fieldpath"
	"github.com/k2m30/a9s/v3/core/jsonyaml"
)

// ─── Injectable resource-registry callbacks ───────────────────────────────
//
// projection cannot import core/resource (no import cycle from
// core/semantics back to core/resource). The callbacks
// below are set by core/resource at init time so GenericWithConfig and
// GenericWithConfigAndNavProvider can access per-type metadata without
// creating that cycle.
//
// Callers that don't wire these (e.g. isolated unit tests that never import
// core/resource) get graceful fallback: nil callbacks → no navigability,
// no alias normalisation, no per-type field ordering (falls through to
// Fields-only flat rendering).

// NavFieldsProvider returns the navigable field definitions for a resource
// type, enabling GenericWithConfig to mark matching detail items as
// navigable. Set by core/resource.init().
var NavFieldsProvider func(shortName string) []domain.NavigableField

// NavIDProvider resolves an ARN/value to a bare resource ID for navigation.
// Set by core/resource.init().
var NavIDProvider func(targetType, value string) string

// FieldAliasProvider normalises a Fields map by adding PascalCase aliases for
// the snake_case keys produced by fetchers. Set by core/resource.init().
var FieldAliasProvider func(shortName string, fields map[string]string) map[string]string

// FieldKeysProvider returns the registered field keys for a resource type.
// Set by core/resource.init().
var FieldKeysProvider func(shortName string) []string

// ─── Generic projectors ───────────────────────────────────────────────────

// GenericWithConfig returns a DetailProjector using the provided view
// config. Pass nil to suppress view-config-driven detail paths entirely
// (produces flat alphabetical Fields rendering).
//
// Intended for callers (e.g. DetailModel) that already hold a loaded config
// and want to avoid a disk read, or that want nil-config semantics.
func GenericWithConfig(cfg *config.ViewsConfig) domain.DetailProjector {
	return func(r domain.Resource) []domain.Section {
		items := buildItems(r, cfg, NavFieldsProvider)
		if len(items) == 0 {
			return nil
		}
		return groupIntoSections(items)
	}
}

// GenericWithConfigAndNavProvider returns a DetailProjector equivalent to
// GenericWithConfig but uses an explicit nav fields provider instead of the
// global NavFieldsProvider. This allows callers to scope navigability to a
// specific registry (e.g. the ACTIVE-only registry for DetailModel) without
// affecting the global NavFieldsProvider.
//
// Pass nil for navProvider to suppress navigable-field annotations entirely.
func GenericWithConfigAndNavProvider(cfg *config.ViewsConfig, navProvider func(string) []domain.NavigableField) domain.DetailProjector {
	return func(r domain.Resource) []domain.Section {
		items := buildItems(r, cfg, navProvider)
		if len(items) == 0 {
			return nil
		}
		return groupIntoSections(items)
	}
}

// buildItems produces a []fieldpath.FieldItem for the non-ct-events path. It
// is the one field-list builder: the TUI renders the items the controller
// derives from it, and no view builds its own.
// cfg is the view config to use; nil means no detail-path ordering is applied.
// navProvider is the navigable-fields provider; nil suppresses navigability.
func buildItems(r domain.Resource, cfg *config.ViewsConfig, navProvider func(string) []domain.NavigableField) []fieldpath.FieldItem {

	// ── type-specific setup ───────────────────────────────────────────────
	shortName := r.Type

	// Only look up detail paths when a config is provided. cfg==nil preserves
	// the legacy nil-viewConfig behaviour from detail_fields.go: flat alphabetical
	// field rendering with raw (un-normalised) key names.
	var detailFields []config.DetailField
	if shortName != "" && cfg != nil {
		vd := config.GetViewDef(cfg, shortName)
		detailFields = vd.Detail
	}

	var detailPaths []string
	for _, df := range detailFields {
		if df.Path != "" {
			detailPaths = append(detailPaths, df.Path)
		}
	}

	// Build nav map: fieldPath → targetType.
	navMap := make(map[string]string)
	if shortName != "" && navProvider != nil {
		for _, nf := range navProvider(shortName) {
			navMap[nf.FieldPath] = nf.TargetType
		}
	}

	// Normalise Fields with PascalCase aliases when a provider is available.
	fields := r.Fields
	if shortName != "" && FieldAliasProvider != nil && len(fields) > 0 {
		fields = FieldAliasProvider(shortName, fields)
	}

	// Synthesise minimal Fields from ID/Name when the resource is a bare
	// stub (no Fields, no RawStruct).
	if len(fields) == 0 && r.RawStruct == nil && (r.ID != "" || r.Name != "") {
		var fieldKeys []string
		if shortName != "" && FieldKeysProvider != nil {
			fieldKeys = FieldKeysProvider(shortName)
		}
		synth := make(map[string]string, 2)
		if r.ID != "" && len(fieldKeys) > 0 {
			synth[fieldKeys[0]] = r.ID
		}
		if r.Name != "" && len(fieldKeys) > 1 {
			synth[fieldKeys[1]] = r.Name
		}
		if len(synth) > 0 {
			fields = synth
		}
	}

	// ── no detail paths: flat Fields rendering ────────────────────────────
	// When no detail paths are configured, use flat alphabetical rendering.
	// Pass nil navMap to suppress navigability — navigable affordances are
	// only meaningful when the operator has a configured detail path that maps
	// field keys to target types. Without that contract, an Enter press on
	// a randomly-ordered flat field would be surprising. This preserves
	// parity with the original TUI field-list builder's nil-config path.
	if len(detailPaths) == 0 {
		if len(fields) == 0 {
			return nil
		}
		keys := make([]string, 0, len(fields))
		for k := range fields {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		return expandJSONItems(flattenTagItems(fieldpath.ExtractFieldList(nil, fields, keys, nil)))
	}

	// ── structured path rendering ─────────────────────────────────────────

	// Extract items for all path-form detail fields at once.
	pathItems := expandJSONItems(flattenTagItems(fieldpath.ExtractFieldList(r.RawStruct, fields, detailPaths, navMap)))

	// Index items by path for ordered interleaving.
	pathItemsByPath := make(map[string][]fieldpath.FieldItem, len(detailPaths))
	for _, item := range pathItems {
		p := item.Path
		pathItemsByPath[p] = append(pathItemsByPath[p], item)
	}

	// Build the final ordered item list from detailFields ordering.
	var items []fieldpath.FieldItem
	emittedPath := make(map[string]bool, len(detailPaths))
	for _, df := range detailFields {
		if df.Key != "" {
			// Key-form: read from Fields[].
			val := "-"
			if v, ok := fields[df.Key]; ok && v != "" {
				val = v
			}
			items = append(items, fieldpath.FieldItem{
				Key:   df.DisplayLabel(),
				Value: val,
				Path:  df.Key,
			})
		} else if !emittedPath[df.Path] {
			emittedPath[df.Path] = true
			pItems := pathItemsByPath[df.Path]
			if df.Label != "" && len(pItems) > 0 {
				pItems = append([]fieldpath.FieldItem(nil), pItems...)
				pItems[0].Key = df.Label
			}
			items = append(items, pItems...)
		}
	}

	// ── sub-field navigability post-processing ────────────────────────────
	// Walk the item list and annotate sub-fields whose composed path matches a
	// navMap entry.
	currentPath := ""
	ancestorByLevel := map[int]string{}
	for i, item := range items {
		if item.IsHeader {
			currentPath = item.Path
			clear(ancestorByLevel)
			continue
		}
		if !item.IsSubField {
			continue
		}
		if item.Path != currentPath {
			currentPath = item.Path
			clear(ancestorByLevel)
		}
		rawLine := item.Value
		trimmed := strings.TrimSpace(rawLine)
		if trimmed == "" {
			continue
		}
		level := 0
		if leading := len(rawLine) - len(strings.TrimLeft(rawLine, " ")); leading > 0 {
			level = leading / jsonyaml.YAMLIndentSpaces
		}
		hasDash := strings.HasPrefix(trimmed, "- ")
		trimmed = strings.TrimPrefix(trimmed, "- ")
		subKey, subVal, hasSep := strings.Cut(trimmed, ":")
		if !hasSep {
			continue
		}
		subKey = strings.TrimSpace(subKey)
		subVal = strings.TrimSpace(subVal)
		for depth := range ancestorByLevel {
			if depth >= level {
				delete(ancestorByLevel, depth)
			}
		}
		pathParts := []string{item.Path}
		for depth := 0; depth < level; depth++ {
			if ancestor, ok := ancestorByLevel[depth]; ok && ancestor != "" {
				pathParts = append(pathParts, ancestor)
			}
		}
		pathParts = append(pathParts, subKey)
		composedPath := strings.Join(pathParts, ".")
		if tt, ok := navMap[composedPath]; ok && subVal != "" {
			items[i].IsNavigable = true
			items[i].TargetType = tt
			if NavIDProvider != nil {
				if navID := NavIDProvider(tt, subVal); navID != "" && navID != subVal {
					items[i].NavID = navID
				}
			}
			if hasDash {
				items[i].Key = "- " + subKey
			} else {
				items[i].Key = subKey
			}
			items[i].Value = subVal
		}
		if subVal == "" {
			ancestorByLevel[level] = subKey
		}
	}

	// Apply NavIDFromValue to top-level scalar navigable items.
	if NavIDProvider != nil {
		for i, item := range items {
			if item.IsNavigable && !item.IsSubField && item.TargetType != "" && item.Value != "" {
				if navID := NavIDProvider(item.TargetType, item.Value); navID != "" && navID != item.Value {
					items[i].NavID = navID
				}
			}
		}
	}

	return items
}

// groupIntoSections converts a []fieldpath.FieldItem into []domain.Section.
// FieldItems with IsSection=true start a new domain.Section.
// All items before the first IsSection marker are placed in a leading section
// with an empty title.  For typical resource types (no IsSection items), the
// entire list ends up in a single unnamed section — callers that want the
// section title to match the resource type inject it themselves.
func groupIntoSections(items []fieldpath.FieldItem) []domain.Section {
	var sections []domain.Section
	current := &domain.Section{}

	for _, item := range items {
		if item.IsSection {
			if len(current.Items) > 0 || current.Title != "" {
				sections = append(sections, *current)
			}
			current = &domain.Section{Title: item.Key}
			continue
		}
		current.Items = append(current.Items, fieldItemToDomainItem(item))
	}

	if len(current.Items) > 0 || current.Title != "" {
		sections = append(sections, *current)
	}
	return sections
}

// fieldItemToDomainItem maps a fieldpath.FieldItem to a domain.Item.
func fieldItemToDomainItem(fi fieldpath.FieldItem) domain.Item {
	kind := domain.ItemField
	switch {
	case fi.IsHeader:
		kind = domain.ItemHeader
	case fi.IsSubField:
		kind = domain.ItemSubfield
	case fi.IsSpacer:
		kind = domain.ItemSpacer
	}

	// Strip trailing colon from header labels (ExtractFieldList sets Key to
	// path, not "path:" — but a caller may set Key="Tags:").
	label := fi.Key
	if kind == domain.ItemHeader {
		label = strings.TrimSuffix(label, ":")
	}

	return domain.Item{
		Kind:        kind,
		Label:       label,
		Value:       fi.Value,
		Path:        fi.Path,
		Tier:        fi.ColorTier,
		Navigable:   fi.IsNavigable,
		TargetType:  fi.TargetType,
		NavID:       fi.NavID,
		IndentLevel: fi.IndentLevel,
	}
}

// ─── flattenTagItems ───────────────────────────────────────────────────────

// flattenTagItems post-processes FieldItems to render tag sections as flat
// Key:Value pairs instead of verbose Key/Value struct sub-fields.
// Only sections with Path "Tags" or "TagList" are flattened.
func flattenTagItems(items []fieldpath.FieldItem) []fieldpath.FieldItem {
	if len(items) == 0 {
		return items
	}
	result := make([]fieldpath.FieldItem, 0, len(items))
	i := 0
	for i < len(items) {
		item := items[i]
		if item.IsHeader && (item.Path == "Tags" || item.Path == "TagList") {
			result = append(result, item)
			i++
			parentPath := item.Path
			var subs []fieldpath.FieldItem
			for i < len(items) && items[i].IsSubField && items[i].Path == parentPath {
				subs = append(subs, items[i])
				i++
			}
			if len(subs) == 0 {
				continue
			}
			isSimpleTagList := false
			hasExtraFields := false
			for _, sub := range subs {
				trimmed := strings.TrimSpace(sub.Value)
				if rest, ok := strings.CutPrefix(trimmed, "- "); ok {
					k, _, hasSep := strings.Cut(rest, ":")
					if hasSep && strings.EqualFold(strings.TrimSpace(k), "key") {
						isSimpleTagList = true
					}
				} else {
					k, _, hasSep := strings.Cut(trimmed, ":")
					if hasSep {
						field := strings.ToLower(strings.TrimSpace(k))
						if field != "key" && field != "value" {
							hasExtraFields = true
						}
					}
				}
			}
			if !isSimpleTagList || hasExtraFields {
				result = append(result, subs...)
				continue
			}
			var currentKey string
			emitPending := func() {
				if currentKey != "" {
					result = append(result, fieldpath.FieldItem{
						IsSubField:  true,
						IndentLevel: 1,
						Key:         currentKey,
						Value:       "",
						Path:        parentPath,
					})
					currentKey = ""
				}
			}
			for _, sub := range subs {
				line := sub.Value
				trimmed := strings.TrimSpace(line)
				if trimmed == "" {
					continue
				}
				if rest, ok := strings.CutPrefix(trimmed, "- "); ok {
					emitPending()
					k, v, hasSep := strings.Cut(rest, ":")
					if !hasSep {
						continue
					}
					k = strings.TrimSpace(k)
					v = strings.TrimSpace(v)
					if strings.EqualFold(k, "key") {
						currentKey = v
					}
				} else {
					k, v, hasSep := strings.Cut(trimmed, ":")
					if !hasSep {
						continue
					}
					k = strings.TrimSpace(k)
					v = strings.TrimSpace(v)
					if strings.EqualFold(k, "value") && currentKey != "" {
						result = append(result, fieldpath.FieldItem{
							IsSubField:  true,
							IndentLevel: 1,
							Key:         currentKey,
							Value:       v,
							Path:        parentPath,
						})
						currentKey = ""
					}
				}
			}
			emitPending()
			continue
		}
		result = append(result, item)
		i++
	}
	return result
}

// ─── expandJSONItems ───────────────────────────────────────────────────────

// expandJSONItems detects JSON strings in field values and expands them as
// YAML-formatted sub-fields.
func expandJSONItems(items []fieldpath.FieldItem) []fieldpath.FieldItem {
	if len(items) == 0 {
		return items
	}
	result := make([]fieldpath.FieldItem, 0, len(items))
	for _, item := range items {
		if item.IsHeader || item.IsSection || item.IsNavigable {
			result = append(result, item)
			continue
		}
		if item.IsSubField {
			rawLine := item.Value
			trimmed := strings.TrimSpace(rawLine)
			trimmed = strings.TrimPrefix(trimmed, "- ")
			_, valuePart, hasSep := strings.Cut(trimmed, ":")
			if !hasSep {
				result = append(result, item)
				continue
			}
			valuePart = strings.TrimSpace(valuePart)
			lines := jsonyaml.TryJSONToYAMLLines(valuePart)
			if lines == nil {
				result = append(result, item)
				continue
			}
			keyPart, _, _ := strings.Cut(trimmed, ":")
			keyLine := keyPart + ":"
			result = append(result, fieldpath.FieldItem{
				Path:        item.Path,
				Key:         keyLine,
				Value:       keyLine,
				IsSubField:  true,
				IndentLevel: item.IndentLevel,
			})
			for _, line := range lines {
				leading := len(line) - len(strings.TrimLeft(line, " "))
				level := leading/jsonyaml.YAMLIndentSpaces + item.IndentLevel + 1
				result = append(result, fieldpath.FieldItem{
					Path:        item.Path,
					Key:         line,
					Value:       line,
					IsSubField:  true,
					IndentLevel: level,
				})
			}
			continue
		}
		lines := jsonyaml.TryJSONToYAMLLines(item.Value)
		if lines == nil {
			result = append(result, item)
			continue
		}
		result = append(result, fieldpath.FieldItem{
			Path:     item.Path,
			Key:      item.Key,
			IsHeader: true,
		})
		for _, line := range lines {
			leading := len(line) - len(strings.TrimLeft(line, " "))
			level := leading/jsonyaml.YAMLIndentSpaces + 1
			result = append(result, fieldpath.FieldItem{
				Path:        item.Path,
				Key:         line,
				Value:       line,
				IsSubField:  true,
				IndentLevel: level,
			})
		}
	}
	return result
}
