// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package app

import (
	"reflect"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"gopkg.in/yaml.v3"

	"github.com/k2m30/a9s/v3/core/fieldpath"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
)

// CopyContent resolves the (content, label) pair for the copy action ('c' in
// the TUI, the web copy button) from the controller's current screen — the
// single source of truth both renderers delegate to, so a resolution fix
// (e.g. consulting the child-type registry for CopyField) lands for both at
// once instead of drifting between a TUI-local implementation and a web
// no-op.
func (c *Controller) CopyContent() (content, label string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.copyContent()
}

// copyContent is the lock-free core of CopyContent, also called by
// snapshot() to populate ViewState.CopyText/CopyLabel from the identical
// resolution. Callers must already hold c.mu for WRITE — copyContentDetail
// reaches buildDetailBody, which may maps.Copy into the shared resource's
// AttentionDetails map (detail_body.go), the same mutation-on-read hazard
// Snapshot() documents on its own write-lock choice.
func (c *Controller) copyContent() (content, label string) {
	if len(c.stack) == 0 {
		return "", ""
	}
	switch bodyKindForScreen(c.stack[len(c.stack)-1]) {
	case BodyKindList:
		return c.copyContentList()
	case BodyKindDetail:
		return c.copyContentDetail()
	case BodyKindText:
		return copyContentText(c.stack[len(c.stack)-1].ID, c.topTextState())
	case BodyKindIdentity:
		return copyContentIdentity(c.buildIdentityBody())
	default:
		return "", ""
	}
}

// copyContentList resolves copy content for the active resource/child list:
// the type's CopyField value when the catalog registers one and the selected
// row has it set, else the row's ID. The type lookup consults both the
// top-level and child catalog registries — every CopyField entry in the
// installed catalog is on a child type (core/aws/catalog_*.go) — so a
// top-level-only lookup would never resolve one. Callers must hold c.mu.
func (c *Controller) copyContentList() (string, string) {
	r, ok := c.listSelected()
	if !ok {
		return "", ""
	}
	typeName := c.stack[len(c.stack)-1].Ctx.ResourceType
	td := resource.FindResourceType(typeName)
	if td == nil {
		td = resource.GetChildType(typeName)
	}
	if td != nil && td.CopyField != "" {
		if val := r.Fields[td.CopyField]; val != "" {
			return val, "Copied: " + val
		}
	}
	return r.ID, "Copied: " + r.ID
}

// copyContentDetail resolves copy content for the active detail screen: the
// focused related row's DisplayName when the related panel has focus,
// otherwise the FieldCursor row's Value (falling back to its Key when Value
// is empty), otherwise the raw YAML of the detail resource. Callers must
// hold c.mu.
func (c *Controller) copyContentDetail() (string, string) {
	ds := c.topDetailState()
	if ds == nil {
		return "", ""
	}
	if ds.RelatedFocus {
		row := ds.focusedRelatedRow()
		if row == nil || row.DisplayName == "" {
			return "", ""
		}
		return row.DisplayName, "Copied: " + row.DisplayName
	}
	body := buildDetailBody(ds, c.viewConfig)
	if fc := body.FieldCursor; fc >= 0 && fc < len(body.Fields) {
		item := body.Fields[fc]
		val := item.Value
		if val == "" {
			val = item.Key
		}
		if val != "" {
			return val, "Copied: " + val
		}
	}
	content := detailResourceRawYAML(ds.Resource)
	if content == "" {
		return "", ""
	}
	return content, "Copied detail to clipboard"
}

// detailResourceRawYAML marshals res to YAML text for the detail-screen copy
// fallback, mirroring views.DetailModel.RawYAML's RawStruct-then-Fields
// precedence. Kept in core (duplicated rather than shared) so CopyContent
// never imports internal/tui.
func detailResourceRawYAML(res resource.Resource) string {
	var data []byte
	var err error
	if res.RawStruct != nil {
		safe := fieldpath.ToSafeValue(reflect.ValueOf(res.RawStruct))
		data, err = yaml.Marshal(safe)
	} else if len(res.Fields) > 0 {
		data, err = yaml.Marshal(res.Fields)
	}
	if err != nil || len(data) == 0 {
		return ""
	}
	return string(data)
}

// copyContentText resolves copy content for a text screen (YAML, JSON, or
// error log): every line joined with newlines, ANSI escape codes stripped.
// The label is screen-exact — BodyKindText covers all three screen IDs
// (bodyKindForScreen), so a bare "YAML" label was wrong for JSON/error-log.
func copyContentText(id runtime.ScreenID, ts *TextState) (string, string) {
	if ts == nil || len(ts.Lines) == 0 {
		return "", ""
	}
	var sb strings.Builder
	for i, line := range ts.Lines {
		if i > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(ansi.Strip(line))
	}
	label := "Copied text to clipboard"
	switch id {
	case runtime.ScreenYAML:
		label = "Copied YAML to clipboard"
	case runtime.ScreenJSON:
		label = "Copied JSON to clipboard"
	}
	return sb.String(), label
}

// copyContentIdentity resolves copy content for the identity screen: the
// resolved caller ARN once loaded, or ("", "") while loading or on an empty ARN.
func copyContentIdentity(body *IdentityBody) (string, string) {
	if body == nil || body.Loading || body.ARN == "" {
		return "", ""
	}
	return body.ARN, "Copied!"
}
