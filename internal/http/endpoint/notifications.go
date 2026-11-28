package endpoint

import (
	"net/http"
	"strconv"

	"github.com/gi8lino/keycloak-warden/pkg/model"
)

// Notifications displays retained incident deliveries and their current status.
func Notifications(api API, pages *Pages) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		values, before, err := parseCursorQuery(r.URL.RawQuery, "before-id", "notice")
		if err != nil {
			pages.Problem(r, w, http.StatusBadRequest, "Invalid notification page.")
			return
		}

		result, err := api.ListNotifications(r.Context(), before)
		if err != nil {
			pages.fail(r, w, currentUser(r), err)
			return
		}

		var notice string
		if values.Get("notice") == "retry-queued" {
			notice = "Notification retry queued. Delivery has not yet been acknowledged."
		}

		pages.render(r, w, http.StatusOK, pageData{
			Title:         "Notifications",
			Page:          "notifications",
			User:          currentUser(r).Name,
			Notice:        notice,
			Notifications: result,
			Cursor:        cursorLinks(pages.prefix, "/notifications", "before-id", result.NextBeforeID, values),
		})
	}
}

// NotificationAttempts displays completed sends for a single notification.
func NotificationAttempts(api API, pages *Pages) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || id < 1 {
			pages.Problem(r, w, http.StatusBadRequest, "Invalid notification ID.")
			return
		}

		values, before, err := parseCursorQuery(r.URL.RawQuery, "before-number")
		if err != nil {
			pages.Problem(r, w, http.StatusBadRequest, "Invalid attempt page.")
			return
		}

		result, err := api.NotificationAttempts(r.Context(), id, before)
		if err != nil {
			pages.fail(r, w, currentUser(r), err)
			return
		}

		path := "/notifications/" + strconv.FormatInt(id, 10) + "/attempts"
		pages.render(r, w, http.StatusOK, pageData{
			Title:                "Notification attempts",
			Page:                 "notification-attempts",
			User:                 currentUser(r).Name,
			NotificationID:       id,
			NotificationAttempts: result,
			Cursor:               cursorLinks(pages.prefix, path, "before-number", result.NextBeforeNumber, values),
		})
	}
}

// RetryNotification requests a fresh attempt budget using the verified operator identity.
func RetryNotification(api API, pages *Pages) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || id < 1 {
			pages.Problem(r, w, http.StatusBadRequest, "Invalid notification ID.")
			return
		}
		if err := parseForm(w, r); err != nil {
			pages.Problem(r, w, http.StatusBadRequest, "Invalid retry form.")
			return
		}

		request := model.NotificationRetryRequest{Actor: currentUser(r).Name}
		err = pages.auditAction(r, actionNotificationRetry, func() error {
			return api.RetryNotification(r.Context(), id, request)
		}, "notification_id", id)
		if err != nil {
			pages.fail(r, w, currentUser(r), err)
			return
		}

		pages.redirect(w, r, "/notifications?notice=retry-queued")
	}
}
