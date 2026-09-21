// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// AWS namespaces some enums with a dot — a NAT gateway's FailureCode is
// "Gateway.NotAttached", a target's health reason is "Target.FailedHealthChecks".
// The dot separates words; a cell that keeps it reads as a sentence that lost
// its subject.
package unit_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/domain"
)

func TestHumanizeStatusPhrase_DottedEnumReadsAsWords(t *testing.T) {
	for raw, want := range map[string]string{
		"Gateway.NotAttached":               "gateway not attached",
		"Target.FailedHealthChecks":         "target failed health checks",
		"Elb.InternalError":                 "elb internal error",
		"InsufficientFreeAddressesInSubnet": "insufficient free addresses in subnet",
		"CREATE_FAILED":                     "create failed",
		"unhealthy targets: 2/5":            "unhealthy targets: 2/5",
		"1.2.3":                             "1.2.3",
	} {
		if got := domain.HumanizeStatusPhrase(raw); got != want {
			t.Errorf("HumanizeStatusPhrase(%q) = %q, want %q", raw, got, want)
		}
	}
}
