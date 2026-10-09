package api

import (
	"github.com/donnyhardyanto/dxlib/api"
)

// Define registers the desk's endpoints.
func Define(a *api.DXAPI) {
	a.NewEndPoint("List loans", "Lists the loans a page at a time.", "/loans/list", "POST", api.EndPointTypeHTTPJSON, 0,
		[]api.DXAPIEndPointParameter{{NameId: "search_text"}, {NameId: "row_per_page"}, {NameId: "page_index"}},
		LoanList.RequestSearchPagingList, nil, nil, []api.DXAPIEndPointExecuteFunc{RequireDeskKey, RequireChecks}, []string{"LOANS.READ"}, 0, "")

	a.NewEndPoint("List branches", "Lists the branches.", "/branches/list", "POST", api.EndPointTypeHTTPJSON, 0, nil,
		BranchList.RequestSearchPagingList, nil, nil, []api.DXAPIEndPointExecuteFunc{RequireDeskKey}, []string{"BRANCHES.READ"}, 0, "")
}
