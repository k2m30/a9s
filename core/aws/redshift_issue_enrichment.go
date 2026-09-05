// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// redshift_issue_enrichment.go — Wave 2 issue enrichment for the redshift
// resource type: the two cluster-security settings that are not carried on
// DescribeClusters and each need their own read-only call.
package aws

import (
	"context"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/redshift"
	redshifttypes "github.com/aws/aws-sdk-go-v2/service/redshift/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// redshift canonical Wave-2 FindingCodes.
const (
	redshiftCodeAuditLoggingOff domain.FindingCode = "redshift.audit-logging-off"
	redshiftCodeRequireSSLOff   domain.FindingCode = "redshift.require-ssl-off"
)

// S5 operator sentences for the redshift posture findings.
const (
	redshiftAuditLoggingOffDetail = "Nothing records connections and queries against this cluster, so an incident leaves no trail to follow. Enable audit logging to an S3 bucket or a CloudWatch log group."
	redshiftRequireSSLOffDetail   = "The cluster accepts unencrypted client connections, so credentials and query results can be read off the wire. Set the parameter group's require-SSL parameter (require_ssl) to true and reboot."
)

// EnrichRedshiftPosture reports the two cluster-security settings
// DescribeClusters does not carry:
//
//   - DescribeLoggingStatus per cluster → redshift.audit-logging-off
//   - DescribeClusterParameters per distinct parameter group → redshift.require-ssl-off
//
// Parameter groups are shared across clusters in every fleet worth the name,
// so the parameter walk is cached per run and costs one call per group rather
// than one per cluster.
func EnrichRedshiftPosture(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]bool),
		FieldUpdates: make(map[string]map[string]string),
	}
	if clients == nil || clients.Redshift == nil {
		return result, nil
	}

	n := min(len(resources), EnrichmentCap)
	if n < len(resources) {
		result.Truncated = true
	}
	var failures []string
	var mu sync.Mutex
	requireSSLByGroup := map[string]string{}

	_ = ForEachParallel(ctx, n, EnrichmentParallelism, func(i int) {
		r := resources[i]
		if r.ID == "" || resourceIsTearingDown(r.RawStruct) {
			return
		}
		logging, logErr := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*redshift.DescribeLoggingStatusOutput, error) {
			return clients.Redshift.DescribeLoggingStatus(ctx, &redshift.DescribeLoggingStatusInput{
				ClusterIdentifier: aws.String(r.ID),
			})
		})

		mu.Lock()
		defer mu.Unlock()
		switch {
		case logErr != nil && IsNotFoundErr(logErr):
			result.TruncatedIDs[r.ID] = true
		case logErr != nil:
			MarkSkipped(&result, r.ID, &failures, "DescribeLoggingStatus", logErr)
		case !aws.ToBool(logging.LoggingEnabled):
			// No supporting row: "Audit logging: off" is the phrase split on a
			// colon, and U11 forbids restating it under the finding it belongs to.
			setWave2Finding(&result, r.ID, redshiftCodeAuditLoggingOff, "audit logging off", "~", "redshift",
				nil, redshiftAuditLoggingOffDetail)
		}

		value, ok := redshiftRequireSSL(ctx, clients, r, requireSSLByGroup, &result, &failures)
		if ok && !strings.EqualFold(value, "true") {
			shown := value
			if shown == "" {
				shown = "unset"
			}
			setWave2Finding(&result, r.ID, redshiftCodeRequireSSLOff, "SSL not required", "~", "redshift",
				[]domain.DetailRow{{Label: "Requires encrypted connections", Value: shown, Tier: "~"}}, redshiftRequireSSLOffDetail)
		}
	})

	err := Finish(&result, failures, n, "redshift-enrich: cluster posture")
	return result, err
}

// redshiftRequireSSL returns the effective require_ssl value for a cluster,
// reading each parameter group at most once per run. ok is false when no
// parameter group could be read, in which case no finding is emitted:
// an unread setting is unknown, not disabled.
//
// The caller must hold the mutex guarding result, failures and cache; the
// AWS call inside runs under it, which serializes the parameter-group walk.
// That is deliberate — it is what makes the per-group cache a cache rather
// than a race, and the group count is small by construction.
func redshiftRequireSSL(
	ctx context.Context,
	clients *ServiceClients,
	r resource.Resource,
	cache map[string]string,
	result *IssueEnricherResult,
	failures *[]string,
) (string, bool) {
	cluster, isCluster := assertStruct[redshifttypes.Cluster](r.RawStruct)
	if !isCluster {
		return "", false
	}
	found := false
	value := ""
	for _, pg := range cluster.ClusterParameterGroups {
		name := aws.ToString(pg.ParameterGroupName)
		if name == "" {
			continue
		}
		v, cached := cache[name]
		if !cached {
			// A parameter group holds dozens of parameters and AWS pages
			// them, so require_ssl can land past the first page.
			var marker *string
			failed := false
			for range PerParentPageCap {
				out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*redshift.DescribeClusterParametersOutput, error) {
					return clients.Redshift.DescribeClusterParameters(ctx, &redshift.DescribeClusterParametersInput{
						ParameterGroupName: aws.String(name),
						Marker:             marker,
					})
				})
				if err != nil {
					MarkSkipped(result, r.ID, failures, "DescribeClusterParameters", err)
					failed = true
					break
				}
				for _, param := range out.Parameters {
					if strings.EqualFold(aws.ToString(param.ParameterName), "require_ssl") {
						v = aws.ToString(param.ParameterValue)
					}
				}
				if v != "" || out.Marker == nil || *out.Marker == "" {
					break
				}
				marker = out.Marker
			}
			if failed {
				continue
			}
			cache[name] = v
		}
		found = true
		// Any group that requires SSL settles the question for the cluster.
		if strings.EqualFold(v, "true") {
			return v, true
		}
		value = v
	}
	return value, found
}
