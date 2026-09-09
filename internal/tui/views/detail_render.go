// SPDX-License-Identifier: GPL-3.0-or-later

// detail_render.go contains YAML generation and config-driven rendering for DetailModel.
// Specifically: RawYAML.
package views

import (
	"reflect"

	"github.com/k2m30/a9s/v3/core/fieldpath"

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
