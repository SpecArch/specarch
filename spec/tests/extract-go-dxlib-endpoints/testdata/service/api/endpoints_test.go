package api

import "github.com/donnyhardyanto/dxlib/api"

func define(a *api.DXAPI) {
	a.NewEndPoint("Test", "", "/test", "POST", api.EndPointTypeHTTPJSON, 0, nil, nil, nil, nil, nil, nil, 0, "")
}
