package api

import (
	"os"

	"github.com/donnyhardyanto/dxlib/api"
	dxos "github.com/donnyhardyanto/dxlib/utils/os"
)

// RequireDeskKey refuses a request without the desk's key, and lets every
// request through while no key is set.
func RequireDeskKey(aepr *api.DXAPIEndPointRequest) error {
	key := os.Getenv("DESK_API_KEY")
	if key == "" {
		return nil
	}
	if aepr.Request.Header.Get("X-Desk-Key") != key {
		return aepr.WriteResponseAndNewErrorf(401, "DESK_KEY_WRONG", "wrong key")
	}
	return nil
}

// RequireChecks runs the desk's checks while they are switched on.
func RequireChecks(aepr *api.DXAPIEndPointRequest) error {
	if !dxos.GetEnvDefaultValueAsBool("DESK_CHECKS_ON", true) {
		return nil
	}
	return nil
}
