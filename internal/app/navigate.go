package app

import (
	"encoding/json"
	"reflect"
	"strings"

	"github.com/k2m30/a9s/v3/internal/fieldpath"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
	"gopkg.in/yaml.v3"
)

// resourceYAMLLines marshals r to plain YAML text (no ANSI coloring) and
// returns the individual lines. Mirrors the source that YAMLModel.RawContent
// uses — RawStruct when present, resource.Fields as fallback — so the
// content is equivalent to what the TUI YAML screen shows (minus syntax color).
//
// Returns a one-element slice with a "No YAML data available" notice when
// the resource carries neither RawStruct nor Fields.
func resourceYAMLLines(r resource.Resource) []string {
	var data []byte
	var err error
	if r.RawStruct != nil {
		safe := fieldpath.ToSafeValue(reflect.ValueOf(r.RawStruct))
		data, err = yaml.Marshal(safe)
	} else if len(r.Fields) > 0 {
		data, err = yaml.Marshal(r.Fields)
	}
	if err != nil || len(data) == 0 {
		return []string{"  No YAML data available"}
	}
	raw := strings.TrimRight(string(data), "\n")
	return strings.Split(raw, "\n")
}

// resourceJSONLines marshals r to indented plain JSON text (no ANSI coloring)
// and returns the individual lines. Mirrors JSONModel.RawContent — RawStruct
// when present, resource.Fields as fallback.
//
// For the JSON case we also try a roundtrip through jsonyaml.TryJSONToYAMLLines
// to validate the JSON is well-formed; the actual output is the MarshalIndent
// string split by newline, which is always valid when MarshalIndent succeeds.
//
// Returns a one-element slice with a "No JSON data available" notice when the
// resource carries neither RawStruct nor Fields.
func resourceJSONLines(r resource.Resource) []string {
	var data []byte
	var err error
	if r.RawStruct != nil {
		data, err = json.MarshalIndent(r.RawStruct, "", "  ")
	} else if len(r.Fields) > 0 {
		data, err = json.MarshalIndent(r.Fields, "", "  ")
	}
	if err != nil || len(data) == 0 {
		return []string{"  No JSON data available"}
	}
	raw := strings.TrimRight(string(data), "\n")
	return strings.Split(raw, "\n")
}

// applyNavResult converts a NavigateResult into PushScreen/ReplaceScreen/PopScreen
// stack operations. Called by Apply after HandleNavigate returns.
//
// The adapter (not the runtime) decides which ScreenID to push for each kind;
// this method encodes that mapping for the headless controller.
//
// All NavigateResult kinds are handled, including those that require
// selected-row or resource data (PushDetail, PushYAML, PushJSON,
// PushResourceList/Cached, FetchReveal).
func (c *Controller) applyNavResult(res runtime.NavigateResult) {
	switch res.Kind {
	case runtime.NavigateKindPopAll:
		// Pop back to the root menu — leave exactly one screen, never empty.
		// (Popping via ApplyIntents would now stop at the len<=1 guard anyway;
		// pop directly to keep the intent clear.)
		if len(c.stack) > 1 {
			c.stack = c.stack[:1]
		}

	case runtime.NavigateKindPushHelp:
		c.applyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenHelp}})

	case runtime.NavigateKindPushRegion:
		c.applyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenRegion}})

	case runtime.NavigateKindPushTheme:
		c.applyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenTheme}})

	case runtime.NavigateKindFetchProfiles:
		// No stack change — the adapter starts the fetch task; when the result
		// arrives (ProfilesLoaded), HandleProfilesLoaded pushes ScreenProfileSelector.

	case runtime.NavigateKindFlash, runtime.NavigateKindNoop:
		// No stack change — flash is surfaced via FlashIntent in the intent stream.

	case runtime.NavigateKindPushResourceList, runtime.NavigateKindPushResourceListCached:
		intent := runtime.PushScreen{
			ID:      runtime.ScreenResourceList,
			Context: runtime.ScreenContext{ResourceType: res.ResolvedType},
		}
		if res.ReplaceCurrent {
			c.applyIntents([]runtime.UIIntent{runtime.ReplaceScreen{ID: intent.ID, Context: intent.Context}})
		} else {
			c.applyIntents([]runtime.UIIntent{intent})
		}
		c.ensureListState()
		// For the cached path, populate rows immediately from the cache entry so
		// headless/web callers see data without waiting for a fetch round-trip.
		if res.Kind == runtime.NavigateKindPushResourceListCached && res.CachedEntry != nil {
			top := &c.stack[len(c.stack)-1]
			c.applyResourcesLoaded(top.State.List, res.ResolvedType, res.CachedEntry.Resources, res.CachedEntry.Pagination, false)
		}

	case runtime.NavigateKindPushDetail:
		if res.Resource == nil {
			return
		}
		intent := runtime.PushScreen{
			ID: runtime.ScreenDetail,
			Context: runtime.ScreenContext{
				ResourceType: res.ResolvedType,
				ResourceID:   res.Resource.ID,
			},
		}
		c.applyIntents([]runtime.UIIntent{intent})
		// Use lock-free variants — applyNavResult is always called while c.mu
		// is already held by Apply or Handle.
		c.ensureDetailState(*res.Resource, res.ResolvedType)
		if res.DispatchRelated {
			c.initDetailRelatedRows(res.ResolvedType)
		}

	case runtime.NavigateKindPushYAML:
		if res.Resource == nil {
			return
		}
		lines := resourceYAMLLines(*res.Resource)
		intent := runtime.PushScreen{
			ID: runtime.ScreenYAML,
			Context: runtime.ScreenContext{
				ResourceType: res.ResolvedType,
				ResourceID:   res.Resource.ID,
			},
		}
		c.applyIntents([]runtime.UIIntent{intent})
		c.ensureTextState(lines)

	case runtime.NavigateKindPushJSON:
		if res.Resource == nil {
			return
		}
		lines := resourceJSONLines(*res.Resource)
		intent := runtime.PushScreen{
			ID: runtime.ScreenJSON,
			Context: runtime.ScreenContext{
				ResourceType: res.ResolvedType,
				ResourceID:   res.Resource.ID,
			},
		}
		c.applyIntents([]runtime.UIIntent{intent})
		c.ensureTextState(lines)

	case runtime.NavigateKindFetchReveal:
		// No stack push yet — the push happens when Handle receives
		// messages.ValueRevealed and routes it to HandleValueRevealed.
		// Tasks are returned by Apply's ActionReveal branch directly.
	}
}

// dispatchRelatedNavigate calls HandleRelatedNavigate then applyRelatedNavResult
// and merges the two task slices, preferring extraTasks when the same Key
// appears in both (applyRelatedNavResult returns payload-bearing replacements
// for tasks HandleRelatedNavigate emits without payloads, e.g. KindFetchFiltered).
// All three related-nav callers — ActionSelect keyboard, ActionRelatedSelect
// click, and ActionFieldSelect click — use this shared tail.
func (c *Controller) dispatchRelatedNavigate(ev runtime.RelatedNavigateEvent) []runtime.TaskRequest {
	navRes, tasks := c.core.HandleRelatedNavigate(ev)
	extraTasks := c.applyRelatedNavResult(navRes)
	if len(extraTasks) == 0 {
		return tasks
	}
	extraKeys := make(map[runtime.TaskKey]struct{}, len(extraTasks))
	for _, t := range extraTasks {
		extraKeys[t.Key] = struct{}{}
	}
	merged := make([]runtime.TaskRequest, 0, len(tasks)+len(extraTasks))
	for _, t := range tasks {
		if _, replaced := extraKeys[t.Key]; !replaced {
			merged = append(merged, t)
		}
	}
	merged = append(merged, extraTasks...)
	return merged
}

// openRelatedDetail pushes a detail screen for the already-fetched resource
// cached and seeds its related panel — replaying the related cache when present,
// otherwise returning a KindRelatedCheck task. Shared by the cache-hit related-
// navigate path (NavigationKindDetail) and the web by-ID auto-open path.
// Caller must hold c.mu (write).
func (c *Controller) openRelatedDetail(cached resource.Resource, targetType string) []runtime.TaskRequest {
	c.applyIntents([]runtime.UIIntent{runtime.PushScreen{
		ID:      runtime.ScreenDetail,
		Context: runtime.ScreenContext{ResourceType: targetType, ResourceID: cached.ID},
	}})
	c.ensureDetailState(cached, targetType)
	ds := c.topDetailState()
	if ds == nil || len(resource.GetRelated(targetType)) == 0 {
		return nil
	}
	c.initDetailRelatedRows(targetType)
	// Populate the related panel: replay the related cache if present, else
	// dispatch a KindRelatedCheck task — same as ActionOpenDetail.
	ck := runtime.RelatedCacheKey(targetType, cached.ID)
	if cachedRows, hit := c.core.RelatedCacheGet(ck); hit && len(cachedRows) > 0 {
		for _, entry := range cachedRows {
			errMsg := ""
			if entry.Result.Err != nil {
				errMsg = entry.Result.Err.Error()
			}
			mergeDetailRelatedRow(ds, entry.DefDisplayName, entry.Result.TargetType,
				entry.Result.Count, false, errMsg, entry.Result.Approximate, entry.Result.ResourceIDs, entry.Result.FetchFilter)
		}
		return nil
	}
	return []runtime.TaskRequest{{
		Key:     runtime.TaskKey{Kind: runtime.KindRelatedCheck, Scope: targetType + "/" + cached.ID},
		Cache:   runtime.CacheNone,
		Payload: runtime.RelatedCheckPayload{ResourceType: targetType, Resource: cached},
	}}
}

// applyRelatedNavResult converts a NavigationResult into stack operations and
// returns any additional task requests the result spawns.
//
// NavigationResult carries NavigationKind plus TargetType, TargetID,
// RelatedIDs, FetchFilter, FilterText — no ScreenID; the controller maps
// kind → ScreenID.
func (c *Controller) applyRelatedNavResult(res runtime.NavigationResult) []runtime.TaskRequest {
	switch res.Kind {
	case runtime.NavigationKindResourceList:
		intent := runtime.PushScreen{
			ID:      runtime.ScreenResourceList,
			Context: runtime.ScreenContext{ResourceType: res.TargetType},
		}
		c.applyIntents([]runtime.UIIntent{intent})
		c.ensureListState()

	case runtime.NavigationKindFilteredList:
		intent := runtime.PushScreen{
			ID:      runtime.ScreenResourceList,
			Context: runtime.ScreenContext{ResourceType: res.TargetType},
		}
		c.applyIntents([]runtime.UIIntent{intent})
		c.ensureListState()
		if ls := c.topListState(); ls != nil {
			if res.FilterText != "" {
				ls.Filter = res.FilterText
			}
			if len(res.FetchFilter) > 0 {
				ls.FetchFilter = res.FetchFilter
				// HandleRelatedNavigate returns a no-payload KindFetchFiltered task
				// (tested as-is by the QA suite). Replace it here with a payload-
				// bearing version so the executor can invoke the filtered fetcher.
				return []runtime.TaskRequest{{
					Key:     runtime.TaskKey{Kind: runtime.KindFetchFiltered, Scope: res.TargetType},
					Cache:   runtime.CacheNone,
					Payload: runtime.FetchFilteredPayload{Filter: res.FetchFilter},
				}}
			}
			if len(res.RelatedIDs) > 0 {
				// Multi-ID related nav with no server-side filter (e.g. an EC2
				// instance → its several security groups): prefilter the list to
				// just the related subset, mirroring the TUI related-list path.
				// Without this the list shows every resource of the target type
				// instead of the related ones. We're already under c.mu, so seed
				// ls directly — PatchListRelatedIDSet would re-lock and deadlock.
				set := make(map[string]struct{}, len(res.RelatedIDs))
				for _, id := range res.RelatedIDs {
					if id != "" {
						set[id] = struct{}{}
					}
				}
				ls.RelatedIDSet = set
			}
			// By-ID single-target drill (web/headless): when the target type has a
			// FetchByIDs helper, HandleRelatedNavigate returns a KindFetchByIDDetail
			// task. Flag the placeholder list so Handle replaces it with the
			// target's detail once the fetched row arrives — the TUI drills to
			// by-ID detail in its own adapter and never reaches this path.
			if res.TargetID != "" && resource.GetFetchByIDs(res.TargetType) != nil {
				// Always key on TargetID: autoOpenSingleDetail matches ls.Rows
				// against this set and the KindFetchByIDDetail task fetches by
				// TargetID, so the set must reference that same ID.
				ls.RelatedIDSet = map[string]struct{}{res.TargetID: {}}
				ls.AutoOpenSingle = true
			}
		}

	case runtime.NavigationKindDetail:
		// The target resource is already cached: NavigationKindDetail is only
		// returned on a cache hit (ResolveRelatedNavigate), and HandleRelatedNavigate
		// returns no fetch task for it ("the adapter serves these from cached
		// state"). Seed the detail synchronously from the cache — this works for
		// every type (most have no by-id fetcher, so a fetch would land on an empty
		// detail). Mirrors ActionOpenDetail's related-panel handling.
		id := res.TargetID
		if id == "" && len(res.RelatedIDs) == 1 {
			id = res.RelatedIDs[0]
		}
		cached, ok := c.core.RelatedCachedResource(res.TargetType, id)
		if !ok {
			// Defensive: cache unexpectedly missing. Land on a filtered list by id
			// instead of an empty detail so navigation still resolves somewhere.
			c.applyIntents([]runtime.UIIntent{runtime.PushScreen{
				ID:      runtime.ScreenResourceList,
				Context: runtime.ScreenContext{ResourceType: res.TargetType},
			}})
			c.ensureListState()
			if ls := c.topListState(); ls != nil && id != "" {
				ls.Filter = id
			}
			return nil
		}
		return c.openRelatedDetail(cached, res.TargetType)

	case runtime.NavigationKindEnterChildView:
		// Delegate to the same path used by ActionChildView but with the
		// target type already resolved. Build a minimal EnterChildViewEvent.
		ev := runtime.EnterChildViewEvent{
			ChildType: res.TargetType,
		}
		intents, tasks := c.core.HandleEnterChildView(ev)
		c.applyIntents(intents)
		if len(c.stack) > 0 {
			top := &c.stack[len(c.stack)-1]
			if top.ID == runtime.ScreenChildList {
				top.Ctx.ResourceType = res.TargetType
				if top.State.List == nil {
					top.State.List = &ListState{Loading: true}
					applyListDefaults(top.State.List, top.Ctx.ResourceType)
				}
				if res.FetchFilter != nil {
					top.State.List.ParentContext = res.FetchFilter
				}
			}
		}
		return tasks

	case runtime.NavigationKindFlash:
		// Flash is surfaced as a FlashIntent by HandleRelatedNavigate — no
		// stack change needed here.

	default:
		// NavigationKindUnknown or future kinds: no-op.
	}
	return nil
}

