// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package fakes

import (
	"reflect"

	"github.com/k2m30/a9s/v3/core/catalog"
)

// listAnswer is items as shortName's list call returns them: a copy with
// every member the type's ListOmits names left unset. The fixtures hold the
// whole resource, and the Get or Describe call answers from them unchanged.
func listAnswer[T any](shortName string, items []T) []T {
	omits := catalog.FindAny(shortName).ListOmits
	out := make([]T, len(items))
	for i, item := range items {
		v := reflect.ValueOf(&item).Elem()
		for _, member := range omits {
			f := v.FieldByName(member)
			f.SetZero()
		}
		out[i] = item
	}
	return out
}
