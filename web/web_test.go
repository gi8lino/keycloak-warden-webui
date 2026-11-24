package web

import (
	"html/template"
	"io/fs"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmbeddedAssets(t *testing.T) {
	t.Parallel()

	t.Run("contains every page and icon template", func(t *testing.T) {
		t.Parallel()

		templates, err := template.New("base.html").Funcs(template.FuncMap{
			"fmtTime":        func(any) string { return "" },
			"newestAttempts": func(v any) any { return v },
		}).ParseFS(Assets, "templates/*.html")
		require.NoError(t, err, "parse embedded templates")
		for _, name := range []string{"account-menu", "users", "password", "dashboard", "base.html", "maintenance", "login", "setup", "error", "not-found", "detail", "delivery-history", "delivery-status"} {
			assert.NotNil(t, templates.Lookup(name))
		}
	})

	t.Run("embeds compiled Tailwind utilities", func(t *testing.T) {
		t.Parallel()

		css, err := fs.ReadFile(Assets, "static/css/style.css")
		require.NoError(t, err)
		assert.Contains(t, string(css), ".bg-zinc-950", "embedded stylesheet is missing Tailwind background utilities")
	})

	t.Run("embeds progressive loading behavior", func(t *testing.T) {
		t.Parallel()

		js, err := fs.ReadFile(Assets, "static/js/ui.js")
		require.NoError(t, err)
		assert.Contains(t, string(js), "page-loading", "embedded loading behavior is missing")
	})

	t.Run("embeds the SVG logo", func(t *testing.T) {
		t.Parallel()

		logo, err := fs.ReadFile(Assets, "static/assets/warden-webui.svg")
		require.NoError(t, err)
		assert.Contains(t, string(logo), "<svg", "embedded Warden WebUI logo is not SVG")
	})

	t.Run("embeds generated favicon images", func(t *testing.T) {
		t.Parallel()

		icon16, err := fs.ReadFile(Assets, "static/assets/favicon-16x16.png")
		require.NoError(t, err)
		assert.NotEmpty(t, icon16, "embedded 16x16 favicon is empty")

		icon32, err := fs.ReadFile(Assets, "static/assets/favicon-32x32.png")
		require.NoError(t, err)
		assert.NotEmpty(t, icon32, "embedded 32x32 favicon is empty")

		apple, err := fs.ReadFile(Assets, "static/assets/apple-touch-icon.png")
		require.NoError(t, err)
		assert.NotEmpty(t, apple, "embedded Apple touch icon is empty")
	})
}
