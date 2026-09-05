// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import "github.com/k2m30/a9s/v3/core/domain"

const (
	CodeSubnetStatePending                    domain.FindingCode = "subnet.state.pending"
	CodeSubnetStateUnavailable                domain.FindingCode = "subnet.state.unavailable"
	CodeSubnetStateFailed                     domain.FindingCode = "subnet.state.failed"
	CodeSubnetStateFailedInsufficientCapacity domain.FindingCode = "subnet.state.failed-insufficient-capacity"

	// CodeSubnetAutoPublicIP marks a subnet that hands every instance
	// launched into it a public IP without anyone asking for one.
	CodeSubnetAutoPublicIP domain.FindingCode = "subnet.auto-public-ip"
)

// SubnetAutoPublicIPPhrase is the S4 status phrase for CodeSubnetAutoPublicIP.
const SubnetAutoPublicIPPhrase = "auto-assigns public IPs"

// SubnetAutoPublicIPDetail is the S5 operator sentence for CodeSubnetAutoPublicIP.
const SubnetAutoPublicIPDetail = "Every instance launched into this subnet is given a public address by default, " +
	"so a workload reaches the internet whether or not its owner intended it to. Turn the subnet's auto-assign " +
	"public address setting off and attach an elastic address to the instances that genuinely need one."
