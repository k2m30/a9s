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

func init() {
	sub, err := fs.Sub(templateFS, "templates")
	if err != nil {
		panic("web: cannot sub templateFS: " + err.Error())
	}
	templates = template.Must(template.New("").Funcs(tmplFuncs).ParseFS(sub, "*.html"))
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

// renderBodyFragment writes only the body partial for the given Body to w.
// Used by POST /action to return the htmx-swappable fragment.
func renderBodyFragment(w io.Writer, body app.Body) error {
	return templates.ExecuteTemplate(w, "body.html", body)
}
