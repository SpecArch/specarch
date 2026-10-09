package lending

import (
	"context"
	"database/sql"
	_ "embed"
	"net/http"
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

// Checked is a handler that runs only for a caller whose role grants its
// permission. The role is the one the desk's terminal sends in the
// Desk-Role header.
type Checked struct {
	Permission string
	Handler    http.HandlerFunc
	db         *sql.DB
}

// Require is the service's permission check: h runs only for a caller
// whose role grants permission.
func (s *Server) Require(permission string, h http.HandlerFunc) Checked {
	return Checked{Permission: permission, Handler: h, db: s.db}
}

// ServeHTTP answers 403 to a caller whose role does not grant the
// permission, and runs the handler for any other.
func (c Checked) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ok, err := Allowed(r.Context(), c.db, r.Header.Get("Desk-Role"), c.Permission)
	switch {
	case err != nil:
		w.WriteHeader(http.StatusInternalServerError)
	case !ok:
		w.WriteHeader(http.StatusForbidden)
	default:
		c.Handler(w, r)
	}
}
