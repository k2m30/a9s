# Scripted Scenario Harness

`tests/integration/scripted_scenario_helpers_test.go` adds a higher-level harness for focused repro-style integration tests.

It is meant for bugs described as operator instructions, for example:

- open resource `X`
- enter detail
- follow related `Y`
- change region/profile
- press `m`
- confirm a flash or API error

The harness drives the real `tui.Model.Update()` loop and executes the returned Bubble Tea commands. It is not a PTY test. It works by sending the same message types the app uses internally and by applying the resulting command messages back into the model.

## What It Covers

- open top-level resource lists
- open detail views for concrete resources
- open the currently selected row's detail view
- open YAML views
- follow related rows by display name
- execute semantic `:` commands like `region`, `profile`, `help`, or a resource short name
- choose a region or profile from selector flows
- press keys such as `m`, `d`, `esc`, `enter`, `ctrl+r`
- drive `/` list filters and searchable detail/YAML/related views
- drive list sort keys `N`, `I`, `A`
- assert flash text, API errors, current frame/view text, current list type, and current detail resource
- discover a concrete live/demo resource before the scenario starts

## Important Behavior

- `Command("region")` and `Command("ctx")` are semantic command helpers. They mirror the app's command routing, but they do not type one character at a time into command mode.
- `OpenDetailResource(...)` navigates directly to a known resource's detail view. This is intentional: it keeps repro tests stable and avoids coupling them to list cursor position.
- `OpenSelectedDetail()` is available when you explicitly want to validate the selected row after sorting or filtering.
- `FollowRelated("CloudTrail Events")` uses the related-check results already computed for the current detail view, then sends a real `messages.RelatedNavigateMsg` through the root model.
- `StartFilter()` / `ApplyFilter(...)` use the app's real header filter mode for filterable views.
- `StartSearch()` / `ApplySearch(...)` use the active view's real `/` behavior. On detail/YAML this is text search. When the RELATED column has focus, `/` filters the related rows instead.
- Region/profile switching drains the reconnect flow far enough to observe the new `ClientsReadyMsg` and any refreshed active list/error, but it does not deliberately expand unrelated availability background probes.
- Every failing assertion prints the current rendered screen plus the scenario action history.

## Constructors

Demo:

```go
scenario := fullIntegrationNewDemoScenario(t)
```

Live:

```go
scenario := fullIntegrationNewLiveScenario(t, "profile-name", "")
```

If the region argument is empty, the app resolves it from the profile.

## Common Actions

Navigation:

```go
scenario.OpenList("iam-user")
scenario.OpenDetailResource("iam-user", user)
scenario.OpenSelectedDetail()
scenario.OpenYAML()
scenario.FollowRelated("CloudTrail Events")
scenario.Back()
```

Input, filter, and search:

```go
scenario.StartFilter()
scenario.Type("alice")
scenario.ConfirmInput()

scenario.StartSearch()
scenario.Type("AccessKeyId")
scenario.ConfirmInput()
scenario.SearchNext()
scenario.SearchPrev()
```

Sorting:

```go
scenario.SortByName()
scenario.SortByID()
scenario.SortByAge()
```

## Discovery Helpers

Find any resource:

```go
user := fullIntegrationMustFindAnyResource(t, scenario.clients, "iam-user")
```

Find by exact ID:

```go
trail := fullIntegrationMustFindResourceByID(t, scenario.clients, "trail", "my-trail")
```

Find by name substring:

```go
user := fullIntegrationMustFindResourceByNameContains(t, scenario.clients, "iam-user", "alice")
```

Find by field substring, optionally using a filtered fetcher:

```go
event := fullIntegrationMustFindResourceByFieldContains(
	t,
	scenario.clients,
	"ct-events",
	"event_name",
	"DescribeAlarms",
	fullIntegrationFindResourceOptions{},
)
```

## Example 1: Demo Mode Blocks Region Switching

```go
func TestDemoRegionSwitchBlocked(t *testing.T) {
	scenario := fullIntegrationNewDemoScenario(t)

	scenario.Command("region")

	scenario.ExpectFlashContains("region switching is disabled in demo mode")
	scenario.ExpectNoAPIError()
}
```

## Example 2: Open A Known User And Follow Related CloudTrail Events

```go
func TestLiveIAMUserToCloudTrail(t *testing.T) {
	scenario := fullIntegrationNewLiveScenario(t, "profile-name", "")
	user := fullIntegrationMustFindResourceByNameContains(t, scenario.clients, "iam-user", "bedrock")

	scenario.OpenDetailResource("iam-user", user)
	scenario.ExpectCurrentResourceType("iam-user")
	scenario.ExpectCurrentResourceID(user.ID)
	scenario.ExpectRelatedRow("CloudTrail Events")

	scenario.FollowRelated("CloudTrail Events")

	scenario.ExpectNoAPIError()
	scenario.ExpectCurrentListType("ct-events")
	scenario.ExpectFrameContains("ct-events(")
}
```

## Example 3: Filter And Sort A Resource List

```go
func TestDemoFilterAndSort(t *testing.T) {
	scenario := fullIntegrationNewDemoScenario(t)
	user := fullIntegrationMustFindAnyResource(t, scenario.clients, "iam-user")

	scenario.OpenList("iam-user")
	scenario.ApplyFilter(user.ID)
	scenario.ExpectFrameContains("iam-user(1/")
	scenario.OpenSelectedDetail()
	scenario.ExpectCurrentResourceID(user.ID)

	scenario.Back()
	scenario.Back()
	scenario.OpenList("iam-user")
	scenario.SortByID()
	scenario.OpenSelectedDetail()
}
```

Use `OpenSelectedDetail()` after `SortByName()` / `SortByID()` / `SortByAge()` when you want the test to prove which row the current selection points to.

## Example 4: Search Detail, YAML, And The RELATED Panel

```go
func TestDemoSearchFlows(t *testing.T) {
	scenario := fullIntegrationNewDemoScenario(t)
	user := fullIntegrationMustFindAnyResource(t, scenario.clients, "iam-user")

	scenario.OpenDetailResource("iam-user", user)
	scenario.ApplySearch(user.ID)
	scenario.ExpectHeaderContains("matches")

	scenario.OpenYAML()
	scenario.ApplySearch(user.ID)
	scenario.ExpectHeaderContains("matches")

	scenario.Back()
	scenario.Press("tab") // focus RELATED
	scenario.StartSearch()
	scenario.Type("CloudTrail")
	scenario.ExpectHeaderContains("/CloudTrail")
	scenario.ConfirmInput()
	scenario.ExpectViewContains("CloudTrail Events")
}
```

On detail/YAML views, `/` is full-text search. On the RELATED column, `/` filters the related rows by display name.

## Example 5: Reproduce A Pagination Bug After Region Switch

```go
func TestLiveNextTokenAfterRegionSwitch(t *testing.T) {
	scenario := fullIntegrationNewLiveScenario(t, "profile-name", "")

	scenario.OpenList("ct-events")
	scenario.LoadMore()
	scenario.ExpectNoAPIError()

	scenario.Command("region")
	scenario.ChooseRegion("eu-west-1")

	scenario.LoadMore()
	scenario.ExpectAPIErrorContains("InvalidNextTokenException")
}
```

This is the kind of repro where the current list survives the reconnect, then `m` exercises `LoadMoreMsg` against a potentially stale token.

## Example 6: Find A Concrete Event And Lock A Rendering Repro To It

```go
func TestLiveDescribeAlarmsJSONRendering(t *testing.T) {
	scenario := fullIntegrationNewLiveScenario(t, "profile-name", "")
	event := fullIntegrationMustFindResourceByFieldContains(
		t,
		scenario.clients,
		"ct-events",
		"event_name",
		"DescribeAlarms",
		fullIntegrationFindResourceOptions{},
	)

	scenario.OpenDetailResource("ct-events", event)
	scenario.ExpectCurrentResourceID(event.ID)
	scenario.ExpectNoAPIError()
	scenario.ExpectViewContains("DescribeAlarms")

	// Add more assertions here after confirming the exact bad render.
}
```

## Suggested Pattern For New Bug Reports

1. Use a discovery helper to find one concrete live resource or event.
2. Use scenario actions to reproduce the path in the fewest stable steps.
3. Assert the bad flash/API error/rendering.
4. Fix the app.
5. Keep the scenario test as the regression.
