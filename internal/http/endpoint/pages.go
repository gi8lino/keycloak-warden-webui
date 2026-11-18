// Package endpoint provides Warden Web UI HTTP handler factories.
package endpoint

import (
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gi8lino/warden-webui/themes"

	appkitauth "github.com/containeroo/appkit/auth"
	"github.com/gi8lino/keycloak-warden/pkg/client"
	"github.com/gi8lino/warden-webui/internal/db"
	"github.com/gi8lino/warden-webui/internal/http/auth"
)

// Pages renders server-controlled data into escaped administrator HTML pages.
type Pages struct {
	// version is the immutable build version displayed in the footer.
	version string
	// catalog lists validated palettes available to all accounts.
	catalog []themes.Theme
	// store persists account appearance preferences.
	store *db.Store
	// templates is the validated HTML template set.
	templates *template.Template
	// prefix is the optional application route prefix.
	prefix string
	// mode identifies the configured authentication mechanism.
	mode appkitauth.Mode
	// logger records unexpected rendering and upstream failures.
	logger *slog.Logger
}

// pageData supplies metadata and view data for one rendered HTML page.
type pageData struct {
	// Version identifies the running Web UI build.
	Version string
	// Theme identifies the palette applied to this page.
	Theme themes.Theme
	// Themes lists choices for the preferences form.
	Themes []themes.Theme
	// Prefix is the application path prefix used by template links and forms.
	Prefix string
	// Title is the browser and page heading.
	Title string
	// Page selects one named template view.
	Page string
	// User is the authenticated administrator's display label.
	User string
	// Mode is the configured browser authentication mechanism.
	Mode appkitauth.Mode
	// Notice is a server-controlled success message.
	Notice string
	// Error is a safe operator-facing error message.
	Error string
	// Bans are Warden's current active decisions.
	Bans []client.Record
	// Ban is the selected persisted decision.
	Ban client.Record
	// LocalUser permits local account actions only for local sessions.
	LocalUser bool
	// Users lists local accounts on the administration page.
	Users []db.LocalUser
	// Links shows the verified identities assigned to accounts.
	Links []db.IdentityLink
	// Pending contains verified subjects awaiting a local account assignment.
	Pending []string
	// Maintenance contains the maintenance scheduler state.
	Maintenance client.Maintenance
}

// NewPages parses HTML templates once during startup.
func NewPages(
	assets fs.FS,
	prefix string,
	mode appkitauth.Mode,
	logger *slog.Logger,
	store *db.Store,
	themesDir string,
	version string,
) (*Pages, error) {
	templates, err := template.New("base.html").
		Funcs(template.FuncMap{
			"fmtTime": formatTime,
			"pending": func(acknowledged, revision int64) bool { return acknowledged < revision },
		}).ParseFS(assets, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parse HTML templates: %w", err)
	}

	catalog, err := themes.Load(themes.Files, themesDir)
	if err != nil {
		return nil, err
	}

	return &Pages{
		version:   version,
		catalog:   catalog,
		store:     store,
		templates: templates,
		prefix:    prefix,
		mode:      mode,
		logger:    logger,
	}, nil
}

// render executes one escaped template and attaches navigation metadata.
func (p *Pages) render(r *http.Request, w http.ResponseWriter, status int, data pageData) {
	data.Theme = p.accountTheme(r.Context(), data.User)
	data.Themes = p.catalog
	w.Header().Set("Cache-Control", "private, no-store")
	data.Version = p.version
	data.Prefix = p.prefix
	data.Mode = p.mode
	user, _ := auth.CurrentUser(r)
	data.LocalUser = user.Local

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	w.WriteHeader(status)

	if err := p.templates.ExecuteTemplate(w, "base.html", data); err != nil {
		p.logger.Error(
			"render HTML failed",
			"event", "render_html_failed",
			"error", err,
		)
	}
}

// redirect sends a prefix-free application path; the Appkit mount rewrites Location.
func (p *Pages) redirect(w http.ResponseWriter, r *http.Request, path string) {
	http.Redirect(w, r, path, http.StatusSeeOther)
}

// currentUser gets the principal already verified by the authenticated-route middleware.
func currentUser(r *http.Request) auth.User {
	user, _ := auth.CurrentUser(r)
	return user
}

// parseForm bounds the size of a browser form before parsing it.
func parseForm(w http.ResponseWriter, r *http.Request) error {
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	return r.ParseForm()
}

// validID ensures downstream decision IDs occupy one safe URL path segment.
func validID(id string) bool {
	if id == "" || id == "." || id == ".." || len(id) > 128 {
		return false
	}
	return !strings.ContainsAny(id, "/%?#\\")
}

// formatTime presents UTC timestamps with an unambiguous timezone.
func formatTime(value any) string {
	var timestamp time.Time
	switch v := value.(type) {
	case time.Time:
		timestamp = v
	case *time.Time:
		if v != nil {
			timestamp = *v
		}
	}
	if timestamp.IsZero() {
		return "—"
	}
	return timestamp.UTC().Format("2006-01-02 15:04:05 UTC")
}
