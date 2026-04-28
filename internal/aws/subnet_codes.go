package aws

import "github.com/k2m30/a9s/v3/internal/domain"

const (
	CodeSubnetStatePending                    domain.FindingCode = "subnet.state.pending"
	CodeSubnetStateUnavailable                domain.FindingCode = "subnet.state.unavailable"
	CodeSubnetStateFailed                     domain.FindingCode = "subnet.state.failed"
	CodeSubnetStateFailedInsufficientCapacity domain.FindingCode = "subnet.state.failed-insufficient-capacity"
)
