package middleware

import (
	"net/http"
	"net/url"

	httpkitmiddleware "github.com/containeroo/httpkit/middleware"
)

// SecurityHeaders prevents embedding, sniffing, and loading unintended third-party resources.
func SecurityHeaders() httpkitmiddleware.Middleware {
	return httpkitmiddleware.SecurityHeaders(http.Header{
		"Content-Security-Policy": {"default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self'; form-action 'self'; base-uri 'none'; frame-ancestors 'none'"},
		"X-Content-Type-Options":  {"nosniff"},
		"Referrer-Policy":         {"same-origin"},
		"X-Frame-Options":         {"DENY"},
		"Cache-Control":           {"no-store"},
	})
}

// RejectCrossSiteWrites requires an exact public origin for state-changing browser forms.
// A missing or malformed Origin and Referer fails closed.
func RejectCrossSiteWrites(publicURL string) httpkitmiddleware.Middleware {
	expected, _ := url.Parse(publicURL)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isReadOnlyMethod(r.Method) {
				next.ServeHTTP(w, r)
				return
			}

			if requestOrigin(r) != expected.Scheme+"://"+expected.Host {
				http.Error(w, "Cross-origin form submission denied.", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// isReadOnlyMethod identifies methods that do not require a browser write-origin check.
func isReadOnlyMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions
}

// requestOrigin prefers Origin and falls back to a credential-free Referer.
func requestOrigin(r *http.Request) string {
	if origin := r.Header.Get("Origin"); origin != "" {
		return origin
	}
	referer, err := url.Parse(r.Header.Get("Referer"))
	if err != nil || referer.User != nil {
		return ""
	}
	return referer.Scheme + "://" + referer.Host
}
