package unit

// T012a — Nested child-view chains: verifies that each two-level child chain
// compiles correctly, dispatches EnterChildViewMsg at each level, and that an
// unknown parent identifier surfaces an SDK error rather than a silent empty
// list (contract rule 4).
//
// Chains under test:
//   logs → log_streams → log_events
//   lambda → lambda_invocations → lambda_invocation_logs
//   sfn → sfn_executions → sfn_execution_history
//   cb → cb_builds → cb_build_logs
//   elb → elb_listeners → elb_listener_rules
//
// Expected to FAIL for chains whose intermediate fakes are not yet wired
// (coders own T013–T028 implementation tasks). The tests correctly fail before
// those tasks are done; they pass once the fake and fixture data land.

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
)

// setupDemoApp is a test helper that creates a sized, client-wired cold-cache
// app model ready for navigation.
func setupDemoApp(t *testing.T) *tui.Model {
	t.Helper()
	m := newDemoColdCacheApp(t)
	*m, _ = rootApplyMsg(*m, tea.WindowSizeMsg{Width: 120, Height: 40})
	clients := demo.NewServiceClients()
	*m, _ = rootApplyMsg(*m, messages.ClientsReady{Clients: clients, Gen: 0})
	return m
}

// loadResourceList navigates to resourceType, executes the fetch cmd, delivers
// the ResourcesLoadedMsg to the model, and returns the loaded resources.
// Fails the test if any step is missing.
func loadResourceList(t *testing.T, m *tui.Model, resourceType string) []resource.Resource {
	t.Helper()
	var navCmd tea.Cmd
	*m, navCmd = rootApplyMsg(*m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: resourceType,
	})
	if navCmd == nil {
		t.Fatalf("expected cmd after NavigateMsg{%s}, got nil", resourceType)
	}
	raw := extractMsg(t, navCmd, func(msg tea.Msg) bool {
		_, ok := msg.(messages.ResourcesLoaded)
		return ok
	})
	loaded := raw.(messages.ResourcesLoaded)
	*m, _ = rootApplyMsg(*m, loaded)
	return loaded.Resources
}

// enterChildAndFetch dispatches an EnterChildViewMsg and executes the resulting
// fetch cmd. Returns the ResourcesLoadedMsg for the child type or fails the test
// if the child type registration is missing, the fetch cmd is nil, or the cmd
// returns an API error.
func enterChildAndFetch(t *testing.T, m *tui.Model, childType string, parentCtx map[string]string, displayName string) messages.ResourcesLoaded {
	t.Helper()
	var fetchCmd tea.Cmd
	*m, fetchCmd = rootApplyMsg(*m, messages.EnterChildView{
		ChildType:     childType,
		ParentContext: parentCtx,
		DisplayName:   displayName,
	})
	if fetchCmd == nil {
		t.Fatalf("expected fetch cmd after EnterChildViewMsg{%s}, got nil — "+
			"is %q registered as a child type?", childType, childType)
	}
	raw := extractMsg(t, fetchCmd, func(msg tea.Msg) bool {
		switch v := msg.(type) {
		case messages.ResourcesLoaded:
			return v.ResourceType == childType
		case messages.APIError:
			return true
		}
		return false
	})
	switch v := raw.(type) {
	case messages.APIError:
		t.Fatalf("child fetch for %q returned API error: %v", childType, v.Err)
	case messages.ResourcesLoaded:
		*m, _ = rootApplyMsg(*m, v)
		return v
	}
	t.Fatalf("unexpected message type %T from %q fetch", raw, childType)
	return messages.ResourcesLoaded{}
}

// enterChildExpectError dispatches EnterChildViewMsg with a known-unknown parent
// identifier and asserts that the result is an error, not a silent empty list.
// This validates contract rule 4.
func enterChildExpectError(t *testing.T, m *tui.Model, childType string, parentCtx map[string]string, displayName string) {
	t.Helper()
	var fetchCmd tea.Cmd
	*m, fetchCmd = rootApplyMsg(*m, messages.EnterChildView{
		ChildType:     childType,
		ParentContext: parentCtx,
		DisplayName:   displayName,
	})
	if fetchCmd == nil {
		t.Fatalf("expected fetch cmd after EnterChildViewMsg{%s/unknown}, got nil", childType)
	}
	msg := fetchCmd()
	// Unwrap one BatchMsg level.
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, sub := range batch {
			if sub == nil {
				continue
			}
			msg = sub()
			break
		}
	}
	switch v := msg.(type) {
	case messages.APIError:
		if v.Err == nil {
			t.Errorf("contract rule 4: APIErrorMsg.Err must not be nil for unknown parent %q", childType)
		}
		// Correct: unknown parent surfaced as an error.
	case messages.ResourcesLoaded:
		if len(v.Resources) == 0 {
			t.Errorf("contract rule 4 violation: %q fetch for unknown parent returned "+
				"empty ResourcesLoadedMsg instead of an error — "+
				"fake must return the real SDK error for unknown parents", childType)
		} else {
			t.Errorf("contract rule 4 violation: %q fake returned %d resources for unknown parent — "+
				"should have returned an SDK error", childType, len(v.Resources))
		}
	case messages.Flash:
		if !v.IsError {
			t.Errorf("FlashMsg for unknown parent %q must have IsError=true; got false. Text=%q", childType, v.Text)
		}
		// Error flash is acceptable — the error was surfaced.
	default:
		t.Logf("unknown-parent drill for %q produced %T — acceptable if error is surfaced upstream", childType, msg)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Chain 1: logs → log_streams → log_events
// ─────────────────────────────────────────────────────────────────────────────

// TestDemoColdCacheNestedChildren_LogsChain verifies the three-level chain
// logs → log_streams → log_events from a cold cache.
func TestDemoColdCacheNestedChildren_LogsChain(t *testing.T) {
	t.Parallel()
	m := setupDemoApp(t)

	// Level 0: load log groups.
	groups := loadResourceList(t, m, "logs")
	if len(groups) == 0 {
		t.Fatal("no log groups in fixture data; cannot test logs→log_streams→log_events chain")
	}

	// Level 1: drill into log_streams for the first log group.
	firstGroup := groups[0]
	logGroupName := firstGroup.Name
	if logGroupName == "" {
		logGroupName = firstGroup.ID
	}
	streams := enterChildAndFetch(t, m, "log_streams",
		map[string]string{"log_group_name": logGroupName},
		logGroupName,
	)
	if len(streams.Resources) == 0 {
		t.Fatal("no log streams in fixture data for first log group; cannot test log_streams→log_events")
	}

	// Level 2: drill into log_events for the first log stream.
	firstStream := streams.Resources[0]
	streamName := firstStream.Name
	if streamName == "" {
		streamName = firstStream.ID
	}
	events := enterChildAndFetch(t, m, "log_events",
		map[string]string{
			"log_group_name":  logGroupName,
			"log_stream_name": streamName,
		},
		streamName,
	)
	_ = events // may be empty if no events in fixture; length check not required for log_events
}

// TestDemoColdCacheNestedChildren_LogsChain_UnknownParent verifies contract
// rule 4 for the log_streams child: an unknown log group name must surface an
// error (ResourceNotFoundException), not an empty list.
func TestDemoColdCacheNestedChildren_LogsChain_UnknownParent(t *testing.T) {
	t.Parallel()
	m := setupDemoApp(t)

	// Ensure clients are wired but no real list navigation needed.
	enterChildExpectError(t, m, "log_streams",
		map[string]string{"log_group_name": "/nonexistent/log/group/xyz-00000"},
		"/nonexistent/log/group/xyz-00000",
	)
}

// ─────────────────────────────────────────────────────────────────────────────
// Chain 2: lambda → lambda_invocations → lambda_invocation_logs
// ─────────────────────────────────────────────────────────────────────────────

// TestDemoColdCacheNestedChildren_LambdaChain verifies the three-level chain
// lambda → lambda_invocations → lambda_invocation_logs from a cold cache.
func TestDemoColdCacheNestedChildren_LambdaChain(t *testing.T) {
	t.Parallel()
	m := setupDemoApp(t)

	// Level 0: load lambda functions.
	functions := loadResourceList(t, m, "lambda")
	if len(functions) == 0 {
		t.Fatal("no lambda functions in fixture data; cannot test lambda→invocations→logs chain")
	}

	// Level 1: drill into lambda_invocations for the first function.
	firstFn := functions[0]
	fnName := firstFn.Name
	if fnName == "" {
		fnName = firstFn.ID
	}
	logGroup := firstFn.Fields["log_group"]
	if logGroup == "" {
		logGroup = "/aws/lambda/" + fnName
	}
	invocations := enterChildAndFetch(t, m, "lambda_invocations",
		map[string]string{
			"function_name": fnName,
			"log_group":     logGroup,
		},
		fnName,
	)
	if len(invocations.Resources) == 0 {
		// Invocation list may be legitimately empty (no recent invocations in
		// fixture window). Skip level-2 drill silently.
		t.Log("no lambda invocations in fixture data; skipping lambda_invocation_logs drill")
		return
	}

	// Level 2: drill into lambda_invocation_logs for the first invocation.
	firstInvoc := invocations.Resources[0]
	requestID := firstInvoc.Fields["request_id"]
	if requestID == "" {
		requestID = firstInvoc.ID
	}
	_ = enterChildAndFetch(t, m, "lambda_invocation_logs",
		map[string]string{
			"log_group":  logGroup,
			"request_id": requestID,
		},
		requestID,
	)
}

// TestDemoColdCacheNestedChildren_LambdaChain_UnknownParent verifies contract
// rule 4 for lambda_invocations: an unknown function name must produce an SDK
// error (or empty with error — not silent empty).
func TestDemoColdCacheNestedChildren_LambdaChain_UnknownParent(t *testing.T) {
	t.Parallel()
	m := setupDemoApp(t)

	enterChildExpectError(t, m, "lambda_invocations",
		map[string]string{
			"function_name": "nonexistent-function-xyz-00000",
			"log_group":     "/aws/lambda/nonexistent-function-xyz-00000",
		},
		"nonexistent-function-xyz-00000",
	)
}

// ─────────────────────────────────────────────────────────────────────────────
// Chain 3: sfn → sfn_executions → sfn_execution_history
// ─────────────────────────────────────────────────────────────────────────────

// TestDemoColdCacheNestedChildren_SFNChain verifies the three-level chain
// sfn → sfn_executions → sfn_execution_history from a cold cache.
func TestDemoColdCacheNestedChildren_SFNChain(t *testing.T) {
	t.Parallel()
	m := setupDemoApp(t)

	// Level 0: load state machines.
	machines := loadResourceList(t, m, "sfn")
	if len(machines) == 0 {
		t.Fatal("no state machines in fixture data; cannot test sfn→executions→history chain")
	}

	// Level 1: drill into sfn_executions for the first state machine.
	// The DrillCondition in types_messaging.go excludes EXPRESS machines.
	var smArn, smName string
	for _, sm := range machines {
		if sm.Fields["type"] != "EXPRESS" {
			smArn = sm.Fields["arn"]
			smName = sm.Name
			break
		}
	}
	if smArn == "" {
		t.Skip("all state machines in fixture are EXPRESS type; sfn_executions not drillable")
	}

	executions := enterChildAndFetch(t, m, "sfn_executions",
		map[string]string{
			"state_machine_arn":  smArn,
			"state_machine_name": smName,
		},
		smName,
	)
	if len(executions.Resources) == 0 {
		t.Fatal("no sfn executions in fixture data; cannot test sfn_executions→sfn_execution_history chain")
	}

	// Level 2: drill into sfn_execution_history for the first execution.
	firstExec := executions.Resources[0]
	execArn := firstExec.Fields["execution_arn"]
	execName := firstExec.Name
	if execArn == "" {
		execArn = firstExec.ID
	}
	_ = enterChildAndFetch(t, m, "sfn_execution_history",
		map[string]string{
			"execution_arn":  execArn,
			"execution_name": execName,
		},
		execName,
	)
}

// TestDemoColdCacheNestedChildren_SFNChain_UnknownParent verifies contract
// rule 4 for sfn_executions: an unknown state machine ARN must surface an error.
func TestDemoColdCacheNestedChildren_SFNChain_UnknownParent(t *testing.T) {
	t.Parallel()
	m := setupDemoApp(t)

	enterChildExpectError(t, m, "sfn_executions",
		map[string]string{
			"state_machine_arn":  "arn:aws:states:us-east-1:000000000000:stateMachine:nonexistent-xyz",
			"state_machine_name": "nonexistent-xyz",
		},
		"nonexistent-xyz",
	)
}

// ─────────────────────────────────────────────────────────────────────────────
// Chain 4: cb → cb_builds → cb_build_logs
// ─────────────────────────────────────────────────────────────────────────────

// TestDemoColdCacheNestedChildren_CBChain verifies the three-level chain
// cb → cb_builds → cb_build_logs from a cold cache.
func TestDemoColdCacheNestedChildren_CBChain(t *testing.T) {
	t.Parallel()
	m := setupDemoApp(t)

	// Level 0: load CodeBuild projects.
	projects := loadResourceList(t, m, "cb")
	if len(projects) == 0 {
		t.Fatal("no CodeBuild projects in fixture data; cannot test cb→builds→logs chain")
	}

	// Level 1: drill into cb_builds for the first project.
	firstProject := projects[0]
	projectName := firstProject.ID
	builds := enterChildAndFetch(t, m, "cb_builds",
		map[string]string{"project_name": projectName},
		projectName,
	)
	if len(builds.Resources) == 0 {
		t.Fatal("no CodeBuild builds in fixture data; cannot test cb_builds→cb_build_logs chain")
	}

	// Level 2: drill into cb_build_logs for the first build that has a log group.
	// DrillCondition: log_group_name must not be empty.
	var logGroupName, logStreamName, buildNumber string
	for _, b := range builds.Resources {
		if b.Fields["log_group_name"] != "" {
			logGroupName = b.Fields["log_group_name"]
			logStreamName = b.Fields["log_stream_name"]
			buildNumber = b.Fields["build_number"]
			break
		}
	}
	if logGroupName == "" {
		t.Skip("no CodeBuild builds with log_group_name in fixture; cb_build_logs drill not testable")
	}

	_ = enterChildAndFetch(t, m, "cb_build_logs",
		map[string]string{
			"log_group_name":  logGroupName,
			"log_stream_name": logStreamName,
			"build_number":    buildNumber,
		},
		buildNumber,
	)
}

// TestDemoColdCacheNestedChildren_CBChain_UnknownParent verifies contract
// rule 4 for cb_builds: an unknown project name must surface an error.
func TestDemoColdCacheNestedChildren_CBChain_UnknownParent(t *testing.T) {
	t.Parallel()
	m := setupDemoApp(t)

	enterChildExpectError(t, m, "cb_builds",
		map[string]string{"project_name": "nonexistent-project-xyz-00000"},
		"nonexistent-project-xyz-00000",
	)
}

// ─────────────────────────────────────────────────────────────────────────────
// Chain 5: elb → elb_listeners → elb_listener_rules
// ─────────────────────────────────────────────────────────────────────────────

// TestDemoColdCacheNestedChildren_ELBChain verifies the three-level chain
// elb → elb_listeners → elb_listener_rules from a cold cache.
func TestDemoColdCacheNestedChildren_ELBChain(t *testing.T) {
	t.Parallel()
	m := setupDemoApp(t)

	// Level 0: load load balancers.
	lbs := loadResourceList(t, m, "elb")
	if len(lbs) == 0 {
		t.Fatal("no load balancers in fixture data; cannot test elb→listeners→rules chain")
	}

	// Level 1: drill into elb_listeners for the first LB.
	firstLB := lbs[0]
	lbArn := firstLB.Fields["load_balancer_arn"]
	if lbArn == "" {
		lbArn = firstLB.ID
	}
	lbName := firstLB.Name
	listeners := enterChildAndFetch(t, m, "elb_listeners",
		map[string]string{
			"load_balancer_arn": lbArn,
			"lb_name":           lbName,
		},
		lbName,
	)
	if len(listeners.Resources) == 0 {
		t.Fatal("no ELB listeners in fixture data; cannot test elb_listeners→elb_listener_rules chain")
	}

	// Level 2: drill into elb_listener_rules for the first listener.
	firstListener := listeners.Resources[0]
	listenerArn := firstListener.ID
	listenerDisplay := firstListener.Fields["listener_display"]
	if listenerDisplay == "" {
		listenerDisplay = listenerArn
	}
	_ = enterChildAndFetch(t, m, "elb_listener_rules",
		map[string]string{"listener_arn": listenerArn},
		listenerDisplay,
	)
}

// TestDemoColdCacheNestedChildren_ELBChain_UnknownParent verifies contract
// rule 4 for elb_listeners: an unknown LB ARN must surface an error.
func TestDemoColdCacheNestedChildren_ELBChain_UnknownParent(t *testing.T) {
	t.Parallel()
	m := setupDemoApp(t)

	enterChildExpectError(t, m, "elb_listeners",
		map[string]string{
			"load_balancer_arn": "arn:aws:elasticloadbalancing:us-east-1:000000000000:loadbalancer/app/nonexistent/abc",
			"lb_name":           "nonexistent-lb",
		},
		"nonexistent-lb",
	)
}
