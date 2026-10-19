package api

import (
	"encoding/json"
	"net/http"

	"github.com/donnyhardyanto/dxlib/api"
	"github.com/donnyhardyanto/dxlib/utils"
)

// CreateTask decodes a new task, checks it and answers its id.
func CreateTask(aepr *api.DXAPIEndPointRequest) (err error) {
	var req CreateTaskRequest
	if err = json.Unmarshal(aepr.RequestBodyAsBytes, &req); err != nil {
		return aepr.WriteResponseAndNewErrorf(http.StatusBadRequest, "BODY_NOT_JSON", "the body is not JSON: %v", err)
	}
	if req.Title == "" || len(req.Title) > 80 {
		return aepr.WriteResponseAndNewErrorf(http.StatusBadRequest, "TITLE_INVALID", "a title of 1 to 80 characters is needed")
	}
	switch req.Priority {
	case "low", "normal", "high":
	default:
		return aepr.WriteResponseAndNewErrorf(http.StatusBadRequest, "PRIORITY_UNKNOWN", "unknown priority %s", req.Priority)
	}
	if req.EstimateHours < 0 || req.EstimateHours > 40 {
		return aepr.WriteResponseAndNewErrorf(http.StatusBadRequest, "ESTIMATE_OUT_OF_RANGE", "estimate %d", req.EstimateHours)
	}
	if len(req.Labels) > 5 {
		return aepr.WriteResponseAndNewErrorf(http.StatusBadRequest, "TOO_MANY_LABELS", "at most 5 labels")
	}
	var taskID int32
	taskID, err = storeTask(req)
	if err != nil {
		return err
	}
	aepr.WriteResponseAsJSON(http.StatusCreated, map[string]string{"Location": "/tasks/show"}, utils.JSON{
		"data": utils.JSON{
			"task_id":  taskID,
			"title":    req.Title,
			"accepted": true,
		},
	})
	return nil
}

// ListTasks answers the tasks a query gives.
func ListTasks(aepr *api.DXAPIEndPointRequest) (err error) {
	rows, err := queryTasks()
	if err != nil {
		return err
	}
	aepr.WriteResponseAsJSON(http.StatusOK, nil, utils.JSON{"data": utils.JSON{"tasks": rows}})
	return nil
}

// UpdateTask marks a task done or not; a deferred update is accepted.
func UpdateTask(aepr *api.DXAPIEndPointRequest) (err error) {
	var req struct {
		TaskID int32 `json:"task_id"`
		Done   bool  `json:"done"`
		Later  bool  `json:"later,omitempty"`
	}
	if err = json.Unmarshal(aepr.RequestBodyAsBytes, &req); err != nil {
		return aepr.WriteResponseAndNewErrorf(http.StatusBadRequest, "BODY_NOT_JSON", "the body is not JSON: %v", err)
	}
	if req.TaskID <= 0 {
		return aepr.WriteResponseAndNewErrorf(http.StatusBadRequest, "TASK_ID_INVALID", "task id %d", req.TaskID)
	}
	if req.Done && req.Later {
		return aepr.WriteResponseAndNewErrorf(http.StatusBadRequest, "DONE_AND_LATER", "a task is done or later, not both")
	}
	if req.Done {
		aepr.WriteResponseAsJSON(http.StatusOK, nil, utils.JSON{"task_id": req.TaskID, "done": true})
		return nil
	}
	if req.Later {
		status := http.StatusAccepted
		aepr.WriteResponseAsJSON(status, nil, utils.JSON{"task_id": req.TaskID})
		return nil
	}
	aepr.WriteResponseAsJSON(http.StatusOK, nil, utils.JSON{"task_id": req.TaskID})
	return nil
}

// TagTask adds a tag; its answers are built elsewhere.
func TagTask(aepr *api.DXAPIEndPointRequest) (err error) {
	var req utils.JSON
	if err = json.Unmarshal(aepr.RequestBodyAsBytes, &req); err != nil {
		return err
	}
	field := "tag"
	if taken(req) {
		aepr.WriteResponseAsJSON(http.StatusConflict, nil, utils.JSON{field: "taken"})
		return nil
	}
	aepr.WriteResponseAsJSON(http.StatusOK, nil, tagBody(req))
	return nil
}

// ImportTasks decodes a list of tasks.
func ImportTasks(aepr *api.DXAPIEndPointRequest) (err error) {
	var rows []CreateTaskRequest
	return json.Unmarshal(aepr.RequestBodyAsBytes, &rows)
}

// ArchiveTask decodes its own body beside the declared parameter.
func ArchiveTask(aepr *api.DXAPIEndPointRequest) (err error) {
	var req ArchiveRequest
	if err = json.Unmarshal(aepr.RequestBodyAsBytes, &req); err != nil {
		return err
	}
	aepr.WriteResponseAsJSON(http.StatusOK, nil, utils.JSON{"archived": true})
	return nil
}

func storeTask(req CreateTaskRequest) (int32, error) { return 1, nil }

func queryTasks() ([]utils.JSON, error) { return nil, nil }

func taken(req utils.JSON) bool { return false }

func tagBody(req utils.JSON) utils.JSON { return utils.JSON{"tag": req["tag"]} }
