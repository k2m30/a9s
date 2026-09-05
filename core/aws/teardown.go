// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// teardown.go — the single answer to "is this resource on its way out?".
//
// A resource being deleted has no posture worth reporting: nobody is going to
// turn encryption on for something that will not exist in a minute, and the
// finding would only add noise to the list while the row drains away. Both
// waves need the same answer — the fetchers hold a status string, the Wave-2
// enrichers hold the RawStruct the fetcher retained — so both forms live here
// rather than as a status comparison repeated at each site.
package aws

import (
	"strings"

	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	efstypes "github.com/aws/aws-sdk-go-v2/service/efs/types"
	elasticachetypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	opensearchtypes "github.com/aws/aws-sdk-go-v2/service/opensearch/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	redshifttypes "github.com/aws/aws-sdk-go-v2/service/redshift/types"

	"github.com/aws/aws-sdk-go-v2/aws"
)

// isTeardownStatus reports whether a lifecycle status string names a resource
// that is being removed. Case-insensitive: the AWS services in this package
// disagree on whether the value is "deleting" or "DELETING".
func isTeardownStatus(status string) bool {
	switch strings.ToLower(status) {
	case "deleting", "deleted":
		return true
	}
	return false
}

// resourceIsTearingDown reports whether the RawStruct a fetcher retained says
// the resource is being removed. Unrecognised shapes answer false — an
// enricher that cannot tell should report what it measured rather than
// silently skip the row.
//
// DynamoDB's archival states join the deleting one here: an archived table is
// already gone as far as posture goes, and its own Wave-1 classifier treats
// them the same way.
func resourceIsTearingDown(raw any) bool {
	switch v := raw.(type) {
	case rdstypes.DBInstance:
		return isTeardownStatus(aws.ToString(v.DBInstanceStatus))
	case rdstypes.DBCluster:
		return isTeardownStatus(aws.ToString(v.Status))
	case docdbtypes.DBCluster:
		return isTeardownStatus(aws.ToString(v.Status))
	case elasticachetypes.ReplicationGroup:
		return isTeardownStatus(aws.ToString(v.Status))
	case redshifttypes.Cluster:
		return isTeardownStatus(aws.ToString(v.ClusterStatus))
	case efstypes.FileSystemDescription:
		return isTeardownStatus(string(v.LifeCycleState))
	case opensearchtypes.DomainStatus:
		return aws.ToBool(v.Deleted)
	case *ddbtypes.TableDescription:
		switch v.TableStatus {
		case ddbtypes.TableStatusDeleting, ddbtypes.TableStatusArchiving, ddbtypes.TableStatusArchived:
			return true
		}
		return false
	default:
		return false
	}
}
