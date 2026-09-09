// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// Package aws — catalog_degraded.go
//
// The catalog declarations for the shared degraded row. They live in a
// catalog_ file with every other FindingDef so a finding's wording is
// declared in one kind of place; the emitters in degraded_resource.go read
// them back through wave1Finding.
package aws

import (
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
)

// DetailsDeniedFindingDef returns the catalog declaration matching what
// DegradedDetailsDenied emits. Every adopting type lists this in its
// `Findings` slice so the coverage gates demand a demo witness for it.
// detail is the type's own S5 sentence; "" takes the name-only default.
func DetailsDeniedFindingDef(shortName, detail string) catalog.FindingDef {
	if detail == "" {
		detail = detailsDeniedDetail
	}
	return catalog.FindingDef{
		Code:     DetailsDeniedCode(shortName),
		Phrase:   detailsDeniedPhrase,
		Severity: domain.SevWarn,
		Source:   "wave1",
		Detail:   detail,
	}
}

// DetailsUnavailableFindingDef returns the catalog declaration matching what
// detailsUnavailableFinding emits. Every adopting type lists this alongside
// DetailsDeniedFindingDef in its `Findings` slice so the coverage gates
// demand a demo witness for the non-auth path too.
func DetailsUnavailableFindingDef(shortName string) catalog.FindingDef {
	return catalog.FindingDef{
		Code:     DetailsUnavailableCode(shortName),
		Phrase:   detailsUnavailablePhrase,
		Severity: domain.SevWarn,
		Source:   "wave1",
		Detail:   detailsUnavailableDetail,
	}
}
