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

// efsW1Signals returns the active Wave-1 issue phrases for this filesystem in
// §4 precedence order: Broken signals first (error, no mount targets), then
// Warning signals (creating, updating, deleting).
//
// The first phrase is the "top" displayed in the Status column (plus (+N-1)
// suffix when more than one phrase is active). All phrases are appended to
// Resource.Issues so the detail view can render every signal individually.
func efsW1Signals(lcs efstypes.LifeCycleState, numMT int32) []string {
	// §4 precedence order (severity first, then table order within severity):
	//   Broken: "error", "no mount targets"
	//   Warning: "creating", "updating", "deleting"
	var phrases []string

	switch lcs {
	case efstypes.LifeCycleStateError:
		phrases = append(phrases, "error")
	case efstypes.LifeCycleStateCreating:
		phrases = append(phrases, "creating")
	case efstypes.LifeCycleStateUpdating:
		phrases = append(phrases, "updating")
	case efstypes.LifeCycleStateDeleting:
		phrases = append(phrases, "deleting")
	// "available" and "deleted" produce no W1 phrase.
	}

	// "no mount targets" applies only while the filesystem exists. A deleted
	// filesystem intrinsically has no mount targets — surfacing that as a
	// broken-severity finding is noise (spec §4: "deleted" produces no W1
	// phrase, and the absence of mount targets is a consequence of deletion,
	// not an independent signal).
	if numMT == 0 && lcs != efstypes.LifeCycleStateDeleted {
		// Insert "no mount targets" before Warning phrases (it is Broken-severity).
		// If "error" is already present, append after it. Otherwise prepend.
		if len(phrases) > 0 && phrases[0] == "error" {
			phrases = append([]string{phrases[0], "no mount targets"}, phrases[1:]...)
		} else {
			// "no mount targets" is Broken, so it precedes any Warning phrase.
			phrases = append([]string{"no mount targets"}, phrases...)
		}
	}

	return phrases
}

// efsW1Findings returns the active Wave-1 findings for this filesystem.
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
	}

	if numMT == 0 && lcs != efstypes.LifeCycleStateDeleted {
		noMTFinding := domain.Finding{Code: CodeEFSNoMountTargets, Phrase: "no mount targets", Severity: domain.SevBroken, Source: "wave1"}
		if len(findings) > 0 && findings[0].Code == CodeEFSError {
			findings = append([]domain.Finding{findings[0], noMTFinding}, findings[1:]...)
		} else {
			findings = append([]domain.Finding{noMTFinding}, findings...)
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

		// Compute Wave-1 findings.
		findings := efsW1Findings(fs.LifeCycleState, fs.NumberOfMountTargets)
		statusPhrase := phraseFromFindings(findings)

		r := resource.Resource{
			ID:       fsID,
			Name:     name,
			Findings: findings,
			Fields: map[string]string{
				"file_system_id":   fsID,
				"name":             name,
				"status":           statusPhrase,
				"performance_mode": performanceMode,
				"throughput_mode":  throughputMode,
				"encrypted":        encrypted,
				"mount_targets":    mountTargets,
			},
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
