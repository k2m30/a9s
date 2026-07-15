// Package aws — degraded_resource.go
//
// The single truth source for the "details denied" degraded row: when a
// list call names a resource but the per-item describe fails (IAM denial,
// transient fault, nil body), the row is KEPT — name visible, cause phrase
// in the Status cell, the failure still aggregated into the composite fetch
// error. Dropping the row fakes an empty or shorter list: "you can't see
// it" must never render as "it isn't there" (live witness 2026-07-14: a
// readonly role allowed mwaa:ListEnvironments while denying
// airflow:GetEnvironment, and the environment vanished).
//
// Every N+1 fetcher (list-of-names + describe-per-name) MUST route its
// per-item failure path through DegradedDetailsDenied and declare
// DetailsDeniedFindingDef on its catalog literal, so the phrase, severity,
// and finding-code shape can never diverge per type.
package aws

import (
	"github.com/k2m30/a9s/v3/internal/catalog"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

const (
	detailsDeniedPhrase = "details denied"
	detailsDeniedDetail = "Access to resource details was denied; only the name is visible."
)

// DetailsDeniedCode returns the per-type FindingCode for the degraded row
// ("<shortName>.warn.details_denied").
func DetailsDeniedCode(shortName string) domain.FindingCode {
	return domain.FindingCode(shortName + ".warn.details_denied")
}

// DetailsDeniedFindingDef returns the catalog declaration matching what
// DegradedDetailsDenied emits. Every adopting type lists this in its
// `Findings` slice so the coverage gates demand a demo witness for it.
func DetailsDeniedFindingDef(shortName string) catalog.FindingDef {
	return catalog.FindingDef{
		Code:     DetailsDeniedCode(shortName),
		Phrase:   detailsDeniedPhrase,
		Severity: domain.SevWarn,
		Source:   "wave1",
	}
}

// detailsDeniedFinding builds the shared details-denied Finding for
// shortName, with detail as the per-type §4 S5 sentence.
func detailsDeniedFinding(shortName, detail string) domain.Finding {
	return domain.Finding{
		Code:     DetailsDeniedCode(shortName),
		Phrase:   detailsDeniedPhrase,
		Detail:   detail,
		Severity: domain.SevWarn,
		Source:   "wave1",
	}
}

// DegradedDetailsDenied builds the name-only row an N+1 fetcher emits when
// the per-item describe for id failed. raw is a minimal typed SDK value
// carrying the identifier (so detail/YAML views render something honest);
// pass the zero struct with the name set.
func DegradedDetailsDenied(shortName, id string, raw any) resource.Resource {
	return resource.Resource{
		ID:   id,
		Name: id,
		Fields: map[string]string{
			"name":   id,
			"status": detailsDeniedPhrase,
		},
		RawStruct: raw,
		Findings:  []domain.Finding{detailsDeniedFinding(shortName, detailsDeniedDetail)},
	}
}
