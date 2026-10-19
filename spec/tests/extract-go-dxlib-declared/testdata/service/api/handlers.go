package api

import (
	"encoding/json"
	"net/http"

	"github.com/donnyhardyanto/dxlib/api"
)

// OpenLoan opens a loan.
func OpenLoan(aepr *api.DXAPIEndPointRequest) (err error) {
	_, _, err = aepr.GetParameterValueAsInt64("member_id")
	return aepr.WriteResponseAndNewErrorf(http.StatusConflict, "ITEM_LENT_OUT", "lent out")
}

// ListLoans lists a member's loans.
func ListLoans(aepr *api.DXAPIEndPointRequest) (err error) {
	return nil
}

// ShowMember answers one member.
func ShowMember(aepr *api.DXAPIEndPointRequest) (err error) {
	return nil
}

// CloseLoan closes a loan.
func CloseLoan(aepr *api.DXAPIEndPointRequest) (err error) {
	return nil
}

// ImportLoans reads its body itself.
func ImportLoans(aepr *api.DXAPIEndPointRequest) (err error) {
	var rows []map[string]any
	return json.NewDecoder(aepr.Request.Body).Decode(&rows)
}

// UploadScan stores a scan.
func UploadScan(aepr *api.DXAPIEndPointRequest) (err error) {
	return nil
}
