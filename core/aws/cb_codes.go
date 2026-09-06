// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// cb_codes.go — canonical FindingCode constants for the cb resource type.
//
// All four are wave 1: FetchCodeBuildProjectsPage calls BatchGetProjects and
// keeps the whole cbtypes.Project in RawStruct (cb.go:51,93), so every
// condition below is readable from data the fetcher already holds and needs
// no second call. EnrichCodeBuildStatus reads builds, not projects.
package aws

import "github.com/k2m30/a9s/v3/core/domain"

const (
	// CodeCBPublicBuilds — Project.ProjectVisibility is PUBLIC_READ.
	CodeCBPublicBuilds domain.FindingCode = "cb.public-builds"

	// CodeCBBuildspecFromSource — Project.Source.Buildspec names a file in
	// the source repository (or is empty, meaning the repo's buildspec.yml)
	// on a source type whose contents a pull-request author controls.
	CodeCBBuildspecFromSource domain.FindingCode = "cb.buildspec-from-source"

	// CodeCBSourceURLCredential — Project.Source.Location carries userinfo.
	//nolint:gosec // G101 false positive: a finding code, not a credential
	CodeCBSourceURLCredential domain.FindingCode = "cb.source-url-credential"

	// CodeCBEnvSecret — a PLAINTEXT environment variable holds a credential.
	//nolint:gosec // G101 false positive: a finding code, not a credential
	CodeCBEnvSecret domain.FindingCode = "cb.env-secret"
)

// S5 operator sentences.
