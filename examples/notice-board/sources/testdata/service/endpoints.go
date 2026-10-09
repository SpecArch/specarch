// Package service defines the notice board's endpoints on dxlib.
package service

import (
	"github.com/donnyhardyanto/dxlib/api"
	"github.com/donnyhardyanto/dxlib/types"
	utilsHttp "github.com/donnyhardyanto/dxlib/utils/http"
)

// DefineEndPoints registers every endpoint of the notice board on a.
func DefineEndPoints(a *api.DXAPI) {
	a.NewEndPoint("List notices",
		"Lists the notices, newest first, whose title holds the filter text.",
		"/v1/notice/list", "POST", api.EndPointTypeHTTPJSON, utilsHttp.RequestContentTypeApplicationJSON,
		[]api.DXAPIEndPointParameter{
			{NameId: "filter_text", Type: types.APIParameterTypeString, Description: "Text the title holds"},
		},
		NoticeList, nil,
		&api.DXAPIEndPointResponsePossibilities{
			"success": {StatusCode: 200, Description: "The notices."},
		},
		[]api.DXAPIEndPointExecuteFunc{RequireSession},
		[]string{"NOTICE.READ"}, 0, "",
	)

	a.NewEndPoint("Read a notice",
		"",
		"/v1/notice/read", "POST", api.EndPointTypeHTTPJSON, utilsHttp.RequestContentTypeApplicationJSON,
		[]api.DXAPIEndPointParameter{
			{NameId: "id", Type: types.APIParameterTypeInt64, Description: "The notice", IsMustExist: true},
		},
		NoticeRead, nil,
		&api.DXAPIEndPointResponsePossibilities{
			"success":   {StatusCode: 200, Description: "The notice."},
			"not_found": {StatusCode: 404, Description: "No notice has this id."},
		},
		[]api.DXAPIEndPointExecuteFunc{RequireSession},
		[]string{"NOTICE.READ"}, 0, "",
	)

	a.NewEndPoint("Post a notice",
		"Posts a notice with a title and a body.",
		"/v1/notice/create", "POST", api.EndPointTypeHTTPJSON, utilsHttp.RequestContentTypeApplicationJSON,
		[]api.DXAPIEndPointParameter{
			{NameId: "title", Type: types.APIParameterTypeNonEmptyString, Description: "The title", IsMustExist: true},
			{NameId: "body", Type: types.APIParameterTypeString, Description: "The text", IsMustExist: true},
		},
		NoticeCreate, nil,
		&api.DXAPIEndPointResponsePossibilities{
			"success": {StatusCode: 200, Description: "The notice posted."},
		},
		[]api.DXAPIEndPointExecuteFunc{RequireSession},
		[]string{"NOTICE.WRITE"}, 0, "",
	)

	a.NewEndPoint("Remove a notice",
		"",
		"/v1/notice/delete", "POST", api.EndPointTypeHTTPJSON, utilsHttp.RequestContentTypeApplicationJSON,
		[]api.DXAPIEndPointParameter{
			{NameId: "id", Type: types.APIParameterTypeInt64, Description: "The notice", IsMustExist: true},
		},
		NoticeDelete, nil,
		&api.DXAPIEndPointResponsePossibilities{
			"success": {StatusCode: 200, Description: "The notice removed."},
		},
		[]api.DXAPIEndPointExecuteFunc{RequireSession},
		[]string{"NOTICE.DELETE", "GLOBAL.ADMIN"}, 0, "",
	)

	a.NewEndPoint("Health",
		"Says the service is up.",
		"/v1/health", "GET", api.EndPointTypeHTTPJSON, utilsHttp.RequestContentTypeApplicationJSON, nil,
		Health, nil,
		&api.DXAPIEndPointResponsePossibilities{
			"success": {StatusCode: 200, Description: "Up."},
		},
		nil, nil, 0, "",
	)

	a.NewEndPoint("Set maintenance mode",
		"Turns maintenance mode on or off.",
		"/v1/system/maintenance", "POST", api.EndPointTypeHTTPJSON, utilsHttp.RequestContentTypeApplicationJSON,
		[]api.DXAPIEndPointParameter{
			{NameId: "on", Type: types.APIParameterTypeBoolean, Description: "Whether maintenance mode is on", IsMustExist: true},
		},
		func(aepr *api.DXAPIEndPointRequest) (err error) {
			_, on, err := aepr.GetParameterValueAsBool("on")
			if err != nil {
				return err
			}
			setMaintenance(on)
			aepr.WriteResponseAsJSON(200, nil, nil)
			return nil
		}, nil,
		&api.DXAPIEndPointResponsePossibilities{
			"success": {StatusCode: 200, Description: "The mode set."},
		},
		[]api.DXAPIEndPointExecuteFunc{RequireSession},
		[]string{"GLOBAL.SET_MAINTENANCE_MODE"}, 0, "",
	)

	a.NewWSEndPoint("Notice stream",
		"Sends each notice as it is posted.",
		"/v1/notice/stream", "GET",
		nil, nil, nil, streamNotices, 0,
		[]api.DXAPIEndPointExecuteFunc{RequireSession},
		[]string{"NOTICE.READ"}, "",
	)
}
