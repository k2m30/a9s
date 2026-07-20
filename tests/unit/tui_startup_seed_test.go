// tui_startup_seed_test.go — RED pin for the startup disk seed, D10 (C1 + goal 4 of
// docs/design/cache-requirements.md; observed live: the TUI main menu renders
// empty at startup until the AWS connect completes).
//
// Root cause: tui.Model.Init (internal/tui/app.go) fired ONLY the connect cmd
// on the live (non-pre-supplied-clients) path; the disk seed
// (TaskKindLoadAvailCache / m.loadAvailabilityCache()) was dispatched from
// handleClientsReadySuccess AFTER ClientsReady — so a cold, disk-cache-warm
// start still showed a blank menu until the live AWS handshake finished, even
// though C1 requires "anything cached renders instantly" with no AWS needed.
//
// Secondary hole (both TUI and web lanes): when session.Region is "" (no -r
// flag), EnsureCacheStore refuses to load (profile/region pair unresolved),
// even though the profile's default region is resolvable synchronously from
// the local AWS config file via awsclient.GetDefaultRegion(
// awsclient.DefaultConfigPath(), profile) — the exact call
// handleClientsReadySuccess already uses post-connect.
//
// Test 1 (TUIInit_SeedsMenuFromDisk_BeforeClientsReady) pins the TUI-level
// contract directly against tui.Model.Init/Update/View: a disk-cache-warm
// start must render cached counts on the FIRST frame, driven purely by
// Init()'s returned cmd tree, without ever delivering a ClientsReady message.
//
// Test 2 (TUIInit_EmptyRegion_ResolvesConfigDefaultForSeed) pins the same
// contract when session.Region == "" (no -r flag): the seed must resolve the
// profile's config-file default region and load THAT pair's disk cache.
//
// Test 3 (TestCoreLoadAvailabilityCache_EmptyRegion_ResolvesConfigDefault)
// pins the shared runtime.Core.LoadAvailabilityCache seam
// (core/runtime/probes.go) that both the TUI Init seed and
// core/web/construct.go's newSession rely on: an empty-region session
// must still resolve the config-file default and seed from that pair's disk
// cache. core/web/construct.go's newSession is unexported and
// unreachable from tests/unit, so this test pins the shared Core-level seam
// it delegates to (LoadAvailabilityCache), which is the same seam
// TestWebBoot_AvailabilityCacheLoaded_AppliesCountsAndIssuesToMenu in
// app_web_live_cold_boot_test.go exercises for the resolved-region case.
package unit

import (
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/cache"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// seedTypeFile writes a minimal, realistic per-type disk cache file for
// shortName under profile/region, mirroring the app_web_live_cold_boot_test.go
// / tui_savecache_routing_test.go Put+SaveType precedent. HasResources must
// be true (or Count/IssuesKnown/Rows non-zero) or
// runtime.cacheStoreToEvent's zero-value TypeFile guard drops the entry.
func seedTypeFile(t *testing.T, profile, region, shortName string, count int) {
	t.Helper()
	store := cache.LoadDirForTest(profile, region)
	if store == nil {
		t.Fatalf("cache.LoadDirForTest(%q, %q) returned nil", profile, region)
	}
	store.Put(shortName, cache.TypeFile{
		HasResources: true,
		Count:        count,
		Exact:        true,
	})
	if err := store.SaveType(shortName); err != nil {
		t.Fatalf("SaveType(%q) fixture write: %v", shortName, err)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Test 1 — cold start with a resolved profile+region must seed the menu from
// disk on the very first Init()-driven frame, without any ClientsReady.
// ────────────────────────────────────────────────────────────────────────────

// TestTUIInit_SeedsMenuFromDisk_BeforeClientsReady pins the startup disk seed: a
// disk-cache-warm TUI start must render cached availability counts on the
// menu before the live AWS connect completes. The model is constructed with
// an explicit profile/region (so region resolution is not in play) and NO
// pre-supplied clients, so Init() takes the live-connect branch
// (messages.InitConnect), not the demo/test preCmd branch.
//
// The connect leg (messages.InitConnect) is deliberately never fed back into
// Update — delivering it would call m.connectAWS and attempt a real AWS
// handshake. Instead, Init()'s returned cmd tree is walked for every
// NON-InitConnect message (the seed leg) and those are the only ones applied.
// This isolates "what does the disk-seed leg alone, reachable before
// ClientsReady, do to the rendered menu".
//
// RED today: Init() returns ONLY connectCmd on the live-connect path — no
// seed leg exists — so after dropping the one InitConnect message there is
// nothing left to apply, and the rendered menu shows no cached count.
func TestTUIInit_SeedsMenuFromDisk_BeforeClientsReady(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)

	const profile, region = "def13-cold-prof", "us-east-1"
	seedTypeFile(t, profile, region, "s3", 7)

	m := tui.New(profile, region,
		tui.WithProfileForTest(profile),
		tui.WithRegionForTest(region))
	t.Cleanup(func() { m.CloseController() })
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 40})

	initCmd := m.Init()
	if initCmd == nil {
		t.Fatal("Init() returned nil cmd — expected at least the live connect leg")
	}

	m = applyNonConnectLeg(t, m, initCmd)

	content := stripANSI(rootViewContent(m))
	if !strings.Contains(content, "(7)") {
		t.Errorf("rendered menu after Init() (no ClientsReady delivered) does not contain the disk-seeded count %q — D10: the disk seed must reach the menu before AWS connect completes:\n%s", "(7)", content)
	}
}

// applyNonConnectLeg walks cmd (executing it, and recursively any
// tea.BatchMsg sub-commands ONE level deep) and applies every resulting
// top-level message to m via Update, EXCEPT messages.InitConnect, which is
// intentionally dropped so the live AWS handshake is never triggered.
//
// Deliberately does NOT chase the tea.Cmd a delivered message's Update call
// returns beyond this first level: Init()'s cmd tree is exactly
// tea.Batch(connectCmd, seedCmd) (plus an optional flash cmd) — applying the
// seed leg's resulting messages.AvailabilityCacheLoaded to Update is enough
// to observe the startup disk seed's "does the disk seed reach the menu on the very first
// frame" contract. Recursively draining every FOLLOW-ON cmd (as
// tui_savecache_routing_test.go's runCmdTree does for a full sweep-completion
// scenario) would additionally execute the real background availability
// sweep this seed kicks off, including its scheduled tea.Tick-based
// auto-clear-flash commands — which block on a real timer channel and are
// unrelated to what this test pins.
func applyNonConnectLeg(t *testing.T, m tui.Model, cmd tea.Cmd) tui.Model {
	t.Helper()
	if cmd == nil {
		return m
	}
	msg := cmd()
	switch v := msg.(type) {
	case nil:
		return m
	case tea.BatchMsg:
		for _, sub := range v {
			if sub == nil {
				continue
			}
			subMsg := sub()
			if _, isConnect := subMsg.(messages.InitConnect); isConnect {
				// Deliberately dropped — feeding this back would call
				// m.connectAWS and attempt a real (unreachable, hermetic-test)
				// AWS connection.
				continue
			}
			m, _ = rootApplyMsg(m, subMsg)
		}
		return m
	case messages.InitConnect:
		return m
	default:
		m, _ = rootApplyMsg(m, msg)
		return m
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Test 2 — an unresolved region (no -r flag) must still seed from the
// profile's config-file default region.
// ────────────────────────────────────────────────────────────────────────────

// TestTUIInit_EmptyRegion_ResolvesConfigDefaultForSeed pins the secondary
// startup-disk-seed hole: session.Region == "" (as it is with no -r flag until the AWS
// connect settles it) must not block the disk seed entirely. The profile's
// default region is resolvable synchronously from a local AWS config file
// via awsclient.GetDefaultRegion(awsclient.DefaultConfigPath(), profile) —
// the exact call handleClientsReadySuccess (core/runtime/handlers.go)
// already makes post-connect — so the seed should resolve and use that same
// region.
//
// A real AWS config file is written to a temp dir and AWS_CONFIG_FILE is
// redirected to it (awsclient.DefaultConfigPath honors this env var; see
// awsclient.GetDefaultRegion's own tests in aws_profile_test.go for the same
// fixture pattern). The disk cache pair dir is seeded under
// "<profile>--<configDefaultRegion>" — the pair the seed must resolve to.
//
// RED today: tui.New(profile, "") leaves session.Region == "", and
// loadAvailabilityCache -> Core.LoadAvailabilityCache -> EnsureCacheStore
// returns nil outright for an empty region (session.EnsureCacheStore's
// `profile == "" || region == ""` guard), with no config-file fallback
// anywhere in that path — so even reaching the (still-unwired, per Test 1)
// seed leg would find nothing to seed from.
func TestTUIInit_EmptyRegion_ResolvesConfigDefaultForSeed(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)

	const profile, configDefaultRegion = "def13-emptyregion-prof", "eu-west-1"
	cfgDir := t.TempDir()
	cfgPath := cfgDir + "/config"
	awsConfig := "[profile " + profile + "]\nregion = " + configDefaultRegion + "\n"
	writeFileOrFatal(t, cfgPath, awsConfig)
	t.Setenv("AWS_CONFIG_FILE", cfgPath)

	// Sanity: confirm the fixture actually resolves the way the production
	// handleClientsReadySuccess call would.
	if got := awsclient.GetDefaultRegion(awsclient.DefaultConfigPath(), profile); got != configDefaultRegion {
		t.Fatalf("test setup: awsclient.GetDefaultRegion(...) = %q, want %q", got, configDefaultRegion)
	}

	seedTypeFile(t, profile, configDefaultRegion, "ec2", 3)

	m := tui.New(profile, "", tui.WithProfileForTest(profile), tui.WithRegionForTest(""))
	t.Cleanup(func() { m.CloseController() })
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 40})

	initCmd := m.Init()
	if initCmd == nil {
		t.Fatal("Init() returned nil cmd — expected at least the live connect leg")
	}
	m = applyNonConnectLeg(t, m, initCmd)

	content := stripANSI(rootViewContent(m))
	if !strings.Contains(content, "(3)") {
		t.Errorf("rendered menu after Init() with an unresolved session.Region does not contain the config-default-region-seeded count %q — D10: an empty region must still resolve the profile's config-file default for the disk seed:\n%s", "(3)", content)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Test 3 — the shared runtime.Core.LoadAvailabilityCache seam (used by both
// the TUI Init seed and core/web/construct.go's newSession) must resolve
// an empty region from the config-file default.
// ────────────────────────────────────────────────────────────────────────────

// TestCoreLoadAvailabilityCache_EmptyRegion_ResolvesConfigDefault pins the
// shared runtime-level seam directly: core/web/construct.go's newSession
// is unexported and unreachable from tests/unit (confirmed: only
// core/web itself can construct it), so this test pins the exact Core
// method newSession's live (no-pre-supplied-clients) branch delegates to —
// runtime.Core.LoadAvailabilityCache (core/runtime/probes.go) — which is
// also the same method tui.Model's probe_adapter.go loadAvailabilityCache
// wraps for Test 1/2 above. A green result here is a green result for both
// callers' empty-region behavior; it does not exercise
// core/web/construct.go's newSession wiring itself (that plumbing is a
// two-line direct call with no branching left to pin once this seam is
// fixed).
//
// RED today: Core.LoadAvailabilityCache calls EnsureCacheStore, which is a
// zero-arg method reading session.Profile/session.Region directly — an empty
// session.Region returns nil unconditionally, with no config-file fallback.
func TestCoreLoadAvailabilityCache_EmptyRegion_ResolvesConfigDefault(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)

	const profile, configDefaultRegion = "def13-web-emptyregion-prof", "ap-southeast-2"
	cfgDir := t.TempDir()
	cfgPath := cfgDir + "/config"
	awsConfig := "[profile " + profile + "]\nregion = " + configDefaultRegion + "\n"
	writeFileOrFatal(t, cfgPath, awsConfig)
	t.Setenv("AWS_CONFIG_FILE", cfgPath)

	seedTypeFile(t, profile, configDefaultRegion, "rds", 4)

	s := session.New()
	s.Profile = profile
	s.Region = ""
	core := runtime.New(s, catalog.All())

	store := core.LoadAvailabilityCache()
	if store == nil {
		t.Fatal("Core.LoadAvailabilityCache() returned nil for an empty session.Region — D10: an unresolved region must still resolve the profile's config-file default and load that pair's disk cache")
	}
	tf, ok := store.Type("rds")
	if !ok {
		t.Fatalf(`store.Type("rds") missing — LoadAvailabilityCache did not resolve to the config-default-region pair %q--%q`, profile, configDefaultRegion)
	}
	if tf.Count != 4 {
		t.Errorf("resolved rds TypeFile.Count = %d, want 4", tf.Count)
	}

	// Confirm the event conversion the TUI/web seed callers both use also
	// carries the count through, exactly as runtime.CacheStoreToEvent would
	// feed messages.AvailabilityCacheLoaded.
	ev := runtime.CacheStoreToEvent(store)
	if got, ok := ev.Entries["rds"]; !ok || got != 4 {
		t.Errorf("CacheStoreToEvent(store).Entries[%q] = %d (ok=%v), want 4, true", "rds", got, ok)
	}
}

// writeFileOrFatal writes a minimal AWS config file fixture, matching the
// writeAWSConfig precedent in app_handlers_theme_profile_test.go (kept
// file-local per this package's existing convention of not sharing test
// helpers across files).
func writeFileOrFatal(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writing fixture file %s: %v", path, err)
	}
}
