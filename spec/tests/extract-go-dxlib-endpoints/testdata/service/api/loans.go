package api

import (
	"net/http"

	"github.com/donnyhardyanto/dxlib/api"
)

// ListLoans answers the loans of a member.
func ListLoans(aepr *api.DXAPIEndPointRequest) (err error) {
	_, member, err := aepr.GetParameterValueAsString("member")
	if member == "" {
		return aepr.WriteResponseAndNewErrorf(http.StatusNotFound, "LOAN_NOT_FOUND", "none")
	}
	return aepr.WriteResponseAndNewErrorf(http.StatusUnprocessableEntity, "LOAN_OVERDUE", "overdue")
}

// CloseLoan closes a loan.
func CloseLoan(aepr *api.DXAPIEndPointRequest) (err error) {
	return nil
}
