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
		},
		RawStruct: cluster,
	}

	// emit canonical Findings for non-healthy lifecycle states, mirroring
	// colorEKSCluster's own precedence (catalog_containers.go): FAILED wins
	// outright (SevBroken, folding in Health.Issues detail if present),
	// then CREATING/UPDATING (SevWarn), then a bare Health.Issues[] on an
	// otherwise-healthy cluster (SevWarn — health is tracked independently
	// of lifecycle state, docs/resources/eks.md §3.2).
	switch {
	case cluster.Status == ekstypes.ClusterStatusFailed:
		f, rows := healthIssueFinding(CodeEKSStateFailed, "failed", issueCodes)
		r.Findings = []domain.Finding{f}
		addWave1Rows(&r, CodeEKSStateFailed, rows...)
	case cluster.Status == ekstypes.ClusterStatusCreating:
		r.Findings = []domain.Finding{{
			Code: CodeEKSStateCreating, Phrase: "creating",
			Severity: domain.SevWarn, Source: "wave1",
		}}
	case cluster.Status == ekstypes.ClusterStatusUpdating:
		r.Findings = []domain.Finding{{
			Code: CodeEKSStateUpdating, Phrase: "updating",
			Severity: domain.SevWarn, Source: "wave1",
		}}
	case healthIssuesCount > 0:
		// SevWarn, not SevBroken: a Health.Issues[] signal on an otherwise
		// healthy lifecycle state ranks below FAILED/CREATING/UPDATING
		// (docs/resources/eks.md §3.2).
		f, rows := healthIssueFindingSev(CodeEKSHealthIssue, "health issue", issueCodes, domain.SevWarn)
		r.Findings = []domain.Finding{f}
		addWave1Rows(&r, CodeEKSHealthIssue, rows...)
	}

	addEKSPostureFindings(&r, cluster, versions)
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
func addEKSPostureFindings(r *resource.Resource, cluster *ekstypes.Cluster, versions map[string]ekstypes.ClusterVersionInformation) {
	if cluster.Status == ekstypes.ClusterStatusDeleting {
		return
	}

	if vpc := cluster.ResourcesVpcConfig; vpc != nil && vpc.EndpointPublicAccess {
		// AWS defaults PublicAccessCidrs to 0.0.0.0/0 and omits it when it was
		// never narrowed, so an empty list on a public endpoint is the open
		// case rather than the unknown one.
		open := len(vpc.PublicAccessCidrs) == 0 || slices.Contains(vpc.PublicAccessCidrs, "0.0.0.0/0")
		severity := domain.SevWarn
		if open {
			severity = domain.SevBroken
		}
		addWave1Finding(r, CodeEKSPublicEndpoint, "cluster endpoint reachable from the internet", severity)
		ranges := "0.0.0.0/0"
		if len(vpc.PublicAccessCidrs) > 0 {
			ranges = strings.Join(vpc.PublicAccessCidrs, ", ")
		}
		addWave1Rows(r, CodeEKSPublicEndpoint, domain.DetailRow{
			Label: "Reachable from", Value: ranges, Tier: "!",
		})
	}

	// A LogSetup entry that lists a type with Enabled=false does not enable
	// it, and the five may arrive spread across several enabled entries, so
	// this is the union of the enabled ones.
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
	if len(missing) > 0 {
		addWave1Finding(r, CodeEKSControlPlaneLoggingOff, "control plane logging incomplete", domain.SevWarn)
		addWave1Rows(r, CodeEKSControlPlaneLoggingOff, domain.DetailRow{
			Label: "Not being sent", Value: strings.Join(missing, ", "), Tier: "~",
		})
	}

	// An encryption configuration covering something other than secrets does
	// not cover secrets, so a non-empty slice is not the question.
	secretsEncrypted := false
	for _, ec := range cluster.EncryptionConfig {
		// Resources is the only field that says WHICH resources a key covers,
		// and "covers secrets specifically" is the condition being reported.
		// The SDK marks it deprecated because EKS now encrypts API data by
		// default, but it is still what DescribeCluster returns and still what
		// distinguishes a cluster with its own key from one without.
		//nolint:staticcheck // SA1019: no replacement field carries this fact
		for _, res := range ec.Resources {
			if res == "secrets" {
				secretsEncrypted = true
			}
		}
	}
	if !secretsEncrypted {
		addWave1Finding(r, CodeEKSSecretsNotKMS, "secrets not encrypted with KMS", domain.SevWarn)
	}

	// A version absent from the catalogue, or a catalogue that could not be
	// read at all, is unknown — not old.
	version := aws.ToString(cluster.Version)
	info, known := versions[version]
	if !known || info.VersionStatus == ekstypes.VersionStatusStandardSupport || info.VersionStatus == "" {
		return
	}
	addWave1Finding(r, CodeEKSVersionUnsupported, "Kubernetes "+version+" is out of standard support", domain.SevBroken)
	support := eksSupportWords(info.VersionStatus)
	if info.EndOfStandardSupportDate != nil {
		support += ", standard support ended " + info.EndOfStandardSupportDate.Format("2006-01-02")
	}
	addWave1Rows(r, CodeEKSVersionUnsupported, domain.DetailRow{
		Label: "Support", Value: support, Tier: "!",
	})
}
