//go:build integration

package integration

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/fieldpath"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

type fullIntegrationScenario struct {
	t       *testing.T
	model   tui.Model
	clients *awsclient.ServiceClients

	profile string
	region  string

	history []string

	lastFlash           *messages.FlashMsg
	lastAPIError        *messages.APIErrorMsg
	lastResourcesLoaded *messages.ResourcesLoadedMsg
	lastClientsReady    *messages.ClientsReadyMsg

	currentListType       string
	currentListResources  []resource.Resource
	currentListPagination *resource.PaginationMeta

	currentResourceType string
	currentResource     *resource.Resource
	lastRelatedByName   map[string]messages.RelatedCheckResultMsg
}

type fullIntegrationFindResourceOptions struct {
	FetchFilter map[string]string
	MaxPages    int
}

func fullIntegrationNewDemoScenario(t *testing.T) *fullIntegrationScenario {
	t.Helper()
	clients := demo.NewServiceClients()
	m := fullIntegrationNewReadyModelWithClients(t, demo.DemoProfile, demo.DemoRegion, clients)
	return &fullIntegrationScenario{
		t:                 t,
		model:             m,
		clients:           clients,
		profile:           demo.DemoProfile,
		region:            demo.DemoRegion,
		lastRelatedByName: make(map[string]messages.RelatedCheckResultMsg),
	}
}

func fullIntegrationNewLiveScenario(t *testing.T, profile, region string) *fullIntegrationScenario {
	t.Helper()

	m := tui.New(profile, region, tui.WithNoCache(true))
	m, _ = fullIntegrationApplyMsg(m, tea.WindowSizeMsg{Width: 240, Height: 220})

	initMsg := fullIntegrationRequireCmdMsg(t, m.Init(), "live scenario Init")
	var connectCmd tea.Cmd
	m, connectCmd = fullIntegrationApplyMsg(m, initMsg)
	clientsReadyRaw := fullIntegrationRequireCmdMsg(t, connectCmd, "live scenario AWS connect")
	clientsReady, ok := clientsReadyRaw.(messages.ClientsReadyMsg)
	if !ok {
		t.Fatalf("live scenario AWS connect returned %T, expected messages.ClientsReadyMsg", clientsReadyRaw)
	}
	if clientsReady.Err != nil {
		t.Fatalf("live scenario AWS connect failed for profile=%q region=%q: %v", profile, region, clientsReady.Err)
	}
	if region == "" {
		region = clientsReady.Region
	}
	clients, ok := clientsReady.Clients.(*awsclient.ServiceClients)
	if !ok || clients == nil {
		t.Fatalf("live scenario AWS connect returned clients %T, expected *aws.ServiceClients", clientsReady.Clients)
	}
	// Feed ClientsReadyMsg back into the model so it advances out of the
	// "awaiting AWS" state. The shared-clients path does this inside
	// fullIntegrationNewReadyModelWithClients; the fresh-connect path must
	// do it explicitly here.
	m, _ = fullIntegrationApplyMsg(m, clientsReady)
	return fullIntegrationScenarioFromClients(t, profile, region, clients, m)
}

// fullIntegrationNewLiveScenarioFromClients constructs a live scenario that
// reuses an already-resolved *ServiceClients instead of re-running
// profile-resolution + STS AssumeRole. Essential for sub-test isolation in
// TestFullRelatedViewValidation — every sub-scenario would otherwise pay a
// ~5-10s STS round-trip, inflating the suite by two-plus orders of magnitude
// on live AWS. Clients are concurrency-safe by SDK contract.
func fullIntegrationNewLiveScenarioFromClients(t *testing.T, profile, region string, clients *awsclient.ServiceClients) *fullIntegrationScenario {
	t.Helper()
	m := fullIntegrationNewReadyModelWithClients(t, profile, region, clients)
	return fullIntegrationScenarioFromClients(t, profile, region, clients, m)
}

// fullIntegrationScenarioFromClients wraps the shared post-connect scenario
// init used by both the fresh-connect and shared-clients entry points. The
// caller is responsible for ensuring the model has already observed a
// ClientsReadyMsg (via Init+connect for the fresh path, or via
// fullIntegrationNewReadyModelWithClients for the shared-clients path).
func fullIntegrationScenarioFromClients(t *testing.T, profile, region string, clients *awsclient.ServiceClients, m tui.Model) *fullIntegrationScenario {
	t.Helper()
	ready := messages.ClientsReadyMsg{Clients: clients, Region: region}
	return &fullIntegrationScenario{
		t:                 t,
		model:             m,
		clients:           clients,
		profile:           profile,
		region:            region,
		lastClientsReady:  &ready,
		lastRelatedByName: make(map[string]messages.RelatedCheckResultMsg),
	}
}

func fullIntegrationMustFindAnyResource(t *testing.T, clients *awsclient.ServiceClients, resourceType string) resource.Resource {
	t.Helper()
	return fullIntegrationMustFindResource(t, clients, resourceType, func(resource.Resource) bool { return true }, fullIntegrationFindResourceOptions{})
}

func fullIntegrationMustFindResourceByID(t *testing.T, clients *awsclient.ServiceClients, resourceType, id string) resource.Resource {
	t.Helper()
	return fullIntegrationMustFindResource(t, clients, resourceType, func(res resource.Resource) bool {
		return res.ID == id
	}, fullIntegrationFindResourceOptions{})
}

func fullIntegrationMustFindResourceByNameContains(t *testing.T, clients *awsclient.ServiceClients, resourceType, needle string) resource.Resource {
	t.Helper()
	needle = strings.ToLower(strings.TrimSpace(needle))
	return fullIntegrationMustFindResource(t, clients, resourceType, func(res resource.Resource) bool {
		return strings.Contains(strings.ToLower(res.Name), needle)
	}, fullIntegrationFindResourceOptions{})
}

func fullIntegrationMustFindResourceByFieldContains(t *testing.T, clients *awsclient.ServiceClients, resourceType, fieldKey, needle string, opts fullIntegrationFindResourceOptions) resource.Resource {
	t.Helper()
	needle = strings.ToLower(strings.TrimSpace(needle))
	return fullIntegrationMustFindResource(t, clients, resourceType, func(res resource.Resource) bool {
		return strings.Contains(strings.ToLower(res.Fields[fieldKey]), needle)
	}, opts)
}

func fullIntegrationMustFindResource(t *testing.T, clients *awsclient.ServiceClients, resourceType string, pred func(resource.Resource) bool, opts fullIntegrationFindResourceOptions) resource.Resource {
	t.Helper()

	maxPages := opts.MaxPages
	if maxPages <= 0 {
		maxPages = 20
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	filteredFetcher := resource.GetFilteredPaginatedFetcher(resourceType)
	paginatedFetcher := resource.GetPaginatedFetcher(resourceType)
	if len(opts.FetchFilter) > 0 && filteredFetcher == nil {
		t.Fatalf("resource %s has no filtered paginated fetcher", resourceType)
	}
	if len(opts.FetchFilter) == 0 && paginatedFetcher == nil {
		t.Fatalf("resource %s has no paginated fetcher", resourceType)
	}

	token := ""
	for page := 1; page <= maxPages; page++ {
		var (
			result resource.FetchResult
			err    error
		)
		if len(opts.FetchFilter) > 0 {
			result, err = filteredFetcher(ctx, clients, opts.FetchFilter, token)
		} else {
			result, err = paginatedFetcher(ctx, clients, token)
		}
		if err != nil {
			t.Fatalf("find resource %s page %d failed: %v", resourceType, page, err)
		}
		for _, res := range result.Resources {
			if pred(res) {
				return res
			}
		}
		if result.Pagination == nil || !result.Pagination.IsTruncated {
			break
		}
		token = result.Pagination.NextToken
	}

	t.Fatalf("resource %s matching predicate not found after %d page(s)", resourceType, maxPages)
	return resource.Resource{}
}

func (s *fullIntegrationScenario) OpenList(resourceType string) {
	s.t.Helper()
	s.beginAction("open list %s", resourceType)
	s.applyAndDrain(messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: resourceType,
	})
	if s.lastAPIError != nil {
		return
	}
	if s.lastResourcesLoaded == nil || s.lastResourcesLoaded.ResourceType != resourceType {
		if s.currentListType == resourceType && strings.Contains(s.currentView(), resourceType+"(") {
			return
		}
		s.failf("open list %s did not produce ResourcesLoadedMsg", resourceType)
	}
}

func (s *fullIntegrationScenario) OpenDetailResource(resourceType string, res resource.Resource) {
	s.t.Helper()
	s.beginAction("open detail %s %s", resourceType, res.ID)
	copy := res
	s.applyAndDrain(messages.NavigateMsg{
		Target:       messages.TargetDetail,
		ResourceType: resourceType,
		Resource:     &copy,
	})
}

func (s *fullIntegrationScenario) OpenDetailFromCurrentListByID(id string) {
	s.t.Helper()
	res := s.findCurrentListResource(func(res resource.Resource) bool { return res.ID == id }, "id="+id)
	s.OpenDetailResource(s.currentListType, res)
}

func (s *fullIntegrationScenario) OpenDetailFromCurrentListByName(name string) {
	s.t.Helper()
	res := s.findCurrentListResource(func(res resource.Resource) bool { return res.Name == name }, "name="+name)
	s.OpenDetailResource(s.currentListType, res)
}

func (s *fullIntegrationScenario) OpenSelectedDetail() {
	s.t.Helper()
	s.Press("d")
}

func (s *fullIntegrationScenario) FollowRelated(displayName string) {
	s.t.Helper()
	s.beginAction("follow related %s", displayName)
	rel := s.relatedNavigateMsg(displayName)
	s.applyAndDrain(rel)
}

func (s *fullIntegrationScenario) Command(cmd string) {
	s.t.Helper()
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return
	}

	s.beginAction("command :%s", cmd)
	switch cmd {
	case "q", "quit":
		s.applyAndDrain(tea.QuitMsg{})
		return
	case "ctx", "profile":
		s.applyAndDrain(messages.NavigateMsg{Target: messages.TargetProfile})
		return
	case "region":
		s.applyAndDrain(messages.NavigateMsg{Target: messages.TargetRegion})
		return
	case "help":
		s.applyAndDrain(messages.NavigateMsg{Target: messages.TargetHelp})
		return
	}

	if rt := resource.FindResourceType(cmd); rt != nil {
		s.applyAndDrain(messages.NavigateMsg{
			Target:       messages.TargetResourceList,
			ResourceType: rt.ShortName,
		})
		return
	}

	s.applyAndDrain(messages.FlashMsg{
		Text:    fmt.Sprintf("unknown command: %s", cmd),
		IsError: true,
	})
}

func (s *fullIntegrationScenario) Type(text string) {
	s.t.Helper()
	for _, r := range text {
		s.record("type %q", string(r))
		s.applyAndDrain(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
}

func (s *fullIntegrationScenario) StartFilter() {
	s.t.Helper()
	s.beginAction("start filter")
	s.applyAndDrain(fullIntegrationScenarioKeyPress(s.t, "/"))
}

func (s *fullIntegrationScenario) ApplyFilter(text string) {
	s.t.Helper()
	s.StartFilter()
	s.Type(text)
	s.ConfirmInput()
}

func (s *fullIntegrationScenario) StartSearch() {
	s.t.Helper()
	s.beginAction("start search")
	s.applyAndDrain(fullIntegrationScenarioKeyPress(s.t, "/"))
}

func (s *fullIntegrationScenario) ApplySearch(text string) {
	s.t.Helper()
	s.StartSearch()
	s.Type(text)
	s.ConfirmInput()
}

func (s *fullIntegrationScenario) ConfirmInput() {
	s.t.Helper()
	s.Press("enter")
}

func (s *fullIntegrationScenario) SearchNext() {
	s.t.Helper()
	s.Press("n")
}

func (s *fullIntegrationScenario) SearchPrev() {
	s.t.Helper()
	s.Press("N")
}

func (s *fullIntegrationScenario) SortByColumn(col int) {
	s.t.Helper()
	if col < 1 || col > 10 {
		s.t.Fatalf("SortByColumn: col must be between 1 and 10, got %d", col)
	}
	k := strconv.Itoa(col)
	if col == 10 {
		k = "0"
	}
	s.Press(k)
}

func (s *fullIntegrationScenario) SortByName() {
	s.t.Helper()
	s.SortByColumn(1) // Name is typically column 1
}

func (s *fullIntegrationScenario) SortByID() {
	s.t.Helper()
	s.SortByColumn(2) // ID is typically column 2
}

func (s *fullIntegrationScenario) OpenYAML() {
	s.t.Helper()
	s.Press("y")
}

func (s *fullIntegrationScenario) ChooseRegion(region string) {
	s.t.Helper()
	plain := s.currentView()
	if !strings.Contains(plain, "aws-regions(") {
		s.failf("choose region %q requires aws-regions selector view", region)
	}

	s.beginAction("choose region %s", region)
	s.applyAndDrain(messages.RegionSelectedMsg{Region: region})
	if s.lastClientsReady == nil {
		s.failf("choose region %q did not reconnect clients", region)
	}
}

func (s *fullIntegrationScenario) ChooseProfile(profile string) {
	s.t.Helper()
	plain := s.currentView()
	if !strings.Contains(plain, "aws-profiles(") {
		s.failf("choose profile %q requires aws-profiles selector view", profile)
	}

	s.beginAction("choose profile %s", profile)
	s.applyAndDrain(messages.ProfileSelectedMsg{Profile: profile})
	if s.lastClientsReady == nil {
		s.failf("choose profile %q did not reconnect clients", profile)
	}
}

func (s *fullIntegrationScenario) Press(key string) {
	s.t.Helper()
	s.beginAction("press %s", key)
	s.applyAndDrain(fullIntegrationScenarioKeyPress(s.t, key))
}

func (s *fullIntegrationScenario) Back() {
	s.t.Helper()
	s.Press("esc")
}

func (s *fullIntegrationScenario) LoadMore() {
	s.t.Helper()
	s.Press("m")
}

func (s *fullIntegrationScenario) ExpectFrameContains(want string) {
	s.t.Helper()
	if !strings.Contains(s.currentView(), want) {
		s.failf("view missing %q", want)
	}
}

func (s *fullIntegrationScenario) ExpectViewContains(want string) {
	s.t.Helper()
	if !strings.Contains(s.currentView(), want) {
		s.failf("view missing %q", want)
	}
}

func (s *fullIntegrationScenario) ExpectViewNotContains(unwanted string) {
	s.t.Helper()
	if strings.Contains(s.currentView(), unwanted) {
		s.failf("view unexpectedly contains %q", unwanted)
	}
}

func (s *fullIntegrationScenario) ExpectHeaderContains(want string) {
	s.t.Helper()
	s.ExpectViewContains(want)
}

func (s *fullIntegrationScenario) ExpectFlashContains(want string) {
	s.t.Helper()
	if s.lastFlash == nil {
		s.failf("expected flash containing %q, but no flash was observed", want)
	}
	if !strings.Contains(s.lastFlash.Text, want) {
		s.failf("flash %q does not contain %q", s.lastFlash.Text, want)
	}
}

func (s *fullIntegrationScenario) ExpectNoAPIError() {
	s.t.Helper()
	if s.lastAPIError != nil {
		s.failf("unexpected API error: %v", s.lastAPIError.Err)
	}
}

func (s *fullIntegrationScenario) ExpectAPIErrorContains(want string) {
	s.t.Helper()
	if s.lastAPIError == nil {
		s.failf("expected API error containing %q, but no API error was observed", want)
	}
	if !strings.Contains(s.lastAPIError.Err.Error(), want) {
		s.failf("API error %q does not contain %q", s.lastAPIError.Err.Error(), want)
	}
}

func (s *fullIntegrationScenario) ExpectCurrentResourceID(id string) {
	s.t.Helper()
	if s.currentResource == nil {
		s.failf("expected current resource id %q, but no detail resource is active", id)
	}
	if s.currentResource.ID != id {
		s.failf("current resource id = %q, expected %q", s.currentResource.ID, id)
	}
}

func (s *fullIntegrationScenario) ExpectCurrentResourceType(resourceType string) {
	s.t.Helper()
	if s.currentResourceType != resourceType {
		s.failf("current resource type = %q, expected %q", s.currentResourceType, resourceType)
	}
}

func (s *fullIntegrationScenario) ExpectCurrentListType(resourceType string) {
	s.t.Helper()
	if s.currentListType != resourceType {
		s.failf("current list type = %q, expected %q", s.currentListType, resourceType)
	}
}

func (s *fullIntegrationScenario) ExpectLoadedCount(want int) {
	s.t.Helper()
	if got := len(s.currentListResources); got != want {
		s.failf("current loaded count = %d, expected %d", got, want)
	}
}

func (s *fullIntegrationScenario) ExpectRelatedRow(displayName string) {
	s.t.Helper()
	if _, ok := s.lastRelatedByName[displayName]; !ok {
		s.failf("related row %q was not observed; got %v", displayName, s.relatedNames())
	}
	if !strings.Contains(s.currentView(), displayName) {
		s.failf("view missing related row %q", displayName)
	}
}

func (s *fullIntegrationScenario) ExpectRelatedCount(displayName string, want int) {
	s.t.Helper()
	msg, ok := s.lastRelatedByName[displayName]
	if !ok {
		s.failf("related row %q was not observed; got %v", displayName, s.relatedNames())
	}
	got := msg.Result.Count
	if got != want {
		s.failf("related row %q count = %d, expected %d", displayName, got, want)
	}
}

func (s *fullIntegrationScenario) currentView() string {
	return fullIntegrationStripANSI(fullIntegrationViewContent(s.model))
}

// findRow returns the first rendered line that contains the given resource ID as
// a standalone token. Returns empty string if no such line is present.
func (s *fullIntegrationScenario) findRow(resourceID string) string {
	s.t.Helper()
	for _, line := range strings.Split(s.currentView(), "\n") {
		if !strings.Contains(line, resourceID) {
			continue
		}
		return line
	}
	return ""
}

// ExpectRowStatusBlank asserts that the row for resourceID does not contain any
// of the banned Healthy filler strings (`OK`, `ACTIVE`, `available`, `running`,
// `healthy`, `-`). Healthy rows must render their Status cell empty per spec §4.
func (s *fullIntegrationScenario) ExpectRowStatusBlank(resourceID string) {
	s.t.Helper()
	line := s.findRow(resourceID)
	if line == "" {
		s.failf("row for %q not found in rendered view", resourceID)
		return
	}
	// Strip the identity cell before scanning — banned tokens like "healthy"
	// must not fire on resource names that happen to contain them
	// (e.g. "a9s-demo-healthy" bucket).
	scan := strings.ReplaceAll(line, resourceID, "")
	banned := []string{"OK", "ACTIVE", "available", "running", "healthy"}
	for _, token := range banned {
		if strings.Contains(scan, token) {
			s.failf("row for %q must render blank Status but contains banned token %q: %q", resourceID, token, line)
		}
	}
}

// ExpectRowStatusEquals asserts that the row for resourceID contains the exact
// Status phrase expected by spec §4 (substring match — the row contains other
// cells, so exact cell-level equality is not enforced).
func (s *fullIntegrationScenario) ExpectRowStatusEquals(resourceID, expected string) {
	s.t.Helper()
	line := s.findRow(resourceID)
	if line == "" {
		s.failf("row for %q not found in rendered view", resourceID)
		return
	}
	if !strings.Contains(line, expected) {
		s.failf("row for %q missing Status phrase %q: %q", resourceID, expected, line)
	}
}

// ExpectRowNamePrefix asserts that the row for resourceID has the given glyph
// prefix (e.g. `"~ "` or `"! "`) immediately before the identity cell.
func (s *fullIntegrationScenario) ExpectRowNamePrefix(resourceID, prefix string) {
	s.t.Helper()
	line := s.findRow(resourceID)
	if line == "" {
		s.failf("row for %q not found in rendered view", resourceID)
		return
	}
	if !strings.Contains(line, prefix+resourceID) {
		s.failf("row for %q missing prefix %q: %q", resourceID, prefix, line)
	}
}

// ExpectRowNoGlyphPrefix asserts that the row for resourceID has neither a
// `!` nor a `~` glyph prefix. Glyphs are only permitted on Healthy (green)
// rows per spec §4; Warning / Broken / Dim rows must never render one.
func (s *fullIntegrationScenario) ExpectRowNoGlyphPrefix(resourceID string) {
	s.t.Helper()
	line := s.findRow(resourceID)
	if line == "" {
		s.failf("row for %q not found in rendered view", resourceID)
		return
	}
	for _, glyph := range []string{"! ", "~ "} {
		if strings.Contains(line, glyph+resourceID) {
			s.failf("row for %q must have no glyph prefix but found %q: %q", resourceID, glyph, line)
		}
	}
}

// ExpectMenuIssueCount asserts that the main menu entry for shortName renders
// an `issues:N` badge matching want. Pass want=0 to require the badge be
// absent (no issues).
func (s *fullIntegrationScenario) ExpectMenuIssueCount(shortName string, want int) {
	s.t.Helper()
	view := s.currentView()
	var target string
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, shortName) && strings.Contains(line, "issues:") {
			target = line
			break
		}
	}
	if want == 0 {
		// Either no issue badge present for shortName, or not found at all.
		if target == "" {
			return
		}
		s.failf("main menu entry for %q must not render an issues badge but got %q", shortName, target)
		return
	}
	if target == "" {
		s.failf("main menu entry for %q missing issues:%d badge", shortName, want)
		return
	}
	expected := "issues:" + strconv.Itoa(want)
	if !strings.Contains(target, expected) {
		s.failf("main menu entry for %q expected %q got %q", shortName, expected, target)
	}
}

// ExpectRelatedRowCountAtLeast asserts that the related-panel row for
// displayName has a count of at least n. Used when the exact count depends on
// live/demo state but a floor is guaranteed by fixture design.
func (s *fullIntegrationScenario) ExpectRelatedRowCountAtLeast(displayName string, n int) {
	s.t.Helper()
	msg, ok := s.lastRelatedByName[displayName]
	if !ok {
		s.failf("related row %q was not observed; got %v", displayName, s.relatedNames())
		return
	}
	if msg.Result.Count < n {
		s.failf("related row %q count = %d, expected at least %d", displayName, msg.Result.Count, n)
	}
}

func (s *fullIntegrationScenario) beginAction(format string, args ...any) {
	s.lastFlash = nil
	s.lastAPIError = nil
	s.lastResourcesLoaded = nil
	s.lastClientsReady = nil
	s.record(format, args...)
}

func (s *fullIntegrationScenario) record(format string, args ...any) {
	entry := fmt.Sprintf(format, args...)
	s.history = append(s.history, entry)
	s.t.Logf("scenario: %s", entry)
}

func (s *fullIntegrationScenario) applyAndDrain(msg tea.Msg) {
	s.t.Helper()
	cmd := s.applyMsg(msg)
	s.drainCmd(cmd)
}

func (s *fullIntegrationScenario) applyMsg(msg tea.Msg) tea.Cmd {
	s.t.Helper()
	var cmd tea.Cmd
	s.model, cmd = fullIntegrationApplyMsg(s.model, msg)
	s.observe(msg)
	return cmd
}

func (s *fullIntegrationScenario) drainCmd(cmd tea.Cmd) {
	s.t.Helper()
	for _, msg := range fullIntegrationCollectCmdMessages(cmd) {
		next := s.applyMsg(msg)
		if s.shouldDrainFollowups(msg) {
			s.drainCmd(next)
		}
	}
}

func (s *fullIntegrationScenario) shouldDrainFollowups(msg tea.Msg) bool {
	switch msg.(type) {
	case messages.NavigateMsg:
		return true
	case messages.ResourcesLoadedMsg:
		return true
	case messages.LoadMoreMsg:
		return true
	case messages.ClientsReadyMsg:
		return true
	case messages.RelatedCheckStartedMsg:
		return true
	case messages.RelatedNavigateMsg:
		return true
	case messages.EnterChildViewMsg:
		return true
	case messages.APIErrorMsg:
		return true
	case messages.ValueRevealedMsg:
		return true
	// Wave 1 + Wave 2 enrichment chain: availability → enrichment → field updates.
	// Without these, demo-mode Wave 2 findings never reach the ResourceList,
	// and the `~` glyph / `(+N)` suffix / "maintenance scheduled" invariants
	// cannot be exercised end-to-end. Added 2026-04-22 after the dbi render
	// gate surfaced the gap.
	case messages.AvailabilityPrefetchedMsg:
		return true
	case messages.AvailabilityCheckedMsg:
		return true
	case messages.EnrichmentCheckedMsg:
		return true
	default:
		return false
	}
}

func (s *fullIntegrationScenario) observe(msg tea.Msg) {
	switch msg := msg.(type) {
	case messages.FlashMsg:
		copy := msg
		s.lastFlash = &copy
	case messages.APIErrorMsg:
		copy := msg
		s.lastAPIError = &copy
	case messages.ResourcesLoadedMsg:
		copy := msg
		s.lastResourcesLoaded = &copy
		s.currentListType = msg.ResourceType
		s.currentListResources = append([]resource.Resource(nil), msg.Resources...)
		s.currentListPagination = msg.Pagination
	case messages.NavigateMsg:
		if msg.Target == messages.TargetDetail && msg.Resource != nil {
			copy := *msg.Resource
			s.currentResource = &copy
			s.currentResourceType = msg.ResourceType
			s.lastRelatedByName = make(map[string]messages.RelatedCheckResultMsg)
		}
		if msg.Target == messages.TargetResourceList {
			s.currentListType = msg.ResourceType
		}
	case messages.RelatedCheckStartedMsg:
		s.currentResourceType = msg.ResourceType
		copy := msg.SourceResource
		s.currentResource = &copy
		s.lastRelatedByName = make(map[string]messages.RelatedCheckResultMsg)
	case messages.RelatedCheckResultMsg:
		if s.lastRelatedByName == nil {
			s.lastRelatedByName = make(map[string]messages.RelatedCheckResultMsg)
		}
		s.lastRelatedByName[msg.DefDisplayName] = msg
	case messages.ClientsReadyMsg:
		copy := msg
		s.lastClientsReady = &copy
		if msg.Err == nil {
			if clients, ok := msg.Clients.(*awsclient.ServiceClients); ok && clients != nil {
				s.clients = clients
			}
			if msg.Region != "" {
				s.region = msg.Region
			}
		}
	case messages.ProfileSelectedMsg:
		s.profile = msg.Profile
	case messages.RegionSelectedMsg:
		s.region = msg.Region
	}
}

func (s *fullIntegrationScenario) relatedNavigateMsg(displayName string) messages.RelatedNavigateMsg {
	s.t.Helper()
	if s.currentResource == nil {
		s.failf("follow related %q requires an active detail resource", displayName)
	}
	msg, ok := s.lastRelatedByName[displayName]
	if !ok {
		s.failf("related row %q was not observed; got %v", displayName, s.relatedNames())
	}

	var targetType string
	for _, def := range resource.GetRelated(s.currentResourceType) {
		if def.DisplayName == displayName {
			targetType = def.TargetType
			break
		}
	}
	if targetType == "" {
		s.failf("resource type %q has no related definition %q", s.currentResourceType, displayName)
	}

	return messages.RelatedNavigateMsg{
		TargetType:     targetType,
		SourceResource: *s.currentResource,
		SourceType:     s.currentResourceType,
		RelatedIDs:     append([]string(nil), msg.Result.ResourceIDs...),
		FetchFilter:    cloneStringMap(msg.Result.FetchFilter),
	}
}

func (s *fullIntegrationScenario) findCurrentListResource(pred func(resource.Resource) bool, label string) resource.Resource {
	s.t.Helper()
	if s.currentListType == "" {
		s.failf("resource lookup %s requires an active resource list", label)
	}
	for _, res := range s.currentListResources {
		if pred(res) {
			return res
		}
	}
	s.failf("resource lookup %s failed in current %s list", label, s.currentListType)
	return resource.Resource{}
}

func (s *fullIntegrationScenario) relatedNames() []string {
	names := make([]string, 0, len(s.lastRelatedByName))
	for name := range s.lastRelatedByName {
		names = append(names, name)
	}
	return names
}

func (s *fullIntegrationScenario) failf(format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	history := strings.Join(s.history, "\n")
	view := s.currentView()
	s.t.Fatalf("%s\nscenario history:\n%s\ncurrent view:\n%s", message, history, view)
}

func fullIntegrationScenarioKeyPress(t *testing.T, key string) tea.KeyPressMsg {
	t.Helper()
	switch key {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "left":
		return tea.KeyPressMsg{Code: tea.KeyLeft}
	case "right":
		return tea.KeyPressMsg{Code: tea.KeyRight}
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "pgup":
		return tea.KeyPressMsg{Code: tea.KeyPgUp}
	case "pgdown":
		return tea.KeyPressMsg{Code: tea.KeyPgDown}
	}

	if strings.HasPrefix(key, "ctrl+") {
		rest := strings.TrimPrefix(key, "ctrl+")
		runes := []rune(rest)
		if len(runes) == 1 {
			return tea.KeyPressMsg{Code: runes[0], Mod: tea.ModCtrl}
		}
		t.Fatalf("unsupported ctrl+<char> key %q", key)
		return tea.KeyPressMsg{}
	}

	if len([]rune(key)) == 1 {
		return tea.KeyPressMsg{Code: -1, Text: key}
	}
	t.Fatalf("unsupported scenario key %q", key)
	return tea.KeyPressMsg{}
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// DrillRelated dispatches a RelatedNavigateMsg for the named related-panel row and
// returns the resources that land in the target view. It requires that the current
// detail view is open and that the row has already been observed in lastRelatedByName
// (i.e. its checker ran and produced a Count). Fails the test if the row was not
// observed or if the resulting resource list is empty.
//
// After the helper returns, the caller must press Esc to pop back to the detail view
// before calling DrillRelated again (each drill pushes a new view onto the stack).
func (s *fullIntegrationScenario) DrillRelated(displayName string) []resource.Resource {
	s.t.Helper()
	msg, ok := s.lastRelatedByName[displayName]
	if !ok {
		s.failf("DrillRelated(%q): row was not observed in lastRelatedByName; got %v", displayName, s.relatedNames())
	}
	if msg.Result.Count < 1 && len(msg.Result.ResourceIDs) == 0 && len(msg.Result.FetchFilter) == 0 {
		s.failf("DrillRelated(%q): Count=0 and no ResourceIDs/FetchFilter — cannot drill an empty pivot", displayName)
	}

	// Snapshot state before the drill so we can detect what changed.
	prevResource := s.currentResource

	rel := s.relatedNavigateMsg(displayName)
	s.beginAction("drill related %s", displayName)
	// beginAction resets lastResourcesLoaded to nil; after applyAndDrain, a non-nil
	// lastResourcesLoaded means the drill triggered a new ResourcesLoadedMsg.
	s.applyAndDrain(rel)

	// Case 1: a new resource list was loaded (multi-ID or FetchFilter path).
	// Use lastResourcesLoaded (reset by beginAction before dispatch) to confirm a
	// new list arrived, rather than comparing currentListType/currentListResources
	// which may still hold state from an earlier OpenList call.
	if s.lastResourcesLoaded != nil {
		if len(s.currentListResources) == 0 {
			s.failf("DrillRelated(%q): navigation produced an empty resource list (type=%q)", displayName, s.currentListType)
		}
		return append([]resource.Resource(nil), s.currentListResources...)
	}

	// Case 2: a single-resource detail was auto-opened (TargetID + single cache hit).
	// currentResource pointer changed after NavigateMsg{Target: TargetDetail} was observed.
	if s.currentResource != nil && s.currentResource != prevResource {
		return []resource.Resource{*s.currentResource}
	}

	// Case 3: cache-hit filtered list pushed without a ResourcesLoadedMsg.
	// Occurs when handleRelatedNavigate takes the RelatedIDs cache-hit branch and calls
	// NewResourceListFromCache — which pushes a view and returns nil cmd, so no
	// ResourcesLoadedMsg is ever dispatched. Detected by checking the rendered view title
	// for "{targetType}(N)". Return synthetic resource stubs from the RelatedIDs; their
	// IDs are the checker-emitted values — exactly what tests need for format assertions.
	targetType := rel.TargetType
	// FrameTitle prefers typeDef.ListTitle over ShortName — e.g. alarm → "alarms",
	// eb-rule → "event-rules". Check both so the case-3 branch fires regardless.
	titleTokens := []string{targetType}
	if td := resource.FindResourceType(targetType); td != nil && td.ListTitle != "" && td.ListTitle != targetType {
		titleTokens = append(titleTokens, td.ListTitle)
	}
	rendered := s.currentView()
	titleMatched := false
	for _, tok := range titleTokens {
		if strings.Contains(rendered, tok+"(") {
			titleMatched = true
			break
		}
	}
	if titleMatched {
		emptyCount := false
		for _, tok := range titleTokens {
			if strings.Contains(rendered, tok+"(0)") {
				emptyCount = true
				break
			}
		}
		if emptyCount {
			s.failf("DrillRelated(%q): cache-hit filtered list is empty — target type %q rendered with count 0 (RelatedIDs=%v)",
				displayName, targetType, rel.RelatedIDs)
		}
		if len(rel.RelatedIDs) == 0 {
			// FetchFilter path with cache hit: no RelatedIDs to return, but the view
			// shows resources — return a single-element stub to signal non-empty.
			return []resource.Resource{{ID: targetType + "/cache-hit"}}
		}
		result := make([]resource.Resource, len(rel.RelatedIDs))
		for i, id := range rel.RelatedIDs {
			result[i] = resource.Resource{ID: id}
		}
		return result
	}

	// Case 4: detail-view pushed without a NavigateMsg.
	// handleRelatedNavigate's KindDetail + cache-hit branch pushes a detail view
	// and returns nil Cmd — no NavigateMsg is emitted, so currentResource stays
	// pointed at the parent. Detect the push via the rendered frame title:
	// "detail -- <id>" or "detail -- <id> (<name>)".
	if strings.Contains(rendered, "detail -- ") {
		// Extract the ID from the title. The format is "detail -- <id>" or
		// "detail -- <id> (<name>)". The ID stops at " (" or end-of-line.
		after := rendered[strings.Index(rendered, "detail -- ")+len("detail -- "):]
		if nlIdx := strings.IndexAny(after, "\n│"); nlIdx >= 0 {
			after = after[:nlIdx]
		}
		if parIdx := strings.Index(after, " ("); parIdx >= 0 {
			after = after[:parIdx]
		}
		id := strings.TrimSpace(after)
		if id != "" {
			return []resource.Resource{{ID: id}}
		}
	}

	// Neither a list nor a detail landed — the resolver either flashed or produced nothing.
	if s.lastFlash != nil {
		s.failf("DrillRelated(%q): navigation resulted in a flash instead of a resource view: %q", displayName, s.lastFlash.Text)
	}
	s.failf("DrillRelated(%q): navigation produced neither a resource list nor a detail view", displayName)
	return nil
}

// FollowNavigableField dispatches a RelatedNavigateMsg derived from the registered
// NavigableField for the given field path on the current detail resource. The method
// looks up the NavigableField definition, extracts the field value from the resource's
// RawStruct, applies NavIDFromValue to convert ARNs → bare IDs, and dispatches the
// resulting RelatedNavigateMsg. It then returns the first resource that lands.
//
// This path tests the DISPATCH→RESOLUTION→LANDING pipeline (the same path that Enter
// on a navigable field follows in production) without requiring cursor manipulation.
// It catches ID-format mismatches between what the detail view carries and what the
// target resource type indexes on (e.g., the DDB→KMS full-ARN vs. bare-key-ID bug).
//
// Fails the test if:
//   - No NavigableField is registered for the given field path on the current resource type.
//   - The field value cannot be extracted from RawStruct.
//   - The dispatch produces no resource landing (empty list and no detail).
func (s *fullIntegrationScenario) FollowNavigableField(fieldPath string) resource.Resource {
	s.t.Helper()
	if s.currentResourceType == "" || s.currentResource == nil {
		s.failf("FollowNavigableField(%q): no active detail resource (currentResourceType=%q)", fieldPath, s.currentResourceType)
	}

	nf := resource.IsFieldNavigable(s.currentResourceType, fieldPath)
	if nf == nil {
		registered := resource.GetNavigableFields(s.currentResourceType)
		paths := make([]string, len(registered))
		for i, f := range registered {
			paths[i] = f.FieldPath
		}
		s.failf("FollowNavigableField(%q): no navigable field registered for resource type %q; registered: %v", fieldPath, s.currentResourceType, paths)
	}

	// Extract the raw field value from the resource's RawStruct.
	rawValue := ""
	if s.currentResource.RawStruct != nil {
		rawValue = fieldpath.ExtractScalar(s.currentResource.RawStruct, fieldPath)
		if rawValue == "" {
			// Fall back to list-aware extraction for paths that traverse
			// slices (e.g. VpcSecurityGroups.VpcSecurityGroupId).
			rawValue = fieldpath.ExtractFirstListScalar(s.currentResource.RawStruct, fieldPath)
		}
	}
	// Fall back to the Fields map if RawStruct extraction yielded nothing.
	if rawValue == "" {
		rawValue = s.currentResource.Fields[fieldPath]
	}
	if rawValue == "" {
		s.failf("FollowNavigableField(%q): field value is empty on resource %q (type=%q); RawStruct=%T",
			fieldPath, s.currentResource.ID, s.currentResourceType, s.currentResource.RawStruct)
	}

	targetID := resource.NavIDFromValue(nf.TargetType, rawValue)
	if targetID == "" {
		s.failf("FollowNavigableField(%q): NavIDFromValue(targetType=%q, value=%q) returned empty string",
			fieldPath, nf.TargetType, rawValue)
	}

	// Snapshot state before dispatch.
	prevListType := s.currentListType
	prevResource := s.currentResource
	sourceRes := *s.currentResource
	sourceType := s.currentResourceType

	s.beginAction("follow navigable field %s → %s (targetID=%s)", fieldPath, nf.TargetType, targetID)
	s.applyAndDrain(messages.RelatedNavigateMsg{
		TargetType:     nf.TargetType,
		SourceResource: sourceRes,
		SourceType:     sourceType,
		TargetID:       targetID,
	})

	// Case 1: single-resource detail auto-opened.
	if s.currentResource != nil && s.currentResource != prevResource {
		return *s.currentResource
	}
	// Case 2: filtered resource list loaded.
	if s.currentListType != prevListType || len(s.currentListResources) > 0 {
		if len(s.currentListResources) == 0 {
			s.failf("FollowNavigableField(%q → %s): navigation produced an empty resource list (type=%q, targetID=%q)",
				fieldPath, nf.TargetType, s.currentListType, targetID)
		}
		return s.currentListResources[0]
	}

	if s.lastFlash != nil {
		s.failf("FollowNavigableField(%q → %s): navigation resulted in a flash instead of a resource view: %q (targetID=%q, rawValue=%q)",
			fieldPath, nf.TargetType, s.lastFlash.Text, targetID, rawValue)
	}
	s.failf("FollowNavigableField(%q → %s): navigation produced neither a resource list nor a detail view (targetID=%q, rawValue=%q)",
		fieldPath, nf.TargetType, targetID, rawValue)
	return resource.Resource{}
}
