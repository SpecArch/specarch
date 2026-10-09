package api

import (
	"net/http"

	"github.com/donnyhardyanto/dxlib/api"

	"example.com/desk/handlers"
)

var exportOn bool

func archivePath() string { return "/loans/archive" }

// Define registers the desk's endpoints.
func Define(a *api.DXAPI) {
	a.NewEndPoint("List loans", "Lists the loans.", "/loans"+"/list", http.MethodPost, api.EndPointTypeHTTPJSON, 0,
		[]api.DXAPIEndPointParameter{{NameId: "member"}},
		ListLoans, nil, nil, []api.DXAPIEndPointExecuteFunc{RequireSession}, []string{"LOANS.READ"}, 0, "")

	a.NewEndPoint("Show a loan", "", "/loans/{id}", "GET", api.EndPointTypeHTTPJSON, 0,
		[]api.DXAPIEndPointParameter{{NameId: "id"}},
		handlers.ShowLoan, nil, nil, nil, nil, 0, "")

	for _, kind := range []string{"daily", "weekly"} {
		a.NewEndPoint("Report", "", "/reports/"+kind, "POST", api.EndPointTypeHTTPJSON, 0, nil,
			ListLoans, nil, nil, nil, []string{"reports.run"}, 0, "")
	}

	if exportOn {
		a.NewEndPoint("Export", "", "/export", "POST", api.EndPointTypeHTTPJSON, 0, nil,
			ListLoans, nil, nil, nil, []string{"loans.export"}, 0, "")
	}

	a.NewEndPoint("Archive", "", archivePath(), "POST", api.EndPointTypeHTTPJSON, 0, nil,
		ListLoans, nil, nil, nil, []string{"loans.write"}, 0, "")

	a.NewEndPoint("Renew a loan", "Renews a loan for another period.", "/loans/renew", "POST", api.EndPointTypeHTTPJSON, 0,
		[]api.DXAPIEndPointParameter{{NameId: "id"}},
		func(aepr *api.DXAPIEndPointRequest) (err error) {
			field := "id"
			_, _, err = aepr.GetParameterValueAsInt64("id")
			_, _, err = aepr.GetParameterValueAsString(field)
			_, _, err = aepr.GetParameterValueAsString("note")
			if err != nil {
				return aepr.WriteResponseAndNewErrorf(http.StatusConflict, "LOAN_OVERDUE", "overdue")
			}
			if field == "" {
				return aepr.WriteResponseAndNewErrorf(409, "LOAN_ON_HOLD", "on hold")
			}
			if field == "x" {
				return aepr.WriteResponseAndNewErrorf(http.StatusUnprocessableEntity, "", "no reason")
			}
			if field == "y" {
				aepr.WriteResponseAsError(http.StatusBadRequest, err)
			}
			return aepr.WriteResponseAndNewErrorf(statusFor(field), "LOAN_ODD", "odd")
		}, nil, nil, nil, []string{"loans.write"}, 0, "")

	a.NewEndPoint("List loans again", "", "/loans/list", "POST", api.EndPointTypeHTTPJSON, 0, nil,
		ListLoans, nil, nil, nil, nil, 0, "")

	a.NewEndPoint("Probe", "", "/loans/list", "HEAD", api.EndPointTypeHTTPJSON, 0, nil,
		ListLoans, nil, nil, nil, nil, 0, "")

	a.NewEndPoint("Short", "", "/short", "POST", api.EndPointTypeHTTPJSON, 0, nil,
		ListLoans, nil, nil, nil, nil, 0)

	a.NewWSEndPoint("Loan feed", "", "/loans/feed", "GET", nil, nil, nil, nil, 0, nil, nil, "")

	a.RegisterHandler("closeLoan", CloseLoan)
}

func statusFor(string) int { return 400 }
