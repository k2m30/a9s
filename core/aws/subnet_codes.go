// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import "github.com/k2m30/a9s/v3/core/domain"

const (
	CodeSubnetStatePending                    domain.FindingCode = "subnet.state.pending"
	CodeSubnetStateUnavailable                domain.FindingCode = "subnet.state.unavailable"
	CodeSubnetStateFailed                     domain.FindingCode = "subnet.state.failed"
	CodeSubnetStateFailedInsufficientCapacity domain.FindingCode = "subnet.state.failed-insufficient-capacity"
)
