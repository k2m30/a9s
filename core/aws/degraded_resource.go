// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

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
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
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

// detailsDeniedFinding builds the shared details-denied Finding for shortName.
func detailsDeniedFinding(shortName string) domain.Finding {
	code := DetailsDeniedCode(shortName)
	return domain.Finding{
		Code:     code,
		Phrase:   detailsDeniedPhrase,
		Detail:   catalog.Detail(code),
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
		Detail:   detailsUnavailableDetail,
	}
}

// detailsUnavailableFinding builds the shared details-unavailable Finding
// for shortName.
func detailsUnavailableFinding(shortName string) domain.Finding {
	code := DetailsUnavailableCode(shortName)
	return domain.Finding{
		Code:     code,
		Phrase:   detailsUnavailablePhrase,
		Detail:   catalog.Detail(code),
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
func degradedDetailsFinding(shortName string, err error) domain.Finding {
	if degradedAuthDenial(err) {
		return detailsDeniedFinding(shortName)
	}
	return detailsUnavailableFinding(shortName)
}

// DegradedDetails builds the name-only row an N+1 fetcher emits when the
// per-item describe for id failed or came back empty. err is the describe
// call's error (nil for an absent-from-response row); it decides whether the
// row renders "details denied" or "details unavailable".
//
// RawStruct stays nil on purpose. A synthetic struct carrying only the name
// is assertable, so every pivot that reads a field off the struct would find
// it empty and answer a confident zero — "this node group has no IAM role" —
// about a resource nobody was allowed to describe. Nil makes those pivots
// answer Unknown instead. Identity a pivot can legitimately use goes in
// Fields, which the disk cache preserves and RawStruct does not.
func DegradedDetails(shortName, id string, err error) resource.Resource {
	finding := degradedDetailsFinding(shortName, err)
	return resource.Resource{
		ID:   id,
		Name: id,
		Fields: map[string]string{
			"name":   id,
			"status": finding.Phrase,
		},
		Findings: []domain.Finding{finding},
	}
}
