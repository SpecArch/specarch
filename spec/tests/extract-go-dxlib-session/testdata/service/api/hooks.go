package api

import (
	"encoding/json"
	"errors"

	"github.com/donnyhardyanto/dxlib/api"
	"github.com/donnyhardyanto/dxlib/utils"
	"github.com/donnyhardyanto/dxlib/utils/crypto/datablock"
	"github.com/donnyhardyanto/dxlib/utils/lv"
)

// session is what the exchange stored for an index.
type session struct {
	clientKey, signingKey, shared1, shared2 []byte
}

var sessions = map[string]*session{}

// Hooks sets dxlib's persistent-session hooks: V4 carries the pre-key
// envelope through the datablock, V3 only passes the body through.
func Hooks() {
	api.OnE2EEV4Unpack = openSession
	api.OnE2EEV4Pack = sealSession
	api.OnE2EEV3Unpack = passThrough
	api.OnE2EEV3Pack = passBack
}

func openSession(aepr *api.DXAPIEndPointRequest, body utils.JSON) ([]*lv.LVLE, utils.JSON, error) {
	index, _ := body["i"].(string)
	data, _ := body["d"].(string)
	elements, err := open(index, data)
	if err != nil {
		return nil, nil, err
	}
	out := make([]*lv.LVLE, 0, len(elements))
	for _, e := range elements {
		le, err := lv.NewLVLE(e.Value)
		if err != nil {
			return nil, nil, err
		}
		out = append(out, le)
	}
	return out, utils.JSON{"index": index}, nil
}

// open finds the session and opens the envelope with its keys.
func open(index, data string) ([]*lv.LV, error) {
	s := sessions[index]
	if s == nil {
		return nil, errors.New("SESSION_NOT_FOUND")
	}
	return datablock.UnpackLVPayload(index, s.clientKey, s.shared1, data)
}

func sealSession(aepr *api.DXAPIEndPointRequest, state utils.JSON, payloads ...*lv.LVLE) ([]byte, error) {
	index, _ := state["index"].(string)
	s := sessions[index]
	if s == nil {
		return nil, errors.New("SESSION_NOT_FOUND")
	}
	elements := make([]*lv.LV, 0, len(payloads))
	for _, p := range payloads {
		e, err := lv.NewLV(p.Value)
		if err != nil {
			return nil, err
		}
		elements = append(elements, e)
	}
	data, err := datablock.PackLVPayload(index, s.signingKey, s.shared2, elements...)
	if err != nil {
		return nil, err
	}
	return json.Marshal(utils.JSON{"d": data})
}

func passThrough(aepr *api.DXAPIEndPointRequest, body utils.JSON) ([]*lv.LV, utils.JSON, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, nil, err
	}
	e, err := lv.NewLV(raw)
	if err != nil {
		return nil, nil, err
	}
	return []*lv.LV{e}, utils.JSON{}, nil
}

func passBack(aepr *api.DXAPIEndPointRequest, state utils.JSON, payloads ...*lv.LV) ([]byte, error) {
	return payloads[len(payloads)-1].Value, nil
}
