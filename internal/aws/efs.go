package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/efs"
	efstypes "github.com/aws/aws-sdk-go-v2/service/efs/types"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("efs", []string{"file_system_id", "name", "status", "performance_mode", "throughput_mode", "encrypted", "mount_targets"})

	resource.RegisterPaginated("efs", func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchEFSFileSystemsPage(ctx, c.EFS, continuationToken)
	})
}

// FetchEFSFileSystems calls the EFS DescribeFileSystems API and converts
// the response into a slice of generic Resource structs.
func FetchEFSFileSystems(ctx context.Context, api EFSDescribeFileSystemsAPI) ([]resource.Resource, error) {
	var all []resource.Resource
	token := ""
	for {
		result, err := FetchEFSFileSystemsPage(ctx, api, token)
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

// efsW1Findings returns the active Wave-1 findings for this filesystem in
// precedence order: Broken signals first (error, no mount targets), then
// Warning signals (creating, updating, deleting).
//
// The first finding's Phrase is displayed in the Status column. All findings
// are stored in Resource.Findings so the detail view can render every signal.
func efsW1Findings(lcs efstypes.LifeCycleState, numMT int32) []domain.Finding {
	var findings []domain.Finding

	switch lcs {
	case efstypes.LifeCycleStateError:
		findings = append(findings, domain.Finding{Code: CodeEFSError, Phrase: "error", Severity: domain.SevBroken, Source: "wave1"})
	case efstypes.LifeCycleStateCreating:
		findings = append(findings, domain.Finding{Code: CodeEFSCreating, Phrase: "creating", Severity: domain.SevWarn, Source: "wave1"})
	case efstypes.LifeCycleStateUpdating:
		findings = append(findings, domain.Finding{Code: CodeEFSUpdating, Phrase: "updating", Severity: domain.SevWarn, Source: "wave1"})
	case efstypes.LifeCycleStateDeleting:
		findings = append(findings, domain.Finding{Code: CodeEFSDeleting, Phrase: "deleting", Severity: domain.SevWarn, Source: "wave1"})
		// "available" and "deleted" produce no Wave-1 phrase.
	}

	// "no mount targets" applies only while the filesystem exists. A deleted
	// filesystem intrinsically has no mount targets — surfacing that as a
	// broken-severity finding is noise.
	if numMT == 0 && lcs != efstypes.LifeCycleStateDeleted {
		noMT := domain.Finding{Code: CodeEFSNoMountTargets, Phrase: "no mount targets", Severity: domain.SevBroken, Source: "wave1"}
		if len(findings) > 0 && findings[0].Code == CodeEFSError {
			// "error" is Broken, insert "no mount targets" after it.
			findings = append([]domain.Finding{findings[0], noMT}, findings[1:]...)
		} else {
			// "no mount targets" is Broken, so it precedes any Warning finding.
			findings = append([]domain.Finding{noMT}, findings...)
		}
	}

	return findings
}

// FetchEFSFileSystemsPage fetches a single page of EFS file systems.
func FetchEFSFileSystemsPage(ctx context.Context, api EFSDescribeFileSystemsAPI, continuationToken string) (resource.FetchResult, error) {
	input := &efs.DescribeFileSystemsInput{
		MaxItems: aws.Int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.Marker = &continuationToken
	}

	output, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*efs.DescribeFileSystemsOutput, error) {
		return api.DescribeFileSystems(ctx, input)
	})
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching EFS file systems: %w", err)
	}

	var resources []resource.Resource

	for _, fs := range output.FileSystems {
		fsID := ""
		if fs.FileSystemId != nil {
			fsID = *fs.FileSystemId
		}

		name := ""
		if fs.Name != nil {
			name = *fs.Name
		}

		performanceMode := string(fs.PerformanceMode)
		throughputMode := string(fs.ThroughputMode)

		encrypted := "false"
		if fs.Encrypted != nil && *fs.Encrypted {
			encrypted = "true"
		}

		mountTargets := fmt.Sprintf("%d", fs.NumberOfMountTargets)

		findings := efsW1Findings(fs.LifeCycleState, fs.NumberOfMountTargets)
		statusPhrase := ""
		if len(findings) > 0 {
			statusPhrase = findings[0].Phrase
			if len(findings) > 1 {
				statusPhrase = fmt.Sprintf("%s (+%d)", statusPhrase, len(findings)-1)
			}
		}

		r := resource.Resource{
			ID:   fsID,
			Name: name,
			Fields: map[string]string{
				"file_system_id":   fsID,
				"name":             name,
				"status":           statusPhrase,
				"performance_mode": performanceMode,
				"throughput_mode":  throughputMode,
				"encrypted":        encrypted,
				"mount_targets":    mountTargets,
			},
			Findings:  findings,
			RawStruct: fs,
		}

		resources = append(resources, r)
	}

	nextToken := ""
	isTruncated := false
	if output.NextMarker != nil {
		nextToken = *output.NextMarker
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
