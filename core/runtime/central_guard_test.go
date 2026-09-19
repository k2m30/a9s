// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package runtime

// central_guard_test.go — the central GenStamped guard in HandleEvent
// discards stale events before the per-type switch, so no handler can skip
// the staleness check.

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

func TestCentralGuard_StaleAvailabilityChecked(t *testing.T) {
	c := newCore()
	c.session.AvailabilityGen = domain.Gen(2)

	staleEvent := messages.AvailabilityChecked{
		ResourceType: "ec2",
		HasResources: true,
		Count:        3,
		Gen:          domain.Gen(1), // old gen
	}

	intents, tasks := c.HandleEvent(staleEvent)

	if len(intents) != 0 || len(tasks) != 0 {
		t.Fatalf("stale AvailabilityChecked: want (nil, nil), got (%v, %v)", intents, tasks)
	}
	if c.session.AvailChecked != 0 {
		t.Fatalf("stale event must not advance AvailChecked, got %d", c.session.AvailChecked)
	}
}

func TestCentralGuard_FreshAvailabilityChecked(t *testing.T) {
	c := newCore()
	c.session.AvailabilityGen = domain.Gen(2)
	c.session.AvailTotal = 1

	freshEvent := messages.AvailabilityChecked{
		ResourceType: "ec2",
		HasResources: true,
		Count:        3,
		Gen:          domain.Gen(2), // current gen
	}

	_, _ = c.HandleEvent(freshEvent)

	if c.session.AvailChecked != 1 {
		t.Fatalf("fresh AvailabilityChecked must advance AvailChecked to 1, got %d", c.session.AvailChecked)
	}
}

func TestCentralGuard_StaleEnrichmentChecked(t *testing.T) {
	c := newCore()
	c.session.EnrichmentGen = domain.Gen(5)

	staleEvent := messages.EnrichmentChecked{
		ResourceType: "s3",
		Gen:          domain.Gen(3), // old gen
	}

	intents, tasks := c.HandleEvent(staleEvent)

	if len(intents) != 0 || len(tasks) != 0 {
		t.Fatalf("stale EnrichmentChecked: want (nil, nil), got (%v, %v)", intents, tasks)
	}
	if c.session.EnrichChecked != 0 {
		t.Fatalf("stale event must not advance EnrichChecked, got %d", c.session.EnrichChecked)
	}
}

// AcceptZeroGen is false for AvailabilityChecked: Gen=0 is stale once the
// session counter is non-zero.
func TestCentralGuard_ZeroGenAvailabilityChecked(t *testing.T) {
	c := newCore()
	c.session.AvailabilityGen = domain.Gen(1) // non-zero session counter

	zeroEvent := messages.AvailabilityChecked{
		ResourceType: "ec2",
		Gen:          0, // zero stamp
	}

	intents, tasks := c.HandleEvent(zeroEvent)

	if len(intents) != 0 || len(tasks) != 0 {
		t.Fatalf("zero-gen AvailabilityChecked against non-zero session gen: want discard, got (%v, %v)", intents, tasks)
	}
}

// AcceptZeroGen is true for EnrichmentChecked: Gen=0 is a sentinel.
func TestCentralGuard_ZeroGenEnrichmentChecked(t *testing.T) {
	c := newCore()
	c.session.EnrichmentGen = domain.Gen(5)
	c.session.EnrichTotal = 1
	c.session.EnrichSweepMembers = map[string]bool{"s3": true}

	sentinelEvent := messages.EnrichmentChecked{
		ResourceType: "s3",
		Gen:          0, // zero = sentinel, must pass the guard
	}

	_, _ = c.HandleEvent(sentinelEvent)

	if c.session.EnrichChecked != 1 {
		t.Fatalf("zero-gen EnrichmentChecked sentinel must pass the guard, EnrichChecked=%d", c.session.EnrichChecked)
	}
}
