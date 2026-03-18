package views

// View is the interface all view models must implement.
// It covers rendering, sizing, frame title, clipboard copy, and help context —
// the methods that every view in the stack needs. Update is intentionally
// excluded because each view returns its own concrete type from Update
// (Bubble Tea pattern).
type View interface {
	View() string
	SetSize(w, h int)
	FrameTitle() string
	CopyContent() (content string, label string) // for clipboard copy
	GetHelpContext() HelpContext                  // for context-sensitive help
}

// Filterable is an optional interface for views that support filtering.
// Only navigable list views (main menu, resource list, profile, region)
// implement this. Static views (detail, yaml, help, reveal) do not.
type Filterable interface {
	SetFilter(text string)
}
