package domain

// Section is the output unit of a DetailProjector. Each Section corresponds
// to one labelled group of items in a resource detail view.
type Section struct {
	Title string
	Items []Item
}

// ItemKind classifies the role of an Item within a Section.
type ItemKind int

const (
	ItemField    ItemKind = iota // standard key/value field row
	ItemHeader                   // section sub-header (bold, no value)
	ItemSubfield                 // indented sub-field under a header
	ItemSpacer                   // blank spacing row
)

// Item is a single entry within a Section. Its Kind determines how it is
// rendered by the detail view.
type Item struct {
	Kind        ItemKind
	Label       string
	Value       string
	Severity    Severity
	Tier        string // "!" | "~" | "" — matches ct-event coloring vocabulary
	Navigable   bool
	TargetType  string // resource short name for navigation (e.g. "vpc", "role")
	NavID       string // resolved bare ID for navigation (e.g. stripped ARN); empty when Value is already the bare ID
	IndentLevel int    // nesting depth for ItemSubfield items; 0 = top level, 1 = first indent, etc.
}

// DetailProjector transforms a Resource into an ordered list of Sections for
// the detail view. Nil means "use the generic projector". Pure function.
type DetailProjector func(r Resource) []Section

// Augmenter runs after the projector to inject per-type sections (e.g. EC2 status checks).
// Receives the resource and the projector's []Section output; returns possibly-extended []Section.
type Augmenter func(r Resource, sections []Section) []Section
