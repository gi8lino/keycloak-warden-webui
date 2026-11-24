package endpoint

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	appkitauth "github.com/containeroo/appkit/auth"
	"github.com/gi8lino/keycloak-warden/pkg/client"
	"github.com/gi8lino/warden-webui/internal/http/auth"
	"github.com/gi8lino/warden-webui/web"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// auditAPI injects an outcome while asserting the request is logged before execution.
type auditAPI struct {
	// API provides the unused read operations for this endpoint test.
	API
	// err is the simulated backend outcome.
	err error
	// before inspects the request log at the backend boundary.
	before func()
}

// OptimizeDatabase returns the configured maintenance result.
func (a auditAPI) OptimizeDatabase(context.Context) (client.Maintenance, error) {
	a.before()
	return client.Maintenance{}, a.err
}

// Ban returns the configured manual decision result.
func (a auditAPI) Ban(context.Context, client.ManualBanRequest) (client.Record, error) {
	a.before()
	return client.Record{}, a.err
}

// Unban returns the configured source-release result.
func (a auditAPI) Unban(context.Context, string) (client.Record, error) {
	a.before()
	return client.Record{}, a.err
}

func TestAdministrativeActionLogs(t *testing.T) {
	t.Parallel()
	t.Run("successful maintenance is attributed to authenticated user", func(t *testing.T) {
		t.Parallel()
		assertActionLogs(t, actionOptimize, nil)
	})
	t.Run("failed maintenance is never logged as completed", func(t *testing.T) {
		t.Parallel()
		assertActionLogs(t, actionOptimize, errors.New("backend unavailable"))
	})
	t.Run("manual ban is audited", func(t *testing.T) {
		t.Parallel()
		assertActionLogs(t, actionBan, nil)
	})
	t.Run("manual release is audited", func(t *testing.T) {
		t.Parallel()
		assertActionLogs(t, actionUnban, nil)
	})
}

// assertActionLogs checks actual endpoint outcomes and excludes form secrets from audit records.
func assertActionLogs(t *testing.T, operation administrativeAction, failure error) {
	t.Helper()
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	pages, err := NewPages(web.Assets, "", appkitauth.ModeLocal, logger, nil, "", "test")
	require.NoError(t, err)
	api := auditAPI{err: failure, before: func() {
		assert.Contains(t, output.String(), `"event":"admin_action_requested"`)
		assert.NotContains(t, output.String(), "admin_action_completed")
	}}
	body := url.Values{"ip": {"203.0.113.1"}, "duration": {"1h"}, "reason": {"private reason"}}
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Authorization", "Bearer private-token")
	request = auth.WithUser(request, auth.User{Name: "operator", Local: true})
	response := httptest.NewRecorder()
	switch operation {
	case actionBan:
		CreateBan(api, pages)(response, request)
		assert.Contains(t, output.String(), `"ip_address":"203.0.113.1"`)
	case actionUnban:
		request.SetPathValue("id", "decision-one")
		ReleaseBan(api, pages)(response, request)
		assert.Contains(t, output.String(), `"decision_id":"decision-one"`)
	default:
		request.SetPathValue("operation", "optimize")
		RunMaintenance(api, pages)(response, request)
	}
	assert.Contains(t, output.String(), `"actor":"operator"`)
	assert.Contains(t, output.String(), `"operation":"`+string(operation)+`"`)
	assert.Contains(t, output.String(), `"duration_ms":`)
	assert.NotContains(t, output.String(), "private reason")
	assert.NotContains(t, output.String(), "private-token")
	if failure != nil {
		assert.Equal(t, http.StatusBadGateway, response.Code)
		assert.Contains(t, output.String(), `"event":"admin_action_failed"`)
		assert.NotContains(t, output.String(), "admin_action_completed")
		return
	}
	assert.Equal(t, http.StatusSeeOther, response.Code)
	assert.Contains(t, output.String(), `"event":"admin_action_completed"`)
	assert.Contains(t, output.String(), `"level":"INFO"`)
}

func TestAuditActionReturnsOriginalError(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	pages := &Pages{logger: slog.New(slog.NewJSONHandler(&output, nil))}
	request := httptest.NewRequest(http.MethodPost, "/maintenance/optimize", nil)
	failure := errors.New("backend unavailable")
	calls := 0

	err := pages.auditAction(request, actionOptimize, func() error {
		calls++
		assert.Contains(t, output.String(), "admin_action_requested")
		assert.NotContains(t, output.String(), "admin_action_failed")
		return failure
	})

	assert.Same(t, failure, err)
	assert.Equal(t, 1, calls)
	assert.Contains(t, output.String(), "admin_action_failed")
	assert.NotContains(t, output.String(), "admin_action_completed")
}
