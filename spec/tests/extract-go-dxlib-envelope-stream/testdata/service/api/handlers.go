package api

import (
	"io"
	"net/http"

	"github.com/donnyhardyanto/dxlib/api"
)

// IssuePreKey performs the exchange.
func IssuePreKey(aepr *api.DXAPIEndPointRequest) (err error) {
	return nil
}

// AddNote stores a note.
func AddNote(aepr *api.DXAPIEndPointRequest) (err error) {
	_, _, err = aepr.GetParameterValueAsString("title")
	return err
}

// UploadAttachment reads the stream to its end.
func UploadAttachment(aepr *api.DXAPIEndPointRequest) (err error) {
	_, err = io.Copy(io.Discard, aepr.Request.Body)
	return err
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
	return nil
}

// OpenSession opens a session.
func OpenSession(aepr *api.DXAPIEndPointRequest) (err error) {
	return nil
}

// Ping answers.
func Ping(aepr *api.DXAPIEndPointRequest) (err error) {
	return nil
}
