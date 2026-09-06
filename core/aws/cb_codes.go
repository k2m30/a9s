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
const (
	cbPublicBuildsDetail = "Build logs, environment variables and artifacts for this project are readable by anyone on the internet without an AWS account, so any credential or internal hostname a build prints is public. Set the project's visibility back to private and rotate anything the logs have already exposed."

	cbBuildspecFromSourceDetail = "The build instructions come from a file in the source repository, so anyone who can open a pull request can change what runs inside the build role. Move the buildspec inline into the project definition, or restrict who can trigger builds from unmerged branches."

	//nolint:gosec // G101 false positive: operator prose about a credential, not one
	cbSourceURLCredentialDetail = "The source repository address embeds a username and password or token, which is stored in the project definition and printed in build logs in clear text. Move the credential into a CodeBuild source credential or Secrets Manager entry and rotate it, because it must be assumed leaked."

	//nolint:gosec // G101 false positive: operator prose about a credential, not one
	cbEnvSecretDetail = "A plaintext environment variable on this project holds what looks like a credential; every build log and anyone who can read the project definition sees its value. Move it to Secrets Manager or Parameter Store, reference it by type, and rotate the exposed value."
)
