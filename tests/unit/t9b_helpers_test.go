package unit_test

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
	unit "github.com/k2m30/a9s/v3/tests/unit"
	"github.com/k2m30/a9s/v3/tests/unit/tuitest"
)

type t9bRow struct {
	typ string
	res resource.Resource
}

func t9bDemoRows(t *testing.T) []t9bRow {
	t.Helper()
	clients := demo.NewServiceClients()
	var out []t9bRow
	for _, td := range resource.AllResourceTypes() {
		parents, ok := unit.DrainFixtures(t, td, clients)
		if !ok {
			continue
		}
		for _, p := range parents {
			out = append(out, t9bRow{typ: td.ShortName, res: p})
		}
		for _, child := range td.Children {
			cd := catalog.FindAny(child.ChildType)
			if cd == nil || cd.ChildFetcher == nil {
				continue
			}
			for i := range parents {
				pctx := resource.ResolveChildContext(child, &parents[i], nil)
				got, _ := cd.ChildFetcher(context.Background(), clients, pctx, "") //nolint:errcheck // a partial page still carries the rows the drill shows
				for _, r := range got.Resources {
					out = append(out, t9bRow{typ: child.ChildType, res: r})
				}
			}
		}
	}
	if len(out) < 1000 {
		t.Fatalf("demo bench returned %d rows; the sweep needs the whole bench", len(out))
	}
	return out
}

func t9bDetailRows(t *testing.T, res resource.Resource, typ string) []app.FieldRow {
	t.Helper()
	c := newVisibilityDetailController(t)
	c.EnsureDetailState(res, typ)
	body := c.Snapshot().Body.Detail
	if body == nil {
		t.Fatalf("%s %s: no detail body", typ, res.ID)
	}
	return body.Fields
}

// t9bCoreText opens the YAML and the JSON screen from the resource's detail
// screen on the headless controller, the lane the web client renders, and
// returns the lines each text body holds.
func t9bCoreText(t *testing.T, res resource.Resource, typ string) (yamlLines, jsonLines []string) {
	t.Helper()
	c := newVisibilityDetailController(t)
	c.EnsureDetailState(res, typ)
	c.Apply(app.Action{Kind: app.ActionOpenYAML})
	if tb := c.Snapshot().Body.Text; tb != nil {
		yamlLines = append(yamlLines, tb.Lines...)
	}
	c.Apply(app.Action{Kind: app.ActionBack})
	c.Apply(app.Action{Kind: app.ActionOpenJSON})
	if tb := c.Snapshot().Body.Text; tb != nil {
		jsonLines = append(jsonLines, tb.Lines...)
	}
	return t9bTrimRight(yamlLines), t9bTrimRight(jsonLines)
}

type t9bTUI struct {
	t *testing.T
	m tui.Model
}

func newT9bTUI(t *testing.T) *t9bTUI {
	t.Helper()
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	clients := demo.NewServiceClients()
	m := newBlessedModel(t, demo.DemoProfile, demo.DemoRegion, tui.WithClients(clients), tui.WithIsDemo(true), tui.WithNoCache(true))
	m = tuitest.StepModel(m, tea.WindowSizeMsg{Width: 200, Height: 60})
	m = tuitest.StepModel(m, messages.ClientsReady{Clients: clients, Region: demo.DemoRegion, Gen: m.Core().ConnectGen()})
	return &t9bTUI{t: t, m: m}
}

// The caller sizes the terminal to the text it expects, so a clipped line or
// a scrolled viewport cannot hide a difference between the lanes.
func (d *t9bTUI) textOf(res resource.Resource, typ, key string, width, height int) []string {
	d.t.Helper()
	d.m = tuitest.StepModel(d.m, tea.WindowSizeMsg{Width: width, Height: height})
	r := res
	d.m = tuitest.StepModel(d.m, messages.Navigate{Target: messages.TargetDetail, ResourceType: typ, Resource: &r})
	d.press(key)
	screen := tuitest.StripANSI(tuitest.Render(d.m))
	d.m = tuitest.StepModel(d.m, tea.KeyPressMsg{Code: tea.KeyEscape})
	d.m = tuitest.StepModel(d.m, tea.KeyPressMsg{Code: tea.KeyEscape})
	return t9bFrameLines(screen)
}

// press sends a key and delivers the navigation the key asks for, the way
// the program loop runs the command a key returns.
func (d *t9bTUI) press(key string) {
	d.t.Helper()
	var cmd tea.Cmd
	d.m, cmd = tuitest.Step(d.m, tea.KeyPressMsg{Code: rune(key[0]), Text: key})
	if cmd == nil {
		return
	}
	msgs := []tea.Msg{cmd()}
	if batch, ok := msgs[0].(tea.BatchMsg); ok {
		msgs = msgs[:0]
		for _, c := range batch {
			if c != nil {
				msgs = append(msgs, c())
			}
		}
	}
	for _, msg := range msgs {
		if nav, ok := msg.(messages.Navigate); ok {
			d.m = tuitest.StepModel(d.m, nav)
		}
	}
}

func t9bFrameLines(screen string) []string {
	var out []string
	rows := strings.Split(screen, "\n")
	for i, line := range rows {
		if i == 0 || i == len(rows)-1 {
			continue
		}
		first := strings.Index(line, "│")
		last := strings.LastIndex(line, "│")
		if first < 0 || last <= first {
			continue
		}
		out = append(out, line[first+len("│"):last])
	}
	out = t9bTrimRight(out)
	for len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	return out
}

func t9bTrimRight(lines []string) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = strings.TrimRight(l, " ")
	}
	return out
}

func t9bHasLine(lines []string, want string) bool {
	for _, l := range lines {
		if strings.TrimSpace(l) == want {
			return true
		}
	}
	return false
}
