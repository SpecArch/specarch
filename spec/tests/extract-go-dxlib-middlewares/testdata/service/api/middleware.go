package api

import (
	"net/http"

	"github.com/donnyhardyanto/dxlib/api"
)

// RequireSession refuses a request that carries no session.
func RequireSession(aepr *api.DXAPIEndPointRequest) (err error) {
	if aepr.Request.Header.Get("Authorization") == "" {
		return aepr.WriteResponseAndNewErrorf(http.StatusUnauthorized, "SESSION_MISSING", "no session")
	}
	return nil
}

// RequireStaff refuses a caller who is not on the desk's staff, and one
// whose staff card has expired.
func RequireStaff(aepr *api.DXAPIEndPointRequest) (err error) {
	if aepr.Request.Header.Get("X-Staff") == "" {
		return aepr.WriteResponseAndNewErrorf(http.StatusForbidden, "NOT_STAFF", "not staff")
	}
	if aepr.Request.Header.Get("X-Card") == "old" {
		return aepr.WriteResponseAndNewErrorf(http.StatusForbidden, "CARD_EXPIRED", "card expired")
	}
	return nil
}

func Throttle(aepr *api.DXAPIEndPointRequest) (err error) {
	return aepr.WriteResponseAndNewErrorf(statusFor(), "SLOW_DOWN", "slow down")
}

func statusFor() int { return http.StatusTooManyRequests }

// Show answers the desk.
func Show(aepr *api.DXAPIEndPointRequest) (err error) {
	return nil
}
