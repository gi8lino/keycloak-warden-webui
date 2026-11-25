package endpoint

import (
	"cmp"
	"errors"
	"net/url"
	"slices"
	"strconv"

	"github.com/gi8lino/keycloak-warden/pkg/client"
	"github.com/gi8lino/keycloak-warden/pkg/deliverystate"
)

const (
	historyPageParam            = "history-page"
	historyThroughRevisionParam = "history-through-revision"
)

// deliveryHistoryView separates current delivery progress from paginated history.
type deliveryHistoryView struct {
	// Page identifies the currently displayed history page.
	Page int
	// Previous links to the preceding page within the same revision boundary.
	Previous string
	// Next links to the next older page within the same revision boundary.
	Next string
	// Latest refreshes history without retaining an old revision boundary.
	Latest string
	// Current combines known destinations and their latest delivery progress.
	Current []deliveryDestinationView
	// Entries are the selected historical groups, independent of the current decision.
	Entries []client.Delivery
}

// parseHistoryQuery validates the supported pagination selectors before contacting Warden.
func parseHistoryQuery(raw string) (page int, through int64, err error) {
	values, err := url.ParseQuery(raw)
	if err != nil {
		return 0, 0, err
	}

	page = 1
	for key, entries := range values {
		if len(entries) != 1 {
			return 0, 0, errors.New("duplicate history selector")
		}

		switch key {
		case historyPageParam:
			page, err = strconv.Atoi(entries[0])
			if err != nil || page < 1 {
				return 0, 0, errors.New("invalid history page")
			}
		case historyThroughRevisionParam:
			through, err = strconv.ParseInt(entries[0], 10, 64)
			if err != nil || through < 1 {
				return 0, 0, errors.New("invalid history revision")
			}
		default:
			return 0, 0, errors.New("unsupported history selector")
		}
	}

	return page, through, nil
}

// historyView builds prefix-aware links while keeping the traversal's revision boundary stable.
func historyView(
	prefix, id string,
	page int,
	through int64,
	next int,
	current []deliveryDestinationView,
	entries []client.Delivery,
) deliveryHistoryView {
	path := prefix + "/bans/" + url.PathEscape(id)
	view := deliveryHistoryView{
		Page:    page,
		Latest:  path + "#delivery-history",
		Current: current,
		Entries: entries,
	}

	if page > 1 {
		view.Previous = historyPageURL(path, page-1, through)
	}

	if next > 0 {
		view.Next = historyPageURL(path, next, through)
	}

	return view
}

// historyPageURL encodes selectors without interpolating unchecked query strings into links.
func historyPageURL(path string, page int, through int64) string {
	query := url.Values{historyPageParam: {strconv.Itoa(page)}}
	if through > 0 {
		query.Set(historyThroughRevisionParam, strconv.FormatInt(through, 10))
	}
	return path + "?" + query.Encode() + "#delivery-history"
}

// deliveryStatusView keeps typed delivery progress separate from missing history.
type deliveryStatusView struct {
	// Status is Warden's delivery lifecycle value when known.
	Status deliverystate.Status
	// Known distinguishes available progress from an unacknowledged revision with no history.
	Known bool
}

// deliveryDestinationView combines acknowledgement and current status even when
// a destination has never acknowledged a revision.
type deliveryDestinationView struct {
	// Destination identifies Warden's configured target.
	Destination string
	// AcknowledgedRevision is the last confirmed revision, if any.
	AcknowledgedRevision int64
	// HasAcknowledgement distinguishes a missing acknowledgement from revision zero.
	HasAcknowledgement bool
	// Progress reports current delivery state only when supported by Warden's data.
	Progress deliveryStatusView
}

// deliveryStatus reports known current progress without guessing from older attempts.
func deliveryStatus(ack client.Acknowledgement, revision int64, history []client.Delivery) deliveryStatusView {
	if ack.Revision > 0 && ack.Revision >= revision {
		return deliveryStatusView{
			Status: deliverystate.StatusDelivered,
			Known:  true,
		}
	}

	for _, delivery := range history {
		if delivery.Destination == ack.Destination && delivery.Revision == revision {
			return deliveryStatusView{
				Status: delivery.Status,
				Known:  delivery.Status.Valid(),
			}
		}
	}

	return deliveryStatusView{}
}

// currentDeliveryViews merges acknowledgements with the complete current-revision
// status snapshot, independently of paginated delivery attempts.
// Older history is never interpreted as evidence of current delivery progress.
func currentDeliveryViews(record client.Record) []deliveryDestinationView {
	views := make(map[string]deliveryDestinationView)

	for _, ack := range record.Deliveries {
		views[ack.Destination] = deliveryDestinationView{
			Destination:          ack.Destination,
			AcknowledgedRevision: ack.Revision,
			HasAcknowledgement:   true,
			Progress:             deliveryStatus(ack, record.Decision.Revision, record.CurrentDeliveries),
		}
	}

	for _, entry := range record.CurrentDeliveries {
		if entry.Revision != record.Decision.Revision {
			continue
		}
		view, exists := views[entry.Destination]
		if !exists {
			view = deliveryDestinationView{
				Destination: entry.Destination,
				Progress: deliveryStatus(
					client.Acknowledgement{Destination: entry.Destination},
					record.Decision.Revision,
					record.CurrentDeliveries,
				),
			}
		}
		// Completed releases may have had their delivery queue rows removed.
		// A delivered history status remains evidence of an accepted revision.
		if entry.Status == deliverystate.StatusDelivered && view.AcknowledgedRevision < entry.Revision {
			view.AcknowledgedRevision = entry.Revision
			view.HasAcknowledgement = true
			view.Progress = deliveryStatus(client.Acknowledgement{
				Destination: entry.Destination,
				Revision:    entry.Revision,
			}, record.Decision.Revision, nil)
		}
		views[entry.Destination] = view
	}

	result := make([]deliveryDestinationView, 0, len(views))
	for _, view := range views {
		result = append(result, view)
	}

	slices.SortFunc(result, func(a, b deliveryDestinationView) int {
		return cmp.Compare(a.Destination, b.Destination)
	})

	return result
}

// newestAttempts returns a reversed copy so recent outcomes appear first without mutating API data.
func newestAttempts(attempts []client.DeliveryAttempt) []client.DeliveryAttempt {
	result := slices.Clone(attempts)
	slices.Reverse(result)
	return result
}
