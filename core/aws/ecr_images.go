// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// FetchECRImages calls the ECR DescribeImages API once per invocation, reads
// each image's scan results through DescribeImageScanFindings, and converts the
// response into a FetchResult with pagination support. Images are sorted by
// push time (newest first) before being returned. IsTruncated and NextToken
// are forwarded as pagination metadata for the caller to request the next page.
func FetchECRImages(ctx context.Context, api ECRDescribeImagesAPI, parentCtx map[string]string, continuationToken string) (resource.FetchResult, error) {
	repositoryName := parentCtx["repository_name"]
	repositoryURI := parentCtx["repository_uri"]

	input := &ecr.DescribeImagesInput{
		RepositoryName: &repositoryName,
	}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	output, err := api.DescribeImages(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("describing images for %s: %w", repositoryName, err)
	}

	pageImages := output.ImageDetails

	if len(pageImages) == 0 {
		return resource.FetchResult{
			Resources: []resource.Resource{},
			Pagination: &resource.PaginationMeta{
				IsTruncated: false,
				TotalHint:   0,
				PageSize:    0,
			},
		}, nil
	}

	sort.Slice(pageImages, func(i, j int) bool {
		if pageImages[i].ImagePushedAt == nil {
			return false
		}
		if pageImages[j].ImagePushedAt == nil {
			return true
		}
		return pageImages[i].ImagePushedAt.After(*pageImages[j].ImagePushedAt)
	})

	scanAPI, _ := api.(ECRDescribeImageScanFindingsAPI)
	unread := make([]bool, len(pageImages))
	if err := ForEachParallel(ctx, len(pageImages), EnrichmentParallelism, func(i int) {
		unread[i] = ecrReadScanResults(ctx, scanAPI, repositoryName, &pageImages[i]) != nil
	}); err != nil {
		return resource.FetchResult{}, fmt.Errorf("reading scan results for %s: %w", repositoryName, err)
	}

	resources := make([]resource.Resource, 0, len(pageImages))
	for i, img := range pageImages {
		r := convertECRImage(img, repositoryURI, repositoryName)
		if unread[i] {
			// Not inspected, not clean: the scan results exist but could not
			// be read, so the counts are unknown rather than empty.
			r.Fields["scan_status"] = "?"
			r.Fields["finding_counts"] = "?"
			r.Findings = append(r.Findings, wave1Finding(CodeECRImageScanUnread))
		}
		resources = append(resources, r)
	}

	nextToken := ""
	isTruncated := false
	if output.NextToken != nil {
		nextToken = *output.NextToken
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

// ecrReadScanResults fills img's scan status and severity summary from
// DescribeImageScanFindings: under Basic Scanning DescribeImages leaves both
// empty. An image that already carries a summary is left as it is, and so is
// one never scanned (ScanNotFoundException). A nil api is a client that
// predates the call and reads nothing.
func ecrReadScanResults(ctx context.Context, api ECRDescribeImageScanFindingsAPI, repo string, img *ecrtypes.ImageDetail) error {
	if img.ImageScanFindingsSummary != nil || api == nil {
		return nil
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*ecr.DescribeImageScanFindingsOutput, error) {
		return api.DescribeImageScanFindings(ctx, &ecr.DescribeImageScanFindingsInput{
			RepositoryName: aws.String(repo),
			ImageId:        &ecrtypes.ImageIdentifier{ImageDigest: img.ImageDigest},
		})
	})
	if _, notScanned := errors.AsType[*ecrtypes.ScanNotFoundException](err); notScanned {
		return nil
	}
	if err != nil {
		return err
	}
	img.ImageScanStatus = out.ImageScanStatus
	if f := out.ImageScanFindings; f != nil {
		img.ImageScanFindingsSummary = &ecrtypes.ImageScanFindingsSummary{
			FindingSeverityCounts:        f.FindingSeverityCounts,
			ImageScanCompletedAt:         f.ImageScanCompletedAt,
			VulnerabilitySourceUpdatedAt: f.VulnerabilitySourceUpdatedAt,
		}
	}
	return nil
}

// convertECRImage converts a single ecrtypes.ImageDetail into a generic Resource.
func convertECRImage(img ecrtypes.ImageDetail, repositoryURI, repositoryName string) resource.Resource {
	digest := ""
	if img.ImageDigest != nil {
		digest = *img.ImageDigest
	}

	imageTags := "<untagged>"
	if len(img.ImageTags) > 0 {
		imageTags = strings.Join(img.ImageTags, ", ")
	}

	digestShort := ""
	if digest != "" {
		short := strings.TrimPrefix(digest, "sha256:")
		if len(short) > 12 {
			short = short[:12]
		}
		digestShort = short
	}

	pushedAt := ""
	if img.ImagePushedAt != nil {
		pushedAt = img.ImagePushedAt.UTC().Format("2006-01-02 15:04")
	}

	imageSize, imageSizeRaw := "", ""
	if img.ImageSizeInBytes != nil {
		imageSize = formatBytes(*img.ImageSizeInBytes)
		imageSizeRaw = strconv.FormatInt(*img.ImageSizeInBytes, 10)
	}

	scanStatus := ""
	if img.ImageScanStatus != nil {
		scanStatus = string(img.ImageScanStatus.Status)
	}

	findingCounts := ""
	if img.ImageScanFindingsSummary != nil {
		findingCounts = formatFindingCounts(img.ImageScanFindingsSummary.FindingSeverityCounts)
	}

	imageURI := ""
	if len(img.ImageTags) > 0 {
		imageURI = repositoryURI + ":" + img.ImageTags[0]
	} else if digest != "" {
		imageURI = repositoryURI + "@" + digest
	}

	name := digestShort
	if len(img.ImageTags) > 0 {
		name = imageTags
	}

	status := computeImageStatus(img)

	return resource.Resource{
		ID:   digest,
		Name: name,
		Fields: map[string]string{
			"status":          status,
			"image_tags":      imageTags,
			"digest_short":    digestShort,
			"pushed_at":       pushedAt,
			"image_size":      imageSize,
			"image_size_raw":  imageSizeRaw,
			"scan_status":     scanStatus,
			"finding_counts":  findingCounts,
			"image_uri":       imageURI,
			"image_digest":    digest,
			"repository_name": repositoryName,
		},
		Findings:  ecrImageFindings(img),
		RawStruct: img,
	}
}

// ecrImageFindings derives Wave-1 Findings directly from the same
// ImageScanStatus / ImageScanFindingsSummary fields computeImageStatus reads,
// so the row's color always matches the same priority order: a failed scan
// outranks a CRITICAL count, which outranks a HIGH count, which outranks an
// untagged (dangling) image. Healthy/unscanned images carry no finding.
func ecrImageFindings(img ecrtypes.ImageDetail) []domain.Finding {
	if img.ImageScanStatus != nil && img.ImageScanStatus.Status == ecrtypes.ScanStatusFailed {
		return []domain.Finding{wave1Finding(CodeECRImageScanFailed)}
	}

	if img.ImageScanFindingsSummary != nil {
		counts := img.ImageScanFindingsSummary.FindingSeverityCounts
		if c, ok := counts["CRITICAL"]; ok && c > 0 {
			return []domain.Finding{wave1Finding(CodeECRImageCritical, strconv.Itoa(int(c)))}
		}
		if h, ok := counts["HIGH"]; ok && h > 0 {
			return []domain.Finding{wave1Finding(CodeECRImageHigh, strconv.Itoa(int(h)))}
		}
	}

	if len(img.ImageTags) == 0 {
		return []domain.Finding{wave1Finding(CodeECRImageUntagged)}
	}

	return nil
}

// computeImageStatus determines the resource status based on scan findings
// and tag state.
func computeImageStatus(img ecrtypes.ImageDetail) string {
	if img.ImageScanStatus != nil && img.ImageScanStatus.Status == ecrtypes.ScanStatusFailed {
		return "failed"
	}

	if img.ImageScanFindingsSummary != nil && len(img.ImageScanFindingsSummary.FindingSeverityCounts) > 0 {
		counts := img.ImageScanFindingsSummary.FindingSeverityCounts
		if c, ok := counts["CRITICAL"]; ok && c > 0 {
			return "failed"
		}
		if h, ok := counts["HIGH"]; ok && h > 0 {
			return "pending"
		}
	}

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

	severityOrder := []struct {
		key  string
		abbr string
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
