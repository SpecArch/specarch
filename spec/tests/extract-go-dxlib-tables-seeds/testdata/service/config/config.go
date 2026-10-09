package config

import (
	"os"

	"github.com/donnyhardyanto/dxlib/configuration"
	"github.com/donnyhardyanto/dxlib/utils"
	dxos "github.com/donnyhardyanto/dxlib/utils/os"
)

// Define declares the desk's settings.
func Define() {
	configuration.Manager.NewConfiguration("storage", "config/storage.json", "json", false, true, utils.JSON{
		"pool": 4,
		"tls":  utils.JSON{"enabled": false},
	}, []string{"password"})
	_ = os.Getenv("DESK_LOG_LEVEL")
	_ = dxos.GetEnvDefaultValueAsInt("DESK_PORT", 8080)
	_ = dxos.GetEnvDefaultValue("DESK_SIGNING_TOKEN", "")
	_ = os.Getenv(settingName())
}

func settingName() string { return "DESK_REGION" }
