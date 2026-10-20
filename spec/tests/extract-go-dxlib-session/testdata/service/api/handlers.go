package api

import (
	"net/http"

	"github.com/donnyhardyanto/dxlib/api"
	"github.com/donnyhardyanto/dxlib/utils"
)

// StartSession performs the exchange.
func StartSession(aepr *api.DXAPIEndPointRequest) (err error) {
	return nil
}

// AddEntry stores an entry.
func AddEntry(aepr *api.DXAPIEndPointRequest) (err error) {
	_, text, err := aepr.GetParameterValueAsString("text")
	if err != nil {
		return err
	}
	aepr.WriteResponseAsJSON(http.StatusOK, nil, utils.JSON{"entry_id": "e-" + text})
	return nil
}

// Sync answers what changed.
func Sync(aepr *api.DXAPIEndPointRequest) (err error) {
	_, _, err = aepr.GetParameterValueAsString("since")
	if err != nil {
		return err
	}
	aepr.WriteResponseAsJSON(http.StatusOK, nil, utils.JSON{})
	return nil
}
