package api

// Count answers how many lockers there are.
func Count(aepr *Request) error {
	aepr.WriteResponseAsJSON(200, nil, map[string]any{"count": 12})
	return nil
}

// A line added after the record was taken moves the next function down.

// Ping answers that the service is up.
func Ping(aepr *Request) error {
	aepr.WriteResponseAsJSON(200, nil, nil)
	return nil
}

// Reset clears the lockers, and gained a check since the record was taken.
func Reset(aepr *Request) error {
	if aepr.Request.Header.Get("X-Confirm") == "" {
		return aepr.WriteResponseAndNewErrorf(409, "NOT_CONFIRMED", "confirm first")
	}
	return nil
}
