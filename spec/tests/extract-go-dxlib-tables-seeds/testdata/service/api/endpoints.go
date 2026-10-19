package api

import (
	"net/http"

	"github.com/donnyhardyanto/dxlib/api"
	"github.com/donnyhardyanto/dxlib/utils"
)

// Define registers the desk's endpoints.
func Define(a *api.DXAPI) {
	a.NewEndPoint("List loans", "Lists the loans a page at a time.", "/loans/list", "POST", api.EndPointTypeHTTPJSON, 0,
		[]api.DXAPIEndPointParameter{{NameId: "search_text"}, {NameId: "row_per_page"}, {NameId: "page_index"}},
		LoanList.RequestSearchPagingList, nil, nil, []api.DXAPIEndPointExecuteFunc{RequireDeskKey, RequireChecks}, []string{"LOANS.READ"}, 0, "")

	a.NewEndPoint("List branches", "Lists the branches.", "/branches/list", "POST", api.EndPointTypeHTTPJSON, 0, nil,
		BranchList.RequestSearchPagingList, nil, nil, []api.DXAPIEndPointExecuteFunc{RequireDeskKey}, []string{"BRANCHES.READ"}, 0, "")

	a.NewEndPoint("Open a loan", "Opens a loan at a branch.", "/loans/open", "POST", api.EndPointTypeHTTPJSON, 0,
		[]api.DXAPIEndPointParameter{{NameId: "branch_code"}, {NameId: "title"}},
		OpenLoan, nil, nil, []api.DXAPIEndPointExecuteFunc{RequireDeskKey}, []string{"LOANS.WRITE"}, 0, "")
}

// OpenLoan opens a loan at a branch, in the desk's tables.
func OpenLoan(aepr *api.DXAPIEndPointRequest) (err error) {
	_, code, err := aepr.GetParameterValueAsString("branch_code")
	if err != nil {
		return err
	}
	_, branch, err := BranchList.ShouldSelectOne(&aepr.Log, utils.JSON{"code": code}, nil)
	if err != nil {
		return aepr.WriteResponseAndNewErrorf(http.StatusNotFound, "BRANCH_NOT_FOUND", "no branch %s", code)
	}
	_, err = LoanList.Insert(&aepr.Log, utils.JSON{"branch_id": branch["id"], "state": "open"})
	if err != nil {
		return err
	}
	_, err = BranchList.TxUpdate(&aepr.Log, utils.JSON{"open_loans": 1}, utils.JSON{"id": branch["id"]})
	if err != nil {
		return err
	}
	aepr.WriteResponseAsJSON(http.StatusOK, nil, utils.JSON{"branch": branch})
	return nil
}
