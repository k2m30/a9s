# QA-12: Context-Sensitive Help Screen

---

## CONTEXT-SENSITIVE HELP

### HC-01: Help from main menu shows main menu keys

**Given** the main menu is displayed
**When** I press `?`
**Then** the help screen shows these key categories:
  - NAVIGATION: `j`/`k` (up/down), `g` (top), `G` (bottom)
  - ACTIONS: `enter` (select), `/` (filter), `:` (command), `q` (quit), `?` (help), `ctrl+c` (force quit)
**And** keys not relevant to main menu do NOT appear:
  - No `d` (detail), no `y` (yaml), no `c` (copy), no `h/l` (scroll cols)
  - No `N`/`S`/`A` (sort), no `pgup`/`pgdn`, no `x` (reveal)
  - No `w` (wrap), no `ctrl+r` (refresh)

### HC-02: Help from resource list shows resource list keys

**Given** a resource list is displayed (e.g., ec2-instances)
**When** I press `?`
**Then** the help screen shows these key categories:
  - NAVIGATION: `j`/`k` (up/down), `g`/`G` (top/bottom), `pgup`/`pgdn` (page up/down)
  - SCROLL: `h`/`l` (scroll columns left/right)
  - ACTIONS: `enter`/`d` (detail), `y` (yaml), `c` (copy), `/` (filter), `:` (command)
  - SORT: `N` (sort name), `S` (sort status), `A` (sort age)
  - OTHER: `ctrl+r` (refresh), `esc` (back), `q` (quit), `?` (help)
**And** keys not relevant to resource list do NOT appear:
  - No `w` (wrap toggle) — that's detail/yaml only
  - No `x` (reveal) — that's secrets only

### HC-03: Help from secrets resource list includes reveal key

**Given** a secrets resource list is displayed
**When** I press `?`
**Then** the help screen shows all resource list keys from HC-02
**And** additionally shows `x` (reveal) key
**And** "reveal" text appears in the help content

### HC-04: Help from non-secrets resource list excludes reveal key

**Given** a non-secrets resource list is displayed (e.g., ec2)
**When** I press `?`
**Then** the help screen does NOT show `x` (reveal) key
**And** "reveal" text does NOT appear in the help content

### HC-05: Help from detail view shows detail keys

**Given** a detail view is displayed
**When** I press `?`
**Then** the help screen shows:
  - SCROLL: `j`/`k` (up/down), `g`/`G` (top/bottom)
  - ACTIONS: `y` (yaml), `c` (copy), `w` (wrap toggle)
  - OTHER: `esc` (back), `?` (help)
**And** keys not relevant to detail do NOT appear:
  - No `d` (detail), no `h/l` (scroll cols), no sort keys
  - No `pgup`/`pgdn`, no `x` (reveal), no `/` (filter)
  - No `ctrl+r` (refresh), no `enter`

### HC-06: Help from YAML view shows YAML keys

**Given** a YAML view is displayed
**When** I press `?`
**Then** the help screen shows:
  - SCROLL: `j`/`k` (up/down), `g`/`G` (top/bottom)
  - ACTIONS: `c` (copy), `w` (wrap toggle)
  - OTHER: `esc` (back), `?` (help)
**And** keys not relevant to YAML do NOT appear:
  - No `d` (detail), no `y` (yaml), no `h/l` (scroll cols)
  - No sort keys, no `pgup`/`pgdn`, no `x` (reveal), no `/` (filter)
  - No `ctrl+r` (refresh), no `enter`

### HC-07: Help from profile/region selector shows selector keys

**Given** a profile selector or region selector is displayed
**When** I press `?`
**Then** the help screen shows:
  - NAVIGATION: `j`/`k` (up/down), `g`/`G` (top/bottom), `/` (filter)
  - ACTIONS: `enter` (select), `esc` (cancel)
  - OTHER: `?` (help)
**And** keys not relevant to selectors do NOT appear:
  - No `d` (detail), no `y` (yaml), no `c` (copy), no `h/l`
  - No sort keys, no `pgup`/`pgdn`, no `x` (reveal), no `w` (wrap)
  - No `ctrl+r` (refresh)

### HC-08: Help from reveal view shows reveal keys

**Given** a reveal view is displayed (showing a secret value)
**When** I press `?`
**Then** the help screen shows:
  - ACTIONS: `c` (copy secret), `esc` (close)
  - OTHER: `?` (help)
**And** keys not relevant to reveal do NOT appear:
  - No navigation keys, no sort keys, no `w`, no `d`, no `y`

---

## ALL KEYS COVERAGE

### HC-09: Every key in keys.go appears in at least one help context

**Given** the full set of key bindings defined in keys.go
**Then** every key appears in the help screen for at least one view context:
  - `j`/`k` (Up/Down): main menu, resource list, detail, yaml, profile, region
  - `g`/`G` (Top/Bottom): main menu, resource list, detail, yaml, profile, region
  - `enter`: main menu, resource list, profile, region
  - `esc`: all views
  - `q`: main menu, resource list
  - `ctrl+c`: main menu, resource list
  - `?`: all views
  - `ctrl+r`: resource list
  - `:`: main menu, resource list
  - `/`: main menu, resource list, profile, region
  - `d`: resource list
  - `y`: resource list, detail
  - `x`: secrets resource list only
  - `c`: resource list, detail, yaml, reveal
  - `h`/`l`: resource list
  - `N`/`S`/`A`: resource list
  - `pgup`/`pgdn`: resource list
  - `w`: detail, yaml

---

## HELP DISPLAY

### HC-10: Frame title reads "help"

**Given** help is opened from any view
**Then** the frame title is "help"

### HC-11: Any key press closes help

**Given** the help screen is displayed
**When** I press any key (letter, number, special key)
**Then** help closes and I return to the previous view

### HC-12: Help content fits within terminal width

**Given** help is opened at any terminal width >= 60 columns
**Then** no line in the help content exceeds the terminal width
**And** categories are clearly labeled with uppercase headers

---

## EDGE CASES

### HC-13: Help from narrow terminal (60 cols)

**Given** terminal width is 60 columns
**When** I open help from any view
**Then** content renders without crashing
**And** key bindings are still readable (not garbled)

### HC-14: ? on help view closes help (not help-on-help)

**Given** the help screen is displayed
**When** I press `?`
**Then** help closes (does NOT open another help screen)
**And** I return to the view I was on before opening help

### HC-15: Help preserves view context when closed

**Given** I am on a resource list with cursor at row 3
**When** I press `?` to open help, then press any key to close
**Then** I return to the resource list with cursor still at row 3
**And** any active filter is preserved
**And** scroll position is unchanged
