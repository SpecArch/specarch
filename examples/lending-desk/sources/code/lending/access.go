package lending

import (
	"context"
	"database/sql"
	_ "embed"
)

// grantsQuery lists every role and permission it grants. The permission
// table printer runs the same query.
//
//go:embed grants.sql
var grantsQuery string

// Allowed reports whether the role grants the permission, as the
// role_permissions table, filled by seeds/001_roles.sql, says.
func Allowed(ctx context.Context, db *sql.DB, role, permission string) (bool, error) {
	var ok bool
	err := db.QueryRowContext(ctx,
		"SELECT EXISTS (SELECT 1 FROM ("+grantsQuery+") g WHERE g.role = $1 AND g.permission = $2)",
		role, permission).Scan(&ok)
	return ok, err
}
