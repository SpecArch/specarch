package api

import (
	"net/http"

	"github.com/donnyhardyanto/dxlib/api"
	"github.com/donnyhardyanto/dxlib/types"
	dxhttp "github.com/donnyhardyanto/dxlib/utils/http"
)

func ptr[T any](v T) *T { return &v }

// The parameters of a member, shared by two endpoints.
var memberParameters = []api.DXAPIEndPointParameter{
	{NameId: "member_id", Type: types.APIParameterTypeInt64P, IsMustExist: true, Description: "The member."},
}

// pagingParameters is the list every listing takes.
func pagingParameters() []api.DXAPIEndPointParameter {
	return []api.DXAPIEndPointParameter{
		{NameId: "page_size", Type: types.APIParameterTypeInt32, Minimum: ptr(1.0), Maximum: ptr(100.0)},
		{NameId: "order", Type: "string", Enum: []any{"newest", "oldest"}},
	}
}

// builtParameters adds a parameter by a condition, which syntax does not run.
func builtParameters(strict bool) []api.DXAPIEndPointParameter {
	list := []api.DXAPIEndPointParameter{{NameId: "note", Type: types.APIParameterTypeString}}
	if strict {
		list = append(list, api.DXAPIEndPointParameter{NameId: "reason", Type: types.APIParameterTypeString})
	}
	return list
}

// Define registers the desk's endpoints.
func Define(a *api.DXAPI) {
	a.NewEndPoint("Open a loan", "Opens a loan for a member.", "/loans/open", http.MethodPost, api.EndPointTypeHTTPJSON, dxhttp.RequestContentTypeApplicationJSON,
		[]api.DXAPIEndPointParameter{
			{NameId: "member_id", Type: types.APIParameterTypeInt64P, IsMustExist: true, Description: "The member who borrows."},
			{NameId: "item_code", Type: types.APIParameterTypeNonEmptyString, IsMustExist: true, MaxLength: ptr(16), Pattern: "^[A-Z0-9]+$"},
			{NameId: "due_date", Type: types.APIParameterTypeDate, IsNullable: true},
			{NameId: "amount", Type: types.APIParameterTypeMoney},
			{NameId: "address", Type: types.APIParameterTypeJSON, Children: []api.DXAPIEndPointParameter{
				{NameId: "city", Type: types.APIParameterTypeString, IsMustExist: true},
				{NameId: "postal_code", Type: lookupType()},
			}},
			{NameId: "copies", Type: types.APIParameterTypeArrayJSONTemplate, Children: []api.DXAPIEndPointParameter{
				{NameId: "barcode", Type: types.APIParameterTypeString},
			}},
			{NameId: "labels", Type: types.APIParameterTypeMapStringString},
			{NameId: "tags", Type: types.APIParameterTypeArrayString},
		},
		OpenLoan, nil, &api.DXAPIEndPointResponsePossibilities{
			"success": {StatusCode: http.StatusOK, Description: "The loan is open.", DataTemplate: []*api.DXAPIEndPointParameter{
				{NameId: "loan_id", Type: types.APIParameterTypeID, IsMustExist: true},
			}},
			"conflict": {StatusCode: 409, Description: "The item is lent out.", Headers: map[string]string{"Retry-After": "When to ask again."}},
		}, nil, []string{"loans.write"}, 0, "")

	a.NewEndPoint("List a member's loans", "", "/loans/list", "GET", api.EndPointTypeHTTPJSON, dxhttp.RequestContentTypeApplicationJSON,
		pagingParameters(),
		ListLoans, nil, nil, nil, []string{"loans.read"}, 0, "")

	a.NewEndPoint("Show a member", "", "/members/show", "POST", api.EndPointTypeHTTPJSON, dxhttp.RequestContentTypeApplicationJSON,
		memberParameters,
		ShowMember, nil, &api.DXAPIEndPointResponsePossibilities{
			"success": {StatusCode: 200},
		}, nil, []string{"members.read"}, 0, "")

	a.NewEndPoint("Close a loan", "", "/loans/close", "POST", api.EndPointTypeHTTPJSON, dxhttp.RequestContentTypeApplicationJSON,
		builtParameters(true),
		CloseLoan, nil, closeResponses(), nil, []string{"loans.write"}, 0, "")

	a.NewEndPoint("Import loans", "", "/loans/import", "POST", api.EndPointTypeHTTPJSON, dxhttp.RequestContentTypeApplicationJSON, nil,
		ImportLoans, nil, nil, nil, []string{"loans.write"}, 0, "")

	a.NewEndPoint("Upload a scan", "", "/loans/scan", "PUT", api.EndPointTypeHTTPJSON, dxhttp.RequestContentTypeApplicationOctetStream,
		[]api.DXAPIEndPointParameter{{NameId: "loan_id", Type: types.APIParameterTypeID, IsMustExist: true}},
		UploadScan, nil, nil, nil, []string{"loans.write"}, 0, "")

	a.NewEndPoint("Rename a loan", "", "/loans/rename", "PATCH", api.EndPointTypeHTTPJSON, dxhttp.RequestContentTypeApplicationJSON,
		[]api.DXAPIEndPointParameter{{NameId: "name", Type: types.APIParameterTypeString, IsMustExist: true}},
		UploadScan, nil, &api.DXAPIEndPointResponsePossibilities{"success": {StatusCode: 200, Description: "The loan is renamed."}}, nil, []string{"loans.write"}, 0, "")
}

func lookupType() types.APIParameterType { return types.APIParameterTypeString }

func closeResponses() *api.DXAPIEndPointResponsePossibilities {
	r := api.DXAPIEndPointResponsePossibilities{}
	r["success"] = api.DXAPIEndPointResponsePossibility{StatusCode: 200}
	return &r
}
