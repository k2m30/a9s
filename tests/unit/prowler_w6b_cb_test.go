package unit

// prowler_w6b_cb_test.go — behavioural pins for the four cb posture signals
// of batch w6b: public builds, a repo-controlled buildspec, a credential in
// the source address and a credential in a plaintext environment variable.
//
// All four are wave 1. FetchCodeBuildProjectsPage calls BatchGetProjects and
// keeps the whole Project, so every condition reads data the fetcher already
// holds; computing them in EnrichCodeBuildStatus instead would mean a second
// BatchGetProjects for the same bytes. That is why these assert
// Source == "wave1" and drive the fetcher rather than the enricher.

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/codebuild"
	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

const (
	w6bCBCodePublicBuilds       = domain.FindingCode("cb.public-builds")
	w6bCBCodeBuildspecFromSrc   = domain.FindingCode("cb.buildspec-from-source")
	w6bCBCodeSourceURLCredetial = domain.FindingCode("cb.source-url-credential")
	w6bCBCodeEnvSecret          = domain.FindingCode("cb.env-secret")
)

// Registered phrases. cb.source-url-credential does not read "credential in
// source URL" as the batch table first wrote it: "URL" is a bare uppercase
// token the rendered-surface ruling bans from every phrase.
const (
	w6bCBPhrasePublicBuilds     = "build results publicly visible"
	w6bCBPhraseBuildspecFromSrc = "buildspec taken from the source repository"
	w6bCBPhraseSourceURLCred    = "credential in the source repository address"
	w6bCBPhraseEnvSecret        = "credential in environment variables"
)

// w6bCBFake serves ListProjects + BatchGetProjects for the wave-1 fetcher.
type w6bCBFake struct {
	projects []cbtypes.Project
}

func (f *w6bCBFake) ListProjects(_ context.Context, _ *codebuild.ListProjectsInput, _ ...func(*codebuild.Options)) (*codebuild.ListProjectsOutput, error) {
	names := make([]string, 0, len(f.projects))
	for _, p := range f.projects {
		names = append(names, aws.ToString(p.Name))
	}
	return &codebuild.ListProjectsOutput{Projects: names}, nil
}

func (f *w6bCBFake) BatchGetProjects(_ context.Context, in *codebuild.BatchGetProjectsInput, _ ...func(*codebuild.Options)) (*codebuild.BatchGetProjectsOutput, error) {
	want := make(map[string]bool, len(in.Names))
	for _, n := range in.Names {
		want[n] = true
	}
	var out []cbtypes.Project
	for _, p := range f.projects {
		if want[aws.ToString(p.Name)] {
			out = append(out, p)
		}
	}
	return &codebuild.BatchGetProjectsOutput{Projects: out}, nil
}

// w6bCBProject builds a healthy private project: an inline buildspec, a
// credential-free source address and no plaintext environment variables.
func w6bCBProject(name string) cbtypes.Project {
	return cbtypes.Project{
		Name:              aws.String(name),
		Arn:               aws.String("arn:aws:codebuild:us-east-1:123456789012:project/" + name),
		Description:       aws.String("acme " + name),
		LastModified:      aws.Time(time.Now().Add(-14 * 24 * time.Hour)),
		ProjectVisibility: cbtypes.ProjectVisibilityTypePrivate,
		Source: &cbtypes.ProjectSource{
			Type:      cbtypes.SourceTypeGithub,
			Location:  aws.String("https://github.com/acme/" + name + ".git"),
			Buildspec: aws.String("version: 0.2\nphases:\n  build:\n    commands:\n      - make ci\n"),
		},
		Environment: &cbtypes.ProjectEnvironment{
			Type:        cbtypes.EnvironmentTypeLinuxContainer,
			Image:       aws.String("aws/codebuild/standard:7.0"),
			ComputeType: cbtypes.ComputeTypeBuildGeneral1Small,
		},
	}
}

func w6bFetchCB(t *testing.T, projects ...cbtypes.Project) []resource.Resource {
	t.Helper()
	fake := &w6bCBFake{projects: projects}
	out, err := awsclient.FetchCodeBuildProjectsPage(context.Background(), fake, fake, "")
	if err != nil {
		t.Fatalf("FetchCodeBuildProjectsPage: %v", err)
	}
	return out.Resources
}

// ─── row 3: cb.public-builds ────────────────────────────────────────────────

// A PUBLIC_READ project publishes its build logs and artifacts to anyone with
// the URL, which is where pipeline environment dumps leak.
func TestW6BCB_PublicBuilds_PublicRead(t *testing.T) {
	const name = "acme-docs-publish"
	p := w6bCBProject(name)
	p.ProjectVisibility = cbtypes.ProjectVisibilityTypePublicRead
	rs := w6bFetchCB(t, p)
	r := pw1ResourceByID(t, rs, name)

	pw1RequireFinding(t, r.Findings, w6bCBCodePublicBuilds,
		w6bCBPhrasePublicBuilds, domain.SevBroken, "wave1")
	// "Build visibility: public" under "build results publicly visible" is the
	// same fact twice, and the raw PUBLIC_READ the table proposed is a banned
	// enum on a rendered surface.
	w6bRequireNoRows(t, w6bWave1Rows(r, w6bCBCodePublicBuilds))
}

func TestW6BCB_PrivateProject_IsHealthy(t *testing.T) {
	const name = "acme-frontend-build"
	rs := w6bFetchCB(t, w6bCBProject(name))
	pw1RequireNoFinding(t, pw1ResourceByID(t, rs, name).Findings, w6bCBCodePublicBuilds)
}

// AWS leaves Visibility empty on projects created before the setting existed;
// unknown is not public.
func TestW6BCB_VisibilityUnset_IsHealthy(t *testing.T) {
	const name = "acme-ancient-project"
	p := w6bCBProject(name)
	p.ProjectVisibility = ""
	rs := w6bFetchCB(t, p)
	pw1RequireNoFinding(t, pw1ResourceByID(t, rs, name).Findings, w6bCBCodePublicBuilds)
}

// ─── row 4: cb.buildspec-from-source ────────────────────────────────────────

// A buildspec path resolves inside the repository, so whoever can open a pull
// request can rewrite the commands the build runs with the project's role.
func TestW6BCB_BuildspecFromSource_PathInRepo(t *testing.T) {
	const name = "acme-frontend-build"
	p := w6bCBProject(name)
	p.Source.Buildspec = aws.String("ci/buildspec.yml")
	rs := w6bFetchCB(t, p)
	r := pw1ResourceByID(t, rs, name)

	pw1RequireFinding(t, r.Findings, w6bCBCodeBuildspecFromSrc,
		w6bCBPhraseBuildspecFromSrc, domain.SevWarn, "wave1")
	w6bRequireRowValue(t, w6bWave1Rows(r, w6bCBCodeBuildspecFromSrc), "ci/buildspec.yml")
}

// An empty Buildspec means CodeBuild reads buildspec.yml from the repository
// root — the same exposure, with nothing in the API to show for it, so the row
// has to say which file will be used.
func TestW6BCB_BuildspecFromSource_EmptyMeansRepoDefault(t *testing.T) {
	const name = "acme-default-buildspec"
	p := w6bCBProject(name)
	p.Source.Buildspec = nil
	rs := w6bFetchCB(t, p)
	r := pw1ResourceByID(t, rs, name)

	pw1RequireFinding(t, r.Findings, w6bCBCodeBuildspecFromSrc,
		w6bCBPhraseBuildspecFromSrc, domain.SevWarn, "wave1")
	w6bRequireRowValue(t, w6bWave1Rows(r, w6bCBCodeBuildspecFromSrc), "repo default")
}

// An inline buildspec lives in the project definition, so changing it needs
// CodeBuild permissions, not a pull request.
func TestW6BCB_InlineBuildspec_IsHealthy(t *testing.T) {
	const name = "acme-inline-spec"
	rs := w6bFetchCB(t, w6bCBProject(name))
	pw1RequireNoFinding(t, pw1ResourceByID(t, rs, name).Findings, w6bCBCodeBuildspecFromSrc)
}

// The exposure is "a contributor controls the file". An S3 or NO_SOURCE
// project has no contributors, so a buildspec path there is not this finding.
func TestW6BCB_NonRepoSourceTypes_NotEvaluated(t *testing.T) {
	for _, st := range []cbtypes.SourceType{cbtypes.SourceTypeS3, cbtypes.SourceTypeNoSource} {
		t.Run(string(st), func(t *testing.T) {
			name := "acme-source-" + string(st)
			p := w6bCBProject(name)
			p.Source.Type = st
			p.Source.Buildspec = aws.String("buildspec.yml")
			rs := w6bFetchCB(t, p)
			pw1RequireNoFinding(t, pw1ResourceByID(t, rs, name).Findings, w6bCBCodeBuildspecFromSrc)
		})
	}
}

// Every source type a pull-request author can write to is in scope.
func TestW6BCB_AllContributorSourceTypes_Evaluated(t *testing.T) {
	for _, st := range []cbtypes.SourceType{
		cbtypes.SourceTypeGithub, cbtypes.SourceTypeGithubEnterprise,
		cbtypes.SourceTypeBitbucket, cbtypes.SourceTypeCodecommit,
	} {
		t.Run(string(st), func(t *testing.T) {
			name := "acme-contrib-" + string(st)
			p := w6bCBProject(name)
			p.Source.Type = st
			p.Source.Buildspec = aws.String("buildspec.yaml")
			rs := w6bFetchCB(t, p)
			pw1RequireFinding(t, pw1ResourceByID(t, rs, name).Findings,
				w6bCBCodeBuildspecFromSrc, w6bCBPhraseBuildspecFromSrc, domain.SevWarn, "wave1")
		})
	}
}

// A project with no Source at all must not panic and must not be flagged.
func TestW6BCB_NilSource_IsHealthy(t *testing.T) {
	const name = "acme-no-source"
	p := w6bCBProject(name)
	p.Source = nil
	rs := w6bFetchCB(t, p)
	r := pw1ResourceByID(t, rs, name)
	pw1RequireNoFinding(t, r.Findings, w6bCBCodeBuildspecFromSrc)
	pw1RequireNoFinding(t, r.Findings, w6bCBCodeSourceURLCredetial)
}

// ─── row 5: cb.source-url-credential ────────────────────────────────────────

// A token in the clone address is readable by anyone who can describe the
// project, and it is usually a long-lived personal access token.
func TestW6BCB_SourceURLCredential_TokenInUserinfo(t *testing.T) {
	const name = "acme-legacy-mirror"
	const token = "ghp_aaaabbbbccccddddeeeeffff0000111122"
	p := w6bCBProject(name)
	p.Source.Location = aws.String("https://" + token + ":x-oauth-basic@github.com/acme/legacy-mirror.git")
	rs := w6bFetchCB(t, p)
	r := pw1ResourceByID(t, rs, name)

	f := pw1RequireFinding(t, r.Findings, w6bCBCodeSourceURLCredetial,
		w6bCBPhraseSourceURLCred, domain.SevBroken, "wave1")
	rows := w6bWave1Rows(r, w6bCBCodeSourceURLCredetial)
	// The row exists to show operators which repository to re-point; it shows
	// the address with the credential stripped, never the credential.
	w6bRequireRowValue(t, rows, "https://github.com/acme/legacy-mirror.git")
	w6bRequireNoSecretLeak(t, f, rows, token)
}

func TestW6BCB_SourceURLCredential_PasswordInUserinfo(t *testing.T) {
	const name = "acme-mirror-basic"
	const pw = "hunter2hunter2"
	p := w6bCBProject(name)
	p.Source.Location = aws.String("https://acmebot:" + pw + "@bitbucket.org/acme/mirror.git")
	p.Source.Type = cbtypes.SourceTypeBitbucket
	rs := w6bFetchCB(t, p)
	r := pw1ResourceByID(t, rs, name)

	f := pw1RequireFinding(t, r.Findings, w6bCBCodeSourceURLCredetial,
		w6bCBPhraseSourceURLCred, domain.SevBroken, "wave1")
	rows := w6bWave1Rows(r, w6bCBCodeSourceURLCredetial)
	w6bRequireRowValue(t, rows, "https://bitbucket.org/acme/mirror.git")
	w6bRequireNoSecretLeak(t, f, rows, pw)
}

func TestW6BCB_PlainSourceURL_IsHealthy(t *testing.T) {
	const name = "acme-clean-mirror"
	rs := w6bFetchCB(t, w6bCBProject(name))
	pw1RequireNoFinding(t, pw1ResourceByID(t, rs, name).Findings, w6bCBCodeSourceURLCredetial)
}

// A CodeCommit or S3 location is not a URL with userinfo; parsing must not
// invent a credential out of it.
func TestW6BCB_NonURLSourceLocation_IsHealthy(t *testing.T) {
	const name = "acme-s3-source"
	p := w6bCBProject(name)
	p.Source.Type = cbtypes.SourceTypeS3
	p.Source.Location = aws.String("acme-build-inputs/source.zip")
	rs := w6bFetchCB(t, p)
	pw1RequireNoFinding(t, pw1ResourceByID(t, rs, name).Findings, w6bCBCodeSourceURLCredetial)
}

// ─── row 6: cb.env-secret ───────────────────────────────────────────────────

// A PLAINTEXT environment variable is stored on the project and printed in
// build logs, so a password there is a password published to every build.
func TestW6BCB_EnvSecret_PlaintextVariable(t *testing.T) {
	const name = "acme-integration-tests"
	const secret = "hunter2hunter2"
	p := w6bCBProject(name)
	p.Environment.EnvironmentVariables = []cbtypes.EnvironmentVariable{
		{Name: aws.String("AWS_REGION"), Value: aws.String("us-east-1"), Type: cbtypes.EnvironmentVariableTypePlaintext},
		{Name: aws.String("DB_PASSWORD"), Value: aws.String(secret), Type: cbtypes.EnvironmentVariableTypePlaintext},
	}
	rs := w6bFetchCB(t, p)
	r := pw1ResourceByID(t, rs, name)

	f := pw1RequireFinding(t, r.Findings, w6bCBCodeEnvSecret,
		w6bCBPhraseEnvSecret, domain.SevBroken, "wave1")
	rows := w6bWave1Rows(r, w6bCBCodeEnvSecret)
	pw1RequireRow(t, rows, "DB_PASSWORD", "keyword")
	w6bRequireNoSecretLeak(t, f, rows, secret)
}

// PARAMETER_STORE and SECRETS_MANAGER variables hold a reference, not a value.
// Flagging them would report the recommended fix as the defect.
func TestW6BCB_EnvSecretReferences_AreHealthy(t *testing.T) {
	for _, typ := range []cbtypes.EnvironmentVariableType{
		cbtypes.EnvironmentVariableTypeParameterStore,
		cbtypes.EnvironmentVariableTypeSecretsManager,
	} {
		t.Run(string(typ), func(t *testing.T) {
			name := "acme-ref-" + string(typ)
			p := w6bCBProject(name)
			p.Environment.EnvironmentVariables = []cbtypes.EnvironmentVariable{
				{Name: aws.String("DB_PASSWORD"), Value: aws.String("/acme/prod/db-password"), Type: typ},
			}
			rs := w6bFetchCB(t, p)
			pw1RequireNoFinding(t, pw1ResourceByID(t, rs, name).Findings, w6bCBCodeEnvSecret)
		})
	}
}

func TestW6BCB_NilEnvironment_IsHealthy(t *testing.T) {
	const name = "acme-no-env"
	p := w6bCBProject(name)
	p.Environment = nil
	rs := w6bFetchCB(t, p)
	pw1RequireNoFinding(t, pw1ResourceByID(t, rs, name).Findings, w6bCBCodeEnvSecret)
}

// ─── independence ───────────────────────────────────────────────────────────

// Contract rule 4: four conditions on one project are four findings.
func TestW6BCB_AllFourConditions_ProduceFourFindings(t *testing.T) {
	const name = "acme-worst-case"
	p := w6bCBProject(name)
	p.ProjectVisibility = cbtypes.ProjectVisibilityTypePublicRead
	p.Source.Buildspec = aws.String("buildspec.yml")
	p.Source.Location = aws.String("https://acmebot:hunter2hunter2@github.com/acme/worst.git")
	p.Environment.EnvironmentVariables = []cbtypes.EnvironmentVariable{
		{Name: aws.String("API_KEY"), Value: aws.String("swordfish99"), Type: cbtypes.EnvironmentVariableTypePlaintext},
	}
	rs := w6bFetchCB(t, p)
	r := pw1ResourceByID(t, rs, name)

	pw1RequireFinding(t, r.Findings, w6bCBCodePublicBuilds, w6bCBPhrasePublicBuilds, domain.SevBroken, "wave1")
	pw1RequireFinding(t, r.Findings, w6bCBCodeBuildspecFromSrc, w6bCBPhraseBuildspecFromSrc, domain.SevWarn, "wave1")
	pw1RequireFinding(t, r.Findings, w6bCBCodeSourceURLCredetial, w6bCBPhraseSourceURLCred, domain.SevBroken, "wave1")
	pw1RequireFinding(t, r.Findings, w6bCBCodeEnvSecret, w6bCBPhraseEnvSecret, domain.SevBroken, "wave1")
}

// ─── catalog ────────────────────────────────────────────────────────────────

func TestW6BCB_FindingDefsRegistered(t *testing.T) {
	w2AssertFindingDef(t, "cb", string(w6bCBCodePublicBuilds), w6bCBPhrasePublicBuilds, domain.SevBroken, "wave1")
	w2AssertFindingDef(t, "cb", string(w6bCBCodeBuildspecFromSrc), w6bCBPhraseBuildspecFromSrc, domain.SevWarn, "wave1")
	w2AssertFindingDef(t, "cb", string(w6bCBCodeSourceURLCredetial), w6bCBPhraseSourceURLCred, domain.SevBroken, "wave1")
	w2AssertFindingDef(t, "cb", string(w6bCBCodeEnvSecret), w6bCBPhraseEnvSecret, domain.SevBroken, "wave1")
}
