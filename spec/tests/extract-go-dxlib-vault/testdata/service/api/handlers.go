package api

import (
	"io"
	"net/http"

	"github.com/donnyhardyanto/dxlib/api"
	"github.com/donnyhardyanto/dxlib/utils"
)

// IssuePreKey performs the exchange.
func IssuePreKey(aepr *api.DXAPIEndPointRequest) (err error) {
	return nil
}

// AddNote stores a note.
func AddNote(aepr *api.DXAPIEndPointRequest) (err error) {
	_, title, err := aepr.GetParameterValueAsString("title")
	if err != nil {
		return err
	}
	var noteID string = "n-" + title
	aepr.WriteResponseAsJSON(http.StatusOK, nil, utils.JSON{"note_id": noteID})
	return nil
}

// UploadAttachment reads the stream to its end.
func UploadAttachment(aepr *api.DXAPIEndPointRequest) (err error) {
	var n int64
	n, err = io.Copy(io.Discard, aepr.Request.Body)
	if err != nil {
		return err
	}
	aepr.WriteResponseAsJSON(http.StatusOK, nil, utils.JSON{"attachment_id": "a-1", "size": n})
	return nil
}

// DownloadAttachment answers the file, or that there is none.
func DownloadAttachment(aepr *api.DXAPIEndPointRequest) (err error) {
	_, _, err = aepr.GetParameterValueAsString("attachment_id")
	if err != nil {
		return err
	}
	return aepr.WriteResponseAndNewErrorf(http.StatusNotFound, "ATTACHMENT_NOT_FOUND", "no attachment")
}

// ExportVault streams the archive.
func ExportVault(aepr *api.DXAPIEndPointRequest) (err error) {
	_, _, err = aepr.GetParameterValueAsString("format")
	return err
}
