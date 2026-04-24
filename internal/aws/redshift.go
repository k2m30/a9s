package aws

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/redshift"
	redshifttypes "github.com/aws/aws-sdk-go-v2/service/redshift/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("redshift", []string{"cluster_id", "status", "cluster_status", "node_type", "num_nodes", "db_name", "endpoint", "publicly_accessible", "encrypted", "cluster_availability_status"})

	resource.RegisterPaginated("redshift", func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchRedshiftClustersPage(ctx, c.Redshift, continuationToken)
	})
}

// FetchRedshiftClusters calls the Redshift DescribeClusters API and converts the
// response into a slice of generic Resource structs.
func FetchRedshiftClusters(ctx context.Context, api RedshiftDescribeClustersAPI) ([]resource.Resource, error) {
	var all []resource.Resource
	token := ""
	for {
		result, err := FetchRedshiftClustersPage(ctx, api, token)
		if err != nil {
			return nil, err
		}
		all = append(all, result.Resources...)
		if result.Pagination == nil || !result.Pagination.IsTruncated {
			break
		}
		token = result.Pagination.NextToken
	}
	return all, nil
}

// FetchRedshiftClustersPage fetches a single page of Redshift clusters.
func FetchRedshiftClustersPage(ctx context.Context, api RedshiftDescribeClustersAPI, continuationToken string) (resource.FetchResult, error) {
	input := &redshift.DescribeClustersInput{
		MaxRecords: aws.Int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.Marker = &continuationToken
	}

	output, err := api.DescribeClusters(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching Redshift clusters: %w", err)
	}

	var resources []resource.Resource

	for _, cluster := range output.Clusters {
		clusterID := ""
		if cluster.ClusterIdentifier != nil {
			clusterID = *cluster.ClusterIdentifier
		}

		clusterStatus := ""
		if cluster.ClusterStatus != nil {
			clusterStatus = *cluster.ClusterStatus
		}

		nodeType := ""
		if cluster.NodeType != nil {
			nodeType = *cluster.NodeType
		}

		numNodes := ""
		if cluster.NumberOfNodes != nil {
			numNodes = strconv.Itoa(int(*cluster.NumberOfNodes))
		}

		dbName := ""
		if cluster.DBName != nil {
			dbName = *cluster.DBName
		}

		endpoint := ""
		if cluster.Endpoint != nil && cluster.Endpoint.Address != nil {
			endpoint = *cluster.Endpoint.Address
		}

		masterUser := ""
		if cluster.MasterUsername != nil {
			masterUser = *cluster.MasterUsername
		}

		createTime := ""
		if cluster.ClusterCreateTime != nil {
			createTime = cluster.ClusterCreateTime.Format("2006-01-02 15:04")
		}

		publiclyAccessible := "false"
		if cluster.PubliclyAccessible != nil && *cluster.PubliclyAccessible {
			publiclyAccessible = "true"
		}

		encrypted := "false"
		if cluster.Encrypted != nil && *cluster.Encrypted {
			encrypted = "true"
		}

		clusterAvailabilityStatus := ""
		if cluster.ClusterAvailabilityStatus != nil {
			clusterAvailabilityStatus = *cluster.ClusterAvailabilityStatus
		}

		derivedStatus, issues := computeRedshiftStatusAndIssues(cluster)

		r := resource.Resource{
			ID:     clusterID,
			Name:   clusterID,
			Status: derivedStatus,
			Issues: issues,
			Fields: map[string]string{
				"cluster_id":                  clusterID,
				"status":                      derivedStatus,
				"cluster_status":              clusterStatus,
				"node_type":                   nodeType,
				"num_nodes":                   numNodes,
				"db_name":                     dbName,
				"endpoint":                    endpoint,
				"master_user":                 masterUser,
				"create_time":                 createTime,
				"publicly_accessible":         publiclyAccessible,
				"encrypted":                   encrypted,
				"cluster_availability_status": clusterAvailabilityStatus,
			},
			RawStruct: cluster,
		}

		resources = append(resources, r)
	}

	nextToken := ""
	isTruncated := false
	if output.Marker != nil {
		nextToken = *output.Marker
		isTruncated = true
	}

	totalHint := len(resources)
	if isTruncated {
		totalHint = -1
	}

	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: isTruncated,
			NextToken:   nextToken,
			PageSize:    len(resources),
			TotalHint:   totalHint,
		},
	}, nil
}

// computeRedshiftStatusAndIssues returns the top S4 phrase (with `(+N)` suffix
// when multiple warnings stack) AND the full ordered list of every active issue
// phrase. Mirrors computeDBIStatusAndIssues from rds.go but uses Redshift signals
// per spec §3.1 and §4. Broken / transitional / healthy states each produce at
// most one phrase; configuration-warning states stack per rule 7.
func computeRedshiftStatusAndIssues(cluster redshifttypes.Cluster) (string, []string) {
	clusterStatus := ""
	if cluster.ClusterStatus != nil {
		clusterStatus = *cluster.ClusterStatus
	}

	clusterAvailStatus := ""
	if cluster.ClusterAvailabilityStatus != nil {
		clusterAvailStatus = *cluster.ClusterAvailabilityStatus
	}

	// Step 1: Broken bucket — ClusterStatus-driven (highest severity, beats availability).
	switch clusterStatus {
	case "incompatible-hsm":
		phrase := "broken: incompatible-hsm"
		return phrase, []string{phrase}
	case "incompatible-network":
		phrase := "broken: incompatible-network"
		return phrase, []string{phrase}
	case "incompatible-parameters":
		phrase := "broken: incompatible-parameters"
		return phrase, []string{phrase}
	case "incompatible-restore":
		phrase := "broken: incompatible-restore"
		return phrase, []string{phrase}
	case "hardware-failure":
		phrase := "broken: hardware-failure"
		return phrase, []string{phrase}
	case "storage-full":
		phrase := "broken: storage-full"
		return phrase, []string{phrase}
	}

	// Step 1b: Broken bucket — ClusterAvailabilityStatus-driven.
	switch clusterAvailStatus {
	case "Unavailable":
		return "unavailable", []string{"unavailable"}
	case "Failed":
		return "failed", []string{"failed"}
	}

	// Step 2: Transitional bucket (Warning, ClusterStatus-driven) — return immediately.
	switch clusterStatus {
	case "creating":
		return "creating", []string{"creating"}
	case "modifying":
		return "modifying", []string{"modifying"}
	case "resizing":
		return "resizing", []string{"resizing"}
	case "rebooting":
		return "rebooting", []string{"rebooting"}
	case "renaming":
		return "renaming", []string{"renaming"}
	case "deleting":
		return "deleting", []string{"deleting"}
	}

	// Steps 3–5: Warning bucket — collect all active warnings and stack them.
	var warnings []string

	// Step 3: ClusterAvailabilityStatus warnings (first in precedence).
	switch clusterAvailStatus {
	case "Maintenance":
		warnings = append(warnings, "maintenance")
	case "Modifying":
		warnings = append(warnings, "modifying")
	}

	// Step 4: Configuration / maintenance warnings (in §4 precedence order).

	// 4.1: PendingModifiedValues non-nil and at least one sub-field non-empty.
	if hasPendingRedshiftModifiedValues(cluster.PendingModifiedValues) {
		warnings = append(warnings, "pending change queued")
	}

	// 4.2: DeferredMaintenanceWindows with active window (now ∈ [start, end]).
	if hasActiveDeferredMaintenanceWindow(cluster.DeferredMaintenanceWindows, time.Now().UTC()) {
		warnings = append(warnings, "maintenance deferred")
	}

	// 4.3: PubliclyAccessible == true.
	if cluster.PubliclyAccessible != nil && *cluster.PubliclyAccessible {
		warnings = append(warnings, "publicly accessible")
	}

	// 4.4: Encrypted == false.
	if cluster.Encrypted != nil && !*cluster.Encrypted {
		warnings = append(warnings, "unencrypted at rest")
	}

	// Step 5: Combine.
	switch len(warnings) {
	case 0:
		return "", nil
	case 1:
		return warnings[0], warnings
	default:
		return fmt.Sprintf("%s (+%d)", warnings[0], len(warnings)-1), warnings
	}
}

// hasPendingRedshiftModifiedValues returns true when PendingModifiedValues is
// non-nil and at least one sub-field carries a value.
func hasPendingRedshiftModifiedValues(pmv *redshifttypes.PendingModifiedValues) bool {
	if pmv == nil {
		return false
	}
	if pmv.NodeType != nil && *pmv.NodeType != "" {
		return true
	}
	if pmv.NumberOfNodes != nil {
		return true
	}
	if pmv.ClusterType != nil && *pmv.ClusterType != "" {
		return true
	}
	if pmv.ClusterVersion != nil && *pmv.ClusterVersion != "" {
		return true
	}
	if pmv.AutomatedSnapshotRetentionPeriod != nil {
		return true
	}
	if pmv.ClusterIdentifier != nil && *pmv.ClusterIdentifier != "" {
		return true
	}
	if pmv.PubliclyAccessible != nil {
		return true
	}
	if pmv.EnhancedVpcRouting != nil {
		return true
	}
	if pmv.MaintenanceTrackName != nil && *pmv.MaintenanceTrackName != "" {
		return true
	}
	if pmv.MasterUserPassword != nil && *pmv.MasterUserPassword != "" {
		return true
	}
	if pmv.EncryptionType != nil && *pmv.EncryptionType != "" {
		return true
	}
	return false
}

// hasActiveDeferredMaintenanceWindow returns true when at least one window
// contains `now` within [DeferMaintenanceStartTime, DeferMaintenanceEndTime].
// A nil start time is treated as active-from-epoch; a nil end time is treated
// as active-forever. `now` is injected so tests can pin boundary behavior.
func hasActiveDeferredMaintenanceWindow(windows []redshifttypes.DeferredMaintenanceWindow, now time.Time) bool {
	for _, w := range windows {
		startActive := true
		if w.DeferMaintenanceStartTime != nil {
			startActive = !now.Before(*w.DeferMaintenanceStartTime)
		}
		endActive := true
		if w.DeferMaintenanceEndTime != nil {
			endActive = !now.After(*w.DeferMaintenanceEndTime)
		}
		if startActive && endActive {
			return true
		}
	}
	return false
}
