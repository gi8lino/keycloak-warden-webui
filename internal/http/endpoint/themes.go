package endpoint

import (
	"context"
	"net/http"
	"text/template"

	"github.com/gi8lino/warden-webui/themes"
)

// accountTheme selects the account preference, falling back when a custom theme has been removed.
func (p *Pages) accountTheme(ctx context.Context, name string) themes.Theme {
	if name != "" && p.store != nil {
		title, err := p.store.Theme(ctx, name)
		if err != nil {
			p.logger.Error("read theme preference", "event", "theme_read_failed", "error", err)
		}
		if theme, ok := themes.Find(p.catalog, title); ok {
			return theme
		}
	}
	fallback, _ := themes.Find(p.catalog, themes.DefaultTheme)
	return fallback
}

// SaveTheme records a catalog selection for the authenticated local account.
func (p *Pages) SaveTheme(w http.ResponseWriter, r *http.Request) {
	if err := parseForm(w, r); err != nil {
		http.Error(w, "Invalid theme form.", 400)
		return
	}
	theme, ok := themes.Find(p.catalog, r.PostForm.Get("theme"))
	if !ok {
		http.Error(w, "Unknown theme.", 400)
		return
	}
	if err := p.store.SaveTheme(r.Context(), currentUser(r).Name, theme.Title); err != nil {
		http.Error(w, "Unable to save theme.", 500)
		return
	}
	p.redirect(w, r, "/preferences")
}

// ThemeStylesheet serves only validated catalog colors under the existing same-origin style policy.
func (p *Pages) ThemeStylesheet(w http.ResponseWriter, r *http.Request) {
	theme, ok := themes.Find(p.catalog, r.URL.Query().Get("name"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	if err := palette.Execute(w, theme); err != nil {
		p.logger.Error("write theme stylesheet", "event", "theme_write_failed", "error", err)
	}
}

var palette = template.Must(template.New("palette").Parse(`
:root {
 color-scheme: {{.ColorScheme}};
 {{with .Colors}}
 --color-zinc-950: {{.Background}};
 --color-zinc-900: {{.Surface}};
 --color-zinc-800: {{.SurfaceElevated}};
 --color-zinc-700: {{.BorderStrong}};
 --color-zinc-600: {{.Muted}};
 --color-zinc-500: {{.TextTertiary}};
 --color-zinc-400: {{.TextSecondary}};
 --color-zinc-300: {{.TextSecondary}};
 --color-zinc-200: {{.Text}};
 --color-zinc-100: {{.Text}};
 --color-white: {{.Text}};
 --color-cyan-500: {{.Accent}};
 --color-cyan-400: {{.AccentSecondary}};
 --color-cyan-300: {{.AccentSoft}};
 --color-red-200: {{.Error}};
 --color-red-400: {{.Danger}};
 --color-red-900: {{.Danger}};
 --color-red-950: {{.Error}};
 --color-amber-200: {{.Warning}};
 --color-amber-900: {{.Warning}};
 --color-amber-950: {{.Warning}};
 --color-emerald-200: {{.Success}};
 --color-emerald-900: {{.Success}};
 --color-emerald-950: {{.Success}};
}
.border-zinc-800 {border-color: {{.Border}}}
.text-zinc-950 {color: {{.SelectionText}}}
@media (hover:hover) { .hover\:bg-zinc-800:hover {background-color: {{.SurfaceHover}}} }
::selection {color: {{.SelectionText}};background: {{.SelectionBackground}}}
{{end}}`))
