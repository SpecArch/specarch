package lending

// Roles is the seed of the roles table: each role and the permissions it
// grants.
var Roles = map[string][]string{
	"desk-staff": {"members.write", "loans.write", "loans.read"},
}
