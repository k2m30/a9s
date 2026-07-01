package app

import (
	"fmt"
	"strings"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
)

// handleActionOpenHelp handles ActionOpenHelp.
func (c *Controller) handleActionOpenHelp(a Action) (ViewState, []runtime.TaskRequest) {
	res, tasks := c.core.HandleNavigate(runtime.NavigateEvent{Target: runtime.NavigateTargetHelp})
	c.applyNavResult(res)
	return c.snapshot(), tasks
}

// handleActionOpenIdentity handles ActionOpenIdentity.
func (c *Controller) handleActionOpenIdentity(_ Action) (ViewState, []runtime.TaskRequest) {
	// The runtime has no NavigateTargetIdentity: the TUI opens the identity
	// screen via direct key-handling (not HandleNavigate). The headless
	// controller pushes ScreenIdentity directly so tests can assert the stack
	// without standing up a full TUI.
	c.applyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenIdentity}})
	c.core.SetIdentityFetching(true)
	c.identityLoading = true
	c.identityResult = nil
	c.identityErrMsg = ""
	fetchTask := runtime.TaskRequest{
		Key:     runtime.TaskKey{Kind: runtime.TaskKindFetchIdentity},
		Payload: runtime.FetchIdentityPayload{},
	}
	return c.snapshot(), []runtime.TaskRequest{fetchTask}
}

// handleActionOpenErrorLog handles ActionOpenErrorLog.
func (c *Controller) handleActionOpenErrorLog(_ Action) (ViewState, []runtime.TaskRequest) {
	// Mirror the TUI's '!' key: flash when no errors recorded; otherwise push
	// a text screen with the log entries newest-first.
	c.showErrorHint = false
	if len(c.errorHistory) == 0 {
		intents, tasks := c.core.HandleFlash(runtime.FlashEvent{
			Text:    "No errors this session",
			IsError: false,
			NewGen:  c.core.ConnectGen(),
		})
		c.applyIntents(intents)
		return c.snapshot(), tasks
	}
	var sb strings.Builder
	for i := len(c.errorHistory) - 1; i >= 0; i-- {
		e := c.errorHistory[i]
		fmt.Fprintf(&sb, "[%s] %s\n", e.t.Format("15:04:05"), e.message)
	}
	lines := strings.Split(strings.TrimRight(sb.String(), "\n"), "\n")
	c.applyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenErrorLog}})
	c.ensureTextState(lines)
	return c.snapshot(), nil
}

// handleActionSelectProfile handles ActionSelectProfile.
func (c *Controller) handleActionSelectProfile(a Action) (ViewState, []runtime.TaskRequest) {
	// ConnectGen is read pre-Rotate; HandleProfileSelected calls Rotate internally.
	// NewGen is passed as the bumped flash gen for the "Switching to …" tick.
	// The headless controller has no flash.gen to bump, so we pass the current
	// ConnectGen as a stable stand-in — the ClearFlash tick is adapter-owned.
	intents, tasks := c.core.HandleProfileSelected(runtime.ProfileSelectedEvent{
		Profile: a.Arg,
		NewGen:  c.core.ConnectGen(),
	})
	c.applyIntents(intents)
	return c.snapshot(), tasks
}

// handleActionSelectRegion handles ActionSelectRegion.
func (c *Controller) handleActionSelectRegion(a Action) (ViewState, []runtime.TaskRequest) {
	intents, tasks := c.core.HandleRegionSelected(runtime.RegionSelectedEvent{
		Region: a.Arg,
		NewGen: c.core.ConnectGen(),
	})
	c.applyIntents(intents)
	return c.snapshot(), tasks
}

// handleActionSelectTheme handles ActionSelectTheme.
func (c *Controller) handleActionSelectTheme(a Action) (ViewState, []runtime.TaskRequest) {
	intents, tasks := c.core.HandleThemeSelected(runtime.ThemeSelectedEvent{
		Theme: a.Arg,
	})
	c.applyIntents(intents)
	return c.snapshot(), tasks
}

// handleActionCommand handles ActionCommand.
func (c *Controller) handleActionCommand(a Action) (ViewState, []runtime.TaskRequest) {
	// Arg carries a colon-command token (mirrors executeCommand in app_input.go).
	// Arg-driven tokens (navigate to a resource type, profile, region, etc.) are
	// dispatched here; "q"/"quit" is intentionally left to the renderer.
	switch a.Arg {
	case "root", "main":
		res, tasks := c.core.HandleNavigate(runtime.NavigateEvent{Target: runtime.NavigateTargetMainMenu})
		c.applyNavResult(res)
		return c.snapshot(), tasks

	case "profile", "ctx":
		res, tasks := c.core.HandleNavigate(runtime.NavigateEvent{Target: runtime.NavigateTargetProfile})
		c.applyNavResult(res)
		return c.snapshot(), tasks

	case "region":
		res, tasks := c.core.HandleNavigate(runtime.NavigateEvent{Target: runtime.NavigateTargetRegion})
		c.applyNavResult(res)
		return c.snapshot(), tasks

	case "theme":
		res, tasks := c.core.HandleNavigate(runtime.NavigateEvent{Target: runtime.NavigateTargetTheme})
		c.applyNavResult(res)
		return c.snapshot(), tasks

	case "help":
		res, tasks := c.core.HandleNavigate(runtime.NavigateEvent{Target: runtime.NavigateTargetHelp})
		c.applyNavResult(res)
		return c.snapshot(), tasks

	default:
		// Resource short-name or alias (e.g. "ec2", "s3", "dbi").
		if rt := resource.FindResourceType(a.Arg); rt != nil {
			res, tasks := c.core.HandleNavigate(runtime.NavigateEvent{
				Target:       runtime.NavigateTargetResourceList,
				ResourceType: a.Arg,
			})
			c.applyNavResult(res)
			return c.snapshot(), tasks
		}
		// "q"/"quit" is intentionally not handled here: quitting requires tea.Quit,
		// a renderer concern the controller cannot (and must not) own. Unknown
		// command tokens are silently dropped at this layer; the renderer flashes.
	}
	return c.snapshot(), nil
}

// handleActionOpenYAML handles ActionOpenYAML.
func (c *Controller) handleActionOpenYAML(_ Action) (ViewState, []runtime.TaskRequest) {
	r, typeName, ok := c.selectedResourceForAction()
	if !ok {
		return c.snapshot(), nil
	}
	res, tasks := c.core.HandleNavigate(runtime.NavigateEvent{
		Target:       runtime.NavigateTargetYAML,
		ResourceType: typeName,
		Resource:     &r,
	})
	c.applyNavResult(res)
	return c.snapshot(), tasks
}

// handleActionOpenJSON handles ActionOpenJSON.
func (c *Controller) handleActionOpenJSON(_ Action) (ViewState, []runtime.TaskRequest) {
	r, typeName, ok := c.selectedResourceForAction()
	if !ok {
		return c.snapshot(), nil
	}
	res, tasks := c.core.HandleNavigate(runtime.NavigateEvent{
		Target:       runtime.NavigateTargetJSON,
		ResourceType: typeName,
		Resource:     &r,
	})
	c.applyNavResult(res)
	return c.snapshot(), tasks
}

// handleActionReveal handles ActionReveal.
func (c *Controller) handleActionReveal(_ Action) (ViewState, []runtime.TaskRequest) {
	// Resolve the resource from the active list or detail screen.
	var revealRes *resource.Resource
	var revealType string
	if ds := c.topDetailState(); ds != nil {
		r := ds.Resource
		revealRes = &r
		revealType = ds.ResourceType
	} else if ls := c.topListState(); ls != nil {
		r, ok := c.listSelected()
		if ok {
			revealRes = &r
			if top := c.stack[len(c.stack)-1]; len(c.stack) > 0 {
				revealType = top.Ctx.ResourceType
			}
		}
	}
	if revealRes == nil {
		return c.snapshot(), nil
	}
	res, tasks := c.core.HandleNavigate(runtime.NavigateEvent{
		Target:       runtime.NavigateTargetReveal,
		ResourceType: revealType,
		Resource:     revealRes,
	})
	// KindFetchReveal: no stack push yet — the push happens when
	// Handle receives messages.ValueRevealed and calls HandleValueRevealed.
	_ = res
	return c.snapshot(), tasks
}

// handleActionChildView handles ActionChildView.
func (c *Controller) handleActionChildView(a Action) (ViewState, []runtime.TaskRequest) {
	// Arg carries the trigger key (e, L, R, r, s, Enter, t …).
	triggerKey := a.Arg
	if triggerKey == "" {
		return c.snapshot(), nil
	}
	r, typeName, ok := c.selectedResourceForAction()
	if !ok {
		return c.snapshot(), nil
	}
	td := resource.FindResourceType(typeName)
	if td == nil {
		return c.snapshot(), nil
	}
	// Walk the type's children to find the one registered under this key.
	var matchedChild *resource.ChildViewDef
	for i := range td.Children {
		ch := &td.Children[i]
		if ch.Key != triggerKey {
			continue
		}
		if ch.DrillCondition != nil && !ch.DrillCondition(r) {
			continue
		}
		matchedChild = ch
		break
	}
	if matchedChild == nil {
		return c.snapshot(), nil
	}
	// Build the parent context from ContextKeys.
	ctx := make(map[string]string, len(matchedChild.ContextKeys))
	for param, source := range matchedChild.ContextKeys {
		switch source {
		case "ID":
			ctx[param] = r.ID
		case "Name":
			ctx[param] = r.Name
		default:
			ctx[param] = r.Fields[source]
		}
	}
	displayName := ctx[matchedChild.DisplayNameKey]
	ev := runtime.EnterChildViewEvent{
		ChildType:     matchedChild.ChildType,
		ParentContext: ctx,
		DisplayName:   displayName,
	}
	intents, tasks := c.core.HandleEnterChildView(ev)
	c.applyIntents(intents)
	// Seed the child list screen's context and state after PushScreen.
	if len(c.stack) > 0 {
		top := &c.stack[len(c.stack)-1]
		if top.ID == runtime.ScreenChildList {
			top.Ctx.ResourceType = matchedChild.ChildType
			if top.State.List == nil {
				top.State.List = &ListState{
					Loading:       true,
					ParentContext: ctx,
				}
				applyListDefaults(top.State.List, top.Ctx.ResourceType)
			}
		}
	}
	return c.snapshot(), tasks
}

// handleActionCloudTrail handles ActionCloudTrail.
func (c *Controller) handleActionCloudTrail(_ Action) (ViewState, []runtime.TaskRequest) {
	// Navigate to the CloudTrail Events ("ct-events") list filtered to the
	// active resource. Mirrors the TUI's 't' key: BuildCloudTrailFilter →
	// RelatedNavigate to "ct-events" with a FetchFilter (server-side filtered
	// fetch). No-ops when the resource type has no CloudTrailKey.
	r, typeName, ok := c.selectedResourceForAction()
	if !ok {
		return c.snapshot(), nil
	}
	ff := resource.BuildCloudTrailFilter(r, typeName)
	if ff == nil {
		return c.snapshot(), nil
	}
	ev := runtime.RelatedNavigateEvent{
		TargetType:     "ct-events",
		SourceResource: r,
		SourceType:     typeName,
		FetchFilter:    ff,
	}
	tasks := c.dispatchRelatedNavigate(ev)
	return c.snapshot(), tasks
}
