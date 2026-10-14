package seed

import (
	"context"

	"github.com/donnyhardyanto/dxlib/log"
	"github.com/donnyhardyanto/dxlib/utils"
	"github.com/donnyhardyanto/dxlib_module/module/user_management"
)

// Seed fills the role and privilege tables the permission check reads.
func Seed(ctx context.Context, l *log.DXLog) {
	um := &user_management.ModuleUserManagement
	um.Privilege.InsertReturningId(ctx, l, utils.JSON{"nameid": "LOANS.READ", "name": "Read loans", "description": "Read the loans of any member."})
	staff, _ := um.Role.InsertReturningId(ctx, l, utils.JSON{"nameid": "DESK_STAFF", "name": "Desk staff", "description": "Works at the lending desk."})
	um.RolePrivilegeMustInsert(l, staff, "LOANS.READ")
	um.RolePrivilegeMustInsert(l, staff, "LOANS.RENEW")
	super, _ := um.Role.InsertReturningId(ctx, l, utils.JSON{"nameid": "DESK_SUPERVISOR", "description": "Supervises the desk."})
	um.RolePrivilegeMustInsert(l, super, "LOANS.READ")
	runner, _ := um.Role.InsertReturningId(ctx, l, utils.JSON{"nameid": "DESK__RUNNER"})
	um.RolePrivilegeMustInsert(l, runner, "LOANS.READ")
}
