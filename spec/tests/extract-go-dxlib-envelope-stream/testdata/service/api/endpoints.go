package api

import (
	"net/http"

	"github.com/donnyhardyanto/dxlib/api"
	"github.com/donnyhardyanto/dxlib/types"
	dxhttp "github.com/donnyhardyanto/dxlib/utils/http"
)

// sessionType is an endpoint type the code computes.
var sessionType = api.EndPointTypeHTTPJSON

// Define registers the vault's endpoints: a pre-key exchange, operations in
// its envelope, a streamed upload and two streamed downloads.
func Define(a *api.DXAPI) {
	a.NewEndPoint("Issue a pre-key", "Performs the key exchange a client calls before anything else.", "/prekey", http.MethodPost, api.EndPointTypeHTTPJSON, dxhttp.RequestContentTypeApplicationJSON,
		[]api.DXAPIEndPointParameter{
			{NameId: "a0", Type: types.APIParameterTypeNonEmptyString, IsMustExist: true, Description: "The client's Ed25519 public key, hex."},
			{NameId: "a1", Type: types.APIParameterTypeNonEmptyString, IsMustExist: true, Description: "The client's first X25519 public key, hex."},
			{NameId: "a2", Type: types.APIParameterTypeNonEmptyString, IsMustExist: true, Description: "The client's second X25519 public key, hex."},
		},
		IssuePreKey, nil, &api.DXAPIEndPointResponsePossibilities{
			"success": {StatusCode: http.StatusOK, Description: "The service's keys and the pre-key's index.", DataTemplate: []*api.DXAPIEndPointParameter{
				{NameId: "i", Type: types.APIParameterTypeNonEmptyString, IsMustExist: true},
				{NameId: "b0", Type: types.APIParameterTypeNonEmptyString, IsMustExist: true},
				{NameId: "b1", Type: types.APIParameterTypeNonEmptyString, IsMustExist: true},
				{NameId: "b2", Type: types.APIParameterTypeNonEmptyString, IsMustExist: true},
			}},
		}, nil, nil, 0, "")

	a.NewEndPoint("Add a note", "Stores a note in the vault.", "/notes/add", http.MethodPost, api.EndPointTypeHTTPEndToEndEncryptionV2, dxhttp.RequestContentTypeApplicationJSON,
		[]api.DXAPIEndPointParameter{
			{NameId: "title", Type: types.APIParameterTypeNonEmptyString, IsMustExist: true},
			{NameId: "text", Type: types.APIParameterTypeString},
		},
		AddNote, nil, &api.DXAPIEndPointResponsePossibilities{
			"success": {StatusCode: http.StatusOK, Description: "The note is stored.", DataTemplate: []*api.DXAPIEndPointParameter{
				{NameId: "note_id", Type: types.APIParameterTypeNonEmptyString, IsMustExist: true},
			}},
		}, nil, []string{"NOTES.WRITE"}, 0, "")

	a.NewEndPoint("Upload an attachment", "Streams a file into the vault.", "/attachments/upload", http.MethodPost, api.EndPointTypeHTTPUploadStream, dxhttp.RequestContentTypeApplicationOctetStream,
		nil, UploadAttachment, nil, &api.DXAPIEndPointResponsePossibilities{
			"success": {StatusCode: http.StatusOK, Description: "The attachment is stored."},
		}, nil, []string{"NOTES.WRITE"}, 1073741824, "")

	a.NewEndPoint("Download an attachment", "Streams a file out of the vault.", "/attachments/download", http.MethodPost, api.EndPointTypeHTTPDownloadStream, dxhttp.RequestContentTypeApplicationJSON,
		[]api.DXAPIEndPointParameter{
			{NameId: "attachment_id", Type: types.APIParameterTypeNonEmptyString, IsMustExist: true},
		},
		DownloadAttachment, nil, &api.DXAPIEndPointResponsePossibilities{
			"success":   {StatusCode: http.StatusOK, Description: "The attachment's bytes."},
			"not_found": {StatusCode: http.StatusNotFound, Description: "No attachment has this id."},
		}, nil, []string{"NOTES.READ"}, 0, "")

	a.NewEndPoint("Export the vault", "Streams an archive, asked for in the envelope.", "/vault/export", http.MethodPost, api.EndPointTypeHTTPDownloadStreamV2, dxhttp.RequestContentTypeApplicationJSON,
		[]api.DXAPIEndPointParameter{
			{NameId: "format", Type: "string", Enum: []any{"zip", "tar"}},
		},
		ExportVault, nil, &api.DXAPIEndPointResponsePossibilities{
			"success": {StatusCode: http.StatusOK, Description: "The archive.", DataTemplate: []*api.DXAPIEndPointParameter{
				{NameId: "size", Type: types.APIParameterTypeInt64},
			}},
		}, nil, []string{"NOTES.READ"}, 0, "")

	a.NewEndPoint("Open a session", "Travels in the persistent-session envelope.", "/session/open", http.MethodPost, api.EndPointTypeHTTPEndToEndEncryptionV3, dxhttp.RequestContentTypeApplicationJSON,
		nil, OpenSession, nil, &api.DXAPIEndPointResponsePossibilities{
			"success": {StatusCode: http.StatusOK, Description: "The session is open."},
		}, nil, nil, 0, "")

	a.NewEndPoint("Ping", "Answers in the first envelope dxlib never finished.", "/ping", http.MethodPost, api.EndPointTypeHTTPEndToEndEncryptionV1, dxhttp.RequestContentTypeApplicationJSON,
		nil, Ping, nil, &api.DXAPIEndPointResponsePossibilities{
			"success": {StatusCode: http.StatusOK, Description: "Pong."},
		}, nil, nil, 0, "")

	a.NewEndPoint("Health", "Registered with a type the code computes.", "/health", http.MethodGet, sessionType, dxhttp.RequestContentTypeNone,
		nil, Ping, nil, &api.DXAPIEndPointResponsePossibilities{
			"success": {StatusCode: http.StatusOK, Description: "Up."},
		}, nil, nil, 0, "")

	a.NewEndPoint("Upload a form", "An upload stream with a JSON content type.", "/forms/upload", http.MethodPost, api.EndPointTypeHTTPUploadStream, dxhttp.RequestContentTypeApplicationJSON,
		nil, UploadAttachment, nil, &api.DXAPIEndPointResponsePossibilities{
			"success": {StatusCode: http.StatusOK, Description: "Stored."},
		}, nil, []string{"NOTES.WRITE"}, 0, "")
}
