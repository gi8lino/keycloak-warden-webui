package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRejectCrossSiteWrites(t *testing.T) {
	t.Parallel()

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	t.Run("read without origin", func(t *testing.T) {
		t.Parallel()

		request := httptest.NewRequest(http.MethodGet, "https://warden.example/ui/bans", nil)
		recorder := httptest.NewRecorder()

		RejectCrossSiteWrites("https://warden.example/ui")(next).ServeHTTP(recorder, request)

		assert.Equal(t, http.StatusNoContent, recorder.Code)
	})

	t.Run("same origin", func(t *testing.T) {
		t.Parallel()

		request := httptest.NewRequest(http.MethodPost, "https://warden.example/ui/bans", nil)
		request.Header.Set("Origin", "https://warden.example")
		recorder := httptest.NewRecorder()

		RejectCrossSiteWrites("https://warden.example/ui")(next).ServeHTTP(recorder, request)

		assert.Equal(t, http.StatusNoContent, recorder.Code)
	})

	t.Run("referer fallback", func(t *testing.T) {
		t.Parallel()

		request := httptest.NewRequest(http.MethodPost, "https://warden.example/ui/bans", nil)
		request.Header.Set("Referer", "https://warden.example/ui/bans")
		recorder := httptest.NewRecorder()

		RejectCrossSiteWrites("https://warden.example/ui")(next).ServeHTTP(recorder, request)

		assert.Equal(t, http.StatusNoContent, recorder.Code)
	})

	t.Run("origin takes precedence", func(t *testing.T) {
		t.Parallel()

		request := httptest.NewRequest(http.MethodPost, "https://warden.example/ui/bans", nil)
		request.Header.Set("Origin", "https://other.example")
		request.Header.Set("Referer", "https://warden.example/ui/bans")
		recorder := httptest.NewRecorder()

		RejectCrossSiteWrites("https://warden.example/ui")(next).ServeHTTP(recorder, request)

		assert.Equal(t, http.StatusForbidden, recorder.Code)
	})

	t.Run("credentialed referer", func(t *testing.T) {
		t.Parallel()

		request := httptest.NewRequest(http.MethodPost, "https://warden.example/ui/bans", nil)
		request.Header.Set("Referer", "https://user@warden.example/ui/bans")
		recorder := httptest.NewRecorder()

		RejectCrossSiteWrites("https://warden.example/ui")(next).ServeHTTP(recorder, request)

		assert.Equal(t, http.StatusForbidden, recorder.Code)
	})

	t.Run("malformed referer", func(t *testing.T) {
		t.Parallel()

		request := httptest.NewRequest(http.MethodPost, "https://warden.example/ui/bans", nil)
		request.Header.Set("Referer", "://invalid")
		recorder := httptest.NewRecorder()

		RejectCrossSiteWrites("https://warden.example/ui")(next).ServeHTTP(recorder, request)

		assert.Equal(t, http.StatusForbidden, recorder.Code)
	})

	t.Run("missing origin", func(t *testing.T) {
		t.Parallel()

		request := httptest.NewRequest(http.MethodPost, "https://warden.example/ui/bans", nil)
		recorder := httptest.NewRecorder()

		RejectCrossSiteWrites("https://warden.example/ui")(next).ServeHTTP(recorder, request)

		assert.Equal(t, http.StatusForbidden, recorder.Code)
	})

	t.Run("different scheme", func(t *testing.T) {
		t.Parallel()

		request := httptest.NewRequest(http.MethodPost, "https://warden.example/ui/bans", nil)
		request.Header.Set("Origin", "http://warden.example")
		recorder := httptest.NewRecorder()

		RejectCrossSiteWrites("https://warden.example/ui")(next).ServeHTTP(recorder, request)

		assert.Equal(t, http.StatusForbidden, recorder.Code)
	})
}

func TestSecurityHeaders(t *testing.T) {
	t.Parallel()

	t.Run("allows same-origin branding images", func(t *testing.T) {
		t.Parallel()

		handler := SecurityHeaders()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}))
		request := httptest.NewRequest(http.MethodGet, "/static/assets/warden-webui.svg", nil)
		recorder := httptest.NewRecorder()

		handler.ServeHTTP(recorder, request)

		policy := recorder.Header().Get("Content-Security-Policy")

		assert.Equal(t, http.StatusNoContent, recorder.Code)
		assert.Contains(t, policy, "default-src 'none'")
		assert.Contains(t, policy, "img-src 'self'")
		assert.NotContains(t, policy, "img-src *")
		assert.NotContains(t, policy, "img-src https:")
	})
}
