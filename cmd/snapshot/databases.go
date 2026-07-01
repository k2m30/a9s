package main

import (
	"context"
	"errors"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/docdb"
	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/efs"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	elasticachetypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/aws/aws-sdk-go-v2/service/redshift"
	smithy "github.com/aws/smithy-go"
)

// dbFormatTime mirrors formatTime in s3.go under a distinct name to avoid
// collisions between files in this package.
func dbFormatTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02 15:04")
}

// ---------------------------------------------------------------------------
// dbi — RDS DB Instances
// ---------------------------------------------------------------------------

type dbiData struct {
	Instances          []dbiInstance           `json:"instances"`
	PendingMaintenance []dbiPendingMaintenance `json:"pending_maintenance"`
}

type dbiInstance struct {
	DBInstanceIdentifier         string   `json:"db_instance_identifier"`
	DBInstanceArn                string   `json:"db_instance_arn"`
	DBInstanceStatus             string   `json:"db_instance_status"`
	Engine                       string   `json:"engine,omitempty"`
	EngineVersion                string   `json:"engine_version,omitempty"`
	DBInstanceClass              string   `json:"db_instance_class,omitempty"`
	AllocatedStorage             int32    `json:"allocated_storage,omitempty"`
	AvailabilityZone             string   `json:"availability_zone,omitempty"`
	MultiAZ                      bool     `json:"multi_az"`
	InstanceCreateTime           string   `json:"instance_create_time,omitempty"`
	BackupRetentionPeriod        int32    `json:"backup_retention_period"`
	PubliclyAccessible           bool     `json:"publicly_accessible"`
	StorageEncrypted             bool     `json:"storage_encrypted"`
	DeletionProtection           bool     `json:"deletion_protection"`
	KmsKeyId                     string   `json:"kms_key_id,omitempty"`
	DBClusterIdentifier          string   `json:"db_cluster_identifier,omitempty"`
	EnabledCloudwatchLogsExports []string `json:"enabled_cloudwatch_logs_exports,omitempty"`
	MonitoringRoleArn            string   `json:"monitoring_role_arn,omitempty"`
	AssociatedRoleArns           []string `json:"associated_role_arns,omitempty"`
	MasterUserSecretArn          string   `json:"master_user_secret_arn,omitempty"`
	VpcSecurityGroupIds          []string `json:"vpc_security_group_ids,omitempty"`
	DBSubnetGroupVpcId           string   `json:"db_subnet_group_vpc_id,omitempty"`
	DBSubnetGroupSubnetIds       []string `json:"db_subnet_group_subnet_ids,omitempty"`
	PendingModifiedValuesSet     bool     `json:"pending_modified_values_set"`
}

// dbiPendingMaintenance captures the account-wide DescribePendingMaintenanceActions
// result, bucketed by ResourceIdentifier (instance or cluster ARN) so the checklist generator
// can join it against dbi/dbc rows without a second live call.
type dbiPendingMaintenance struct {
	ResourceIdentifier   string `json:"resource_identifier"`
	Action               string `json:"action"`
	Description          string `json:"description,omitempty"`
	ForcedApplyDate      string `json:"forced_apply_date,omitempty"`
	AutoAppliedAfterDate string `json:"auto_applied_after_date,omitempty"`
	CurrentApplyDate     string `json:"current_apply_date,omitempty"`
}

func captureDBI(ctx context.Context, cfg aws.Config) (any, error) {
	client := rds.NewFromConfig(cfg)

	var instances []dbiInstance
	var marker *string
	for {
		out, err := client.DescribeDBInstances(ctx, &rds.DescribeDBInstancesInput{Marker: marker})
		if err != nil {
			return nil, err
		}
		for _, d := range out.DBInstances {
			instances = append(instances, dbiInstanceFromSDK(d))
		}
		if out.Marker == nil || *out.Marker == "" {
			break
		}
		marker = out.Marker
	}

	pending, err := captureDBIPendingMaintenance(ctx, client)
	if err != nil {
		return nil, err
	}

	return dbiData{Instances: instances, PendingMaintenance: pending}, nil
}

func dbiInstanceFromSDK(d rdstypes.DBInstance) dbiInstance {
	inst := dbiInstance{
		DBInstanceIdentifier:     aws.ToString(d.DBInstanceIdentifier),
		DBInstanceArn:            aws.ToString(d.DBInstanceArn),
		DBInstanceStatus:         aws.ToString(d.DBInstanceStatus),
		Engine:                   aws.ToString(d.Engine),
		EngineVersion:            aws.ToString(d.EngineVersion),
		DBInstanceClass:          aws.ToString(d.DBInstanceClass),
		AllocatedStorage:         aws.ToInt32(d.AllocatedStorage),
		AvailabilityZone:         aws.ToString(d.AvailabilityZone),
		MultiAZ:                  aws.ToBool(d.MultiAZ),
		InstanceCreateTime:       dbFormatTime(d.InstanceCreateTime),
		BackupRetentionPeriod:    aws.ToInt32(d.BackupRetentionPeriod),
		PubliclyAccessible:       aws.ToBool(d.PubliclyAccessible),
		StorageEncrypted:         aws.ToBool(d.StorageEncrypted),
		DeletionProtection:       aws.ToBool(d.DeletionProtection),
		KmsKeyId:                 aws.ToString(d.KmsKeyId),
		DBClusterIdentifier:      aws.ToString(d.DBClusterIdentifier),
		MonitoringRoleArn:        aws.ToString(d.MonitoringRoleArn),
		PendingModifiedValuesSet: d.PendingModifiedValues != nil,
	}
	inst.EnabledCloudwatchLogsExports = append(inst.EnabledCloudwatchLogsExports, d.EnabledCloudwatchLogsExports...)
	for _, r := range d.AssociatedRoles {
		inst.AssociatedRoleArns = append(inst.AssociatedRoleArns, aws.ToString(r.RoleArn))
	}
	if d.MasterUserSecret != nil {
		inst.MasterUserSecretArn = aws.ToString(d.MasterUserSecret.SecretArn)
	}
	for _, sg := range d.VpcSecurityGroups {
		inst.VpcSecurityGroupIds = append(inst.VpcSecurityGroupIds, aws.ToString(sg.VpcSecurityGroupId))
	}
	if d.DBSubnetGroup != nil {
		inst.DBSubnetGroupVpcId = aws.ToString(d.DBSubnetGroup.VpcId)
		for _, sn := range d.DBSubnetGroup.Subnets {
			inst.DBSubnetGroupSubnetIds = append(inst.DBSubnetGroupSubnetIds, aws.ToString(sn.SubnetIdentifier))
		}
	}
	return inst
}

func captureDBIPendingMaintenance(ctx context.Context, client *rds.Client) ([]dbiPendingMaintenance, error) {
	var out []dbiPendingMaintenance
	var marker *string
	for {
		resp, err := client.DescribePendingMaintenanceActions(ctx, &rds.DescribePendingMaintenanceActionsInput{Marker: marker})
		if err != nil {
			return nil, err
		}
		for _, pma := range resp.PendingMaintenanceActions {
			resourceID := aws.ToString(pma.ResourceIdentifier)
			for _, action := range pma.PendingMaintenanceActionDetails {
				out = append(out, dbiPendingMaintenance{
					ResourceIdentifier:   resourceID,
					Action:               aws.ToString(action.Action),
					Description:          aws.ToString(action.Description),
					ForcedApplyDate:      dbFormatTime(action.ForcedApplyDate),
					AutoAppliedAfterDate: dbFormatTime(action.AutoAppliedAfterDate),
					CurrentApplyDate:     dbFormatTime(action.CurrentApplyDate),
				})
			}
		}
		if resp.Marker == nil || *resp.Marker == "" {
			break
		}
		marker = resp.Marker
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// dbc — DocDB / Aurora / Multi-AZ DB Clusters (merged docdb + rds endpoints)
// ---------------------------------------------------------------------------

type dbcData struct {
	Clusters []dbcCluster `json:"clusters"`
}

type dbcCluster struct {
	Source                       string   `json:"source"` // "docdb" or "rds"
	DBClusterIdentifier          string   `json:"db_cluster_identifier"`
	DBClusterArn                 string   `json:"db_cluster_arn"`
	Status                       string   `json:"status"`
	Engine                       string   `json:"engine,omitempty"`
	EngineVersion                string   `json:"engine_version,omitempty"`
	HasWriterMember              bool     `json:"has_writer_member"`
	MemberInstanceIdentifiers    []string `json:"member_instance_identifiers,omitempty"`
	DeletionProtection           bool     `json:"deletion_protection"`
	StorageEncrypted             bool     `json:"storage_encrypted"`
	BackupRetentionPeriod        int32    `json:"backup_retention_period"`
	KmsKeyId                     string   `json:"kms_key_id,omitempty"`
	EnabledCloudwatchLogsExports []string `json:"enabled_cloudwatch_logs_exports,omitempty"`
	MasterUserSecretArn          string   `json:"master_user_secret_arn,omitempty"`
	VpcSecurityGroupIds          []string `json:"vpc_security_group_ids,omitempty"`
	DBSubnetGroupName            string   `json:"db_subnet_group_name,omitempty"`
}

func captureDBC(ctx context.Context, cfg aws.Config) (any, error) {
	docdbClient := docdb.NewFromConfig(cfg)
	rdsClient := rds.NewFromConfig(cfg)

	var clusters []dbcCluster

	var docdbMarker *string
	for {
		out, err := docdbClient.DescribeDBClusters(ctx, &docdb.DescribeDBClustersInput{Marker: docdbMarker})
		if err != nil {
			return nil, err
		}
		for _, c := range out.DBClusters {
			clusters = append(clusters, dbcClusterFromDocDB(c))
		}
		if out.Marker == nil || *out.Marker == "" {
			break
		}
		docdbMarker = out.Marker
	}

	var rdsMarker *string
	for {
		out, err := rdsClient.DescribeDBClusters(ctx, &rds.DescribeDBClustersInput{Marker: rdsMarker})
		if err != nil {
			return nil, err
		}
		for _, c := range out.DBClusters {
			clusters = append(clusters, dbcClusterFromRDS(c))
		}
		if out.Marker == nil || *out.Marker == "" {
			break
		}
		rdsMarker = out.Marker
	}

	return dbcData{Clusters: dedupDBCByID(clusters)}, nil
}

// dedupDBCByID dedups by DBClusterIdentifier, first-occurrence-wins, matching
// the a9s fetcher contract (docdb concatenated first, then rds) documented in
// docs/resources/dbc.md §1.
func dedupDBCByID(in []dbcCluster) []dbcCluster {
	seen := make(map[string]bool, len(in))
	out := make([]dbcCluster, 0, len(in))
	for _, c := range in {
		if seen[c.DBClusterIdentifier] {
			continue
		}
		seen[c.DBClusterIdentifier] = true
		out = append(out, c)
	}
	return out
}

func dbcClusterFromDocDB(c docdbtypes.DBCluster) dbcCluster {
	out := dbcCluster{
		Source:                "docdb",
		DBClusterIdentifier:   aws.ToString(c.DBClusterIdentifier),
		DBClusterArn:          aws.ToString(c.DBClusterArn),
		Status:                aws.ToString(c.Status),
		Engine:                aws.ToString(c.Engine),
		EngineVersion:         aws.ToString(c.EngineVersion),
		DeletionProtection:    aws.ToBool(c.DeletionProtection),
		StorageEncrypted:      aws.ToBool(c.StorageEncrypted),
		BackupRetentionPeriod: aws.ToInt32(c.BackupRetentionPeriod),
		KmsKeyId:              aws.ToString(c.KmsKeyId),
		DBSubnetGroupName:     aws.ToString(c.DBSubnetGroup),
	}
	out.EnabledCloudwatchLogsExports = append(out.EnabledCloudwatchLogsExports, c.EnabledCloudwatchLogsExports...)
	if c.MasterUserSecret != nil {
		out.MasterUserSecretArn = aws.ToString(c.MasterUserSecret.SecretArn)
	}
	for _, sg := range c.VpcSecurityGroups {
		out.VpcSecurityGroupIds = append(out.VpcSecurityGroupIds, aws.ToString(sg.VpcSecurityGroupId))
	}
	for _, m := range c.DBClusterMembers {
		out.MemberInstanceIdentifiers = append(out.MemberInstanceIdentifiers, aws.ToString(m.DBInstanceIdentifier))
		if aws.ToBool(m.IsClusterWriter) {
			out.HasWriterMember = true
		}
	}
	return out
}

func dbcClusterFromRDS(c rdstypes.DBCluster) dbcCluster {
	out := dbcCluster{
		Source:                "rds",
		DBClusterIdentifier:   aws.ToString(c.DBClusterIdentifier),
		DBClusterArn:          aws.ToString(c.DBClusterArn),
		Status:                aws.ToString(c.Status),
		Engine:                aws.ToString(c.Engine),
		EngineVersion:         aws.ToString(c.EngineVersion),
		DeletionProtection:    aws.ToBool(c.DeletionProtection),
		StorageEncrypted:      aws.ToBool(c.StorageEncrypted),
		BackupRetentionPeriod: aws.ToInt32(c.BackupRetentionPeriod),
		KmsKeyId:              aws.ToString(c.KmsKeyId),
		DBSubnetGroupName:     aws.ToString(c.DBSubnetGroup),
	}
	out.EnabledCloudwatchLogsExports = append(out.EnabledCloudwatchLogsExports, c.EnabledCloudwatchLogsExports...)
	if c.MasterUserSecret != nil {
		out.MasterUserSecretArn = aws.ToString(c.MasterUserSecret.SecretArn)
	}
	for _, sg := range c.VpcSecurityGroups {
		out.VpcSecurityGroupIds = append(out.VpcSecurityGroupIds, aws.ToString(sg.VpcSecurityGroupId))
	}
	for _, m := range c.DBClusterMembers {
		out.MemberInstanceIdentifiers = append(out.MemberInstanceIdentifiers, aws.ToString(m.DBInstanceIdentifier))
		if aws.ToBool(m.IsClusterWriter) {
			out.HasWriterMember = true
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// dbi-snap — RDS DB Instance Snapshots
// ---------------------------------------------------------------------------

type dbiSnapData struct {
	Snapshots []dbiSnapshot `json:"snapshots"`
}

type dbiSnapshot struct {
	DBSnapshotIdentifier string `json:"db_snapshot_identifier"`
	DBSnapshotArn        string `json:"db_snapshot_arn"`
	DBInstanceIdentifier string `json:"db_instance_identifier,omitempty"`
	Status               string `json:"status"`
	SnapshotType         string `json:"snapshot_type,omitempty"`
	Encrypted            bool   `json:"encrypted"`
	KmsKeyId             string `json:"kms_key_id,omitempty"`
	SnapshotCreateTime   string `json:"snapshot_create_time,omitempty"`
	PercentProgress      int32  `json:"percent_progress"`
}

func captureDBISnap(ctx context.Context, cfg aws.Config) (any, error) {
	client := rds.NewFromConfig(cfg)

	var snaps []dbiSnapshot
	var marker *string
	for {
		out, err := client.DescribeDBSnapshots(ctx, &rds.DescribeDBSnapshotsInput{Marker: marker})
		if err != nil {
			return nil, err
		}
		for _, s := range out.DBSnapshots {
			snaps = append(snaps, dbiSnapshot{
				DBSnapshotIdentifier: aws.ToString(s.DBSnapshotIdentifier),
				DBSnapshotArn:        aws.ToString(s.DBSnapshotArn),
				DBInstanceIdentifier: aws.ToString(s.DBInstanceIdentifier),
				Status:               aws.ToString(s.Status),
				SnapshotType:         aws.ToString(s.SnapshotType),
				Encrypted:            aws.ToBool(s.Encrypted),
				KmsKeyId:             aws.ToString(s.KmsKeyId),
				SnapshotCreateTime:   dbFormatTime(s.SnapshotCreateTime),
				PercentProgress:      aws.ToInt32(s.PercentProgress),
			})
		}
		if out.Marker == nil || *out.Marker == "" {
			break
		}
		marker = out.Marker
	}

	return dbiSnapData{Snapshots: snaps}, nil
}

// ---------------------------------------------------------------------------
// dbc-snap — DocDB / Aurora / Multi-AZ DB Cluster Snapshots (merged)
// ---------------------------------------------------------------------------

type dbcSnapData struct {
	Snapshots []dbcSnapshot `json:"snapshots"`
}

type dbcSnapshot struct {
	Source                      string `json:"source"` // "docdb" or "rds"
	DBClusterSnapshotIdentifier string `json:"db_cluster_snapshot_identifier"`
	DBClusterSnapshotArn        string `json:"db_cluster_snapshot_arn"`
	DBClusterIdentifier         string `json:"db_cluster_identifier,omitempty"`
	Status                      string `json:"status"`
	SnapshotType                string `json:"snapshot_type,omitempty"`
	StorageEncrypted            bool   `json:"storage_encrypted"`
	KmsKeyId                    string `json:"kms_key_id,omitempty"`
	VpcId                       string `json:"vpc_id,omitempty"`
	SnapshotCreateTime          string `json:"snapshot_create_time,omitempty"`
	PercentProgress             int32  `json:"percent_progress"`
}

func captureDBCSnap(ctx context.Context, cfg aws.Config) (any, error) {
	docdbClient := docdb.NewFromConfig(cfg)
	rdsClient := rds.NewFromConfig(cfg)

	var snaps []dbcSnapshot

	var docdbMarker *string
	for {
		out, err := docdbClient.DescribeDBClusterSnapshots(ctx, &docdb.DescribeDBClusterSnapshotsInput{Marker: docdbMarker})
		if err != nil {
			return nil, err
		}
		for _, s := range out.DBClusterSnapshots {
			snaps = append(snaps, dbcSnapshot{
				Source:                      "docdb",
				DBClusterSnapshotIdentifier: aws.ToString(s.DBClusterSnapshotIdentifier),
				DBClusterSnapshotArn:        aws.ToString(s.DBClusterSnapshotArn),
				DBClusterIdentifier:         aws.ToString(s.DBClusterIdentifier),
				Status:                      aws.ToString(s.Status),
				SnapshotType:                aws.ToString(s.SnapshotType),
				StorageEncrypted:            aws.ToBool(s.StorageEncrypted),
				KmsKeyId:                    aws.ToString(s.KmsKeyId),
				VpcId:                       aws.ToString(s.VpcId),
				SnapshotCreateTime:          dbFormatTime(s.SnapshotCreateTime),
				PercentProgress:             aws.ToInt32(s.PercentProgress),
			})
		}
		if out.Marker == nil || *out.Marker == "" {
			break
		}
		docdbMarker = out.Marker
	}

	var rdsMarker *string
	for {
		out, err := rdsClient.DescribeDBClusterSnapshots(ctx, &rds.DescribeDBClusterSnapshotsInput{Marker: rdsMarker})
		if err != nil {
			return nil, err
		}
		for _, s := range out.DBClusterSnapshots {
			snaps = append(snaps, dbcSnapshot{
				Source:                      "rds",
				DBClusterSnapshotIdentifier: aws.ToString(s.DBClusterSnapshotIdentifier),
				DBClusterSnapshotArn:        aws.ToString(s.DBClusterSnapshotArn),
				DBClusterIdentifier:         aws.ToString(s.DBClusterIdentifier),
				Status:                      aws.ToString(s.Status),
				SnapshotType:                aws.ToString(s.SnapshotType),
				StorageEncrypted:            aws.ToBool(s.StorageEncrypted),
				KmsKeyId:                    aws.ToString(s.KmsKeyId),
				VpcId:                       aws.ToString(s.VpcId),
				SnapshotCreateTime:          dbFormatTime(s.SnapshotCreateTime),
				PercentProgress:             aws.ToInt32(s.PercentProgress),
			})
		}
		if out.Marker == nil || *out.Marker == "" {
			break
		}
		rdsMarker = out.Marker
	}

	return dbcSnapData{Snapshots: dedupDBCSnapByID(snaps)}, nil
}

// dedupDBCSnapByID dedups by DBClusterSnapshotIdentifier, first-occurrence-wins
// (docdb-side concatenated first), matching the fetcher contract in
// docs/resources/dbc-snap.md §1.
func dedupDBCSnapByID(in []dbcSnapshot) []dbcSnapshot {
	seen := make(map[string]bool, len(in))
	out := make([]dbcSnapshot, 0, len(in))
	for _, s := range in {
		if seen[s.DBClusterSnapshotIdentifier] {
			continue
		}
		seen[s.DBClusterSnapshotIdentifier] = true
		out = append(out, s)
	}
	return out
}

// ---------------------------------------------------------------------------
// ddb — DynamoDB Tables
// ---------------------------------------------------------------------------

type ddbData struct {
	Tables []ddbTable `json:"tables"`
}

type ddbTable struct {
	TableName                  string `json:"table_name"`
	TableArn                   string `json:"table_arn,omitempty"`
	TableStatus                string `json:"table_status"`
	CreationDateTime           string `json:"creation_date_time,omitempty"`
	ItemCount                  int64  `json:"item_count"`
	TableSizeBytes             int64  `json:"table_size_bytes"`
	SSEKMSMasterKeyArn         string `json:"sse_kms_master_key_arn,omitempty"`
	LatestStreamArn            string `json:"latest_stream_arn,omitempty"`
	ArchivalReason             string `json:"archival_reason,omitempty"`
	ArchivalDateTime           string `json:"archival_date_time,omitempty"`
	ArchivalBackupArn          string `json:"archival_backup_arn,omitempty"`
	DescribeTableOutcome       string `json:"describe_table_outcome"`
	DescribeTableErrorCode     string `json:"describe_table_error_code,omitempty"`
	PITRStatus                 string `json:"pitr_status,omitempty"`
	ContinuousBackupsOutcome   string `json:"continuous_backups_outcome"`
	ContinuousBackupsErrorCode string `json:"continuous_backups_error_code,omitempty"`
}

func captureDDB(ctx context.Context, cfg aws.Config) (any, error) {
	client := dynamodb.NewFromConfig(cfg)

	var names []string
	var startTable *string
	for {
		out, err := client.ListTables(ctx, &dynamodb.ListTablesInput{ExclusiveStartTableName: startTable})
		if err != nil {
			return nil, err
		}
		names = append(names, out.TableNames...)
		if out.LastEvaluatedTableName == nil || *out.LastEvaluatedTableName == "" {
			break
		}
		startTable = out.LastEvaluatedTableName
	}

	tables := make([]ddbTable, 0, len(names))
	for _, name := range names {
		t := ddbTable{TableName: name}

		descOut, err := client.DescribeTable(ctx, &dynamodb.DescribeTableInput{TableName: aws.String(name)})
		if err != nil {
			t.DescribeTableOutcome = "error"
			t.DescribeTableErrorCode = ddbErrorCode(err)
		} else {
			t.DescribeTableOutcome = "ok"
			td := descOut.Table
			if td != nil {
				t.TableArn = aws.ToString(td.TableArn)
				t.TableStatus = string(td.TableStatus)
				t.CreationDateTime = dbFormatTime(td.CreationDateTime)
				t.ItemCount = aws.ToInt64(td.ItemCount)
				t.TableSizeBytes = aws.ToInt64(td.TableSizeBytes)
				t.LatestStreamArn = aws.ToString(td.LatestStreamArn)
				if td.SSEDescription != nil {
					t.SSEKMSMasterKeyArn = aws.ToString(td.SSEDescription.KMSMasterKeyArn)
				}
				if td.ArchivalSummary != nil {
					t.ArchivalReason = aws.ToString(td.ArchivalSummary.ArchivalReason)
					t.ArchivalDateTime = dbFormatTime(td.ArchivalSummary.ArchivalDateTime)
					t.ArchivalBackupArn = aws.ToString(td.ArchivalSummary.ArchivalBackupArn)
				}
			}
		}

		cbOut, err := client.DescribeContinuousBackups(ctx, &dynamodb.DescribeContinuousBackupsInput{TableName: aws.String(name)})
		if err != nil {
			t.ContinuousBackupsOutcome = "error"
			t.ContinuousBackupsErrorCode = ddbErrorCode(err)
		} else {
			t.ContinuousBackupsOutcome = "ok"
			if cbOut.ContinuousBackupsDescription != nil && cbOut.ContinuousBackupsDescription.PointInTimeRecoveryDescription != nil {
				t.PITRStatus = string(cbOut.ContinuousBackupsDescription.PointInTimeRecoveryDescription.PointInTimeRecoveryStatus)
			}
		}

		tables = append(tables, t)
	}

	return ddbData{Tables: tables}, nil
}

func ddbErrorCode(err error) string {
	if apiErr, ok := errors.AsType[smithy.APIError](err); ok {
		return apiErr.ErrorCode()
	}
	return err.Error()
}

// ---------------------------------------------------------------------------
// redis — ElastiCache Replication Groups (Redis engine only)
// ---------------------------------------------------------------------------

type redisData struct {
	ReplicationGroups []redisReplicationGroup `json:"replication_groups"`
}

type redisReplicationGroup struct {
	ReplicationGroupId string           `json:"replication_group_id"`
	ARN                string           `json:"arn,omitempty"`
	Engine             string           `json:"engine,omitempty"`
	Status             string           `json:"status"`
	AutomaticFailover  string           `json:"automatic_failover,omitempty"`
	MultiAZ            string           `json:"multi_az,omitempty"`
	MemberClusters     []string         `json:"member_clusters,omitempty"`
	KmsKeyId           string           `json:"kms_key_id,omitempty"`
	NodeGroups         []redisNodeGroup `json:"node_groups,omitempty"`
}

type redisNodeGroup struct {
	NodeGroupId string            `json:"node_group_id,omitempty"`
	Status      string            `json:"status"`
	Members     []redisNodeMember `json:"members,omitempty"`
}

type redisNodeMember struct {
	CacheClusterId            string `json:"cache_cluster_id,omitempty"`
	CurrentRole               string `json:"current_role,omitempty"`
	PreferredAvailabilityZone string `json:"preferred_availability_zone,omitempty"`
}

func captureRedis(ctx context.Context, cfg aws.Config) (any, error) {
	client := elasticache.NewFromConfig(cfg)

	var groups []redisReplicationGroup
	var marker *string
	for {
		out, err := client.DescribeReplicationGroups(ctx, &elasticache.DescribeReplicationGroupsInput{Marker: marker})
		if err != nil {
			return nil, err
		}
		for _, rg := range out.ReplicationGroups {
			if aws.ToString(rg.Engine) != "redis" {
				continue
			}
			groups = append(groups, redisReplicationGroupFromSDK(rg))
		}
		if out.Marker == nil || *out.Marker == "" {
			break
		}
		marker = out.Marker
	}

	return redisData{ReplicationGroups: groups}, nil
}

func redisReplicationGroupFromSDK(rg elasticachetypes.ReplicationGroup) redisReplicationGroup {
	out := redisReplicationGroup{
		ReplicationGroupId: aws.ToString(rg.ReplicationGroupId),
		ARN:                aws.ToString(rg.ARN),
		Engine:             aws.ToString(rg.Engine),
		Status:             aws.ToString(rg.Status),
		AutomaticFailover:  string(rg.AutomaticFailover),
		MultiAZ:            string(rg.MultiAZ),
		KmsKeyId:           aws.ToString(rg.KmsKeyId),
	}
	out.MemberClusters = append(out.MemberClusters, rg.MemberClusters...)
	for _, ng := range rg.NodeGroups {
		group := redisNodeGroup{
			NodeGroupId: aws.ToString(ng.NodeGroupId),
			Status:      aws.ToString(ng.Status),
		}
		for _, m := range ng.NodeGroupMembers {
			group.Members = append(group.Members, redisNodeMember{
				CacheClusterId:            aws.ToString(m.CacheClusterId),
				CurrentRole:               aws.ToString(m.CurrentRole),
				PreferredAvailabilityZone: aws.ToString(m.PreferredAvailabilityZone),
			})
		}
		out.NodeGroups = append(out.NodeGroups, group)
	}
	return out
}

// ---------------------------------------------------------------------------
// redshift — Redshift Clusters
// ---------------------------------------------------------------------------

type redshiftData struct {
	Clusters []redshiftCluster `json:"clusters"`
}

type redshiftCluster struct {
	ClusterIdentifier         string   `json:"cluster_identifier"`
	ClusterNamespaceArn       string   `json:"cluster_namespace_arn,omitempty"`
	ClusterStatus             string   `json:"cluster_status"`
	ClusterAvailabilityStatus string   `json:"cluster_availability_status,omitempty"`
	PendingModifiedValuesSet  bool     `json:"pending_modified_values_set"`
	DeferredMaintenanceActive bool     `json:"deferred_maintenance_active"`
	DeferMaintenanceEndTime   string   `json:"defer_maintenance_end_time,omitempty"`
	PubliclyAccessible        bool     `json:"publicly_accessible"`
	Encrypted                 bool     `json:"encrypted"`
	KmsKeyId                  string   `json:"kms_key_id,omitempty"`
	IamRoleArns               []string `json:"iam_role_arns,omitempty"`
	MasterPasswordSecretArn   string   `json:"master_password_secret_arn,omitempty"`
	VpcSecurityGroupIds       []string `json:"vpc_security_group_ids,omitempty"`
	ClusterSubnetGroupName    string   `json:"cluster_subnet_group_name,omitempty"`
	VpcId                     string   `json:"vpc_id,omitempty"`
}

func captureRedshift(ctx context.Context, cfg aws.Config) (any, error) {
	client := redshift.NewFromConfig(cfg)

	var clusters []redshiftCluster
	var marker *string
	now := time.Now()
	for {
		out, err := client.DescribeClusters(ctx, &redshift.DescribeClustersInput{Marker: marker})
		if err != nil {
			return nil, err
		}
		for _, c := range out.Clusters {
			rc := redshiftCluster{
				ClusterIdentifier:         aws.ToString(c.ClusterIdentifier),
				ClusterNamespaceArn:       aws.ToString(c.ClusterNamespaceArn),
				ClusterStatus:             aws.ToString(c.ClusterStatus),
				ClusterAvailabilityStatus: aws.ToString(c.ClusterAvailabilityStatus),
				PendingModifiedValuesSet:  c.PendingModifiedValues != nil,
				PubliclyAccessible:        aws.ToBool(c.PubliclyAccessible),
				Encrypted:                 aws.ToBool(c.Encrypted),
				KmsKeyId:                  aws.ToString(c.KmsKeyId),
				MasterPasswordSecretArn:   aws.ToString(c.MasterPasswordSecretArn),
				ClusterSubnetGroupName:    aws.ToString(c.ClusterSubnetGroupName),
				VpcId:                     aws.ToString(c.VpcId),
			}
			for _, w := range c.DeferredMaintenanceWindows {
				if w.DeferMaintenanceStartTime != nil && w.DeferMaintenanceEndTime != nil &&
					!now.Before(*w.DeferMaintenanceStartTime) && !now.After(*w.DeferMaintenanceEndTime) {
					rc.DeferredMaintenanceActive = true
					rc.DeferMaintenanceEndTime = dbFormatTime(w.DeferMaintenanceEndTime)
					break
				}
			}
			for _, r := range c.IamRoles {
				rc.IamRoleArns = append(rc.IamRoleArns, aws.ToString(r.IamRoleArn))
			}
			for _, sg := range c.VpcSecurityGroups {
				rc.VpcSecurityGroupIds = append(rc.VpcSecurityGroupIds, aws.ToString(sg.VpcSecurityGroupId))
			}
			clusters = append(clusters, rc)
		}
		if out.Marker == nil || *out.Marker == "" {
			break
		}
		marker = out.Marker
	}

	return redshiftData{Clusters: clusters}, nil
}

// ---------------------------------------------------------------------------
// efs — EFS File Systems
// ---------------------------------------------------------------------------

type efsData struct {
	FileSystems []efsFileSystem `json:"file_systems"`
}

type efsFileSystem struct {
	FileSystemId          string           `json:"file_system_id"`
	FileSystemArn         string           `json:"file_system_arn,omitempty"`
	Name                  string           `json:"name,omitempty"`
	LifeCycleState        string           `json:"life_cycle_state"`
	NumberOfMountTargets  int32            `json:"number_of_mount_targets"`
	Encrypted             bool             `json:"encrypted"`
	KmsKeyId              string           `json:"kms_key_id,omitempty"`
	CreationTime          string           `json:"creation_time,omitempty"`
	MountTargetsOutcome   string           `json:"mount_targets_outcome"`
	MountTargetsErrorCode string           `json:"mount_targets_error_code,omitempty"`
	MountTargets          []efsMountTarget `json:"mount_targets,omitempty"`
}

type efsMountTarget struct {
	MountTargetId      string `json:"mount_target_id,omitempty"`
	LifeCycleState     string `json:"life_cycle_state"`
	SubnetId           string `json:"subnet_id,omitempty"`
	VpcId              string `json:"vpc_id,omitempty"`
	NetworkInterfaceId string `json:"network_interface_id,omitempty"`
}

func captureEFS(ctx context.Context, cfg aws.Config) (any, error) {
	client := efs.NewFromConfig(cfg)

	var systems []efsFileSystem
	var marker *string
	for {
		out, err := client.DescribeFileSystems(ctx, &efs.DescribeFileSystemsInput{Marker: marker})
		if err != nil {
			return nil, err
		}
		for _, fs := range out.FileSystems {
			systems = append(systems, efsFileSystem{
				FileSystemId:         aws.ToString(fs.FileSystemId),
				FileSystemArn:        aws.ToString(fs.FileSystemArn),
				Name:                 aws.ToString(fs.Name),
				LifeCycleState:       string(fs.LifeCycleState),
				NumberOfMountTargets: fs.NumberOfMountTargets,
				Encrypted:            aws.ToBool(fs.Encrypted),
				KmsKeyId:             aws.ToString(fs.KmsKeyId),
				CreationTime:         dbFormatTime(fs.CreationTime),
			})
		}
		if out.NextMarker == nil || *out.NextMarker == "" {
			break
		}
		marker = out.NextMarker
	}

	for i := range systems {
		mts, err := captureEFSMountTargets(ctx, client, systems[i].FileSystemId)
		if err != nil {
			systems[i].MountTargetsOutcome = "error"
			systems[i].MountTargetsErrorCode = ddbErrorCode(err)
			continue
		}
		systems[i].MountTargetsOutcome = "ok"
		systems[i].MountTargets = mts
	}

	return efsData{FileSystems: systems}, nil
}

func captureEFSMountTargets(ctx context.Context, client *efs.Client, fsID string) ([]efsMountTarget, error) {
	var mts []efsMountTarget
	var marker *string
	for {
		out, err := client.DescribeMountTargets(ctx, &efs.DescribeMountTargetsInput{FileSystemId: aws.String(fsID), Marker: marker})
		if err != nil {
			return nil, err
		}
		for _, mt := range out.MountTargets {
			mts = append(mts, efsMountTarget{
				MountTargetId:      aws.ToString(mt.MountTargetId),
				LifeCycleState:     string(mt.LifeCycleState),
				SubnetId:           aws.ToString(mt.SubnetId),
				VpcId:              aws.ToString(mt.VpcId),
				NetworkInterfaceId: aws.ToString(mt.NetworkInterfaceId),
			})
		}
		if out.NextMarker == nil || *out.NextMarker == "" {
			break
		}
		marker = out.NextMarker
	}
	return mts, nil
}
