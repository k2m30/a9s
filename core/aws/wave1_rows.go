// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// wave1_rows.go — the Wave-1 counterpart of setWave2Finding's AttentionDetail
// packing. Fetchers that emit a Finding with supporting rows attach them here
// so the map keying (Resource.ID → FindingCode → rows) matches what
// ApplyWave2ToRow and buildAttentionEntries already read.
package aws

import (
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// addWave1Rows attaches supporting rows to a Wave-1 Finding already appended
// to r.Findings, keyed by that Finding's own Code so two independently
// evaluated conditions on one resource never crowd out each other's rows.
func addWave1Rows(r *resource.Resource, code domain.FindingCode, rows ...domain.DetailRow) {
	if len(rows) == 0 {
		return
	}
	if r.AttentionDetails == nil {
		r.AttentionDetails = make(map[domain.FindingCode]domain.AttentionDetail, 1)
	}
	ad := r.AttentionDetails[code]
	ad.Rows = append(ad.Rows, rows...)
	r.AttentionDetails[code] = ad
}
