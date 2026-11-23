// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// democount prints the counts a demo session's titles show, read from the
// same fixture store `a9s --demo` loads: one "<name> <count>" line for the
// main menu ("resource-types") and for every resource type, in the form
// the list title prints it ("44", or "50+" when the first page is not the
// whole list). The smoke scripts take their expected counts from it, so a
// fixture row added or removed moves the expectation with it.
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
	title := menu.MenuFrameTitle()
	total, ok := strings.CutPrefix(title, "resource-types(")
	if !ok || !strings.HasSuffix(total, ")") {
		log.Fatalf("democount: menu title %q is not resource-types(<n>)", title)
	}
	fmt.Println("resource-types", strings.TrimSuffix(total, ")"))

	clients := demo.NewServiceClients()
	for _, td := range resource.AllResourceTypes() {
		if td.Fetcher == nil {
			continue
		}
		// A partial read still lists the rows it got, so the title counts
		// them; the demo carries such failures on purpose.
		page, _ := td.Fetcher(context.Background(), clients, "")
		suffix := ""
		if page.Pagination != nil && page.Pagination.IsTruncated {
			suffix = "+"
		}
		fmt.Printf("%s %d%s\n", td.ShortName, len(page.Resources), suffix)
	}
}
