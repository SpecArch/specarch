package seed

import (
	"context"

	"github.com/donnyhardyanto/dxlib/log"
	"github.com/donnyhardyanto/dxlib/utils"
	"github.com/donnyhardyanto/dxlib_module/module/user_management"
)

// Seed fills the role and privilege tables the permission check reads.
func Seed(ctx context.Context, l *log.DXLog, extra []string) {
	um := &user_management.ModuleUserManagement
	um.Privilege.InsertReturningId(ctx, l, utils.JSON{"nameid": "LOANS.READ", "name": "Read loans", "description": "Read the loans of any member."})
	um.Privilege.InsertReturningId(ctx, l, utils.JSON{"nameid": "LOANS.RENEW", "name": "Renew loans"})
	staff, _ := um.Role.InsertReturningId(ctx, l, utils.JSON{"nameid": "desk-staff", "name": "Desk staff", "description": "Works at the lending desk."})
	um.RolePrivilegeMustInsert(l, staff, "LOANS.READ")
	um.RolePrivilegeMustInsert(l, staff, "LOANS.RENEW")
	um.RolePrivilegeMustInsert(l, staff, "BRANCHES.READ")
	super, _ := um.Role.InsertReturningId(ctx, l, utils.JSON{"nameid": "supervisor"})
	um.RolePrivilegeMustInsert(l, super, "EVERYTHING")
	um.RolePrivilegeMustInsert(l, super, "REPORT_RUN")
	um.RolePrivilegeMustInsert(l, super, "REPORT.RUN")
	manager, _ := um.Role.InsertReturningId(ctx, l, utils.JSON{"nameid": "Branch_Manager"})
	um.RolePrivilegeMustInsert(l, manager, "LOANS.READ")
	for _, p := range extra {
		um.RolePrivilegeMustInsert(l, staff, p)
	}
	um.RolePrivilegeMustInsert(l, roleOf("kiosk"), "LOANS.READ")
	um.RolePrivilegeMustInsert(l, staff, privilegeOf())
}

func roleOf(string) int64 { return 0 }

func privilegeOf() string { return "LOANS.WRITE" }
