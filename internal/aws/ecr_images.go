package aws

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/ecr"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

const maxECRImages = 500

func init() {
	resource.RegisterFieldKeys("ecr_images", []string{
		"image_tags", "digest_short", "pushed_at", "image_size",
		"scan_status", "finding_counts", "image_uri", "image_digest",
		"repository_name",
	})

	resource.RegisterChildFetcher("ecr_images", func(ctx context.Context, clients interface{}, parentCtx resource.ParentContext) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchECRImages(ctx, c.ECR, parentCtx)
	})

	resource.RegisterChildType(resource.ResourceTypeDef{
		Name:      "ECR Images",
		ShortName: "ecr_images",
		Columns:   resource.ECRImageColumns(),
		CopyField: "image_uri",
	})
}

// FetchECRImages calls the ECR DescribeImages API with pagination and converts
// the response into a slice of generic Resource structs sorted by push time
// (newest first).
func FetchECRImages(ctx context.Context, api ECRDescribeImagesAPI, parentCtx map[string]string) ([]resource.Resource, error) {
	repositoryName := parentCtx["repository_name"]
	repositoryURI := parentCtx["repository_uri"]

	var allImages []ecrtypes.ImageDetail
	var nextToken *string

	for {
		input := &ecr.DescribeImagesInput{
			RepositoryName: &repositoryName,
			NextToken:      nextToken,
		}

		output, err := api.DescribeImages(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("describing images for %s: %w", repositoryName, err)
		}

		allImages = append(allImages, output.ImageDetails...)

		if len(allImages) >= maxECRImages {
			allImages = allImages[:maxECRImages]
			break
		}

		if output.NextToken == nil {
			break
		}
		nextToken = output.NextToken
	}

	if len(allImages) == 0 {
		return []resource.Resource{}, nil
	}

	// Sort by ImagePushedAt descending (newest first)
	sort.Slice(allImages, func(i, j int) bool {
		if allImages[i].ImagePushedAt == nil {
			return false
		}
		if allImages[j].ImagePushedAt == nil {
			return true
		}
		return allImages[i].ImagePushedAt.After(*allImages[j].ImagePushedAt)
	})

	resources := make([]resource.Resource, 0, len(allImages))
	for _, img := range allImages {
		resources = append(resources, convertECRImage(img, repositoryURI, repositoryName))
	}

	return resources, nil
}

// convertECRImage converts a single ecrtypes.ImageDetail into a generic Resource.
func convertECRImage(img ecrtypes.ImageDetail, repositoryURI, repositoryName string) resource.Resource {
	digest := ""
	if img.ImageDigest != nil {
		digest = *img.ImageDigest
	}

	// image_tags
	imageTags := "<untagged>"
	if len(img.ImageTags) > 0 {
		imageTags = strings.Join(img.ImageTags, ", ")
	}

	// digest_short: strip "sha256:" prefix, first 12 chars
	digestShort := ""
	if digest != "" {
		short := strings.TrimPrefix(digest, "sha256:")
		if len(short) > 12 {
			short = short[:12]
		}
		digestShort = short
	}

	// pushed_at
	pushedAt := ""
	if img.ImagePushedAt != nil {
		pushedAt = img.ImagePushedAt.UTC().Format("2006-01-02 15:04:05")
	}

	// image_size
	imageSize := ""
	if img.ImageSizeInBytes != nil {
		imageSize = formatBytes(*img.ImageSizeInBytes)
	}

	// scan_status
	scanStatus := ""
	if img.ImageScanStatus != nil {
		scanStatus = string(img.ImageScanStatus.Status)
	}

	// finding_counts
	findingCounts := ""
	if img.ImageScanFindingsSummary != nil {
		findingCounts = formatFindingCounts(img.ImageScanFindingsSummary.FindingSeverityCounts)
	}

	// image_uri
	imageURI := ""
	if len(img.ImageTags) > 0 {
		imageURI = repositoryURI + ":" + img.ImageTags[0]
	} else if digest != "" {
		imageURI = repositoryURI + "@" + digest
	}

	// Name: first tag or digest_short
	name := digestShort
	if len(img.ImageTags) > 0 {
		name = imageTags
	}

	status := computeImageStatus(img)

	return resource.Resource{
		ID:     digest,
		Name:   name,
		Status: status,
		Fields: map[string]string{
			"image_tags":      imageTags,
			"digest_short":    digestShort,
			"pushed_at":       pushedAt,
			"image_size":      imageSize,
			"scan_status":     scanStatus,
			"finding_counts":  findingCounts,
			"image_uri":       imageURI,
			"image_digest":    digest,
			"repository_name": repositoryName,
		},
		RawStruct: img,
	}
}

// computeImageStatus determines the resource status based on scan findings
// and tag state.
func computeImageStatus(img ecrtypes.ImageDetail) string {
	// Check scan failures first
	if img.ImageScanStatus != nil && img.ImageScanStatus.Status == ecrtypes.ScanStatusFailed {
		return "failed"
	}

	// Check finding severity counts
	if img.ImageScanFindingsSummary != nil && len(img.ImageScanFindingsSummary.FindingSeverityCounts) > 0 {
		counts := img.ImageScanFindingsSummary.FindingSeverityCounts
		if c, ok := counts["CRITICAL"]; ok && c > 0 {
			return "failed"
		}
		if h, ok := counts["HIGH"]; ok && h > 0 {
			return "pending"
		}
	}

	// Untagged with no findings
	if len(img.ImageTags) == 0 {
		return "terminated"
	}

	return ""
}

// formatFindingCounts formats FindingSeverityCounts as "1C 3H 5M"
// (only non-zero, sorted by severity: CRITICAL, HIGH, MEDIUM, LOW, INFORMATIONAL).
func formatFindingCounts(counts map[string]int32) string {
	if len(counts) == 0 {
		return ""
	}

	// Severity order with abbreviations
	severityOrder := []struct {
		key   string
		abbr  string
	}{
		{"CRITICAL", "C"},
		{"HIGH", "H"},
		{"MEDIUM", "M"},
		{"LOW", "L"},
		{"INFORMATIONAL", "I"},
	}

	var parts []string
	for _, sev := range severityOrder {
		if count, ok := counts[sev.key]; ok && count > 0 {
			parts = append(parts, fmt.Sprintf("%d%s", count, sev.abbr))
		}
	}

	return strings.Join(parts, " ")
}
