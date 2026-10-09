package api

import "github.com/donnyhardyanto/dxlib/tables"

var LoanList = tables.NewDXTableSimple("desk", "desk.loans", "loan", "desk.v_loans",
	"id", "uid", "", "data", nil,
	[][]string{{"title"}},
	[]string{"title", "member_name"},
	[]string{"due_on", "title"},
	[]string{"member_id", "state"},
)

var BranchList = tables.NewDXTableSimple("desk", "public.branches", "branch", "public.branches",
	"code", "", "", "data", nil, nil, searchable, nil, nil)

var searchable = []string{"name"}
