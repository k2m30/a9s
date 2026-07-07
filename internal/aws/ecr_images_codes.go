package aws

import "github.com/k2m30/a9s/v3/internal/domain"

const (
	CodeECRImageScanFailed domain.FindingCode = "ecr_images.broken.scan_failed"
	CodeECRImageCritical   domain.FindingCode = "ecr_images.broken.critical"
	CodeECRImageHigh       domain.FindingCode = "ecr_images.warn.high"
	CodeECRImageUntagged   domain.FindingCode = "ecr_images.dim.untagged"
)
