package unit

import (
	"context"
	"fmt"
	"sort"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/resource"
)

// nilClientPanic runs fn and returns what it panicked with, "" when it did not.
func nilClientPanic(fn func()) (msg string) {
	defer func() {
		if r := recover(); r != nil {
			msg = fmt.Sprint(r)
		}
	}()
	fn()
	return ""
}

// A session can hold a client set with any service's client absent. Every
// related checker, Wave 2 enricher and detail enricher runs on every demo row
// with a client set holding none, and answers without panicking: a missing
// client is a read not made.
func TestNilServiceClient_NoReadPanics(t *testing.T) {
	demoClients := demo.NewServiceClients()
	ctx := context.Background()
	var panics []string
	for _, td := range resource.AllResourceTypes() {
		rows, ok := DrainFixtures(t, td, demoClients)
		if !ok || len(rows) == 0 {
			continue
		}
		for _, def := range td.Related {
			if def.Checker == nil {
				continue
			}
			for _, row := range rows {
				if msg := nilClientPanic(func() {
					def.Checker(ctx, &awsclient.ServiceClients{}, row, resource.ResourceCache{})
				}); msg != "" {
					panics = append(panics, fmt.Sprintf("%s → %s on %s: %s", td.ShortName, def.TargetType, row.ID, msg))
					break
				}
			}
		}
		if e, ok := awsclient.Wave2EnricherFor(td.ShortName); ok && e.Fn != nil {
			if msg := nilClientPanic(func() {
				e.Fn(ctx, &awsclient.ServiceClients{}, rows, resource.ResourceCache{}) //nolint:errcheck // only a panic is under test
			}); msg != "" {
				panics = append(panics, fmt.Sprintf("%s wave 2: %s", td.ShortName, msg))
			}
		}
		if de := resource.GetDetailEnricher(td.ShortName); de != nil {
			for _, row := range rows {
				if msg := nilClientPanic(func() {
					de(ctx, &awsclient.ServiceClients{}, row) //nolint:errcheck // only a panic is under test
				}); msg != "" {
					panics = append(panics, fmt.Sprintf("%s detail on %s: %s", td.ShortName, row.ID, msg))
					break
				}
			}
		}
	}
	sort.Strings(panics)
	for _, p := range panics {
		t.Errorf("%s", p)
	}
}
