package navigation

// ViewType represents the type of view in the application.
type ViewType int

const (
	MainMenuView ViewType = iota
	ResourceListView
	DetailView
	JSONView
	RevealView
	ProfileSelectView
	RegionSelectView
)

// ViewState captures the state of a particular view for navigation purposes.
type ViewState struct {
	ViewType     ViewType
	ResourceType string
	CursorPos    int
	Filter       string
	S3Bucket     string
	S3Prefix     string
}

// NavigationStack provides browser-like back/forward navigation between views.
type NavigationStack struct {
	back    []ViewState
	forward []ViewState
}

// Push adds a new view state to the back stack and clears the forward stack.
func (n *NavigationStack) Push(state ViewState) {
	n.back = append(n.back, state)
	n.forward = nil
}

// Pop removes and returns the most recent view state from the back stack.
// The popped state is added to the forward stack.
// Returns false if the back stack is empty.
func (n *NavigationStack) Pop() (ViewState, bool) {
	if len(n.back) == 0 {
		return ViewState{}, false
	}

	last := n.back[len(n.back)-1]
	n.back = n.back[:len(n.back)-1]
	n.forward = append(n.forward, last)

	return last, true
}

// Forward removes and returns the most recent view state from the forward stack.
// The state is added back to the back stack.
// Returns false if the forward stack is empty.
func (n *NavigationStack) Forward() (ViewState, bool) {
	if len(n.forward) == 0 {
		return ViewState{}, false
	}

	last := n.forward[len(n.forward)-1]
	n.forward = n.forward[:len(n.forward)-1]
	n.back = append(n.back, last)

	return last, true
}

// Clear empties both the back and forward stacks.
func (n *NavigationStack) Clear() {
	n.back = nil
	n.forward = nil
}

// CanGoBack returns true if there are entries in the back stack.
func (n *NavigationStack) CanGoBack() bool {
	return len(n.back) > 0
}

// CanGoForward returns true if there are entries in the forward stack.
func (n *NavigationStack) CanGoForward() bool {
	return len(n.forward) > 0
}
