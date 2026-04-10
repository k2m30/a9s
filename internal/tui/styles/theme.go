package styles

import (
	"fmt"
	"image/color"
	"regexp"

	lipgloss "charm.land/lipgloss/v2"
	"gopkg.in/yaml.v3"
)

// Theme holds all named colors for a UI theme.
type Theme struct {
	Name              string
	HeaderFg          color.Color
	Accent            color.Color
	Dim               color.Color
	Border            color.Color
	RowSelectedBg     color.Color
	RowSelectedFg     color.Color
	RowAltBg          color.Color
	Running           color.Color
	Stopped           color.Color
	Pending           color.Color
	Terminated        color.Color
	DetailKey         color.Color
	DetailVal         color.Color
	DetailSec         color.Color
	YAMLKey           color.Color
	YAMLStr           color.Color
	YAMLNum           color.Color
	YAMLBool          color.Color
	YAMLNull          color.Color
	YAMLTree          color.Color
	HelpKey           color.Color
	HelpCat           color.Color
	Filter            color.Color
	Success           color.Color
	Error             color.Color
	Spinner           color.Color
	Scroll            color.Color
	Warning           color.Color
	KeyHintKey        color.Color
	KeyHintBg         color.Color
	KeyHintFg         color.Color
	OverlayBg         color.Color
	OverlayBorder     color.Color
	SearchHighlightFg color.Color
	SearchHighlightBg color.Color
}

// DefaultTheme returns the Tokyo Night Dark theme with all default hex values.
func DefaultTheme() Theme {
	return Theme{
		Name:              "Tokyo Night Dark",
		HeaderFg:          lipgloss.Color("#c0caf5"),
		Accent:            lipgloss.Color("#7aa2f7"),
		Dim:               lipgloss.Color("#565f89"),
		Border:            lipgloss.Color("#414868"),
		RowSelectedBg:     lipgloss.Color("#7aa2f7"),
		RowSelectedFg:     lipgloss.Color("#1a1b26"),
		RowAltBg:          lipgloss.Color("#1e2030"),
		Running:           lipgloss.Color("#9ece6a"),
		Stopped:           lipgloss.Color("#f7768e"),
		Pending:           lipgloss.Color("#e0af68"),
		Terminated:        lipgloss.Color("#565f89"),
		DetailKey:         lipgloss.Color("#7aa2f7"),
		DetailVal:         lipgloss.Color("#c0caf5"),
		DetailSec:         lipgloss.Color("#e0af68"),
		YAMLKey:           lipgloss.Color("#7aa2f7"),
		YAMLStr:           lipgloss.Color("#9ece6a"),
		YAMLNum:           lipgloss.Color("#ff9e64"),
		YAMLBool:          lipgloss.Color("#bb9af7"),
		YAMLNull:          lipgloss.Color("#565f89"),
		YAMLTree:          lipgloss.Color("#414868"),
		HelpKey:           lipgloss.Color("#9ece6a"),
		HelpCat:           lipgloss.Color("#e0af68"),
		Filter:            lipgloss.Color("#e0af68"),
		Success:           lipgloss.Color("#9ece6a"),
		Error:             lipgloss.Color("#f7768e"),
		Spinner:           lipgloss.Color("#7aa2f7"),
		Scroll:            lipgloss.Color("#414868"),
		Warning:           lipgloss.Color("#e0af68"),
		KeyHintKey:        lipgloss.Color("#7aa2f7"),
		KeyHintBg:         lipgloss.Color("#24283b"),
		KeyHintFg:         lipgloss.Color("#565f89"),
		OverlayBg:         lipgloss.Color("#1a1b26"),
		OverlayBorder:     lipgloss.Color("#7aa2f7"),
		SearchHighlightFg: lipgloss.Color("#1a1b26"),
		SearchHighlightBg: lipgloss.Color("#e0af68"),
	}
}

// activeTheme is the package-level active theme, initialized to DefaultTheme.
var activeTheme = DefaultTheme()

// ActiveTheme returns a copy of the currently active theme.
func ActiveTheme() Theme {
	return activeTheme
}

// ApplyTheme sets the active theme, updates palette vars, and rebuilds composed styles.
func ApplyTheme(t Theme) {
	activeTheme = t
	applyPalette(t)
	initStyles()
}

var hexColorRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// themeYAML is an intermediate struct for YAML deserialization.
// Pointer fields allow distinguishing "not set" from "empty string".
type themeYAML struct {
	Name   *string         `yaml:"name"`
	Colors themeColorsYAML `yaml:"colors"`
}

type themeColorsYAML struct {
	HeaderFg          *string `yaml:"header_fg"`
	Accent            *string `yaml:"accent"`
	Dim               *string `yaml:"dim"`
	Border            *string `yaml:"border"`
	RowSelectedBg     *string `yaml:"row_selected_bg"`
	RowSelectedFg     *string `yaml:"row_selected_fg"`
	RowAltBg          *string `yaml:"row_alt_bg"`
	Running           *string `yaml:"running"`
	Stopped           *string `yaml:"stopped"`
	Pending           *string `yaml:"pending"`
	Terminated        *string `yaml:"terminated"`
	DetailKey         *string `yaml:"detail_key"`
	DetailVal         *string `yaml:"detail_val"`
	DetailSec         *string `yaml:"detail_sec"`
	YAMLKey           *string `yaml:"yaml_key"`
	YAMLStr           *string `yaml:"yaml_str"`
	YAMLNum           *string `yaml:"yaml_num"`
	YAMLBool          *string `yaml:"yaml_bool"`
	YAMLNull          *string `yaml:"yaml_null"`
	YAMLTree          *string `yaml:"yaml_tree"`
	HelpKey           *string `yaml:"help_key"`
	HelpCat           *string `yaml:"help_cat"`
	Filter            *string `yaml:"filter"`
	Success           *string `yaml:"success"`
	Error             *string `yaml:"error"`
	Spinner           *string `yaml:"spinner"`
	Scroll            *string `yaml:"scroll"`
	Warning           *string `yaml:"warning"`
	KeyHintKey        *string `yaml:"key_hint_key"`
	KeyHintBg         *string `yaml:"key_hint_bg"`
	KeyHintFg         *string `yaml:"key_hint_fg"`
	OverlayBg         *string `yaml:"overlay_bg"`
	OverlayBorder     *string `yaml:"overlay_border"`
	SearchHighlightFg *string `yaml:"search_highlight_fg"`
	SearchHighlightBg *string `yaml:"search_highlight_bg"`
}

// ThemeFromYAML parses a YAML theme definition, starting from DefaultTheme as base.
// Only non-nil fields override the base. Unknown keys are silently ignored.
func ThemeFromYAML(data []byte) (Theme, error) {
	var raw themeYAML
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return Theme{}, fmt.Errorf("theme YAML parse error: %w", err)
	}

	t := DefaultTheme()

	if raw.Name != nil {
		t.Name = *raw.Name
	}

	parseColor := func(ptr *string, fieldName string) (color.Color, error) {
		if ptr == nil {
			return nil, nil
		}
		if !hexColorRe.MatchString(*ptr) {
			return nil, fmt.Errorf("invalid hex color for %s: %q", fieldName, *ptr)
		}
		return lipgloss.Color(*ptr), nil
	}

	c := raw.Colors

	if col, err := parseColor(c.HeaderFg, "header_fg"); err != nil {
		return Theme{}, err
	} else if c.HeaderFg != nil {
		t.HeaderFg = col
	}
	if col, err := parseColor(c.Accent, "accent"); err != nil {
		return Theme{}, err
	} else if c.Accent != nil {
		t.Accent = col
	}
	if col, err := parseColor(c.Dim, "dim"); err != nil {
		return Theme{}, err
	} else if c.Dim != nil {
		t.Dim = col
	}
	if col, err := parseColor(c.Border, "border"); err != nil {
		return Theme{}, err
	} else if c.Border != nil {
		t.Border = col
	}
	if col, err := parseColor(c.RowSelectedBg, "row_selected_bg"); err != nil {
		return Theme{}, err
	} else if c.RowSelectedBg != nil {
		t.RowSelectedBg = col
	}
	if col, err := parseColor(c.RowSelectedFg, "row_selected_fg"); err != nil {
		return Theme{}, err
	} else if c.RowSelectedFg != nil {
		t.RowSelectedFg = col
	}
	if col, err := parseColor(c.RowAltBg, "row_alt_bg"); err != nil {
		return Theme{}, err
	} else if c.RowAltBg != nil {
		t.RowAltBg = col
	}
	if col, err := parseColor(c.Running, "running"); err != nil {
		return Theme{}, err
	} else if c.Running != nil {
		t.Running = col
	}
	if col, err := parseColor(c.Stopped, "stopped"); err != nil {
		return Theme{}, err
	} else if c.Stopped != nil {
		t.Stopped = col
	}
	if col, err := parseColor(c.Pending, "pending"); err != nil {
		return Theme{}, err
	} else if c.Pending != nil {
		t.Pending = col
	}
	if col, err := parseColor(c.Terminated, "terminated"); err != nil {
		return Theme{}, err
	} else if c.Terminated != nil {
		t.Terminated = col
	}
	if col, err := parseColor(c.DetailKey, "detail_key"); err != nil {
		return Theme{}, err
	} else if c.DetailKey != nil {
		t.DetailKey = col
	}
	if col, err := parseColor(c.DetailVal, "detail_val"); err != nil {
		return Theme{}, err
	} else if c.DetailVal != nil {
		t.DetailVal = col
	}
	if col, err := parseColor(c.DetailSec, "detail_sec"); err != nil {
		return Theme{}, err
	} else if c.DetailSec != nil {
		t.DetailSec = col
	}
	if col, err := parseColor(c.YAMLKey, "yaml_key"); err != nil {
		return Theme{}, err
	} else if c.YAMLKey != nil {
		t.YAMLKey = col
	}
	if col, err := parseColor(c.YAMLStr, "yaml_str"); err != nil {
		return Theme{}, err
	} else if c.YAMLStr != nil {
		t.YAMLStr = col
	}
	if col, err := parseColor(c.YAMLNum, "yaml_num"); err != nil {
		return Theme{}, err
	} else if c.YAMLNum != nil {
		t.YAMLNum = col
	}
	if col, err := parseColor(c.YAMLBool, "yaml_bool"); err != nil {
		return Theme{}, err
	} else if c.YAMLBool != nil {
		t.YAMLBool = col
	}
	if col, err := parseColor(c.YAMLNull, "yaml_null"); err != nil {
		return Theme{}, err
	} else if c.YAMLNull != nil {
		t.YAMLNull = col
	}
	if col, err := parseColor(c.YAMLTree, "yaml_tree"); err != nil {
		return Theme{}, err
	} else if c.YAMLTree != nil {
		t.YAMLTree = col
	}
	if col, err := parseColor(c.HelpKey, "help_key"); err != nil {
		return Theme{}, err
	} else if c.HelpKey != nil {
		t.HelpKey = col
	}
	if col, err := parseColor(c.HelpCat, "help_cat"); err != nil {
		return Theme{}, err
	} else if c.HelpCat != nil {
		t.HelpCat = col
	}
	if col, err := parseColor(c.Filter, "filter"); err != nil {
		return Theme{}, err
	} else if c.Filter != nil {
		t.Filter = col
	}
	if col, err := parseColor(c.Success, "success"); err != nil {
		return Theme{}, err
	} else if c.Success != nil {
		t.Success = col
	}
	if col, err := parseColor(c.Error, "error"); err != nil {
		return Theme{}, err
	} else if c.Error != nil {
		t.Error = col
	}
	if col, err := parseColor(c.Spinner, "spinner"); err != nil {
		return Theme{}, err
	} else if c.Spinner != nil {
		t.Spinner = col
	}
	if col, err := parseColor(c.Scroll, "scroll"); err != nil {
		return Theme{}, err
	} else if c.Scroll != nil {
		t.Scroll = col
	}
	if col, err := parseColor(c.Warning, "warning"); err != nil {
		return Theme{}, err
	} else if c.Warning != nil {
		t.Warning = col
	}
	if col, err := parseColor(c.KeyHintKey, "key_hint_key"); err != nil {
		return Theme{}, err
	} else if c.KeyHintKey != nil {
		t.KeyHintKey = col
	}
	if col, err := parseColor(c.KeyHintBg, "key_hint_bg"); err != nil {
		return Theme{}, err
	} else if c.KeyHintBg != nil {
		t.KeyHintBg = col
	}
	if col, err := parseColor(c.KeyHintFg, "key_hint_fg"); err != nil {
		return Theme{}, err
	} else if c.KeyHintFg != nil {
		t.KeyHintFg = col
	}
	if col, err := parseColor(c.OverlayBg, "overlay_bg"); err != nil {
		return Theme{}, err
	} else if c.OverlayBg != nil {
		t.OverlayBg = col
	}
	if col, err := parseColor(c.OverlayBorder, "overlay_border"); err != nil {
		return Theme{}, err
	} else if c.OverlayBorder != nil {
		t.OverlayBorder = col
	}
	if col, err := parseColor(c.SearchHighlightFg, "search_highlight_fg"); err != nil {
		return Theme{}, err
	} else if c.SearchHighlightFg != nil {
		t.SearchHighlightFg = col
	}
	if col, err := parseColor(c.SearchHighlightBg, "search_highlight_bg"); err != nil {
		return Theme{}, err
	} else if c.SearchHighlightBg != nil {
		t.SearchHighlightBg = col
	}

	return t, nil
}
