package db

import (
	"github.com/donnyhardyanto/dxlib/databases/models"
	"github.com/donnyhardyanto/dxlib/types"
)

var Model = models.NewModelDB("desk", nil)

var Desk = models.NewModelDBSchema(Model, "desk", 1)

var Public = models.NewModelDBSchema(Model, "public", 2)

func idField() *models.ModelDBField { return &models.ModelDBField{Type: types.DataTypeID} }

// Loans is the loans a member takes out.
var Loans = models.NewModelDBTable(Desk, "loans", 1, map[string]*models.ModelDBField{
	"id":        {Order: 1, Type: types.DataTypeBigSerial, IsPrimaryKey: true},
	"member_id": {Order: 2, Type: types.DataTypeID, IsNotNull: true, References: "desk.members.id"},
	"title":     {Order: 3, Type: types.DataTypeString255, IsNotNull: true, IsUnique: true},
	"due_on":    {Order: 4, Type: types.DataTypeDate},
	"fee":       {Order: 5, Type: types.DataTypeMoney, IsNotNull: true},
	"notes":     {Order: 6, Type: types.DataTypeJSON},
	"place":     {Order: 7, Type: types.DataTypeGeometryPoint},
	"shelf":     {Order: 8, Type: shelfType},
	"weight":    {Order: 9, Type: types.DataTypeFloat32},
}, models.ModelDBTDEConfig{})

var shelfType = types.DataTypeString10

// Branches has no key and a column built by a function.
var Branches = models.NewModelDBTable(Public, "branches", 2, map[string]*models.ModelDBField{
	"code": idField(),
	"name": {Type: types.DataTypeString100, IsNotNull: true},
}, models.ModelDBTDEConfig{})

var Fines = models.NewModelDBTable(Desk, tableName(), 3, nil, models.ModelDBTDEConfig{})

var Holds = models.NewModelDBTable(nil, "holds", 4, nil, models.ModelDBTDEConfig{})

var Archive = models.NewModelDBTable(Desk, "archive", 5, archiveFields, models.ModelDBTDEConfig{})

var archiveFields = map[string]*models.ModelDBField{}

func tableName() string { return "fines" }

func init() {
	for _, year := range []string{"2025", "2026"} {
		models.NewModelDBTable(Desk, "loans_"+year, 9, nil, models.ModelDBTDEConfig{})
	}
}
