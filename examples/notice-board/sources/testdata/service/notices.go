package service

import (
	"errors"
	"net/http"

	"github.com/donnyhardyanto/dxlib/api"
	"github.com/donnyhardyanto/dxlib/utils"
)

// reasonBodyTooLong is the reason a notice whose body is too long is
// refused with.
const reasonBodyTooLong = "NOTICE_BODY_TOO_LONG"

// maxBody is the longest body a notice may have, in bytes.
const maxBody = 4000

// NoticeList answers the notices whose title holds the filter text.
func NoticeList(aepr *api.DXAPIEndPointRequest) (err error) {
	_, filter, err := aepr.GetParameterValueAsString("filter_text", "")
	if err != nil {
		return err
	}
	aepr.WriteResponseAsJSON(http.StatusOK, nil, utils.JSON{"notices": store.list(filter)})
	return nil
}

// NoticeRead answers one notice.
func NoticeRead(aepr *api.DXAPIEndPointRequest) (err error) {
	_, id, err := aepr.GetParameterValueAsInt64("id")
	if err != nil {
		return err
	}
	notice, ok := store.get(id)
	if !ok {
		return aepr.WriteResponseAndNewErrorf(http.StatusNotFound, "NOTICE_NOT_FOUND", "no notice %d", id)
	}
	aepr.WriteResponseAsJSON(http.StatusOK, nil, utils.JSON{"notice": notice})
	return nil
}

// NoticeCreate posts a notice.
func NoticeCreate(aepr *api.DXAPIEndPointRequest) (err error) {
	_, title, err := aepr.GetParameterValueAsString("title")
	if err != nil {
		return err
	}
	_, body, err := aepr.GetParameterValueAsString("body")
	if err != nil {
		return err
	}
	_, pinned, err := aepr.GetParameterValueAsBool("pinned", false)
	if err != nil {
		return err
	}
	if len(body) > maxBody {
		return aepr.WriteResponseAndNewErrorf(http.StatusUnprocessableEntity, reasonBodyTooLong, "the body has %d bytes", len(body))
	}
	if store.titleTaken(title) {
		return aepr.WriteResponseAndNewErrorf(http.StatusConflict, "NOTICE_TITLE_TAKEN", "a notice is titled %q", title)
	}
	id := store.add(title, body, pinned)
	aepr.WriteResponseAsJSON(http.StatusOK, nil, utils.JSON{"id": id})
	return nil
}

// NoticeDelete removes a notice.
func NoticeDelete(aepr *api.DXAPIEndPointRequest) (err error) {
	_, id, err := aepr.GetParameterValueAsInt64("id")
	if err != nil {
		return err
	}
	if !store.remove(id) {
		return aepr.WriteResponseAndNewErrorf(http.StatusNotFound, "NOTICE_NOT_FOUND", "no notice %d", id)
	}
	aepr.WriteResponseAsJSON(http.StatusOK, nil, nil)
	return nil
}

// RequireSession refuses a request that carries no session.
func RequireSession(aepr *api.DXAPIEndPointRequest) (err error) {
	if aepr.Request.Header.Get("Authorization") == "" {
		aepr.WriteResponseAsErrorMessageNotLogged(http.StatusUnauthorized, "SESSION_REQUIRED", "sign in first")
		return errors.New("no session")
	}
	return nil
}
