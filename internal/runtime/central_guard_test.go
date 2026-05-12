package runtime

// central_guard_test.go — locks the central GenStamped guard in HandleEvent
// (AS-74). The guard must discard stale events before the per-type switch so
// no handler can accidentally skip the staleness check.

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
)

// TestCentralGuard_StaleAvailabilityChecked verifies that a stale
// AvailabilityChecked event (Gen mismatches session.AvailabilityGen) is
// discarded at the central guard and does not advance AvailChecked.
func TestCentralGuard_StaleAvailabilityChecked(t *testing.T) {
	c := newCore()
	// session.AvailabilityGen starts at 0; bump it so 0 is stale.
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

// TestCentralGuard_FreshAvailabilityChecked verifies that a fresh
// AvailabilityChecked (Gen matches) is NOT discarded by the guard.
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

// TestCentralGuard_StaleEnrichmentChecked verifies that a stale
// EnrichmentChecked event is discarded and does not advance EnrichChecked.
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

// TestCentralGuard_ZeroGenAvailabilityChecked verifies that AvailabilityChecked
// with Gen=0 IS treated as stale (AcceptZeroGen returns false for this type)
// when the session counter is non-zero.
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

// TestCentralGuard_ZeroGenEnrichmentChecked verifies that EnrichmentChecked
// with Gen=0 is accepted as a safe sentinel (AcceptZeroGen returns true).
func TestCentralGuard_ZeroGenEnrichmentChecked(t *testing.T) {
	c := newCore()
	c.session.EnrichmentGen = domain.Gen(5)
	c.session.EnrichTotal = 1

	sentinelEvent := messages.EnrichmentChecked{
		ResourceType: "s3",
		Gen:          0, // zero = sentinel, must pass the guard
	}

	_, _ = c.HandleEvent(sentinelEvent)

	if c.session.EnrichChecked != 1 {
		t.Fatalf("zero-gen EnrichmentChecked sentinel must pass the guard, EnrichChecked=%d", c.session.EnrichChecked)
	}
}
