package service

import (
	"net/http"

	"github.com/donnyhardyanto/dxlib/api"
	"github.com/donnyhardyanto/dxlib/utils"
)

// Health answers that the service is up.
func Health(aepr *api.DXAPIEndPointRequest) (err error) {
	aepr.WriteResponseAsJSON(http.StatusOK, nil, utils.JSON{"status": "up"})
	return nil
}
