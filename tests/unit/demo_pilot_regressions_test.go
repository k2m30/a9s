package unit

import (
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

// ═══════════════════════════════════════════════════════════════════════════
// Demo pilot regression tests — fail before F1+F2 land, pass after.
// ═══════════════════════════════════════════════════════════════════════════

// ---------------------------------------------------------------------------
// 1. TestDemo_S3ListDoesNotPanic
//
// Pre-fix: demo.NewServiceClients() returns nil S3 client → FetchS3BucketsPage
// calls ListBuckets on nil *s3.Client → panic.
// Post-fix: legacy demo transport handles ListBuckets, no nil client.
// ---------------------------------------------------------------------------

func TestDemo_S3ListDoesNotPanic(t *testing.T) {
	m := newDemoColdCacheApp(t)

	*m, _ = rootApplyMsg(*m, tea.WindowSizeMsg{Width: 120, Height: 40})

	clients := demo.NewServiceClients()
	*m, _ = rootApplyMsg(*m, messages.ClientsReadyMsg{Clients: clients, Gen: 0})

	// Navigate to S3. If the S3 client is nil this will panic inside the fetch cmd.
	var navCmd tea.Cmd
	*m, navCmd = rootApplyMsg(*m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "s3",
	})

	if navCmd == nil {
		t.Fatal("expected a cmd after NavigateMsg{s3}, got nil")
	}

	// Execute the fetch command in a goroutine so a panic is caught as a test
	// failure rather than crashing the whole test binary (pre-fix behaviour).
	type result struct {
		msg      tea.Msg
		panicked bool
		panicVal any
	}
	// findMsg walks a tea.Msg (including BatchMsg) to find ResourcesLoadedMsg or FlashMsg.
	// Returns nil if not found. Does NOT call t.Fatal (safe for goroutine use).
	findMsg := func(root tea.Msg) tea.Msg {
		check := func(m tea.Msg) bool {
			switch m.(type) {
			case messages.ResourcesLoadedMsg, messages.FlashMsg:
				return true
			}
			return false
		}
		if check(root) {
			return root
		}
		if batch, ok := root.(tea.BatchMsg); ok {
			for _, sub := range batch {
				if sub == nil {
					continue
				}
				subMsg := sub()
				if check(subMsg) {
					return subMsg
				}
			}
		}
		return nil
	}

	ch := make(chan result, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				ch <- result{panicked: true, panicVal: r}
			}
		}()
		root := navCmd()
		found := findMsg(root)
		ch <- result{msg: found}
	}()

	res := <-ch
	if res.panicked {
		t.Fatalf("S3 fetch panicked (nil client deref): %v — "+
			"pre-fix: S3 client is nil; post-fix: legacy transport handles ListBuckets", res.panicVal)
	}
	if res.msg == nil {
		t.Fatal("S3 fetch returned no ResourcesLoadedMsg or FlashMsg — unexpected cmd shape")
	}

	switch v := res.msg.(type) {
	case messages.ResourcesLoadedMsg:
		// After F1 lands the legacy transport should populate fixtures.
		if len(v.Resources) == 0 {
			t.Error("S3 fetch returned zero resources; expected demo fixture buckets")
		}
	case messages.FlashMsg:
		// A structured error is acceptable — it means the error was handled, not panicked.
		if !v.IsError {
			t.Error("FlashMsg.IsError should be true for S3 fetch failure")
		}
	}
}

// ---------------------------------------------------------------------------
// 2. TestDemo_EC2RelatedPanelsPopulate
//
// Pre-fix: only EC2-backed checkers succeed. The four transport-dependent
// checkers (tg, asg, alarm, cfn) require prefetch of their respective resource
// types via nil clients → panic inside the fetcher → Count=-1.
// Post-fix: legacy demo transport satisfies the prefetch, all four return Count >= 0.
//
// The test asserts these four named defs specifically — they are the ones that
// need F1 (hybrid client) to pass.
// ---------------------------------------------------------------------------

func TestDemo_EC2RelatedPanelsPopulate(t *testing.T) {
	m := newDemoColdCacheApp(t)

	*m, _ = rootApplyMsg(*m, tea.WindowSizeMsg{Width: 120, Height: 40})

	clients := demo.NewServiceClients()
	*m, _ = rootApplyMsg(*m, messages.ClientsReadyMsg{Clients: clients, Gen: 0})

	// Navigate to EC2 list.
	var navCmd tea.Cmd
	*m, navCmd = rootApplyMsg(*m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	if navCmd == nil {
		t.Fatal("expected cmd after NavigateMsg{ec2}")
	}

	raw := extractMsg(t, navCmd, func(msg tea.Msg) bool {
		_, ok := msg.(messages.ResourcesLoadedMsg)
		return ok
	})
	loaded := raw.(messages.ResourcesLoadedMsg)

	if len(loaded.Resources) == 0 {
		t.Fatal("fixture data has zero EC2 instances; cannot open detail")
	}
	*m, _ = rootApplyMsg(*m, loaded)

	// Open detail for the first EC2 instance.
	firstInstance := loaded.Resources[0]
	var relatedCmd tea.Cmd
	*m, relatedCmd = rootApplyMsg(*m, messages.NavigateMsg{
		Target:       messages.TargetDetail,
		Resource:     &firstInstance,
		ResourceType: "ec2",
	})

	if relatedCmd == nil {
		t.Fatal("expected a related-check command after opening EC2 detail, got nil")
	}

	// Execute to get RelatedCheckStartedMsg.
	relatedMsg := relatedCmd()
	started, ok := relatedMsg.(messages.RelatedCheckStartedMsg)
	if !ok {
		t.Fatalf("expected RelatedCheckStartedMsg from detail init, got %T", relatedMsg)
	}

	// Dispatch started msg so handleRelatedCheckStarted runs the checkers.
	var checkCmds tea.Cmd
	*m, checkCmds = rootApplyMsg(*m, started)
	if checkCmds == nil {
		t.Fatal("handleRelatedCheckStarted returned nil cmd — no checkers dispatched for ec2?")
	}

	// runChecker executes a cmd recovering from panics (pre-fix behaviour).
	runChecker := func(c tea.Cmd) (msg tea.Msg) {
		defer func() {
			if r := recover(); r != nil {
				msg = nil
			}
		}()
		return c()
	}

	// Collect all RelatedCheckResultMsg values from the batch.
	var results []messages.RelatedCheckResultMsg

	collectResults := func(batchResult tea.Msg) {
		switch v := batchResult.(type) {
		case messages.RelatedCheckResultMsg:
			results = append(results, v)
		case tea.BatchMsg:
			for _, subCmd := range v {
				if subCmd == nil {
					continue
				}
				sub := runChecker(subCmd)
				if r, ok2 := sub.(messages.RelatedCheckResultMsg); ok2 {
					results = append(results, r)
				}
			}
		}
	}

	rawCheck := runChecker(checkCmds)
	collectResults(rawCheck)

	// Deliver all results to the model.
	for _, r := range results {
		*m, _ = rootApplyMsg(*m, r)
	}

	// Build a map of DisplayName → Count for easy lookup.
	countByName := make(map[string]int)
	for _, r := range results {
		countByName[r.DefDisplayName] = r.Result.Count
	}

	// These four defs require NeedsTargetCache=true prefetch via nil clients pre-fix.
	// They are the specific ones that crash before F1 lands.
	transportDependentDefs := []string{
		"Target Groups",
		"Auto Scaling Groups",
		"CloudWatch Alarms",
		"CloudFormation Stacks",
	}

	var failures []string
	for _, name := range transportDependentDefs {
		count, seen := countByName[name]
		if !seen {
			failures = append(failures, fmt.Sprintf("%q: no result (checker never ran or panicked unrecovered)", name))
		} else if count < 0 {
			failures = append(failures, fmt.Sprintf("%q: Count=%d (panic-recover sentinel — nil client prefetch failed)", name, count))
		}
	}
	if len(failures) > 0 {
		t.Errorf("transport-dependent EC2 related defs failed (require F1 hybrid client):\n%v\n"+
			"all results: %v", failures, countByName)
	}
}

// ---------------------------------------------------------------------------
// 3. TestDemo_CtxCommandBlocked
//
// Pre-fix: handleNavigate(TargetProfile) calls m.fetchProfiles() with no demo
// guard — no FlashMsg, no error, just starts a real AWS profile lookup that
// would hang or return unexpected results in tests.
// Post-fix (F2): guard checks preSuppliedClients != nil and returns an error
// FlashMsg with a descriptive message.
//
// Same shape for :region.
// ---------------------------------------------------------------------------

func TestDemo_CtxCommandBlocked(t *testing.T) {
	t.Run("ctx_blocked", func(t *testing.T) {
		m := newDemoColdCacheApp(t)

		clients := demo.NewServiceClients()
		*m, _ = rootApplyMsg(*m, messages.ClientsReadyMsg{Clients: clients, Gen: 0})

		// Dispatch TargetProfile — same path as :ctx command.
		var profileCmd tea.Cmd
		*m, profileCmd = rootApplyMsg(*m, messages.NavigateMsg{Target: messages.TargetProfile})

		if profileCmd == nil {
			t.Fatal("expected a cmd after NavigateMsg{TargetProfile}, got nil")
		}

		// Pre-fix: profileCmd is m.fetchProfiles() — it returns a SelectorModel push,
		// not a FlashMsg. The test fails because msg is not a FlashMsg.
		// Post-fix: profileCmd returns FlashMsg{IsError: true}.
		msg := profileCmd()
		flash, ok := msg.(messages.FlashMsg)
		if !ok {
			t.Fatalf("expected FlashMsg blocking :ctx in demo mode; got %T — "+
				"demo guard is missing from handleNavigate(TargetProfile)", msg)
		}
		if !flash.IsError {
			t.Errorf("FlashMsg.IsError must be true for blocked :ctx command; got false. Text=%q", flash.Text)
		}
		if flash.Text == "" {
			t.Error("FlashMsg.Text must not be empty for blocked :ctx command")
		}
	})

	t.Run("region_blocked", func(t *testing.T) {
		m := newDemoColdCacheApp(t)

		clients := demo.NewServiceClients()
		*m, _ = rootApplyMsg(*m, messages.ClientsReadyMsg{Clients: clients, Gen: 0})

		// Dispatch TargetRegion — same path as :region command.
		var regionCmd tea.Cmd
		*m, regionCmd = rootApplyMsg(*m, messages.NavigateMsg{Target: messages.TargetRegion})

		// Pre-fix: TargetRegion pushes a view inline and returns nil cmd (no guard).
		// Post-fix (F2): guard fires before pushing view, returns FlashMsg cmd.
		if regionCmd == nil {
			t.Fatalf("TargetRegion in demo mode returned nil cmd — "+
				"demo guard is missing; pre-fix pushes region selector inline instead of blocking")
		}

		msg := regionCmd()
		flash, ok := msg.(messages.FlashMsg)
		if !ok {
			t.Fatalf("expected FlashMsg blocking :region in demo mode; got %T — "+
				"demo guard is missing from handleNavigate(TargetRegion)", msg)
		}
		if !flash.IsError {
			t.Errorf("FlashMsg.IsError must be true for blocked :region command; got false. Text=%q", flash.Text)
		}
		if flash.Text == "" {
			t.Error("FlashMsg.Text must not be empty for blocked :region command")
		}
	})
}
