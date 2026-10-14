package access

// The grants the permission check reads.
var grants = map[string][]string{"clerk": {"copies.read", "copies.write"}, "visitor": {"copies.read"}}
