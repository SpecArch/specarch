package api

// Count answers how many lockers there are.
func Count(aepr *Request) error {
	aepr.WriteResponseAsJSON(200, nil, map[string]any{"count": 12})
	return nil
}

// Ping answers that the service is up.
func Ping(aepr *Request) error {
	aepr.WriteResponseAsJSON(200, nil, nil)
	return nil
}
