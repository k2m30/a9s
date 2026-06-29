package web

import (
	"embed"
	"html/template"
	"io"
	"io/fs"

	"github.com/k2m30/a9s/v3/internal/app"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static/*
var staticFS embed.FS

// templates holds the parsed template set, initialised once at package init.
var templates *template.Template

// staticContent is the embedded static dir rooted at "static/" so that the
// /static/ route (after StripPrefix) resolves files like app.js directly.
// Serving http.FS(staticFS) without this sub would 404: the embed roots files
// at "static/app.js" while StripPrefix turns the request into "app.js".
var staticContent fs.FS

func init() {
	sub, err := fs.Sub(templateFS, "templates")
	if err != nil {
		panic("web: cannot sub templateFS: " + err.Error())
	}
	templates = template.Must(template.New("").Funcs(tmplFuncs).ParseFS(sub, "*.html"))

	staticContent, err = fs.Sub(staticFS, "static")
	if err != nil {
		panic("web: cannot sub staticFS: " + err.Error())
	}
}

// tmplFuncs is the function map available in all templates.
var tmplFuncs = template.FuncMap{
	"add": func(a, b int) int { return a + b },
	"seq": func(n int) []int {
		s := make([]int, n)
		for i := range s {
			s[i] = i
		}
		return s
	},
	"kindStr": func(k app.BodyKind) string { return string(k) },
}

// pageData is the top-level data passed to the full-page template.
type pageData struct {
	VS    app.ViewState
	Token string
}

// renderPage writes the full HTML page for the given ViewState to w.
func renderPage(w io.Writer, vs app.ViewState, token string) error {
	return templates.ExecuteTemplate(w, "page.html", pageData{VS: vs, Token: token})
}

// renderMainFragment writes the main content fragment (frame-title + body +
// footer) for the given ViewState to w. Used by POST /action and GET /body so
// navigation swaps the title/footer too, not just the body — otherwise the
// frame title stays stale on the prior screen after navigating.
func renderMainFragment(w io.Writer, vs app.ViewState) error {
	return templates.ExecuteTemplate(w, "main.html", vs)
}
