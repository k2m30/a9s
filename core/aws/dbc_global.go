// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/docdb"
	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
)

// DocDBDescribeGlobalClustersAPI lists DocumentDB global clusters and their
// members. The dbc DocumentDB lane asserts its client to it.
type DocDBDescribeGlobalClustersAPI interface {
	DescribeGlobalClusters(ctx context.Context, params *docdb.DescribeGlobalClustersInput, optFns ...func(*docdb.Options)) (*docdb.DescribeGlobalClustersOutput, error)
}

// RDSDescribeGlobalClustersAPI lists Aurora global databases and their
// members. The dbc RDS lane asserts its client to it.
type RDSDescribeGlobalClustersAPI interface {
	DescribeGlobalClusters(ctx context.Context, params *rds.DescribeGlobalClustersInput, optFns ...func(*rds.Options)) (*rds.DescribeGlobalClustersOutput, error)
}

// The live clients must satisfy the interfaces the lanes assert, or the
// lanes would read no member list and judge every global member stand-alone.
var (
	_ DocDBDescribeGlobalClustersAPI = (*docdb.Client)(nil)
	_ RDSDescribeGlobalClustersAPI   = (*rds.Client)(nil)
)

// errGlobalRolesUnread marks a dbc page whose rows stand but where a
// cluster's missing writer got no verdict, because the global database
// member list could not be read.
var errGlobalRolesUnread = errors.New("global database members not read")

// dbcGlobalRoles is the global database member list: cluster ARN → whether
// the member is its global database's writer. err is the failed read; a
// client that cannot list global clusters has none to report.
type dbcGlobalRoles struct {
	writer map[string]bool
	err    error
}

// expectsWriter reports whether a cluster with no writer member is broken.
// A replica, or a global member AWS lists as not the writer, is read-only by
// design. mayBeGlobal says the cluster record cannot rule out a global
// membership; with the list unread such a cluster has no verdict (nil).
func (g dbcGlobalRoles) expectsWriter(clusterARN string, replica, mayBeGlobal bool) *bool {
	if replica {
		return aws.Bool(false)
	}
	if w, ok := g.writer[clusterARN]; ok {
		return aws.Bool(w)
	}
	if g.err != nil && mayBeGlobal {
		return nil
	}
	return aws.Bool(true)
}

// unreadErr is the page's error when a row's verdict was withheld.
func (g dbcGlobalRoles) unreadErr(withheld int) error {
	if withheld == 0 {
		return nil
	}
	return fmt.Errorf("%w for %d cluster(s): %w", errGlobalRolesUnread, withheld, g.err)
}

func readDocDBGlobalRoles(ctx context.Context, api any) dbcGlobalRoles {
	gapi, ok := api.(DocDBDescribeGlobalClustersAPI)
	if !ok {
		return dbcGlobalRoles{}
	}
	// DescribeGlobalClusters pages by Marker
	// (https://docs.aws.amazon.com/documentdb/latest/developerguide/API_DescribeGlobalClusters.html).
	globals, complete, err := PageAll(ctx, PerParentPageCap, func(ctx context.Context, marker *string) ([]docdbtypes.GlobalCluster, *string, error) {
		out, err := gapi.DescribeGlobalClusters(ctx, &docdb.DescribeGlobalClustersInput{Marker: marker})
		if err != nil {
			return nil, nil, err
		}
		return out.GlobalClusters, out.Marker, nil
	})
	if err == nil && !complete {
		err = fmt.Errorf("more than %d pages", PerParentPageCap)
	}
	if err != nil {
		return dbcGlobalRoles{err: fmt.Errorf("DocumentDB DescribeGlobalClusters: %w", err)}
	}
	roles := dbcGlobalRoles{writer: map[string]bool{}}
	for _, g := range globals {
		for _, m := range g.GlobalClusterMembers {
			roles.writer[aws.ToString(m.DBClusterArn)] = aws.ToBool(m.IsWriter)
		}
	}
	return roles
}

func readRDSGlobalRoles(ctx context.Context, api any) dbcGlobalRoles {
	gapi, ok := api.(RDSDescribeGlobalClustersAPI)
	if !ok {
		return dbcGlobalRoles{}
	}
	// DescribeGlobalClusters pages by Marker
	// (https://docs.aws.amazon.com/AmazonRDS/latest/APIReference/API_DescribeGlobalClusters.html).
	globals, complete, err := PageAll(ctx, PerParentPageCap, func(ctx context.Context, marker *string) ([]rdstypes.GlobalCluster, *string, error) {
		out, err := gapi.DescribeGlobalClusters(ctx, &rds.DescribeGlobalClustersInput{Marker: marker})
		if err != nil {
			return nil, nil, err
		}
		return out.GlobalClusters, out.Marker, nil
	})
	if err == nil && !complete {
		err = fmt.Errorf("more than %d pages", PerParentPageCap)
	}
	if err != nil {
		return dbcGlobalRoles{err: fmt.Errorf("RDS DescribeGlobalClusters: %w", err)}
	}
	roles := dbcGlobalRoles{writer: map[string]bool{}}
	for _, g := range globals {
		for _, m := range g.GlobalClusterMembers {
			roles.writer[aws.ToString(m.DBClusterArn)] = aws.ToBool(m.IsWriter)
		}
	}
	return roles
}
