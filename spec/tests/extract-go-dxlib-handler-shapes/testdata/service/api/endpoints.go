package api

import (
	"net/http"

	"github.com/donnyhardyanto/dxlib/api"
	"github.com/donnyhardyanto/dxlib/types"
	dxhttp "github.com/donnyhardyanto/dxlib/utils/http"
)

// Define registers the task desk's endpoints. Most declare neither their
// parameters nor their responses; their handlers decode and answer.
func Define(a *api.DXAPI) {
	a.NewEndPoint("Create a task", "", "/tasks/create", http.MethodPost, api.EndPointTypeHTTPJSON, dxhttp.RequestContentTypeApplicationJSON, nil,
		CreateTask, nil, nil, nil, []string{"tasks.write"}, 0, "")

	a.NewEndPoint("List tasks", "", "/tasks/list", http.MethodGet, api.EndPointTypeHTTPJSON, dxhttp.RequestContentTypeApplicationJSON, nil,
		ListTasks, nil, nil, nil, []string{"tasks.read"}, 0, "")

	a.NewEndPoint("Update a task", "", "/tasks/update", http.MethodPost, api.EndPointTypeHTTPJSON, dxhttp.RequestContentTypeApplicationJSON, nil,
		UpdateTask, nil, nil, nil, []string{"tasks.write"}, 0, "")

	a.NewEndPoint("Tag a task", "", "/tasks/tag", http.MethodPost, api.EndPointTypeHTTPJSON, dxhttp.RequestContentTypeApplicationJSON, nil,
		TagTask, nil, nil, nil, []string{"tasks.write"}, 0, "")

	a.NewEndPoint("Import tasks", "", "/tasks/import", http.MethodPost, api.EndPointTypeHTTPJSON, dxhttp.RequestContentTypeApplicationJSON, nil,
		ImportTasks, nil, nil, nil, []string{"tasks.write"}, 0, "")

	a.NewEndPoint("Archive a task", "", "/tasks/archive", http.MethodPost, api.EndPointTypeHTTPJSON, dxhttp.RequestContentTypeApplicationJSON,
		[]api.DXAPIEndPointParameter{
			{NameId: "task_id", Type: types.APIParameterTypeInt32, IsMustExist: true},
		},
		ArchiveTask, nil, &api.DXAPIEndPointResponsePossibilities{
			"success": {StatusCode: http.StatusOK, Description: "The task is archived.", DataTemplate: []*api.DXAPIEndPointParameter{
				{NameId: "archived_at", Type: types.APIParameterTypeString, IsMustExist: true},
			}},
		}, nil, []string{"tasks.write"}, 0, "")

	a.NewEndPoint("Assign a task", "", "/tasks/assign", http.MethodPost, api.EndPointTypeHTTPJSON, dxhttp.RequestContentTypeApplicationJSON, nil,
		AssignTask, nil, nil, nil, []string{"tasks.write"}, 0, "")

	a.NewEndPoint("Label a task", "", "/tasks/label", http.MethodPost, api.EndPointTypeHTTPJSON, dxhttp.RequestContentTypeApplicationJSON, nil,
		LabelTask, nil, nil, nil, []string{"tasks.write"}, 0, "")

	a.NewEndPoint("Restore a task", "", "/tasks/restore", http.MethodPost, api.EndPointTypeHTTPJSON, dxhttp.RequestContentTypeApplicationJSON,
		[]api.DXAPIEndPointParameter{
			{NameId: "task_id", Type: types.APIParameterTypeInt32, IsMustExist: true},
			{NameId: "reason", Type: types.APIParameterTypeString, IsMustExist: false},
		},
		RestoreTask, nil, &api.DXAPIEndPointResponsePossibilities{
			"success": {StatusCode: http.StatusOK, Description: "The task is restored.", DataTemplate: []*api.DXAPIEndPointParameter{
				{NameId: "restored_at", Type: types.APIParameterTypeString, IsMustExist: true},
			}},
		}, nil, []string{"tasks.write"}, 0, "")
}
