package web

import (
	"html/template"
	"io/fs"
	"strings"
	"testing"
)

func TestEmbeddedAssets(t *testing.T) {
	t.Parallel()

	t.Run("contains every page and icon template", func(t *testing.T) {
		t.Parallel()

		templates, err := template.New("base.html").Funcs(template.FuncMap{
			"fmtTime": func(any) string { return "" },
			"pending": func(int64, int64) bool { return false },
		}).ParseFS(Assets, "templates/*.html")
		if err != nil {
			t.Fatalf("parse embedded templates: %v", err)
		}
		for _, name := range []string{"account-menu", "users", "password", "dashboard", "base.html", "maintenance", "login", "setup", "error", "not-found", "detail"} {
			if templates.Lookup(name) == nil {
				t.Errorf("missing embedded template %q", name)
			}
		}
	})

	t.Run("embeds compiled Tailwind utilities", func(t *testing.T) {
		t.Parallel()

		css, err := fs.ReadFile(Assets, "static/css/style.css")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(css), ".bg-zinc-950") {
			t.Fatal("embedded stylesheet is missing Tailwind background utilities")
		}
	})

	t.Run("embeds progressive loading behavior", func(t *testing.T) {
		t.Parallel()

		js, err := fs.ReadFile(Assets, "static/js/ui.js")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(js), "page-loading") {
			t.Fatal("embedded loading behavior is missing")
		}
	})

	t.Run("embeds the SVG logo", func(t *testing.T) {
		t.Parallel()

		logo, err := fs.ReadFile(Assets, "static/assets/warden-webui.svg")
		if err != nil {
			t.Fatal(err)
		}

		if !strings.Contains(string(logo), "<svg") {
			t.Fatal("embedded Warden WebUI logo is not SVG")
		}
	})

	t.Run("embeds generated favicon images", func(t *testing.T) {
		t.Parallel()

		icon16, err := fs.ReadFile(Assets, "static/assets/favicon-16x16.png")
		if err != nil {
			t.Fatal(err)
		}
		if len(icon16) == 0 {
			t.Fatal("embedded 16x16 favicon is empty")
		}

		icon32, err := fs.ReadFile(Assets, "static/assets/favicon-32x32.png")
		if err != nil {
			t.Fatal(err)
		}
		if len(icon32) == 0 {
			t.Fatal("embedded 32x32 favicon is empty")
		}

		apple, err := fs.ReadFile(Assets, "static/assets/apple-touch-icon.png")
		if err != nil {
			t.Fatal(err)
		}
		if len(apple) == 0 {
			t.Fatal("embedded Apple touch icon is empty")
		}
	})
}
