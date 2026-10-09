package handlers

import "github.com/donnyhardyanto/dxlib/api"

// ShowLoan answers one loan.
func ShowLoan(aepr *api.DXAPIEndPointRequest) (err error) {
	_, _, err = aepr.GetParameterValueAsInt64("id")
	return err
}
