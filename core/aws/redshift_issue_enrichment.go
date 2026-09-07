// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// redshift_issue_enrichment.go — Wave 2 issue enrichment for the redshift
// resource type: the two cluster-security settings that are not carried on
// DescribeClusters and each need their own read-only call.
package aws

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/redshift"
	redshifttypes "github.com/aws/aws-sdk-go-v2/service/redshift/types"
	"golang.org/x/sync/singleflight"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// redshift canonical Wave-2 FindingCodes.
const (
	redshiftCodeAuditLoggingOff domain.FindingCode = "redshift.audit-logging-off"
	redshiftCodeRequireSSLOff   domain.FindingCode = "redshift.require-ssl-off"
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

	resources = capAtEnrichmentCap(&result, resources, resourceIDsOf)
	n := len(resources)
	if n < len(resources) {
		result.Truncated = true
	}
	var failures []string
	var mu sync.Mutex
	var requireSSLByGroup redshiftParamGroupCache

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

		sslValue, sslKnown, sslErr := redshiftRequireSSL(ctx, clients, r, &requireSSLByGroup)

		mu.Lock()
		defer mu.Unlock()
		if sslErr != nil {
			MarkSkipped(&result, r.ID, &failures, "DescribeClusterParameters", sslErr)
		}
		switch {
		case logErr != nil && IsNotFoundErr(logErr):
			result.TruncatedIDs[r.ID] = true
		case logErr != nil:
			MarkSkipped(&result, r.ID, &failures, "DescribeLoggingStatus", logErr)
		case !aws.ToBool(logging.LoggingEnabled):
			// No supporting row: "Audit logging: off" is the phrase split on a
			// colon, and U11 forbids restating it under the finding it belongs to.
			setWave2Finding(&result, r.ID, redshiftCodeAuditLoggingOff, "audit logging off", "~", "redshift",
				nil)

		}

		if sslKnown && !strings.EqualFold(sslValue, "true") {
			// The parameter's stored value is the string "false"; the row says
			// what that means for connections.
			shown := "off"
			if sslValue == "" {
				shown = "unset"
			}
			// The row exists to name the parameter an operator edits, so the
			// identifier rides along as an aside beside the value.
			shown += " (require_ssl)"
			setWave2Finding(&result, r.ID, redshiftCodeRequireSSLOff, "SSL not required", "~", "redshift",
				[]domain.DetailRow{{Label: "Requires encrypted connections", Value: shown, Tier: "~"}})

		}
	})

	err := Finish(&result, failures, n, "redshift-enrich: cluster posture")
	return result, err
}

// errRedshiftParamsCutShort is the answer a parameter walk gives when it ran
// out of pages before require_ssl appeared: the setting is unread, which the
// caller marks as a coverage gap on the cluster rather than a value.
var errRedshiftParamsCutShort = errors.New("require_ssl not found within " +
	strconv.Itoa(PerParentPageCap) + " pages of parameters")

// redshiftParamGroupCache reads each parameter group at most once per run.
// singleflight collapses the clusters that share a group into one call
// without any of them waiting on a cluster that shares nothing with them,
// which is what a mutex around the whole read would have cost.
type redshiftParamGroupCache struct {
	sf     singleflight.Group
	values sync.Map // parameter-group name -> require_ssl value
}

// redshiftRequireSSL returns the effective require_ssl value for a cluster.
// ok is false when no parameter group could be read, in which case no finding
// is emitted: an unread setting is unknown, not disabled. err carries the
// first read failure so the caller can mark the cluster under its own lock.
func redshiftRequireSSL(
	ctx context.Context,
	clients *ServiceClients,
	r resource.Resource,
	groups *redshiftParamGroupCache,
) (string, bool, error) {
	cluster, isCluster := assertStruct[redshifttypes.Cluster](r.RawStruct)
	if !isCluster {
		return "", false, nil
	}
	found := false
	value := ""
	var firstErr error
	for _, pg := range cluster.ClusterParameterGroups {
		name := aws.ToString(pg.ParameterGroupName)
		if name == "" {
			continue
		}
		v, err := groups.requireSSL(ctx, clients, name)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		found = true
		// Any group that requires SSL settles the question for the cluster.
		if strings.EqualFold(v, "true") {
			return v, true, firstErr
		}
		value = v
	}
	return value, found, firstErr
}

// requireSSL reads one parameter group's require_ssl, once per run however
// many clusters ask for it.
func (g *redshiftParamGroupCache) requireSSL(ctx context.Context, clients *ServiceClients, name string) (string, error) {
	if cached, ok := g.values.Load(name); ok {
		return cached.(string), nil
	}
	v, err, _ := g.sf.Do(name, func() (any, error) {
		if cached, ok := g.values.Load(name); ok {
			return cached, nil
		}
		value := ""
		var marker *string
		// A parameter group holds dozens of parameters and AWS pages them, so
		// require_ssl can land past the first page.
		for range PerParentPageCap {
			out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*redshift.DescribeClusterParametersOutput, error) {
				return clients.Redshift.DescribeClusterParameters(ctx, &redshift.DescribeClusterParametersInput{
					ParameterGroupName: aws.String(name),
					Marker:             marker,
				})
			})
			if err != nil {
				return "", err
			}
			for _, param := range out.Parameters {
				if strings.EqualFold(aws.ToString(param.ParameterName), "require_ssl") {
					value = aws.ToString(param.ParameterValue)
				}
			}
			marker = out.Marker
			if value != "" || marker == nil || *marker == "" {
				marker = nil
				break
			}
		}
		if marker != nil {
			// The cap is a limit on what a9s read, not evidence the parameter
			// is unset. Failing here keeps the empty value out of the cache,
			// so the next cluster sharing the group does not inherit it.
			return "", errRedshiftParamsCutShort
		}
		g.values.Store(name, value)
		return value, nil
	})
	if err != nil {
		return "", err
	}
	return v.(string), nil
}
