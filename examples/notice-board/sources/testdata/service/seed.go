package service

import (
	"context"

	"github.com/donnyhardyanto/dxlib/log"
	"github.com/donnyhardyanto/dxlib/utils"
	"github.com/donnyhardyanto/dxlib_module/module/user_management"
)

// Seed fills dxlib_module's role and privilege tables, which its
// permission check reads, the first time the board starts.
func Seed(ctx context.Context, l *log.DXLog) {
	um := &user_management.ModuleUserManagement
	um.Privilege.InsertReturningId(ctx, l, utils.JSON{"nameid": "NOTICE.READ", "name": "Read notices", "description": "Read the notices on the board."})
	um.Privilege.InsertReturningId(ctx, l, utils.JSON{"nameid": "NOTICE.WRITE", "name": "Post notices", "description": "Post a notice to the board."})
	um.Privilege.InsertReturningId(ctx, l, utils.JSON{"nameid": "NOTICE.DELETE", "name": "Remove notices"})

	reader, _ := um.Role.InsertReturningId(ctx, l, utils.JSON{"nameid": "reader", "name": "Reader", "description": "Reads the board."})
	um.RolePrivilegeMustInsert(l, reader, "NOTICE.READ")

	editor, _ := um.Role.InsertReturningId(ctx, l, utils.JSON{"nameid": "editor", "name": "Editor"})
	um.RolePrivilegeMustInsert(l, editor, "NOTICE.READ")
	um.RolePrivilegeMustInsert(l, editor, "NOTICE.WRITE")
	um.RolePrivilegeMustInsert(l, editor, "NOTICE.DELETE")

	admin, _ := um.Role.InsertReturningId(ctx, l, utils.JSON{"nameid": "admin", "name": "Administrator", "description": "Runs the board."})
	um.RolePrivilegeMustInsert(l, admin, "EVERYTHING")
}
