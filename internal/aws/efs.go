package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/efs"
	efstypes "github.com/aws/aws-sdk-go-v2/service/efs/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("efs", []string{"file_system_id", "name", "status", "performance_mode", "encrypted", "mount_targets"})

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

	if numMT == 0 {
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

// FetchEFSFileSystemsPage fetches a single page of EFS file systems.
func FetchEFSFileSystemsPage(ctx context.Context, api EFSDescribeFileSystemsAPI, continuationToken string) (resource.FetchResult, error) {
	input := &efs.DescribeFileSystemsInput{
		MaxItems: aws.Int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.Marker = &continuationToken
	}

	output, err := api.DescribeFileSystems(ctx, input)
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

		// Compute Wave-1 signals in §4 precedence order.
		signals := efsW1Signals(fs.LifeCycleState, fs.NumberOfMountTargets)

		// Derive Status (S4) and Issues from active signals.
		var status string
		var issues []string

		switch len(signals) {
		case 0:
			// Healthy — blank S4.
			status = ""
			issues = nil
		case 1:
			status = signals[0]
			issues = signals
		default:
			// Multiple W1 signals: top phrase + "(+N-1)" suffix where N-1 = len-1.
			status = fmt.Sprintf("%s (+%d)", signals[0], len(signals)-1)
			issues = signals
		}

		r := resource.Resource{
			ID:     fsID,
			Name:   name,
			Status: status,
			Issues: issues,
			Fields: map[string]string{
				"file_system_id":   fsID,
				"name":             name,
				"status":           status,
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
