package resource

import "github.com/k2m30/a9s/v3/core/config"

// ResolveListColumnCascade resolves the list column set for typeName: prefer
// vc's per-session ViewDef.List when non-empty, else the built-in default
// ViewDef.List when it is a strict superset of td.Columns (guarded by a
// first-column-title match so a custom td with a different layout is not
// silently switched to the built-in defaults), else td.Columns (carrying
// Path/Humanize from the defaults by title match), else the raw built-in
// defaults.
//
// Single source of truth for the column-set cascade: internal/app's
// resolveListColumnsForBuild (session-view-config-aware, used for rendering)
// and internal/runtime's resolveSaveColumns (always vc=nil, used for the
// cache-save fallback lane) both delegate here so the two packages can never
// disagree about which columns a resource type renders.
func ResolveListColumnCascade(vc *config.ViewsConfig, typeName string, td *ResourceTypeDef) []config.ListColumn {
	if vc != nil {
		vd := config.GetViewDef(vc, typeName)
		if len(vd.List) > 0 {
			cols := make([]config.ListColumn, len(vd.List))
			for i, lc := range vd.List {
				cols[i] = config.ListColumn{Key: lc.Key, Title: lc.Title, Width: lc.Width, Path: lc.Path, Humanize: lc.Humanize}
			}
			return cols
		}
	}

	defaultVD := config.GetViewDef(nil, typeName)

	if td != nil && len(defaultVD.List) > len(td.Columns) {
		firstMatch := len(td.Columns) == 0 ||
			(len(defaultVD.List) > 0 && defaultVD.List[0].Title == td.Columns[0].Title)
		if firstMatch {
			cols := make([]config.ListColumn, len(defaultVD.List))
			for i, lc := range defaultVD.List {
				cols[i] = config.ListColumn{Key: lc.Key, Title: lc.Title, Width: lc.Width, Path: lc.Path, Humanize: lc.Humanize}
			}
			return cols
		}
	}

	if td != nil && len(td.Columns) > 0 {
		defaultByTitle := make(map[string]config.ListColumn, len(defaultVD.List))
		for _, lc := range defaultVD.List {
			defaultByTitle[lc.Title] = lc
		}
		cols := make([]config.ListColumn, len(td.Columns))
		for i, c := range td.Columns {
			cd := config.ListColumn{Key: c.Key, Title: c.Title, Width: c.Width}
			if def, ok := defaultByTitle[c.Title]; ok {
				cd.Path = def.Path
				cd.Humanize = def.Humanize
			}
			cols[i] = cd
		}
		return cols
	}

	if len(defaultVD.List) > 0 {
		cols := make([]config.ListColumn, len(defaultVD.List))
		for i, lc := range defaultVD.List {
			cols[i] = config.ListColumn{Key: lc.Key, Title: lc.Title, Width: lc.Width, Path: lc.Path}
		}
		return cols
	}
	return nil
}
