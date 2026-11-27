// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// democount prints the counts a demo session's titles show, read from the
// same fixture store `a9s --demo` loads: one "<name> <count>" line for the
// main menu ("resource-types") and for every resource type, as the list title
// prints it after its first page. The smoke scripts take their expected
// counts from it, so a fixture row added or removed moves the expectation
// with it.
//
//	go run ./cmd/democount
package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
)

func main() {
	aws.Install()
	resource.WireProjection()

	menu := app.New(runtime.Bootstrap(demo.DemoProfile, demo.DemoRegion, resource.AllResourceTypes()))
	fmt.Println("resource-types", titleCount("resource-types", menu.MenuFrameTitle()))

	clients := demo.NewServiceClients()
	for _, td := range resource.AllResourceTypes() {
		if td.Fetcher == nil {
			continue
		}
		fmt.Println(td.ShortName, listCount(clients, td.ShortName))
	}
}

func listCount(clients *aws.ServiceClients, shortName string) string {
	core := runtime.Bootstrap(demo.DemoProfile, demo.DemoRegion, resource.AllResourceTypes())
	core.Session().Clients = clients
	c := app.New(core)
	_, tasks := c.Apply(app.Action{Kind: app.ActionCommand, Arg: shortName})
	app.DrainSyncPartition(context.Background(), c, tasks, func(k runtime.TaskKind) bool { return k != runtime.KindFetchResources }, nil)
	return titleCount(shortName, c.ListFrameTitle())
}

func titleCount(name, title string) string {
	_, rest, ok := strings.Cut(title, "(")
	count, _, closed := strings.Cut(rest, ")")
	if !ok || !closed {
		log.Fatalf("democount: %s title %q carries no (<count>)", name, title)
	}
	return count
}
