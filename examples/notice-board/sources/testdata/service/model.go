package service

import (
	"github.com/donnyhardyanto/dxlib/databases/models"
	"github.com/donnyhardyanto/dxlib/types"
)

// boardModel is the notice board's database, as dxlib builds its tables
// from the model; the in-memory store stands in for it in this example.
var boardModel = models.NewModelDB("board", nil)

var boardSchema = models.NewModelDBSchema(boardModel, "board", 1)

// noticeTable keeps the notices.
var noticeTable = models.NewModelDBTable(boardSchema, "notice", 1, map[string]*models.ModelDBField{
	"id":     {Order: 1, Type: types.DataTypeBigSerial, IsPrimaryKey: true},
	"title":  {Order: 2, Type: types.DataTypeString255, IsNotNull: true, IsUnique: true},
	"body":   {Order: 3, Type: types.DataTypeString8096, IsNotNull: true},
	"pinned": {Order: 4, Type: types.DataTypeBool, IsNotNull: true, DefaultValue: false},
}, models.ModelDBTDEConfig{})
