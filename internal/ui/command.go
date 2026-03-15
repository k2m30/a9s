package ui

import (
	"strings"

	lipgloss "charm.land/lipgloss/v2"
)

// CommandInput handles command-mode text input with autocomplete suggestions.
type CommandInput struct {
	// Text is the current input text.
	Text string
	// Cursor is the cursor position within Text.
	Cursor int
	// Active indicates whether command input mode is active.
	Active bool
	// Suggestions is the list of known commands for autocomplete.
	Suggestions []string
}

// defaultCommands is the set of known commands for autocomplete.
var defaultCommands = []string{
	"main", "root", "ctx", "region",
	"s3", "ec2", "rds", "redis", "docdb", "eks", "secrets",
	"q", "quit",
}

// NewCommandInput creates a new CommandInput initialized with the given known
// commands. If knownCommands is nil, the default command list is used.
func NewCommandInput(knownCommands []string) CommandInput {
	cmds := knownCommands
	if cmds == nil {
		cmds = make([]string, len(defaultCommands))
		copy(cmds, defaultCommands)
	}
	return CommandInput{
		Suggestions: cmds,
	}
}

// HandleKey processes a key press. It returns (executed, command) where
// executed is true when the input has been finalized: on "enter" the command
// text is returned; on "escape" an empty string is returned. "backspace"
// deletes the last character. Any other single rune is appended to Text.
func (c *CommandInput) HandleKey(key string) (executed bool, command string) {
	switch key {
	case "enter":
		cmd := c.Text
		c.Reset()
		return true, cmd
	case "escape":
		c.Reset()
		return true, ""
	case "backspace":
		if len(c.Text) > 0 {
			c.Text = c.Text[:len(c.Text)-1]
			c.Cursor = len(c.Text)
		}
		return false, ""
	default:
		if len(key) == 1 {
			c.Text += key
			c.Cursor = len(c.Text)
		}
		return false, ""
	}
}

// Reset clears the command input state.
func (c *CommandInput) Reset() {
	c.Text = ""
	c.Cursor = 0
	c.Active = false
}

// BestMatch returns the first suggestion that starts with the current Text.
// Returns an empty string if there is no match or Text is empty.
func (c CommandInput) BestMatch() string {
	if c.Text == "" {
		return ""
	}
	lower := strings.ToLower(c.Text)
	for _, s := range c.Suggestions {
		if strings.HasPrefix(strings.ToLower(s), lower) {
			return s
		}
	}
	return ""
}

// View renders the command input as ":" followed by the current text and a
// dimmed autocomplete suggestion for the remaining characters.
func (c *CommandInput) View() string {
	dim := lipgloss.NewStyle().Faint(true)

	result := ":" + c.Text

	match := c.BestMatch()
	if match != "" && len(match) > len(c.Text) {
		suffix := match[len(c.Text):]
		result += dim.Render(suffix)
	}

	return result
}
