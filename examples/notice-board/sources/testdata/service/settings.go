package service

import (
	"os"

	"github.com/donnyhardyanto/dxlib/configuration"
	"github.com/donnyhardyanto/dxlib/utils"
)

// DefineSettings declares the board's configuration: config/board.json,
// with a page size the code defaults and a database password dxlib masks
// in its logs.
func DefineSettings() {
	configuration.Manager.NewConfiguration("board", "config/board.json", "json", false, true, utils.JSON{
		"page_size": 20,
	}, []string{"database.password"})
}

// banner is the line the board shows above the notices, if any.
func banner() string {
	return os.Getenv("NOTICE_BOARD_BANNER")
}
