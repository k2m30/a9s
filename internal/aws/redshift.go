package aws

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/redshift"
	redshifttypes "github.com/aws/aws-sdk-go-v2/service/redshift/types"

	"github.com/k2m30/a9s/v3/internal/domain"
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

	output, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*redshift.DescribeClustersOutput, error) {
		return api.DescribeClusters(ctx, input)
	})
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

		findings := computeRedshiftFindings(cluster)
		statusPhrase := ""
		if len(findings) > 0 {
			statusPhrase = findings[0].Phrase
			if len(findings) > 1 {
				statusPhrase = fmt.Sprintf("%s (+%d)", statusPhrase, len(findings)-1)
			}
		}

		r := resource.Resource{
			ID:   clusterID,
			Name: clusterID,
			Fields: map[string]string{
				"cluster_id":                  clusterID,
				"status":                      statusPhrase,
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
			Findings:  findings,
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

// computeRedshiftFindings returns the ordered Wave-1 findings for a Redshift cluster.
// Broken / transitional / healthy states each produce at most one finding;
// configuration-warning states may stack multiple findings.
func computeRedshiftFindings(cluster redshifttypes.Cluster) []domain.Finding {
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
		return []domain.Finding{{Code: CodeRedshiftIncompatibleHSM, Phrase: "broken: incompatible-hsm", Severity: domain.SevBroken, Source: "wave1"}}
	case "incompatible-network":
		return []domain.Finding{{Code: CodeRedshiftIncompatibleNetwork, Phrase: "broken: incompatible-network", Severity: domain.SevBroken, Source: "wave1"}}
	case "incompatible-parameters":
		return []domain.Finding{{Code: CodeRedshiftIncompatibleParameters, Phrase: "broken: incompatible-parameters", Severity: domain.SevBroken, Source: "wave1"}}
	case "incompatible-restore":
		return []domain.Finding{{Code: CodeRedshiftIncompatibleRestore, Phrase: "broken: incompatible-restore", Severity: domain.SevBroken, Source: "wave1"}}
	case "hardware-failure":
		return []domain.Finding{{Code: CodeRedshiftHardwareFailure, Phrase: "broken: hardware-failure", Severity: domain.SevBroken, Source: "wave1"}}
	case "storage-full":
		return []domain.Finding{{Code: CodeRedshiftStorageFull, Phrase: "broken: storage-full", Severity: domain.SevBroken, Source: "wave1"}}
	}

	// Step 1b: Broken bucket — ClusterAvailabilityStatus-driven.
	switch clusterAvailStatus {
	case "Unavailable":
		return []domain.Finding{{Code: CodeRedshiftUnavailable, Phrase: "unavailable", Severity: domain.SevBroken, Source: "wave1"}}
	case "Failed":
		return []domain.Finding{{Code: CodeRedshiftFailed, Phrase: "failed", Severity: domain.SevBroken, Source: "wave1"}}
	}

	// Step 2: Transitional bucket (Warning, ClusterStatus-driven) — return immediately.
	switch clusterStatus {
	case "creating":
		return []domain.Finding{{Code: CodeRedshiftCreating, Phrase: "creating", Severity: domain.SevWarn, Source: "wave1"}}
	case "modifying":
		return []domain.Finding{{Code: CodeRedshiftModifying, Phrase: "modifying", Severity: domain.SevWarn, Source: "wave1"}}
	case "resizing":
		return []domain.Finding{{Code: CodeRedshiftResizing, Phrase: "resizing", Severity: domain.SevWarn, Source: "wave1"}}
	case "rebooting":
		return []domain.Finding{{Code: CodeRedshiftRebooting, Phrase: "rebooting", Severity: domain.SevWarn, Source: "wave1"}}
	case "renaming":
		return []domain.Finding{{Code: CodeRedshiftRenaming, Phrase: "renaming", Severity: domain.SevWarn, Source: "wave1"}}
	case "deleting":
		return []domain.Finding{{Code: CodeRedshiftDeleting, Phrase: "deleting", Severity: domain.SevWarn, Source: "wave1"}}
	}

	// Steps 3–5: Warning bucket — collect all active warnings and stack them.
	var warnings []domain.Finding

	// Step 3: ClusterAvailabilityStatus warnings (first in precedence).
	switch clusterAvailStatus {
	case "Maintenance":
		warnings = append(warnings, domain.Finding{Code: CodeRedshiftMaintenance, Phrase: "maintenance", Severity: domain.SevWarn, Source: "wave1"})
	case "Modifying":
		warnings = append(warnings, domain.Finding{Code: CodeRedshiftAvailabilityModifying, Phrase: "modifying", Severity: domain.SevWarn, Source: "wave1"})
	}

	// Step 4: Configuration / maintenance warnings (in precedence order).

	// 4.1: PendingModifiedValues non-nil and at least one sub-field non-empty.
	if hasPendingRedshiftModifiedValues(cluster.PendingModifiedValues) {
		warnings = append(warnings, domain.Finding{Code: CodeRedshiftPendingChange, Phrase: "pending change queued", Severity: domain.SevWarn, Source: "wave1"})
	}

	// 4.2: DeferredMaintenanceWindows with active window (now ∈ [start, end]).
	if hasActiveDeferredMaintenanceWindow(cluster.DeferredMaintenanceWindows, time.Now().UTC()) {
		warnings = append(warnings, domain.Finding{Code: CodeRedshiftMaintenanceDeferred, Phrase: "maintenance deferred", Severity: domain.SevWarn, Source: "wave1"})
	}

	// 4.3: PubliclyAccessible == true.
	if cluster.PubliclyAccessible != nil && *cluster.PubliclyAccessible {
		warnings = append(warnings, domain.Finding{Code: CodeRedshiftPubliclyAccessible, Phrase: "publicly accessible", Severity: domain.SevWarn, Source: "wave1"})
	}

	// 4.4: Encrypted == false.
	if cluster.Encrypted != nil && !*cluster.Encrypted {
		warnings = append(warnings, domain.Finding{Code: CodeRedshiftUnencryptedAtRest, Phrase: "unencrypted at rest", Severity: domain.SevWarn, Source: "wave1"})
	}

	return warnings
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
