// qa_controller_construction_discipline_test.go — task #54: closes the
// residual TempDir-leak class STRUCTURALLY rather than one call site at a
// time. app_availsave_tempdir_cleanup_race_test.go's doc comment traces the
// mechanism precisely: any test that builds a real *app.Controller (directly
// via app.New, or transitively via tui.New — internal/tui/app.go's New wires
// an app.Controller into the returned Model) and then drives a
// ResourcesLoaded/EnrichmentChecked/AvailabilityChecked event through it can
// queue an async availability-cache save (queueAvailabilitySave starts
// runAvailabilitySaveLoop). If that goroutine outlives the test's
// t.TempDir() cleanup, the writer can still be calling cache.Store.SaveType
// under a directory os.RemoveAll is concurrently tearing down — the flaky
// "TempDir RemoveAll cleanup: ... directory not empty" failure task #41
// reported, and, because cache.cacheRoot() reads A9S_CONFIG_FOLDER LIVE at
// write time, the leaked write can land inside whatever OTHER test's temp
// dir happens to be current by the time the OS scheduler runs it.
//
// The migration fixed this at five call sites by funnelling every hermetic
// controller/model construction through a helper that pairs t.TempDir() +
// t.Cleanup(c.Close) in the correct LIFO order (see each helper's own doc
// comment for the citation):
//
//   - newTestController                 (app_controller_test.go)
//   - newTestControllerWithCore         (app_controller_pr_b_test.go)
//   - newTestControllerAndCore          (app_patch_cache_intents_test.go)
//   - newRootSizedModel                 (tui_root_test.go)
//   - newDetailParityHeadlessController (tui_detail_parity_test.go)
//
// tui.New(profile, region, tui.WithNoCache(true)) is a SEPARATE safe escape
// hatch: WithNoCache(disabled=true) makes internal/tui/app.go's New skip the
// availability-cache wiring entirely, so no writer goroutine is ever queued
// regardless of Close discipline. tui.New(..., tui.WithNoCache(false)) does
// NOT get this exemption — false leaves caching (and the leak class) live.
//
// Any OTHER direct tui.New(...) / app.New(...) construction in this
// directory is exactly the bug class task #41/#54 close: nothing stops a new
// test from reintroducing the leak by hand-rolling its own construction
// instead of calling a blessed helper. This gate is a SOURCE-SCAN ratchet
// (the same construction as count_minus_one_guard_test.go's AST guard over
// core/aws/*_related*.go, and the same allowlist semantics as
// knownStatusColumnDebt / knownVisibilityGaps / knownStateCoverageGaps /
// knownDisconnectedPivots in the sibling qa_*_test.go gates):
//
//   - A found call site NOT in knownConstructionDebt is a NEW regression —
//     always fails, unconditionally, pointing at the blessed helpers above.
//   - An allowlisted call site the live scan no longer finds (the file was
//     migrated to a blessed helper, or deleted) fails with a "prune from
//     allowlist" message — the burn-down signal for the conversion coder.
//   - An allowlisted call site the live scan still finds is skipped
//     (logged), pre-existing debt this gate exists to track down.
//
// SEEDED CENSUS (2026-07-07, first run of this gate): every entry below was
// discovered by this exact scanner against tests/unit/*.go as it stood at
// seeding time — direct tui.New(...)/app.New(...) call sites outside the
// five blessed helpers and outside a literal WithNoCache(true) argument in
// the same call expression. This is deliberately a LARGE allowlist (the
// harness fix task #41 landed converted five call sites; it did not attempt
// to convert the other ~150+ pre-existing direct-construction sites this
// scan finds) — it is the burn-down worklist for future cleanup, not a
// claim that today's suite is race-free at every site. New test files must
// route through a blessed helper (or tui.WithNoCache(true)) instead of
// adding to this list.
package unit_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// ccdBlessedHelpers enumerates the top-level function names inside which a
// direct tui.New(...)/app.New(...) call is exempt from this gate because
// that helper itself pairs t.TempDir() + t.Cleanup(c.Close)/CloseController
// in the correct order — see the file-level doc comment for citations.
var ccdBlessedHelpers = map[string]bool{
	"newTestController":                 true,
	"newTestControllerForProfile":       true, // app_controller_test.go — newTestController for a cache round-trip test, which needs a profile/region pair of its own so its type file is not a sibling's
	"newTestControllerWithCore":         true,
	"newTestControllerAndCore":          true,
	"newRootSizedModel":                 true,
	"newDetailParityHeadlessController": true,
	"newCostsScreenController":          true, // costs_round3_test.go — closure-wave harness dedup collapsed round3CostsController/round6NewCostsController/round8NewCostsController/reviewCostsController into this one
	"leakPinCostsController":            true, // generation_stamping_fetch_test.go — the ConnectGen leak pin's Costs subtest; mirrors costs_state_test.go's already-allowlisted newCostsController but also returns *session.Session so the caller can Rotate() it directly
	"newCachegenController":             true, // cachegen_cache_contract_test.go — pairs t.Cleanup(Close) with the caller's own A9S_CONFIG_FOLDER=t.TempDir() (registered first, so removed last) and fails the test when that is missing; needs a per-test profile/region pair and the full type registry, which no existing blessed helper provides together
}

// knownConstructionDebt pins the exact inventory of call sites (keyed
// "<file>:<enclosing-func-or-package-level>#<occurrence>") this gate found
// at seeding time. See the file-level doc comment for full ratchet
// semantics.
var knownConstructionDebt = map[string]bool{
	"app_availsave_tempdir_cleanup_race_test.go:TestAvailSaveTempDirRace_HelperOrdering_MirrorsTTempDirLIFO#1":           true,
	"app_availsave_tempdir_cleanup_race_test.go:buildAvailSaveRaceController#1":                                          true,
	"app_cache_first_disk_rows_test.go:TestListOpen_ColdStart_SeedsFromDiskCache_WithRefreshing#1":                       true,
	"app_cache_first_seeding_test.go:newSeededTestController#1":                                                          true,
	"app_cancellation_test.go:TestModel_Cancel_CancelsAppContext#1":                                                      true,
	"app_cancellation_test.go:TestModel_HasAppContext#1":                                                                 true,
	"app_cancellation_test.go:TestModel_QuitCancelsAppContext#1":                                                         true,
	"app_count_truncation_drift_test.go:newCountTruncationDriftController#1":                                             true,
	"app_detail_attention_cursor_test.go:newAttentionCursorController#1":                                                 true,
	"app_drainsync_partition_test.go:TestDrainSyncPartition_BlockingFollowUp_IsBackground_ReturnedNotExecuted#1":         true,
	"app_enrichment_menu_badge_test.go:newEnrichmentMenuBadgeController#1":                                               true,
	"app_fetchers_branch_coverage_test.go:TestLoadAvailabilityCache_IssueFieldsMapped#1":                                 true,
	"app_fetchers_branch_coverage_test.go:TestLoadAvailabilityCache_PopulatedCacheReturnsEntries#1":                      true,
	"app_handlers_theme_profile_test.go:TestHandleProfilesLoaded_PushesProfileSelectorView#1":                            true,
	"app_live_shaped_badge_repro_test.go:newLiveShapedBadgeReproController#1":                                            true,
	"app_menu_test.go:newMenuController#1":                                                                               true,
	"app_pilot_defects_test.go:TestAvailabilitySweepAndEnrichment_PersistsRowsPerType_WithoutAnyListOpen#1":              true,
	"app_pilot_defects_test.go:TestEnrichmentChecked_OpenList_FindingsReachPersistedCacheAndColdBootGlyph#1":             true,
	"app_pilot_defects_test.go:TestEnrichmentChecked_OpenList_FindingsReachPersistedCacheAndColdBootGlyph#2":             true,
	"app_pilot_defects_test.go:TestProductionRefresh_OneType_LeavesSiblingTypeFilesByteIdentical#1":                      true,
	"app_pilot_defects_test.go:TestProductionRefresh_TruncatedRefetch_NeverShrinksPersistedRows_HeaderStaysConsistent#1": true,
	"app_pilot_defects_test.go:TestSaveResourceListCache_FindingsSurviveWiredSaveAndColdBootReseed#1":                    true,
	"app_pilot_defects_test.go:TestSaveResourceListCache_FindingsSurviveWiredSaveAndColdBootReseed#2":                    true,
	"app_preconnect_replay_test.go:newHermeticLiveController#1":                                                          true,
	"app_related_cursor_skip_test.go:TestRelatedCursor_MenuParity_SameDimNonDimPatternSameLandingSequence#1":             true,
	"app_related_cursor_skip_test.go:newRelatedSkipController#1":                                                         true,
	"app_related_focus_entry_test.go:newRelatedFocusEntryController#1":                                                   true,
	"app_web_lane_menu_badge_test.go:newWebLaneMenuBadgeController#1":                                                    true,
	"app_web_live_cold_boot_test.go:TestAncientTypeFile_SeedsNormally_NoAgeDiscard#1":                                    true,
	"app_web_live_cold_boot_test.go:TestChildAndFilteredLists_NeverWrittenToTypeFile#1":                                  true,
	"app_web_live_cold_boot_test.go:TestColdBoot_SeedsAllLoadedPages_PerTypeFile_InstantlySeedsBeforeFetchCompletes#1":   true,
	"app_web_live_cold_boot_test.go:TestLoadBeforeSave_PairSwitch_NeverSavesBeforeLoad#1":                                true,
	"app_web_live_cold_boot_test.go:TestNoCache_NeverLoadsPopulatedDir_NeverWritesFiles#1":                               true,
	"app_web_live_cold_boot_test.go:newLiveWebStyleController#1":                                                         true,
	"attention_details_survival_test.go:newAttnController#1":                                                             true,
	"connect_aws_region_test.go:TestBug82_ConnectAWS_NoMissingRegionError#1":                                             true,
	"connect_aws_region_test.go:TestBug82_ProfileSwitch_NoMissingRegionError#1":                                          true,
	"connect_aws_region_test.go:TestBug82_RegionSwitch_PassesExplicitRegion#1":                                           true,
	// costs_codex_test.go's X1/X10 tests need demo.NewServiceClients() (a
	// real demo-backed CostExplorer+EC2 pair) wired onto the session BEFORE
	// construction — the blessed helpers attach no clients at all, and
	// newCostsController hardcodes profile "test-profile" with nil clients.
	"costs_codex_test.go:TestCostsCodex_X1_GrowthStory_ResourceChain_EndToEnd_OverDemoTransport#1": true,
	"costs_codex_test.go:TestCostsCodex_X10_ResourceJump_NotFound_ReturnsToCostsWithHonestNote#1":  true,
	// costs_codex_test.go's X2 test needs THREE separate Controller
	// generations reloading the SAME on-disk profile store at three
	// different injected "now" values (t0/t1/t2), to prove the anomaly TTL
	// stays anchored to the last REAL fetch rather than any intervening
	// SkipAnomalies delivery — no blessed helper supports re-opening a
	// fixed profile under a caller-chosen "now" more than once.
	"costs_codex_test.go:TestCostsCodex_X2_SkipAnomalies_PreservesMarksAndTTL#1": true,
	"costs_codex_test.go:TestCostsCodex_X2_SkipAnomalies_PreservesMarksAndTTL#2": true,
	"costs_codex_test.go:TestCostsCodex_X2_SkipAnomalies_PreservesMarksAndTTL#3": true,
	// costs_codex_test.go's X3/X9 tests must pre-seed an on-disk costs
	// Store (X3: fully-covered cost cache; X9: a corrupt cache file) BEFORE
	// constructing the Controller, under the SAME A9S_CONFIG_FOLDER the
	// Controller's own EnsureCostsState will later read from — same
	// justified-exception shape as
	// costs_selfreview_test.go's C9 entry below (caller must control
	// env-var timing itself).
	"costs_codex_test.go:TestCostsCodex_X3_WarmCostCache_AbsentAnomalies_StillEmitsFetch#1": true,
	"costs_codex_test.go:TestCostsCodex_X9_RecoveredStore_SurfacesFlashOnInit#1":            true,
	"costs_nav_test.go:TestQA_Costs_MainMenu_Enter_EmitsNavigateTargetCosts#1":              true,
	"costs_nav_test.go:newCostsNavController#1":                                             true,
	// costs_selfreview_test.go's C9 test must pre-seed an on-disk costs
	// Store BEFORE constructing the Controller, under the SAME
	// A9S_CONFIG_FOLDER the Controller's own EnsureCostsState will later
	// read from — every blessed helper (newTestController et al.) calls
	// its own t.Setenv(A9S_CONFIG_FOLDER, t.TempDir()) internally, which
	// would silently overwrite the pre-seeded directory with a fresh empty
	// one. Same justified-exception shape as reviewCostsControllerNoIsolation
	// below (caller must control env-var timing itself) — t.Cleanup(c.Close)
	// is still paired correctly, just not through a shared helper.
	"costs_selfreview_test.go:TestCostsSelfReview_C9_DataThrough_DerivesFromWarmStore_NoFetchNeeded#1": true,
	"costs_review2_test.go:TestCostsReview2_R3_MainMenuNavigateToCosts_WarmCache_ZeroFetches#1":        true,
	"costs_review2_test.go:TestCostsReview2_R6_DemoTransport_AnomalyFlowsToCellMarkAndFooter#1":        true,
	"costs_review_findings_test.go:TestCostsReview_F7_StaleCostsLoaded_DroppedAfterProfileSwitch#1":    true,
	"costs_review_findings_test.go:reviewCostsControllerNoIsolation#1":                                 true,
	// costs_review3_test.go's R4 test must wire session.NoCache=true onto
	// session.New() BEFORE constructing the Controller (no blessed helper
	// builds a NoCache-configured core at all), and pre-seed an on-disk
	// costs Store under the SAME A9S_CONFIG_FOLDER the NoCache-configured
	// Controller must then prove it never reads — same justified-exception
	// shape as costs_selfreview_test.go's C9 entry above (caller must
	// control both NoCache wiring and env-var timing itself).
	"costs_review3_test.go:TestCostsReview3_R4_NoCache_MemoryOnlyStore_NoDiskReadOrWrite#1": true,
	// costs_review3_test.go's P1 (coverage-clear) test must pre-seed an
	// on-disk costs Store at a fetch time STRICTLY EARLIER than the
	// Controller's own injected "now" (defeating MergeCoverage's own
	// !FetchedAt.Equal(now) guard, which would otherwise accidentally mask
	// the bug) — same justified-exception shape as costs_selfreview_test.go's
	// C9 entry above.
	"costs_review3_test.go:TestCostsReview3_P1_AnomalyOnlyDelivery_NeverClearsWarmGridCoverage#1": true,
	// costs_review3_test.go's P5 test needs a Controller with genuinely NIL
	// clients (pre-connect) and later delivers a real messages.ClientsReady
	// — no blessed helper constructs a nil-clients core at all.
	"costs_review3_test.go:TestCostsReview3_P5_ClientsReady_RecoversPreConnectCostsError#1": true,
	"costs_state_test.go:newCostsController#1":                                              true,
	// M1's zero-clears-warm-bucket cell needs two genuinely different "now"
	// stamps across two deliveries (MergeCoverage's own
	// !existing.FetchedAt.Equal(now) guard) — seeds an on-disk Store at an
	// earlier now, then constructs a fresh Controller at the later
	// fixedCostsNow, same shape as costs_review3_test.go's own R2/P1 setup.
	"costs_delivery_matrix_test.go:TestCostsDeliveryMatrix_GridEmpty_AnomaliesSkipped_WarmBucket_StampsCoverage#1": true,
	// M2's lane-parity harness needs a headless *app.Controller built the
	// SAME way as the TUI lane's own tui.New (both wrap a fresh
	// runtime.Core/session.New pair) so the two lanes start from identical
	// state — no blessed helper matches this exact construction shape.
	"costs_lane_parity_test.go:m2NewHeadlessController#1":                                                      true,
	"demo_app_test.go:TestNonDemoMode_Unchanged#1":                                                             true,
	"issue119_scenarios_golden_test.go:issue119RootModel#1":                                                    true,
	"issue233_empty_truncated_cache_test.go:setupLiveModeEC2Detail#1":                                          true,
	"issue235_related_check_race_test.go:TestIssue235_EachCheckerGetsIsolatedCacheSnapshot#1":                  true,
	"issue237_239_240_241_related_fixes_test.go:TestIssue237_ColdMissWriteBack_PreservesNextToken#1":           true,
	"issue237_239_240_241_related_fixes_test.go:TestIssue240_CacheDependentChecker_DoesPrefetch#1":             true,
	"issue237_239_240_241_related_fixes_test.go:TestIssue240_FieldOnlyChecker_NoPrefetch#1":                    true,
	"issue237_239_240_241_related_fixes_test.go:TestIssue241_ConcurrentProbesCappedAt4#1":                      true,
	"lazy_add_cache_merge_test.go:setupLiveModeEFSDetail#1":                                                    true,
	"lazy_add_flash_error_test.go:TestLazyAddError_EmitsFlashMsg#1":                                            true,
	"lazy_add_flash_error_test.go:TestLazyAddError_PartialSuccess_StillEmitsFlashMsg#1":                        true,
	"lazy_add_orchestration_edges_test.go:TestLazyAdd_FetchByIDsErrorSwallowed_ChecksResultStillDelivered#1":   true,
	"lazy_add_orchestration_edges_test.go:TestLazyAdd_MissingFromCache_DedupsRepeatedIDsInChecker#1":           true,
	"lazy_add_stories_failures_counts_test.go:Test_LA_020_PartialResolution_ChecksStillDelivered#1":            true,
	"lazy_add_stories_failures_counts_test.go:Test_LA_024_GetPolicyDenied_PartialMetadataOK#1":                 true,
	"lazy_add_stories_failures_counts_test.go:Test_LA_060_PivotCountEqualsRowCount#1":                          true,
	"lazy_add_stories_failures_counts_test.go:Test_LA_061_FooterSuppressed_WhenAllRelatedIDsResolved#1":        true,
	"lazy_add_stories_failures_counts_test.go:Test_LA_062_FooterSuppressed_UpstreamTruncatedDrillResolved#1":   true,
	"lazy_add_stories_happy_test.go:Test_LA_001_KMSDrillAWSManagedKey#1":                                       true,
	"lazy_add_stories_happy_test.go:Test_LA_002_AMIDrillPublicAMI#1":                                           true,
	"lazy_add_stories_happy_test.go:Test_LA_003_EBSSnapDrillSharedSnapshot#1":                                  true,
	"lazy_add_stories_happy_test.go:Test_LA_004_IAMPolicyDrillAWSManaged#1":                                    true,
	"lazy_add_stories_happy_test.go:Test_LA_081_ColdCacheDrillTriggersPrefetch#1":                              true,
	"lazy_add_stories_happy_test.go:Test_LA_082_WarmCacheDrillReusesCache#1":                                   true,
	"lazy_add_stories_lifecycle_race_test.go:Test_LA_030_ProfileSwitch_ClearsLazyAddedTargets#1":               true,
	"lazy_add_stories_lifecycle_race_test.go:Test_LA_030_ProfileSwitch_ClearsLazyAddedTargets#2":               true,
	"lazy_add_stories_lifecycle_race_test.go:Test_LA_031_RegionSwitch_ClearsLazyAddedTargets#1":                true,
	"lazy_add_stories_lifecycle_race_test.go:Test_LA_031_RegionSwitch_ClearsLazyAddedTargets#2":                true,
	"lazy_add_stories_lifecycle_race_test.go:Test_LA_033_SourceDetailRefresh_RerunsChecker#1":                  true,
	"lazy_add_stories_lifecycle_race_test.go:Test_LA_034_MainMenuRoundtrip_LazyAddEntryMarkedTruncated#1":      true,
	"lazy_add_stories_lifecycle_race_test.go:Test_LA_040_RepeatDrill_Idempotent#1":                             true,
	"lazy_add_stories_lifecycle_race_test.go:Test_LA_041_RepeatDrill_DifferentSource_SameTarget_SingleEntry#1": true,
	"lazy_add_stories_lifecycle_race_test.go:Test_LA_042_EscUnrelatedNav_ReDrill_Stable#1":                     true,
	"lazy_add_stories_lifecycle_race_test.go:Test_LA_043_SourceDetailReEntry_UsesCachedResult#1":               true,
	"lazy_add_stories_lifecycle_race_test.go:Test_LA_044_NoRelatedPivots_ReturnsNilCmd#1":                      true,
	"lazy_add_stories_lifecycle_race_test.go:Test_LA_050_DrillDuringEnrichment_ResultLandsWithoutDrop#1":       true,
	"lazy_add_stories_lifecycle_race_test.go:Test_LA_051_EscDuringResolution_StaleResultDropped#1":             true,
	"lazy_add_stories_lifecycle_race_test.go:Test_LA_052_RapidConsecutiveDispatches_CheckerRunsEachTime#1":     true,
	"lazy_add_stories_lifecycle_race_test.go:Test_LA_053_ProfileSwitchMidResolution_StaleResultDiscarded#1":    true,
	"lazy_add_stories_lifecycle_race_test.go:Test_LA_054_RegionSwitchMidResolution_StaleResultDiscarded#1":     true,
	"lazy_add_stories_scope_extremes_test.go:Test_LA_010_MixedInScopeAndOutOfScope#1":                          true,
	"lazy_add_stories_scope_extremes_test.go:Test_LA_011_AllOutOfScopePopulatesDrill#1":                        true,
	"lazy_add_stories_scope_extremes_test.go:Test_LA_012_AllInScopeNoLazyAdd#1":                                true,
	"lazy_add_stories_scope_extremes_test.go:Test_LA_015_ARNvsBareNameTolerance#1":                             true,
	"lazy_add_stories_scope_extremes_test.go:Test_LA_016_UUIDvsAliasDisplay#1":                                 true,
	"lazy_add_stories_scope_extremes_test.go:Test_LA_070_100IDsDrillWithoutTimeout#1":                          true,
	"lazy_add_stories_scope_extremes_test.go:Test_LA_071_MalformedIDsFiltered#1":                               true,
	"lazy_add_stories_scope_extremes_test.go:Test_LA_072_IDSetGrowsAcrossRedrill#1":                            true,
	// list_body_memo_seq_test.go's BenchmarkListSnapshot_LargeList needs a
	// Controller built from a *testing.B, not a *testing.T — every blessed
	// helper takes *testing.T specifically (t.Setenv/t.TempDir/t.Cleanup),
	// and this is the repo's first Benchmark over a Controller.
	"list_body_memo_seq_test.go:newBenchListController#1":                                                         true,
	"phase03_fold_test.go:TestFold_MainMenuCtrlR_ClearsAllCachedWave2#1":                                          true,
	"qa67_terminal_size_test.go:TestQa67_H11_ExtremelyWide_NoCrash#1":                                             true,
	"qa67_terminal_size_test.go:TestQa67_H11_H12_ExtremeSizes_WithResourceList_NoCrash#1":                         true,
	"qa67_terminal_size_test.go:TestQa67_H12_ExtremelyTall_NoCrash#1":                                             true,
	"qa67_terminal_size_test.go:TestQa67_H1_MinimumWidth60_RendersNormally#1":                                     true,
	"qa67_terminal_size_test.go:TestQa67_H2_MinimumHeight7_RendersNormally#1":                                     true,
	"qa67_terminal_size_test.go:TestQa67_H3_Width59_ShowsTooNarrow#1":                                             true,
	"qa67_terminal_size_test.go:TestQa67_H4_Height6_ShowsTooShort#1":                                              true,
	"qa67_terminal_size_test.go:TestQa67_H5_ResizeFromBelowToAboveMinimum_RestoresUI#1":                           true,
	"qa67_terminal_size_test.go:TestQa67_H6_ResizeFromAboveToBelowMinimum_ShowsError#1":                           true,
	"qa_alltypes_cache_sweep_test.go:alltypesSweepPair#1":                                                         true,
	"qa_architect_bugs_test.go:TestBug_Detail_UsesCorrectViewDefForResourceType#1":                                true,
	"qa_cache_field_completeness_test.go:TestPoisonedExact_HealsOnContradiction#1":                                true,
	"qa_cache_field_completeness_test.go:TestSilentSwap_NeverDropsKnownFindings#1":                                true,
	"qa_cache_field_completeness_test.go:TestSilentSwap_Wave1FindingNotCarriedOnResolve#1":                        true,
	"qa_cache_field_completeness_test.go:fieldCompletenessPair#1":                                                 true,
	"qa_cache_lifecycle_test.go:newLifecycleController#1":                                                         true,
	"qa_childview_color_doctrine_test.go:newChildColorDoctrineController#1":                                       true,
	"qa_cli_command_flag_test.go:TestQA_CLICommand_LivePath_AvailabilityCacheLoaded_EmitsNavigateMsg#1":           true,
	"qa_cli_command_flag_test.go:TestQA_CLICommand_LivePath_ClientsReady_ArmsButDoesNotEmitNavigateYet#1":         true,
	"qa_clients_ready_test.go:TestQA_ClientsReady_NilClients_DemoFallback#1":                                      true,
	"qa_copy_test.go:TestQA_Copy_AllResourceTypes#1":                                                              true,
	"qa_copy_test.go:TestQA_Copy_Detail_CopiesFieldValue#1":                                                       true,
	"qa_copy_test.go:TestQA_Copy_ResourceList_CopiesID#1":                                                         true,
	"qa_copy_test.go:TestQA_Copy_YAML_CopiesFullYAML#1":                                                           true,
	"qa_correctness_bugs_test.go:TestBug192_AvailabilityProbes_SaveCacheOnlyAfterAllDone#1":                       true,
	"qa_correctness_bugs_test.go:TestBug193_EmptyProfile_FailedConnect_RollsBack#1":                               true,
	"qa_correctness_bugs_test.go:TestBug193_FailedSwitch_RestoresIdentityAndAvailability#1":                       true,
	"qa_correctness_bugs_test.go:TestBug193_ProfileSwitch_FailedConnect_RollsBackProfile#1":                       true,
	"qa_correctness_bugs_test.go:TestBug193_ProfileSwitch_FailedConnect_ShowsErrorFlash#1":                        true,
	"qa_correctness_bugs_test.go:TestBug193_ProfileSwitch_SuccessfulConnect_CommitsProfile#1":                     true,
	"qa_correctness_bugs_test.go:TestBug193_RapidSwitch_FailedFinalConnect_RollsBackToOriginal#1":                 true,
	"qa_correctness_bugs_test.go:TestBug193_RapidSwitch_StaleResponseIgnored#1":                                   true,
	"qa_correctness_bugs_test.go:TestBug193_RegionSwitch_FailedConnect_RollsBackRegion#1":                         true,
	"qa_correctness_bugs_test.go:TestBug193_RegionSwitch_SuccessfulConnect_CommitsRegion#1":                       true,
	"qa_correctness_bugs_test.go:TestBug194_ClientsReadyMsg_CarriesResolvedRegion#1":                              true,
	"qa_correctness_bugs_test.go:TestBug194_ConnectAWS_FallsBackToConfigFileWhenNoEnvVar#1":                       true,
	"qa_correctness_bugs_test.go:TestBug194_ConnectAWS_RespectsAWSDefaultRegionEnvVar#1":                          true,
	"qa_correctness_bugs_test.go:TestBug194_ConnectAWS_RespectsAWSRegionEnvVar#1":                                 true,
	"qa_correctness_bugs_test.go:bug192Model#1":                                                                   true,
	"qa_ec2_test.go:TestQA_EC2_A12_1_EmptyInstanceList#1":                                                         true,
	"qa_ec2_test.go:TestQA_EC2_A13_1_LoadingState#1":                                                              true,
	"qa_ec2_test.go:TestQA_EC2_A14_1_TerminalTooNarrow#1":                                                         true,
	"qa_ec2_test.go:TestQA_EC2_A14_5_TerminalTooShort#1":                                                          true,
	"qa_ec2_test.go:TestQA_EC2_A4_StatusColoring_StoppedRowHasANSI#1":                                             true,
	"qa_ec2_test.go:TestQA_EC2_D1_FullNavigationStack#1":                                                          true,
	"qa_ec2_test.go:newEC2DetailModel#1":                                                                          true,
	"qa_ec2_test.go:newEC2ListModel#1":                                                                            true,
	"qa_ec2_test.go:newEC2YAMLModel#1":                                                                            true,
	"qa_enrichment_detail_live_test.go:TestHandleEnrichmentChecked_FindingNotAppliedWhenDetailInactive#1":         true,
	"qa_enrichment_detail_live_test.go:navigateToDetailWithRDS#1":                                                 true,
	"qa_enrichment_dispatch_test.go:newTestModel#1":                                                               true,
	"qa_error_log_test.go:TestErrorFlashFullWidth_ExceedsWidthMinus4IsTruncated#1":                                true,
	"qa_error_log_test.go:TestErrorFlashFullWidth_LongMessageNotTruncatedAt80#1":                                  true,
	"qa_fetch_test.go:TestQA_FetchResources_NilClients#1":                                                         true,
	"qa_fetch_test.go:buildModelWithMockClients#1":                                                                true,
	"qa_glyph_continuity_test.go:glyphContinuityPair#1":                                                           true,
	"qa_help_context_test.go:TestQA_HelpContext_NarrowTerminal#1":                                                 true,
	"qa_issue_visibility_gate_test.go:newVisibilityDetailController#1":                                            true,
	"qa_issue_visibility_gate_test.go:newVisibilityListController#1":                                              true,
	"qa_late_replace_and_false_exact_test.go:TestFalseExact_PageOneEntryWithoutPagination_NeverDowngradesExact#1": true,
	"qa_late_replace_and_false_exact_test.go:TestFreshReplace_StillWins#1":                                        true,
	"qa_late_replace_and_false_exact_test.go:TestLateReplace_DoesNotStompDeeperList#1":                            true,
	"qa_lazy_only_fast_path_test.go:TestLazyFastPath_RequiresAllIDs#1":                                            true,
	"qa_load_more_dedup_test.go:TestLoadMore_AppendDedup_Backstop#1":                                              true,
	"qa_load_more_dedup_test.go:TestLoadMore_PersistedPair_NeverMismatched#1":                                     true,
	"qa_load_more_dedup_test.go:TestLoadMore_TUI_ColdOpen_NoDuplicates#1":                                         true,
	"qa_load_more_dedup_test.go:TestLoadMore_TokenPresent_AfterColdOpen#1":                                        true,
	"qa_mainmenu_nav_test.go:TestQA_MainMenu_ExactMinHeightRendersCorrectly#1":                                    true,
	"qa_mainmenu_nav_test.go:TestQA_MainMenu_ExactMinWidthRendersCorrectly#1":                                     true,
	"qa_mainmenu_nav_test.go:TestQA_MainMenu_NarrowTerminalShowsError#1":                                          true,
	"qa_mainmenu_nav_test.go:TestQA_MainMenu_ShortTerminalShowsError#1":                                           true,
	"qa_mainmenu_test.go:TestMainMenu_Viewport_BottomKey_LastItemVisible#1":                                       true,
	"qa_mainmenu_test.go:TestMainMenu_Viewport_CursorVisibleWhenScrolledDown#1":                                   true,
	"qa_mainmenu_test.go:TestMainMenu_Viewport_OnlyVisibleRowsRendered#1":                                         true,
	"qa_mainmenu_test.go:TestMainMenu_Viewport_ScrolledDown_EnterSelectsCorrectItem#1":                            true,
	"qa_mainmenu_test.go:TestMainMenu_Viewport_TopAfterScroll_FirstItemVisible#1":                                 true,
	"qa_mainmenu_test.go:TestQA_MainMenu_AllSevenResourceTypesVisible#1":                                          true,
	"qa_mainmenu_test.go:TestQA_MainMenu_CategoryHeaderAppearsBeforeFirstItem#1":                                  true,
	"qa_mainmenu_test.go:TestQA_MainMenu_CategoryHeadersVisible#1":                                                true,
	"qa_mainmenu_test.go:TestQA_MainMenu_CtrlD_PageDown#1":                                                        true,
	"qa_mainmenu_test.go:TestQA_MainMenu_CtrlU_PageUp#1":                                                          true,
	"qa_mainmenu_test.go:TestQA_MainMenu_EachRowShowsAlias#1":                                                     true,
	"qa_mainmenu_test.go:TestQA_MainMenu_ExactlySevenResourceRows#1":                                              true,
	"qa_mainmenu_test.go:TestQA_MainMenu_FilterHidesCategoriesWithNoMatches#1":                                    true,
	"qa_mainmenu_test.go:TestQA_MainMenu_FirstHeaderVisibleAfterScrollDownAndBackUp#1":                            true,
	"qa_mainmenu_test.go:TestQA_MainMenu_PageDownClampsAtBottom#1":                                                true,
	"qa_mainmenu_test.go:TestQA_MainMenu_PageDownMovesMultipleItems#1":                                            true,
	"qa_mainmenu_test.go:TestQA_MainMenu_PageUpClampsAtTop#1":                                                     true,
	"qa_mainmenu_test.go:TestQA_MainMenu_PageUpMovesMultipleItems#1":                                              true,
	"qa_mainmenu_test.go:TestQA_MainMenu_ScrollAccountsForHeaders#1":                                              true,
	"qa_partial_success_dispatcher_test.go:TestDispatcher_PartialSuccess_HandlerEmitsFlashMsg#1":                  true,
	"qa_profile_switch_test.go:TestBug_ProfileSwitch_ClearsRegion#1":                                              true,
	"qa_profile_switch_test.go:TestBug_RegionShownInHeader#1":                                                     true,
	"qa_profile_switch_test.go:TestBug_RegionShownInHeader_AfterConnect#1":                                        true,
	// qa_yaml_test.go's WrapToggle test needs a real *app.Controller wired
	// onto a pushed ScreenYAML + EnsureTextState to observe ActionToggleWrap
	// through RenderText — same shape as the already-allowlisted
	// text_ctrl_interaction_test.go:newTextController below. No
	// ResourcesLoaded/EnrichmentChecked/AvailabilityChecked event is ever
	// driven through this controller, so it is not subject to the
	// queueAvailabilitySave leak class this gate tracks.
	"qa_yaml_test.go:TestQA_YAML_WrapToggle_KnownLongValue#1":                                                         true,
	"rowstore_fetch_origin_enrich_pin_test.go:newFetchOriginPinSession#1":                                             true,
	"rowstore_stage2_pins_test.go:newStage2PinTestController#1":                                                       true,
	"rowstore_stage3_pins_test.go:newRowStorePinsTestController#1":                                                    true,
	"rowstore_stage4_pins_test.go:newStage4PinController#1":                                                           true,
	"runtime_alias_enrich_scope_test.go:TestControllerHandle_AliasOpenedList_ProbeEnrichScopedToCanonical#1":          true,
	"runtime_cache_rows_exact_totals_test.go:TestLoadMoreExhausted_OnlyIncreaseGuard#1":                               true,
	"runtime_cache_rows_exact_totals_test.go:TestLoadMoreExhausted_PersistsToDiskCache#1":                             true,
	"runtime_cache_rows_exact_totals_test.go:TestLoadMoreExhausted_SurvivesReturnToMenu#1":                            true,
	"runtime_cache_rows_exact_totals_test.go:TestLoadMoreExhausted_UpdatesMenuAvailability_ExactNoTUI#1":              true,
	"runtime_list_open_enrich_dispatch_test.go:TestControllerHandle_ListOpen_ResourcesLoaded_DispatchesProbeEnrich#1": true,
	"runtime_wave2_carry_test.go:TestReconcileTypeFile_Wave2SourcedObservation_ClearsCarriedData#1":                   true,
	"runtime_wave2_carry_test.go:TestRestartSeed_S3_Wave2FindingAndStatusVisibleOnFirstRender#1":                      true,
	"session_rowstore_test.go:newRowStoreControllerPin#1":                                                             true,
	"tui_help_identity_errorlog_parity_test.go:newParityHeadlessController#1":                                         true,
	"tui_intent_parity_test.go:newIntentParityHeadlessController#1":                                                   true,
	"tui_post_sweep_seed_test.go:newPostSweepApp#1":                                                                   true,
	"tui_root_test.go:TestRootView_EmptyWhenWidthZero#1":                                                              true,
	"tui_root_test.go:TestRoot_View_AltScreenOnMinHeight#1":                                                           true,
	"tui_root_test.go:TestRoot_View_AltScreenOnMinWidth#1":                                                            true,
	"tui_root_test.go:TestRoot_View_AltScreenOnZeroWidth#1":                                                           true,
	"tui_savecache_routing_test.go:newSaveCacheApp#1":                                                                 true,
	// tui_selector_test.go's newLiveSelector needs a real *app.Controller
	// wired onto a pushed ScreenProfileSelector + EnsureSelectorState to
	// exercise SelectorModel's Update() cursor logic — same shape as the
	// already-allowlisted costs_state_test.go:newCostsController below (also
	// an EnsureSelectorState caller). No ResourcesLoaded/EnrichmentChecked/
	// AvailabilityChecked event is ever driven through this controller.
	"tui_selector_test.go:newLiveSelector#1":                                                         true,
	"tui_startup_seed_test.go:TestTUIInit_EmptyRegion_ResolvesConfigDefaultForSeed#1":                true,
	"tui_startup_seed_test.go:TestTUIInit_SeedsMenuFromDisk_BeforeClientsReady#1":                    true,
	"tui_toplevel_list_persistence_test.go:TestScreenIDGuard_TopLevelCommandVsPushChildListScreen#1": true,
	"tui_wiring_test.go:TestWiring_ClientsReady_DemoMode_TriggersAvailabilityProbes#1":               true,
	"tui_wiring_test.go:TestWiring_EmptyProfileShowsDefaultInHeader#1":                               true,
	"tui_wiring_test.go:TestWiring_RefreshOnMainMenu_DemoMode_TriggersProbes#1":                      true,
	// detail_ports_test.go:newDetailController#1 — renamed from
	// detail_render_parity_test.go:newDetailController#1 (specs/022-codebase-cleanup
	// Scope C: detail_render_parity_test.go deleted, newDetailController relocated
	// alongside its other same-package callers). Same pre-existing debt, not new.
	"detail_ports_test.go:newDetailController#1": true,
	// text_ports_test.go:newTextScreenController#1 — renamed from
	// text_ctrl_interaction_test.go:newTextController#1 (that file's
	// dead-Update()-driven pins were replaced by real key-path ports, and its
	// live ctrl.Apply-direct precision pins moved here). Same pre-existing
	// debt, not new: no ResourcesLoaded/
	// EnrichmentChecked/AvailabilityChecked event is ever driven through it.
	"text_ports_test.go:newTextScreenController#1": true,
}

// ccdSite is one direct tui.New/app.New call site the scanner found.
type ccdSite struct {
	file     string
	line     int
	funcName string
	pkg      string
}

// ccdIsNewCall reports whether call is a direct <pkg>.New*(...) call where
// pkg is "tui" or "app", returning the package identifier on match.
func ccdIsNewCall(call *ast.CallExpr) (pkg string, matched bool) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok {
		return "", false
	}
	if id.Name != "tui" && id.Name != "app" {
		return "", false
	}
	if !strings.HasPrefix(sel.Sel.Name, "New") {
		return "", false
	}
	return id.Name, true
}

// ccdCallHasNoCacheTrue reports whether call's argument list contains a
// literal WithNoCache(true) sub-expression, e.g.
// tui.New(profile, region, tui.WithNoCache(true)). Detection is deliberately
// narrow (a direct CallExpr argument, not a spread variable or a helper that
// returns an Option) — per this gate's charter, any construction whose
// safety this AST check cannot prove falls through to the census instead of
// being silently treated as safe.
func ccdCallHasNoCacheTrue(call *ast.CallExpr) bool {
	for _, arg := range call.Args {
		inner, ok := arg.(*ast.CallExpr)
		if !ok {
			continue
		}
		sel, ok := inner.Fun.(*ast.SelectorExpr)
		if !ok {
			continue
		}
		if sel.Sel.Name != "WithNoCache" {
			continue
		}
		if len(inner.Args) != 1 {
			continue
		}
		id, ok := inner.Args[0].(*ast.Ident)
		if ok && id.Name == "true" {
			return true
		}
	}
	return false
}

// ccdEnclosingFuncIntervals returns, for every top-level FuncDecl in file
// with a body, its name and the [start,end) token.Pos span of that body.
// Nested ast.FuncLit closures share their enclosing FuncDecl's span (Go has
// no nested named funcs), so a position lookup against these intervals
// alone correctly attributes calls inside t.Run(func(t *testing.T){...})
// closures to the containing top-level function.
func ccdEnclosingFuncIntervals(file *ast.File) []struct {
	name       string
	start, end token.Pos
} {
	var intervals []struct {
		name       string
		start, end token.Pos
	}
	for _, decl := range file.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			continue
		}
		intervals = append(intervals, struct {
			name       string
			start, end token.Pos
		}{fd.Name.Name, fd.Body.Pos(), fd.Body.End()})
	}
	return intervals
}

// ccdScanFile parses path and returns every direct tui.New/app.New call
// site not exempted by a blessed helper or a literal WithNoCache(true) arg.
func ccdScanFile(fset *token.FileSet, path string) ([]ccdSite, error) {
	src, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil, err
	}
	intervals := ccdEnclosingFuncIntervals(src)
	enclosingFunc := func(pos token.Pos) string {
		for _, iv := range intervals {
			if iv.start <= pos && pos < iv.end {
				return iv.name
			}
		}
		return ""
	}

	var sites []ccdSite
	ast.Inspect(src, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		pkg, matched := ccdIsNewCall(call)
		if !matched {
			return true
		}
		if pkg == "tui" && ccdCallHasNoCacheTrue(call) {
			return true
		}
		funcName := enclosingFunc(call.Pos())
		if ccdBlessedHelpers[funcName] {
			return true
		}
		pos := fset.Position(call.Pos())
		sites = append(sites, ccdSite{
			file:     filepath.Base(path),
			line:     pos.Line,
			funcName: funcName,
			pkg:      pkg,
		})
		return true
	})
	return sites, nil
}

// ccdSiteKey builds this gate's allowlist key: "<file>:<func-or-package-
// level>#<occurrence>", where occurrence disambiguates multiple direct
// construction calls inside the same enclosing function.
func ccdSiteKey(site ccdSite, occurrence int) string {
	label := site.funcName
	if label == "" {
		label = "<package-level>"
	}
	return fmt.Sprintf("%s:%s#%d", site.file, label, occurrence)
}

// TestControllerConstructionDisciplineGate is the standing ratchet: every
// direct tui.New(...)/app.New(...) call site under tests/unit/*.go must
// either live inside a blessed helper, carry a literal WithNoCache(true)
// argument, or already be pinned in knownConstructionDebt. See the
// file-level doc comment for full semantics.
func TestControllerConstructionDisciplineGate(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed — cannot locate test file")
	}
	unitDir := filepath.Dir(thisFile)

	pattern := filepath.Join(unitDir, "*.go")
	files, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("filepath.Glob(%q): %v", pattern, err)
	}
	if len(files) == 0 {
		t.Fatalf("no .go files found under %s — check path", unitDir)
	}

	fset := token.NewFileSet()
	found := map[string]ccdSite{}
	fileCounts := map[string]int{}
	perFuncOccurrence := map[string]int{}

	for _, path := range files {
		sites, scanErr := ccdScanFile(fset, path)
		if scanErr != nil {
			t.Errorf("parse error in %s: %v", path, scanErr)
			continue
		}
		for _, site := range sites {
			occKey := site.file + ":" + site.funcName
			perFuncOccurrence[occKey]++
			key := ccdSiteKey(site, perFuncOccurrence[occKey])
			found[key] = site
			fileCounts[site.file]++
		}
	}

	allKeys := map[string]bool{}
	for k := range found {
		allKeys[k] = true
	}
	for k := range knownConstructionDebt {
		allKeys[k] = true
	}
	sortedKeys := make([]string, 0, len(allKeys))
	for k := range allKeys {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)

	var newlyRegressed, readyForBurnDown, stillGapped []string

	for _, key := range sortedKeys {
		key := key
		site, isFound := found[key]
		allowlisted := knownConstructionDebt[key]

		t.Run(key, func(t *testing.T) {
			switch {
			case isFound && allowlisted:
				stillGapped = append(stillGapped, key)
				t.Skipf(
					"KNOWN DEBT (allowlisted): %s:%d func=%q — direct %s.New(...) construction outside "+
						"blessed helpers — pre-existing debt, see knownConstructionDebt",
					site.file, site.line, site.funcName, site.pkg,
				)
			case isFound && !allowlisted:
				newlyRegressed = append(newlyRegressed, key)
				t.Errorf(
					"NEW VIOLATION (not allowlisted): %s:%d func=%q — direct %s.New(...) construction. "+
						"Route through a blessed helper (newTestController, newTestControllerWithCore, "+
						"newTestControllerAndCore, newRootSizedModel, newDetailParityHeadlessController) or "+
						"tui.New(..., tui.WithNoCache(true)); or, if this is pre-existing debt, add %q to "+
						"knownConstructionDebt in this file",
					site.file, site.line, site.funcName, site.pkg, key,
				)
			case !isFound && allowlisted:
				readyForBurnDown = append(readyForBurnDown, key)
				t.Errorf(
					"BURN-DOWN: %q no longer appears in the live scan but is still pinned in "+
						"knownConstructionDebt — remove it from the allowlist in this PR", key,
				)
			}
		})
	}

	var fileBreakdown []string
	fileNames := make([]string, 0, len(fileCounts))
	for f := range fileCounts {
		fileNames = append(fileNames, f)
	}
	sort.Strings(fileNames)
	for _, f := range fileNames {
		fileBreakdown = append(fileBreakdown, fmt.Sprintf("%s=%d", f, fileCounts[f]))
	}

	t.Logf("CENSUS SIZE: %d call site(s) found across %d file(s)", len(found), len(fileCounts))
	t.Logf("PER-FILE BREAKDOWN: %s", strings.Join(fileBreakdown, ", "))
	if len(newlyRegressed) > 0 {
		t.Logf("NEW VIOLATION INVENTORY (%d): %v", len(newlyRegressed), newlyRegressed)
	}
	if len(readyForBurnDown) > 0 {
		t.Logf("READY-FOR-BURN-DOWN INVENTORY (%d): %v", len(readyForBurnDown), readyForBurnDown)
	}
	if len(stillGapped) > 0 {
		t.Logf("STILL-GAPPED (allowlisted, skipped) INVENTORY (%d): %v", len(stillGapped), stillGapped)
	}
}
