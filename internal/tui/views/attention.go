package views

// AttentionFilter holds the shared toggle state for the ctrl+z attention
// filter. Views embed this struct for the enabled/disabled toggle and do
// their own counting and rendering.
type AttentionFilter struct {
	enabled bool
}

// Toggle flips the filter between enabled and disabled.
func (a *AttentionFilter) Toggle() { a.enabled = !a.enabled }

// IsEnabled reports whether the attention filter is currently active.
func (a *AttentionFilter) IsEnabled() bool { return a.enabled }

// SetEnabled sets the filter state explicitly.
func (a *AttentionFilter) SetEnabled(v bool) { a.enabled = v }
