package api

import (
	"github.com/donnyhardyanto/dxlib/api"
	dxhttp "github.com/donnyhardyanto/dxlib/utils/http"

	"example.org/audit"
)

// mw is shorthand for a chain.
func mw(fns ...api.DXAPIEndPointExecuteFunc) []api.DXAPIEndPointExecuteFunc { return fns }

// The chain of every staff endpoint.
var staffChain = []api.DXAPIEndPointExecuteFunc{RequireSession, RequireStaff}

// chainFor adds a check by a flag, which syntax does not run.
func chainFor(strict bool) []api.DXAPIEndPointExecuteFunc {
	chain := []api.DXAPIEndPointExecuteFunc{RequireSession}
	if strict {
		chain = append(chain, RequireStaff)
	}
	return chain
}

// Define registers the desk's endpoints.
func Define(a *api.DXAPI) {
	a.NewEndPoint("Show the desk", "", "/desk/show", "POST", api.EndPointTypeHTTPJSON, dxhttp.RequestContentTypeApplicationJSON, nil,
		Show, nil, &api.DXAPIEndPointResponsePossibilities{"success": {StatusCode: 200, Description: "The desk."}},
		[]api.DXAPIEndPointExecuteFunc{RequireSession, audit.Log}, []string{"desk.read"}, 0, "")

	a.NewEndPoint("Close the desk", "", "/desk/close", "POST", api.EndPointTypeHTTPJSON, dxhttp.RequestContentTypeApplicationJSON, nil,
		Show, nil, &api.DXAPIEndPointResponsePossibilities{"success": {StatusCode: 200, Description: "The desk is closed."}},
		staffChain, []string{"desk.write"}, 0, "")

	a.NewEndPoint("Open the desk", "", "/desk/open", "POST", api.EndPointTypeHTTPJSON, dxhttp.RequestContentTypeApplicationJSON, nil,
		Show, nil, &api.DXAPIEndPointResponsePossibilities{"success": {StatusCode: 200, Description: "The desk is open."}},
		mw(RequireSession, RequireStaff, Throttle), []string{"desk.write"}, 0, "")

	a.NewEndPoint("Audit the desk", "", "/desk/audit", "POST", api.EndPointTypeHTTPJSON, dxhttp.RequestContentTypeApplicationJSON, nil,
		Show, nil, &api.DXAPIEndPointResponsePossibilities{"success": {StatusCode: 200, Description: "The audit."}},
		chainFor(true), []string{"desk.read"}, 0, "")

	a.NewEndPoint("Count the desk", "", "/desk/count", "POST", api.EndPointTypeHTTPJSON, dxhttp.RequestContentTypeApplicationJSON, nil,
		Show, nil, &api.DXAPIEndPointResponsePossibilities{"success": {StatusCode: 200, Description: "The count."}},
		mw(Throttle, RequireSession, Throttle), []string{"desk.read"}, 0, "")

	a.NewEndPoint("Sweep the desk", "", "/desk/sweep", "POST", api.EndPointTypeHTTPJSON, dxhttp.RequestContentTypeApplicationJSON, nil,
		Show, nil, &api.DXAPIEndPointResponsePossibilities{"success": {StatusCode: 200, Description: "The desk is swept."}},
		[]api.DXAPIEndPointExecuteFunc{RequireSession, func(aepr *api.DXAPIEndPointRequest) error { return nil }}, []string{"desk.write"}, 0, "")
}
