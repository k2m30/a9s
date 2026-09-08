// SPDX-License-Identifier: GPL-3.0-or-later

// detail_render.go contains YAML generation and config-driven rendering for DetailModel.
// Specifically: RawYAML and computeKeyWidth.
package views

import (
	"reflect"

	"github.com/k2m30/a9s/v3/core/fieldpath"
	"github.com/k2m30/a9s/v3/internal/tui/text"

	"gopkg.in/yaml.v3"
)

// RawYAML returns the resource as YAML for clipboard copy (same format as YAML view).
func (m DetailModel) RawYAML() string {
	var data []byte
	var err error

	if m.res.RawStruct != nil {
		safe := fieldpath.ToSafeValue(reflect.ValueOf(m.res.RawStruct))
		data, err = yaml.Marshal(safe)
	} else if len(m.res.Fields) > 0 {
		data, err = yaml.Marshal(m.res.Fields)
	}

	if err != nil || len(data) == 0 {
		return ""
	}
	return string(data)
}

// detailKeyFloor is the narrowest the detail key column gets. A resource whose
// longest field name is two characters would otherwise crowd its values against
// the left edge, and they would sit in a different place on every screen.
const detailKeyFloor = 22

// computeKeyWidth returns the width of the key column: the widest key plus its
// colon, never under detailKeyFloor and never over two fifths of the viewport.
// The bound is what keeps a field name wider than the terminal from reserving
// the whole line and leaving the value — the thing the reader opened the
// detail view for — off the right edge. Measured in terminal columns, because
// that is what the padding under it fills: a key counted in bytes reserves
// three times the room a CJK field name paints. A viewport of zero is a model
// that has not been sized yet and bounds nothing.
func computeKeyWidth(keys []string, viewport int) int {
	w := detailKeyFloor
	for _, k := range keys {
		w = max(w, text.Width(k)+1)
	}
	if viewport > 0 {
		return min(w, viewport*2/5)
	}
	return w
}
