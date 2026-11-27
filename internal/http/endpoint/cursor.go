package endpoint

import (
	"errors"
	"net/url"
	"slices"
	"strconv"
)

// cursorNavigation contains links to the newest and next older page.
type cursorNavigation struct {
	// Refresh reloads the newest page with the current filters.
	Refresh string
	// Next loads the next older page, when available.
	Next string
}

// parseCursorQuery validates single-valued filters and a positive exclusive cursor.
func parseCursorQuery(raw, key string, allowed ...string) (url.Values, int64, error) {
	values, err := url.ParseQuery(raw)
	if err != nil {
		return nil, 0, err
	}
	var before int64
	for name, entries := range values {
		if len(entries) != 1 {
			return nil, 0, errors.New("duplicate query parameter")
		}
		if name == key {
			before, err = strconv.ParseInt(entries[0], 10, 64)
			if err != nil || before < 1 {
				return nil, 0, errors.New("invalid cursor")
			}
		} else if !slices.Contains(allowed, name) {
			return nil, 0, errors.New("unsupported query parameter")
		}
	}
	return values, before, nil
}

// cursorLinks preserves filters while replacing the exclusive cursor.
func cursorLinks(prefix, path, key string, next int64, filters url.Values) cursorNavigation {
	values := url.Values{}
	for name, entries := range filters {
		if name != key && name != "notice" {
			values[name] = entries
		}
	}
	link := func() string {
		if encoded := values.Encode(); encoded != "" {
			return prefix + path + "?" + encoded
		}
		return prefix + path
	}
	view := cursorNavigation{Refresh: link()}
	if next > 0 {
		values.Set(key, strconv.FormatInt(next, 10))
		view.Next = link()
	}
	return view
}
