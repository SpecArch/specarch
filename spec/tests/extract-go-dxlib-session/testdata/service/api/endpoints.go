package api

import (
	"net/http"

	"github.com/donnyhardyanto/dxlib/api"
	"github.com/donnyhardyanto/dxlib/types"
	dxhttp "github.com/donnyhardyanto/dxlib/utils/http"
)

// Define registers the journal's endpoints: the exchange, an entry added in
// the persistent-session envelope of type V4, and a legacy sync of type V3
// whose hooks make their own bytes.
func Define(a *api.DXAPI) {
	a.NewEndPoint("Start a session", "The client sends its signing key and two agreement keys and receives the journal's with the session's index.", "/startSession", http.MethodPost, api.EndPointTypeHTTPJSON, dxhttp.RequestContentTypeApplicationJSON,
		[]api.DXAPIEndPointParameter{
			{NameId: "a0", Type: types.APIParameterTypeNonEmptyString, IsMustExist: true},
			{NameId: "a1", Type: types.APIParameterTypeNonEmptyString, IsMustExist: true},
			{NameId: "a2", Type: types.APIParameterTypeNonEmptyString, IsMustExist: true},
		},
		StartSession, nil, &api.DXAPIEndPointResponsePossibilities{
			"success": {StatusCode: http.StatusOK, Description: "The journal's keys and the session's index.", DataTemplate: []*api.DXAPIEndPointParameter{
				{NameId: "i", Type: types.APIParameterTypeNonEmptyString, IsMustExist: true},
				{NameId: "b0", Type: types.APIParameterTypeNonEmptyString, IsMustExist: true},
				{NameId: "b1", Type: types.APIParameterTypeNonEmptyString, IsMustExist: true},
				{NameId: "b2", Type: types.APIParameterTypeNonEmptyString, IsMustExist: true},
			}},
		}, nil, nil, 0, "")

	a.NewEndPoint("Add an entry", "Stores an entry in the journal.", "/addEntry", http.MethodPost, api.EndPointTypeHTTPEndToEndEncryptionV4, dxhttp.RequestContentTypeApplicationJSON,
		[]api.DXAPIEndPointParameter{
			{NameId: "text", Type: types.APIParameterTypeNonEmptyString, IsMustExist: true},
		},
		AddEntry, nil, &api.DXAPIEndPointResponsePossibilities{
			"success": {StatusCode: http.StatusOK, Description: "The entry is stored.", DataTemplate: []*api.DXAPIEndPointParameter{
				{NameId: "entry_id", Type: types.APIParameterTypeNonEmptyString, IsMustExist: true},
			}},
		}, nil, nil, 0, "")

	a.NewEndPoint("Sync", "Sends what changed since the last sync.", "/sync", http.MethodPost, api.EndPointTypeHTTPEndToEndEncryptionV3, dxhttp.RequestContentTypeApplicationJSON,
		[]api.DXAPIEndPointParameter{
			{NameId: "since", Type: types.APIParameterTypeNonEmptyString, IsMustExist: true},
		},
		Sync, nil, &api.DXAPIEndPointResponsePossibilities{
			"success": {StatusCode: http.StatusOK, Description: "What changed."},
		}, nil, nil, 0, "")
}
