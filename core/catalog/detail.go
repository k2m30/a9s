// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package catalog

import "github.com/k2m30/a9s/v3/core/domain"

// detailByCode indexes every installed definition's Detail by finding code.
// Rebuilt whenever SetTypes or SetChildTypes installs a registry.
var detailByCode map[domain.FindingCode]string //nolint:gochecknoglobals // process-scope catalog index, set with the registry

func indexDetails(types []ResourceTypeDef) {
	if detailByCode == nil {
		detailByCode = map[domain.FindingCode]string{}
	}
	for _, t := range types {
		for _, f := range t.Findings {
			if f.Detail != "" {
				detailByCode[f.Code] = f.Detail
			}
		}
	}
}

// Detail returns the S5 sentence the installed catalog declares for code, or
// "" when no definition carries one. Emitters read the sentence here so it
// has exactly one owner. Unlike Find it does not panic on an uninstalled
// catalog: a fetcher under a narrow unit test has nothing to decorate.
func Detail(code domain.FindingCode) string {
	return detailByCode[code]
}
