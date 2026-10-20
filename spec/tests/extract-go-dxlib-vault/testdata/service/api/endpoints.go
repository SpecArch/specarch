package api

import (
	"net/http"

	"github.com/donnyhardyanto/dxlib/api"
	"github.com/donnyhardyanto/dxlib/types"
	dxhttp "github.com/donnyhardyanto/dxlib/utils/http"
)

// Define registers the vault's endpoints: a pre-key exchange, a note added
// in its envelope, an attachment streamed in with the note it belongs to and
// streamed out, and an export asked for in the envelope and streamed out.
func Define(a *api.DXAPI) {
	a.NewEndPoint("Issue a pre-key", "The client sends its signing key and two agreement keys and receives the vault's with the pre-key's index.", "/issuePreKey", http.MethodPost, api.EndPointTypeHTTPJSON, dxhttp.RequestContentTypeApplicationJSON,
		[]api.DXAPIEndPointParameter{
			{NameId: "a0", Type: types.APIParameterTypeNonEmptyString, IsMustExist: true, Description: "The client's Ed25519 public key, hex."},
			{NameId: "a1", Type: types.APIParameterTypeNonEmptyString, IsMustExist: true, Description: "The client's first X25519 public key, hex."},
			{NameId: "a2", Type: types.APIParameterTypeNonEmptyString, IsMustExist: true, Description: "The client's second X25519 public key, hex."},
		},
		IssuePreKey, nil, &api.DXAPIEndPointResponsePossibilities{
			"success": {StatusCode: http.StatusOK, Description: "The vault's keys and the pre-key's index.", DataTemplate: []*api.DXAPIEndPointParameter{
				{NameId: "i", Type: types.APIParameterTypeNonEmptyString, IsMustExist: true},
				{NameId: "b0", Type: types.APIParameterTypeNonEmptyString, IsMustExist: true},
				{NameId: "b1", Type: types.APIParameterTypeNonEmptyString, IsMustExist: true},
				{NameId: "b2", Type: types.APIParameterTypeNonEmptyString, IsMustExist: true},
			}},
		}, nil, nil, 0, "")

	a.NewEndPoint("Add a note", "Stores a note in the vault.", "/addNote", http.MethodPost, api.EndPointTypeHTTPEndToEndEncryptionV2, dxhttp.RequestContentTypeApplicationJSON,
		[]api.DXAPIEndPointParameter{
			{NameId: "title", Type: types.APIParameterTypeNonEmptyString, IsMustExist: true},
			{NameId: "text", Type: types.APIParameterTypeString},
		},
		AddNote, nil, &api.DXAPIEndPointResponsePossibilities{
			"success": {StatusCode: http.StatusOK, Description: "The note is stored.", DataTemplate: []*api.DXAPIEndPointParameter{
				{NameId: "note_id", Type: types.APIParameterTypeNonEmptyString, IsMustExist: true},
			}},
		}, nil, []string{"NOTES.WRITE"}, 0, "")

	a.NewEndPoint("Upload an attachment", "Streams a file into the vault.", "/uploadAttachment", http.MethodPost, api.EndPointTypeHTTPUploadStream, dxhttp.RequestContentTypeApplicationOctetStream,
		[]api.DXAPIEndPointParameter{
			{NameId: "note_id", Type: types.APIParameterTypeNonEmptyString, IsMustExist: true, Description: "The note the attachment belongs to."},
			{NameId: "tags", Type: types.APIParameterTypeArrayString, Description: "Words to find the attachment by."},
		},
		UploadAttachment, nil, &api.DXAPIEndPointResponsePossibilities{
			"success": {StatusCode: http.StatusOK, Description: "The attachment is stored.", DataTemplate: []*api.DXAPIEndPointParameter{
				{NameId: "attachment_id", Type: types.APIParameterTypeNonEmptyString, IsMustExist: true},
				{NameId: "size", Type: types.APIParameterTypeInt64ZP, IsMustExist: true},
			}},
		}, nil, []string{"NOTES.WRITE"}, 1073741824, "")

	a.NewEndPoint("Download an attachment", "Streams a file out of the vault.", "/downloadAttachment", http.MethodPost, api.EndPointTypeHTTPDownloadStream, dxhttp.RequestContentTypeApplicationJSON,
		[]api.DXAPIEndPointParameter{
			{NameId: "attachment_id", Type: types.APIParameterTypeNonEmptyString, IsMustExist: true},
		},
		DownloadAttachment, nil, &api.DXAPIEndPointResponsePossibilities{
			"success":   {StatusCode: http.StatusOK, Description: "The attachment's bytes."},
			"not_found": {StatusCode: http.StatusNotFound, Description: "No attachment has the id."},
		}, nil, []string{"NOTES.READ"}, 0, "")

	a.NewEndPoint("Export the vault", "Asked for in the envelope, answered as a plain stream.", "/exportVault", http.MethodPost, api.EndPointTypeHTTPDownloadStreamV2, dxhttp.RequestContentTypeApplicationJSON,
		[]api.DXAPIEndPointParameter{
			{NameId: "format", Type: types.APIParameterTypeNonEmptyString, IsMustExist: true, Enum: []any{"zip", "tar"}},
		},
		ExportVault, nil, &api.DXAPIEndPointResponsePossibilities{
			"success": {StatusCode: http.StatusOK, Description: "The archive."},
		}, nil, []string{"NOTES.READ"}, 0, "")
}
