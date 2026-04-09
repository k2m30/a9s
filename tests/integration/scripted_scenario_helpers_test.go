//go:build integration

package integration

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
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

	m, _ = fullIntegrationApplyMsg(m, clientsReady)

	return &fullIntegrationScenario{
		t:                 t,
		model:             m,
		clients:           clients,
		profile:           profile,
		region:            region,
		lastClientsReady:  &clientsReady,
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

func (s *fullIntegrationScenario) SortByName() {
	s.t.Helper()
	s.Press("N")
}

func (s *fullIntegrationScenario) SortByID() {
	s.t.Helper()
	s.Press("I")
}

func (s *fullIntegrationScenario) SortByAge() {
	s.t.Helper()
	s.Press("A")
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
	case "ctrl+r":
		return tea.KeyPressMsg{Code: '\x12', Text: "\x12"}
	case "ctrl+z":
		return tea.KeyPressMsg{Code: '\x1a', Text: "\x1a"}
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
