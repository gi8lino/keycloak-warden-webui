package endpoint

import "net/http"

// NotFound renders an application page for unknown routes without requiring sign-in.
func NotFound(pages *Pages) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pages.render(r, w, http.StatusNotFound, pageData{
			Title: "Page not found",
			Page:  "not-found",
		})
	}
}
