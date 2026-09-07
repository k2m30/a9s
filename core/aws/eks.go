// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"

	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// FetchEKSClustersPage fetches a single page of EKS clusters using the registered
// paginated fetcher pattern. For each cluster name returned by ListClusters,
// DescribeCluster is called. Per-item describe failures are aggregated into a
// composite error returned alongside partial results (E2, E3, E5).
func FetchEKSClustersPage(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
	input := &eks.ListClustersInput{
		MaxResults: aws.Int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.NextToken = aws.String(continuationToken)
	}

	listOutput, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*eks.ListClustersOutput, error) {
		return c.EKS.ListClusters(ctx, input)
	})
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("listing EKS clusters: %w", err)
	}

	// The support state is a property of a Kubernetes minor, not of a cluster,
	// so the catalogue is read once for the whole page and every cluster is
	// classified from the same answer.
	versions := eksVersionCatalogue(ctx, c.EKS)

	total := len(listOutput.Clusters)
	var resources []resource.Resource
	var failures []string
	for _, name := range listOutput.Clusters {
		descOutput, descErr := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*eks.DescribeClusterOutput, error) {
			return c.EKS.DescribeCluster(ctx, &eks.DescribeClusterInput{
				Name: aws.String(name),
			})
		})
		if descErr != nil {
			failures = append(failures, fmt.Sprintf("%s: %s", name, descErr.Error()))
			resources = append(resources, DegradedDetails("eks", name, descErr))
			continue
		}
		if descOutput.Cluster == nil {
			failures = append(failures, fmt.Sprintf("%s: nil cluster in response", name))
			resources = append(resources, DegradedDetails("eks", name, nil))
			continue
		}
		resources = append(resources, buildEKSResource(name, descOutput.Cluster, versions))
	}

	isTruncated := listOutput.NextToken != nil
	var nextToken string
	if listOutput.NextToken != nil {
		nextToken = *listOutput.NextToken
	}

	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: isTruncated,
			NextToken:   nextToken,
			PageSize:    len(resources),
			TotalHint:   -1,
		},
	}, AggregateFailures("eks: DescribeCluster", failures, total)
}

// buildEKSResource constructs a Resource from a cluster name and EKS Cluster struct.
func buildEKSResource(name string, cluster *ekstypes.Cluster, versions map[string]ekstypes.ClusterVersionInformation) resource.Resource {
	clusterName := ""
	if cluster.Name != nil {
		clusterName = *cluster.Name
	}

	version := ""
	if cluster.Version != nil {
		version = *cluster.Version
	}

	status := string(cluster.Status)

	endpoint := ""
	if cluster.Endpoint != nil {
		endpoint = *cluster.Endpoint
	}

	platformVersion := ""
	if cluster.PlatformVersion != nil {
		platformVersion = *cluster.PlatformVersion
	}

	// Wave 2: health.issues[] — populated by DescribeCluster (called per cluster in fetcher).
	healthIssuesCount := 0
	var issueCodes []string
	if cluster.Health != nil {
		for _, issue := range cluster.Health.Issues {
			healthIssuesCount++
			issueCodes = append(issueCodes, domain.HumanizeStatusPhrase(string(issue.Code)))
		}
	}

	subnetIDs := ""
	if cluster.ResourcesVpcConfig != nil {
		subnetIDs = strings.Join(cluster.ResourcesVpcConfig.SubnetIds, ",")
	}

	posture := eksPostureOf(cluster, versions)

	r := resource.Resource{
		ID:   name,
		Name: clusterName,
		// Status intentionally unset — lifecycle state is emitted as a Finding.
		Fields: map[string]string{
			"cluster_name":        clusterName,
			"version":             version,
			"status":              status,
			"endpoint":            endpoint,
			"platform_version":    platformVersion,
			"arn":                 aws.ToString(cluster.Arn),
			"health_issues_count": strconv.Itoa(healthIssuesCount),
			"health_issues":       strings.Join(issueCodes, ", "),
			"subnet_ids":          subnetIDs,
			// The four posture verdicts as words. colorEKSCluster's fallback
			// runs eksClusterFindings over Fields, so a row stripped of its
			// findings has to be able to recover them; the config they are
			// derived from is on the DescribeCluster struct and nowhere else.
			"public_endpoint":       posture.PublicEndpoint,
			"control_plane_logging": posture.ControlPlaneLogging,
			"secrets_encryption":    posture.SecretsEncryption,
			"version_support":       posture.VersionSupport,
		},
		RawStruct: cluster,
	}

	var issueRows []domain.DetailRow
	r.Findings, issueRows = eksClusterFindings(status, healthIssuesCount, issueCodes, version, posture)
	addWave1Rows(&r, eksIssueRowCode(status), issueRows...)
	addEKSPostureRows(&r, cluster, versions)
	return r
}

// eksControlPlaneLogTypes is the full set AWS emits. "Complete" means all
// five; anything less leaves a gap in the record of an incident.
var eksControlPlaneLogTypes = []ekstypes.LogType{ //nolint:gochecknoglobals // static SDK enum set
	ekstypes.LogTypeApi, ekstypes.LogTypeAudit, ekstypes.LogTypeAuthenticator,
	ekstypes.LogTypeControllerManager, ekstypes.LogTypeScheduler,
}

// eksVersionCatalogue reads what AWS says about every Kubernetes minor, keyed
// by version. It returns nil when the catalogue cannot be read, which is the
// answer "unknown" rather than "old" — this row exists precisely so a9s never
// asserts a support state it did not get from AWS.
//
// One call for the whole page: the support state belongs to the version, not
// to the cluster, so a per-cluster lookup would ask the same question N times.
func eksVersionCatalogue(ctx context.Context, api EKSAPI) map[string]ekstypes.ClusterVersionInformation {
	versionsAPI, ok := api.(EKSDescribeClusterVersionsAPI)
	if !ok {
		return nil
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*eks.DescribeClusterVersionsOutput, error) {
		return versionsAPI.DescribeClusterVersions(ctx, &eks.DescribeClusterVersionsInput{
			IncludeAll: aws.Bool(true),
		})
	})
	if err != nil {
		return nil
	}
	catalogue := make(map[string]ekstypes.ClusterVersionInformation, len(out.ClusterVersions))
	for _, v := range out.ClusterVersions {
		if key := aws.ToString(v.ClusterVersion); key != "" {
			catalogue[key] = v
		}
	}
	return catalogue
}

// eksSupportWords renders a version status as the words AWS uses in its own
// console: EXTENDED_SUPPORT reads "extended support". The input is always
// VersionStatus, never the deprecated lowercase Status, which AWS leaves empty
// — a reader that consults it flags nothing and says nothing about why.
func eksSupportWords(status ekstypes.VersionStatus) string {
	return strings.ToLower(strings.ReplaceAll(string(status), "_", " "))
}

// addEKSPostureFindings evaluates the four w6b posture signals against the
// DescribeCluster response the fetcher already holds, plus the version
// catalogue read once for the page.
// eksPosture is the four config verdicts, each a word the fetcher derives from
// the DescribeCluster struct and writes into Fields so the classifier can read
// back what the fetcher decided instead of deciding again.
type eksPosture struct {
	PublicEndpoint      string // open | restricted | no
	ControlPlaneLogging string // complete | incomplete
	SecretsEncryption   string // kms | none
	VersionSupport      string // standard | the words eksSupportWords derives
}

func eksPostureOf(cluster *ekstypes.Cluster, versions map[string]ekstypes.ClusterVersionInformation) eksPosture {
	p := eksPosture{PublicEndpoint: "no", ControlPlaneLogging: "complete", SecretsEncryption: "none", VersionSupport: "standard"}

	if vpc := cluster.ResourcesVpcConfig; vpc != nil && vpc.EndpointPublicAccess {
		// AWS defaults PublicAccessCidrs to 0.0.0.0/0 and omits it when it was
		// never narrowed, so an empty list on a public endpoint is the open
		// case rather than the unknown one.
		p.PublicEndpoint = "restricted"
		if len(vpc.PublicAccessCidrs) == 0 || slices.Contains(vpc.PublicAccessCidrs, "0.0.0.0/0") {
			p.PublicEndpoint = "open"
		}
	}

	if len(eksMissingLogTypes(cluster)) > 0 {
		p.ControlPlaneLogging = "incomplete"
	}

	for _, ec := range cluster.EncryptionConfig {
		// Resources is the only field that says WHICH resources a key covers,
		// and "covers secrets specifically" is the condition being reported.
		// The SDK marks it deprecated because EKS now encrypts API data by
		// default, but it is still what DescribeCluster returns and still what
		// distinguishes a cluster with its own key from one without.
		//nolint:staticcheck // SA1019: no replacement field carries this fact
		if slices.Contains(ec.Resources, "secrets") {
			p.SecretsEncryption = "kms"
		}
	}

	// A version absent from the catalogue, or a catalogue that could not be
	// read at all, is unknown — not old.
	if info, known := versions[aws.ToString(cluster.Version)]; known &&
		info.VersionStatus != ekstypes.VersionStatusStandardSupport && info.VersionStatus != "" {
		p.VersionSupport = eksSupportWords(info.VersionStatus)
	}
	return p
}

// eksMissingLogTypes returns the control-plane log types not being sent. A
// LogSetup entry that lists a type with Enabled=false does not enable it, and
// the five may arrive spread across several enabled entries, so this is the
// complement of the union of the enabled ones.
func eksMissingLogTypes(cluster *ekstypes.Cluster) []string {
	enabled := make(map[ekstypes.LogType]bool, len(eksControlPlaneLogTypes))
	if cluster.Logging != nil {
		for _, setup := range cluster.Logging.ClusterLogging {
			if !aws.ToBool(setup.Enabled) {
				continue
			}
			for _, lt := range setup.Types {
				enabled[lt] = true
			}
		}
	}
	var missing []string
	for _, lt := range eksControlPlaneLogTypes {
		if !enabled[lt] {
			missing = append(missing, string(lt))
		}
	}
	return missing
}

// eksPostureFindings decides the four posture findings from the words alone. A
// cluster on its way out reports none of them: nothing an operator does to a
// deleting cluster matters. An absent word is unknown rather than bad — the
// fetcher always writes all four, so a missing one means a row built outside
// it.
func eksPostureFindings(status, version string, p eksPosture) []domain.Finding {
	if status == string(ekstypes.ClusterStatusDeleting) {
		return nil
	}
	var findings []domain.Finding
	if p.PublicEndpoint != "no" && p.PublicEndpoint != "" {
		severity := domain.SevWarn
		if p.PublicEndpoint == "open" {
			severity = domain.SevBroken
		}
		findings = append(findings, domain.Finding{
			Code: CodeEKSPublicEndpoint, Phrase: "cluster endpoint reachable from the internet",
			Detail: catalog.Detail(CodeEKSPublicEndpoint), Severity: severity, Source: "wave1",
		})
	}
	if p.ControlPlaneLogging == "incomplete" {
		findings = append(findings, domain.Finding{
			Code: CodeEKSControlPlaneLoggingOff, Phrase: "control plane logging incomplete",
			Detail: catalog.Detail(CodeEKSControlPlaneLoggingOff), Severity: domain.SevWarn, Source: "wave1",
		})
	}
	if p.SecretsEncryption == "none" {
		findings = append(findings, domain.Finding{
			Code: CodeEKSSecretsNotKMS, Phrase: "secrets not encrypted with KMS",
			Detail: catalog.Detail(CodeEKSSecretsNotKMS), Severity: domain.SevWarn, Source: "wave1",
		})
	}
	if p.VersionSupport != "standard" && p.VersionSupport != "" {
		findings = append(findings, domain.Finding{
			Code: CodeEKSVersionUnsupported, Phrase: "Kubernetes " + version + " is out of standard support",
			Detail: catalog.Detail(CodeEKSVersionUnsupported), Severity: domain.SevBroken, Source: "wave1",
		})
	}
	return findings
}

// addEKSPostureRows attaches the supporting rows for whichever posture
// findings fired. They name the CIDR list, the missing log types and the
// support dates, none of which Fields carries — a detail row is the fetcher's
// alone, and only the colour has to survive a stripped row.
func addEKSPostureRows(r *resource.Resource, cluster *ekstypes.Cluster, versions map[string]ekstypes.ClusterVersionInformation) {
	if hasFinding(r.Findings, CodeEKSPublicEndpoint) {
		ranges := "0.0.0.0/0"
		if vpc := cluster.ResourcesVpcConfig; vpc != nil && len(vpc.PublicAccessCidrs) > 0 {
			ranges = strings.Join(vpc.PublicAccessCidrs, ", ")
		}
		addWave1Rows(r, CodeEKSPublicEndpoint, domain.DetailRow{Label: "Reachable from", Value: ranges, Tier: "!"})
	}
	if hasFinding(r.Findings, CodeEKSControlPlaneLoggingOff) {
		addWave1Rows(r, CodeEKSControlPlaneLoggingOff, domain.DetailRow{
			Label: "Not being sent", Value: strings.Join(eksMissingLogTypes(cluster), ", "), Tier: "~",
		})
	}
	if !hasFinding(r.Findings, CodeEKSVersionUnsupported) {
		return
	}
	info := versions[aws.ToString(cluster.Version)]
	support := eksSupportWords(info.VersionStatus)
	if info.EndOfStandardSupportDate != nil {
		support += ", standard support ended " + info.EndOfStandardSupportDate.Format("2006-01-02")
	}
	addWave1Rows(r, CodeEKSVersionUnsupported, domain.DetailRow{Label: "Support", Value: support, Tier: "!"})
}

// eksClusterFindings is the one predicate for a cluster: the lifecycle state,
// then the posture verdicts. colorEKSCluster runs it over Fields for rows built
// outside the fetcher, which have the issue count but not the codes; the phrase
// is then the fallback wording and the severity, which is what decides the
// colour, is the same either way.
func eksClusterFindings(status string, healthIssuesCount int, issueCodes []string, version string, p eksPosture) ([]domain.Finding, []domain.DetailRow) {
	if f := degradedStatusFindings("eks", status); f != nil {
		return f, nil
	}
	var findings []domain.Finding
	var rows []domain.DetailRow
	switch status {
	case string(ekstypes.ClusterStatusFailed):
		f, r := healthIssueFinding(CodeEKSStateFailed, "failed", issueCodes)
		findings, rows = []domain.Finding{f}, r
	case string(ekstypes.ClusterStatusCreating):
		findings = []domain.Finding{{Code: CodeEKSStateCreating, Phrase: "creating", Detail: catalog.Detail(CodeEKSStateCreating), Severity: domain.SevWarn, Source: "wave1"}}
	case string(ekstypes.ClusterStatusUpdating):
		findings = []domain.Finding{{Code: CodeEKSStateUpdating, Phrase: "updating", Detail: catalog.Detail(CodeEKSStateUpdating), Severity: domain.SevWarn, Source: "wave1"}}
	case string(ekstypes.ClusterStatusDeleting):
		findings = []domain.Finding{{Code: CodeEKSStateDeleting, Phrase: "deleting", Detail: catalog.Detail(CodeEKSStateDeleting), Severity: domain.SevWarn, Source: "wave1"}}
	case string(ekstypes.ClusterStatusPending):
		findings = []domain.Finding{{Code: CodeEKSStatePending, Phrase: "pending", Detail: catalog.Detail(CodeEKSStatePending), Severity: domain.SevWarn, Source: "wave1"}}
	default:
		if healthIssuesCount > 0 {
			f, r := healthIssueFindingSev(CodeEKSHealthIssue, "health issue", issueCodes, domain.SevWarn)
			findings, rows = []domain.Finding{f}, r
		}
	}
	return append(findings, eksPostureFindings(status, version, p)...), rows
}

// eksIssueRowCode names the finding the Health.Issues[] rows belong to: the
// failed state folds them in, any other state leaves them on the health-issue
// finding.
func eksIssueRowCode(status string) domain.FindingCode {
	if status == string(ekstypes.ClusterStatusFailed) {
		return CodeEKSStateFailed
	}
	return CodeEKSHealthIssue
}
