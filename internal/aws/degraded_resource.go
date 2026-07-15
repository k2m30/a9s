// Package aws — degraded_resource.go
//
// The single truth source for the "details denied"/"details unavailable"
// degraded row: when a list call names a resource but the per-item describe
// fails (IAM denial, transient fault, nil body, absent from the response),
// the row is KEPT — name visible, cause phrase in the Status cell, the
// failure still aggregated into the composite fetch error. Dropping the row
// fakes an empty or shorter list: "you can't see it" must never render as
// "it isn't there" (live witness 2026-07-14: a readonly role allowed
// mwaa:ListEnvironments while denying airflow:GetEnvironment, and the
// environment vanished).
//
// The two phrases are not interchangeable: an authorization denial
// (degradedAuthDenial — AccessDenied/AccessDeniedException, or EC2's
// UnauthorizedOperation) is a "details denied" row; everything else — nil
// error, throttling, cancellation, nil response body — is a "details
// unavailable" row. degradedDetailsFinding is the one place that decides
// which.
//
// Every N+1 fetcher (list-of-names + describe-per-name) MUST route its
// per-item failure path through DegradedDetails (or its own
// degradedDetailsFinding-based builder) and declare both
// DetailsDeniedFindingDef and DetailsUnavailableFindingDef on its catalog
// literal, so the phrase, severity, and finding-code shape can never diverge
// per type.
package aws

import (
	"github.com/k2m30/a9s/v3/internal/catalog"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

const (
	detailsDeniedPhrase = "details denied"
	detailsDeniedDetail = "Access to resource details was denied; only the name is visible."

	detailsUnavailablePhrase = "details unavailable"
	detailsUnavailableDetail = "Details could not be retrieved; only the name is visible."
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

// DetailsUnavailableCode returns the per-type FindingCode for the neutral
// degraded row ("<shortName>.warn.details_unavailable").
func DetailsUnavailableCode(shortName string) domain.FindingCode {
	return domain.FindingCode(shortName + ".warn.details_unavailable")
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
	}
}

// detailsUnavailableFinding builds the shared details-unavailable Finding
// for shortName, with detail as the per-type §4 S5 sentence.
func detailsUnavailableFinding(shortName, detail string) domain.Finding {
	return domain.Finding{
		Code:     DetailsUnavailableCode(shortName),
		Phrase:   detailsUnavailablePhrase,
		Detail:   detail,
		Severity: domain.SevWarn,
		Source:   "wave1",
	}
}

// degradedAuthDenial reports whether err is an authorization denial, across
// the SDK's service-specific codes: AccessDenied/AccessDeniedException (most
// services) and UnauthorizedOperation (EC2 — lt's DescribeLaunchTemplateVersions
// is EC2-backed and never returns AccessDenied). Any other error — or a nil
// body (nil err) — is a non-auth failure and yields details_unavailable.
// Deliberately local: accessDeniedErr (kms.go) stays KMS-only, its semantics
// untouched.
func degradedAuthDenial(err error) bool {
	code, _, _ := ClassifyAWSError(err)
	switch code {
	case "AccessDenied", "AccessDeniedException", "UnauthorizedOperation":
		return true
	default:
		return false
	}
}

// degradedDetailsFinding classifies a per-item describe failure: an
// authorization denial (degradedAuthDenial) renders the "details denied"
// finding; everything else — nil error (absent from a batch response),
// throttling, cancellation, a nil response body — renders the neutral
// "details unavailable" finding. "You can't see it" (denied) and "it didn't
// come back" (unavailable) are different facts and must never share one
// phrase.
func degradedDetailsFinding(shortName string, err error, deniedDetail, unavailableDetail string) domain.Finding {
	if degradedAuthDenial(err) {
		return detailsDeniedFinding(shortName, deniedDetail)
	}
	return detailsUnavailableFinding(shortName, unavailableDetail)
}

// DegradedDetails builds the name-only row an N+1 fetcher emits when the
// per-item describe for id failed or came back empty. raw is a minimal
// typed SDK value carrying the identifier (so detail/YAML views render
// something honest); pass the zero struct with the name set. err is the
// describe call's error (nil for an absent-from-response row); it decides
// whether the row renders "details denied" or "details unavailable".
func DegradedDetails(shortName, id string, raw any, err error) resource.Resource {
	finding := degradedDetailsFinding(shortName, err, detailsDeniedDetail, detailsUnavailableDetail)
	return resource.Resource{
		ID:   id,
		Name: id,
		Fields: map[string]string{
			"name":   id,
			"status": finding.Phrase,
		},
		RawStruct: raw,
		Findings:  []domain.Finding{finding},
	}
}
