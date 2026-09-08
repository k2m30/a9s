// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package catalog

import "github.com/k2m30/a9s/v3/core/domain"

// detailByCode and phraseByCode index every installed definition's Detail and
// Phrase by finding code. Rebuilt whenever SetTypes or SetChildTypes installs a
// registry.
var (
	detailByCode   map[domain.FindingCode]string          //nolint:gochecknoglobals // process-scope catalog index, set with the registry
	phraseByCode   map[domain.FindingCode]string          //nolint:gochecknoglobals // process-scope catalog index, set with the registry
	severityByCode map[domain.FindingCode]domain.Severity //nolint:gochecknoglobals // process-scope catalog index, set with the registry
)

func indexDetails(types []ResourceTypeDef) {
	if detailByCode == nil {
		detailByCode = map[domain.FindingCode]string{}
	}
	if phraseByCode == nil {
		phraseByCode = map[domain.FindingCode]string{}
	}
	if severityByCode == nil {
		severityByCode = map[domain.FindingCode]domain.Severity{}
	}
	for _, t := range types {
		for _, f := range t.Findings {
			if f.Detail != "" {
				detailByCode[f.Code] = f.Detail
			}
			if f.Phrase != "" {
				phraseByCode[f.Code] = f.Phrase
			}
			severityByCode[f.Code] = f.Severity
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

// Phrase returns the display wording the installed catalog declares for code.
// An emitter whose condition reads the same however many items tripped it
// passes this rather than a wording of its own, so the row, the detail view and
// the generated signals page cannot drift apart. Codes whose wording carries a
// value the emitter measures ("<N> jobs failed in last 24h") build their phrase
// at the call site and are held to the declared shape by the demo-bench gate.
func Phrase(code domain.FindingCode) string {
	return phraseByCode[code]
}

// Severity returns the severity the installed catalog declares for code. An
// emitter whose condition picks between two tiers picks between two codes and
// reads each one's severity here, so a row's colour and the severity the
// generated signals page prints cannot come apart. Like Detail it does not
// panic on an uninstalled catalog; the zero Severity is what an unknown code
// has always meant.
func Severity(code domain.FindingCode) domain.Severity {
	return severityByCode[code]
}
