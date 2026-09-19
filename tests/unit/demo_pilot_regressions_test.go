package unit

import (
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

func TestDemo_S3ListDoesNotPanic(t *testing.T) {
	m := newDemoColdCacheApp(t)

	*m, _ = rootApplyMsg(*m, tea.WindowSizeMsg{Width: 120, Height: 40})

	clients := demo.NewServiceClients()
	*m, _ = rootApplyMsg(*m, messages.ClientsReady{Clients: clients, Gen: 1})

	// Navigate to S3. If the S3 client is nil this will panic inside the fetch cmd.
	var navCmd tea.Cmd
	*m, navCmd = rootApplyMsg(*m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "s3",
	})

	if navCmd == nil {
		t.Fatal("expected a cmd after NavigateMsg{s3}, got nil")
	}

	// Execute the fetch command in a goroutine so a panic is caught as a test
	// failure rather than crashing the whole test binary.
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
			case messages.ResourcesLoaded, messages.Flash:
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
	case messages.ResourcesLoaded:
		if len(v.Resources) == 0 {
			t.Error("S3 fetch returned zero resources; expected demo fixture buckets")
		}
	case messages.Flash:
		// A structured error is acceptable — it means the error was handled, not panicked.
		if !v.IsError {
			t.Error("FlashMsg.IsError should be true for S3 fetch failure")
		}
	}
}

func TestDemo_EC2RelatedPanelsPopulate(t *testing.T) {
	m := newDemoColdCacheApp(t)

	*m, _ = rootApplyMsg(*m, tea.WindowSizeMsg{Width: 120, Height: 40})

	clients := demo.NewServiceClients()
	*m, _ = rootApplyMsg(*m, messages.ClientsReady{Clients: clients, Gen: 1})

	var navCmd tea.Cmd
	*m, navCmd = rootApplyMsg(*m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	if navCmd == nil {
		t.Fatal("expected cmd after NavigateMsg{ec2}")
	}

	raw := extractMsg(t, navCmd, func(msg tea.Msg) bool {
		_, ok := msg.(messages.ResourcesLoaded)
		return ok
	})
	loaded := raw.(messages.ResourcesLoaded)

	if len(loaded.Resources) == 0 {
		t.Fatal("fixture data has zero EC2 instances; cannot open detail")
	}
	*m, _ = rootApplyMsg(*m, loaded)

	firstInstance := loaded.Resources[0]
	var relatedCmd tea.Cmd
	*m, relatedCmd = rootApplyMsg(*m, messages.Navigate{
		Target:       messages.TargetDetail,
		Resource:     &firstInstance,
		ResourceType: "ec2",
	})

	if relatedCmd == nil {
		t.Fatal("expected a related-check command after opening EC2 detail, got nil")
	}

	// The enrich + related-check tasks dispatch directly off relatedCmd as a
	// (possibly nested) tea.Batch; RunRelatedDef already recovers
	// per-checker panics into a RelatedCheckResult carrying LazyAddError, so
	// this collects every per-def RelatedCheckResult leaf directly.
	leaves := extractLeafMsgs(relatedCmd)
	var results []messages.RelatedCheckResult
	for _, leaf := range leaves {
		if r, ok := leaf.(messages.RelatedCheckResult); ok {
			results = append(results, r)
		}
	}
	if len(results) == 0 {
		types := make([]string, len(leaves))
		for i, leaf := range leaves {
			types[i] = fmt.Sprintf("%T", leaf)
		}
		t.Fatalf("expected at least one RelatedCheckResult from detail init, got: %v", types)
	}

	for _, r := range results {
		*m, _ = rootApplyMsg(*m, r)
	}

	countByName := make(map[string]int)
	for _, r := range results {
		countByName[r.DefDisplayName] = r.Result.Count()
	}

	// These four defs require NeedsTargetCache=true prefetch via nil clients.
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

func TestDemo_CtxCommandBlocked(t *testing.T) {
	t.Run("ctx_blocked", func(t *testing.T) {
		m := newDemoColdCacheApp(t)

		clients := demo.NewServiceClients()
		*m, _ = rootApplyMsg(*m, messages.ClientsReady{Clients: clients, Gen: 1})

		// Dispatch TargetProfile — same path as :ctx command.
		var profileCmd tea.Cmd
		*m, profileCmd = rootApplyMsg(*m, messages.Navigate{Target: messages.TargetProfile})

		if profileCmd == nil {
			t.Fatal("expected a cmd after NavigateMsg{TargetProfile}, got nil")
		}

		// In demo mode profileCmd returns FlashMsg{IsError: true} rather than
		// pushing a selector.
		msg := profileCmd()
		flash, ok := msg.(messages.Flash)
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
		*m, _ = rootApplyMsg(*m, messages.ClientsReady{Clients: clients, Gen: 1})

		// Dispatch TargetRegion — same path as :region command.
		var regionCmd tea.Cmd
		*m, regionCmd = rootApplyMsg(*m, messages.Navigate{Target: messages.TargetRegion})

		// The guard fires before the view is pushed and returns a FlashMsg cmd.
		if regionCmd == nil {
			t.Fatalf("TargetRegion in demo mode returned nil cmd — " +
				"demo guard is missing; pre-fix pushes region selector inline instead of blocking")
		}

		msg := regionCmd()
		flash, ok := msg.(messages.Flash)
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
