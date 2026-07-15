// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import "github.com/k2m30/a9s/v3/core/domain"

const (
	CodeECRImageScanFailed domain.FindingCode = "ecr_images.broken.scan_failed"
	CodeECRImageCritical   domain.FindingCode = "ecr_images.broken.critical"
	CodeECRImageHigh       domain.FindingCode = "ecr_images.warn.high"
	CodeECRImageUntagged   domain.FindingCode = "ecr_images.dim.untagged"
)
