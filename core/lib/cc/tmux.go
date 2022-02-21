package cc

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/fatih/color"
	"github.com/jm33-m0/emp3r0r/core/lib/util"
	"golang.org/x/crypto/ssh/terminal"
)

// Emp3r0rPane a tmux window/pane that makes emp3r0r CC's interface
type Emp3r0rPane struct {
	ID  string   // tmux pane unique ID, needs to be converted to index when using select-pane
	PID int      // PID of the process running in tmux pane
	FD  *os.File // write to this file to get your message displayed on this pane
}

var (
	// Displays system info of selected agent
	AgentInfoWindow *Emp3r0rPane

	// Displays agent output, separated from logs
	AgentOutputWindow *Emp3r0rPane

	// Displays agent list
	AgentListWindow *Emp3r0rPane

	// Displays bash shell for selected agent
	AgentShellWindow *Emp3r0rPane

	// Put all windows in this map
	TmuxWindows = make(map[string]*Emp3r0rPane)
)

// TmuxPrintf like printf, but prints to a tmux pane/window
// id: pane unique id
func (pane *Emp3r0rPane) TmuxPrintf(clear bool, format string, a ...interface{}) {
	id := pane.ID
	msg := fmt.Sprintf(format, a...)

	if clear {
		err := TmuxClearPane(id)
		if err != nil {
			CliPrintWarning("Clear pane: %v", err)
		}
		msg = fmt.Sprintf("%s%s", ClearTerm, msg)
	}

	idx := TmuxPaneID2Index(id)
	if idx < 0 {
		CliPrintWarning("Cannot find tmux window "+id+
			", printing to main window instead.\n\n"+
			format, a...)
	}

	// find target pane and print msg
	for pane_id, window := range TmuxWindows {
		if pane_id != id {
			continue
		}
		_, err = window.FD.WriteString(msg)
		if err != nil {
			CliPrintWarning("Cannot print on tmux window "+id+
				", printing to main window instead.\n\n"+
				format, a...)
		}
		break
	}
}

func TmuxIsPaneAlive(id string) bool {
	for pane_id, pane := range TmuxWindows {
		if id != pane_id {
			continue
		}
		b := make([]byte, 1)
		_, err := pane.FD.Read(b)
		if err == nil {
			return true
		}
		break
	}

	return false
}

func TmuxClearPane(id string) (err error) {
	idx := TmuxPaneID2Index(id)
	job := fmt.Sprintf("tmux clear-history -t %d", idx)
	out, err := exec.Command("/bin/sh", "-c", job).CombinedOutput()
	if err != nil {
		err = fmt.Errorf("exec tmux clear pane: %s\n%v", out, err)
		return
	}
	return
}

// TmuxResizePane resize pane in x/y to number of lines
func TmuxResizePane(id, direction string, lines int) (err error) {
	idx := TmuxPaneID2Index(id)
	if idx < 0 {
		return fmt.Errorf("Pane %s not found", id)
	}
	job := fmt.Sprintf("tmux resize-pane -t %d -%s %d", idx, direction, lines)
	out, err := exec.Command("/bin/sh", "-c", job).CombinedOutput()
	if err != nil {
		err = fmt.Errorf("exec tmux resize-pane: %s\n%v", out, err)
		return
	}
	return
}
func (pane *Emp3r0rPane) TmuxKillPane() (err error) {
	id := pane.ID
	idx := TmuxPaneID2Index(id)
	if idx < 0 {
		return fmt.Errorf("Pane %s not found", id)
	}
	job := fmt.Sprintf("tmux kill-pane -t %d", idx)
	out, err := exec.Command("/bin/sh", "-c", job).CombinedOutput()
	if err != nil {
		err = fmt.Errorf("exec tmux kill-pane: %s\n%v", out, err)
		return
	}
	return
}

// TmuxDeinitWindows close previously opened tmux windows
func TmuxDeinitWindows() {
	for _, pane := range TmuxWindows {
		pane.TmuxKillPane()
	}
}

// TermSize Get terminal size
func TermSize() (width, height int, err error) {
	width, height, err = terminal.GetSize(int(os.Stdin.Fd()))
	return
}

// TmuxInitWindows split current terminal into several windows/panes
// - command output window
// - current agent info
func TmuxInitWindows() (err error) {
	// pane title
	TmuxSetPaneTitle("Command", "")

	// check terminal size, prompt user to run emp3r0r C2 in a bigger window
	w, h, err := TermSize()
	if err != nil {
		CliPrintWarning("Get terminal size: %v", err)
	}
	if w < 200 || h < 60 {
		CliPrintWarning("I need a bigger window, make sure the window size is at least 200x60 (w*h)")
		CliPrintWarning("Please maximize the terminal window if possible")
	}

	// we don't want the tmux pane be killed
	// so easily. Yes, fuck /bin/cat, we use our own cat
	cat := "./cat"
	if !util.IsFileExist(cat) {
		err = fmt.Errorf("Check if ./build/cat exists. If not, build it")
		return
	}

	// system info of selected agent
	pane, err := TmuxNewPane("System Info", "h", "", 24, cat)
	if err != nil {
		return
	}
	AgentInfoWindow = pane
	TmuxWindows[AgentInfoWindow.ID] = AgentInfoWindow
	AgentInfoWindow.TmuxPrintf(true, color.HiYellowString("Try `target 0`?"))

	// Agent output
	pane, err = TmuxNewPane("Agent", "v", "", 24, cat)
	if err != nil {
		return
	}
	AgentOutputWindow = pane
	TmuxWindows[AgentOutputWindow.ID] = AgentOutputWindow
	AgentOutputWindow.TmuxPrintf(true, color.HiYellowString("..."))

	// Agent list: ls_targets
	pane, err = TmuxNewPane("Agent List", "v", "", 33, cat)
	if err != nil {
		return
	}
	AgentListWindow = pane
	TmuxWindows[AgentListWindow.ID] = AgentListWindow
	AgentListWindow.TmuxPrintf(true, color.HiYellowString("No agents connected"))

	return
}

// TmuxNewPane split tmux window, and run command in the new pane
// hV: horizontal or vertical split
// target_pane: target_pane tmux index, split this pane
// size: percentage, do not append %
func TmuxNewPane(title, hV string, target_pane_id string, size int, cmd string) (pane *Emp3r0rPane, err error) {
	if os.Getenv("TMUX") == "" ||
		!util.IsCommandExist("tmux") {

		err = errors.New("You need to run emp3r0r under `tmux`")
		return
	}
	target_pane := TmuxPaneID2Index(target_pane_id)
	if target_pane < 0 {

	}

	job := fmt.Sprintf(`tmux split-window -%s -p %d -P -d -F "#{pane_id}:#{pane_pid}:#{pane_tty}" '%s'`,
		hV, size, cmd)
	if target_pane > 0 {
		job = fmt.Sprintf(`tmux split-window -t %d -%s -p %d -P -d -F "#{pane_id}:#{pane_pid}:#{pane_tty}" '%s'`,
			target_pane, hV, size, cmd)
	}

	out, err := exec.Command("/bin/sh", "-c", job).CombinedOutput()
	if err != nil {
		err = fmt.Errorf("exec tmux: %s\n%v", out, err)
		return
	}
	tmux_result := string(out)
	tmux_res_split := strings.Split(tmux_result, ":")
	if len(tmux_res_split) != 3 {
		err = fmt.Errorf("tmux result cannot be parsed: %s", tmux_result)
		return
	}

	pane = &Emp3r0rPane{}
	pane.ID = tmux_res_split[0]
	pane.PID, err = strconv.Atoi(tmux_res_split[1])
	if err != nil {
		err = fmt.Errorf("parsing pane pid: %v", err)
		return
	}
	tty_path := strings.TrimSpace(tmux_res_split[2])
	tty_file, err := os.OpenFile(tty_path, os.O_RDWR, 0777)
	if err != nil {
		err = fmt.Errorf("open pane tty (%s): %v", tty_path, err)
		return
	}
	pane.FD = tty_file // no need to close files, since CC's interface always needs them

	err = TmuxSetPaneTitle(title, pane.ID)
	return
}

func TmuxSetPaneTitle(title, pane_id string) error {
	// set pane title
	tmux_cmd := []string{"select-pane", "-t", pane_id, "-T", title}
	if pane_id == "" {
		tmux_cmd = []string{"select-pane", "-T", title}
	}
	out, err := exec.Command("tmux", tmux_cmd...).CombinedOutput()
	if err != nil {
		err = fmt.Errorf("%s\n%v", out, err)
	}

	return err
}

// Convert tmux pane's unique ID to index number, for use with select-pane
// returns -1 if failed
func TmuxPaneID2Index(id string) (index int) {
	index = -1

	out, err := exec.Command("/bin/sh", "-c", "tmux list-pane").CombinedOutput()
	if err != nil {
		CliPrintWarning("exec tmux: %s\n%v", out, err)
		return
	}
	tmux_res := strings.Split(string(out), "\n")
	if len(tmux_res) < 1 {
		CliPrintWarning("parse tmux output: no pane found: %s", out)
		return
	}
	for _, line := range tmux_res {
		if strings.Contains(line, id) {
			line_split := strings.Fields(line)
			if len(line_split) < 7 {
				CliPrintWarning("parse tmux output: format error: %s", out)
				return
			}
			idx := strings.TrimSuffix(line_split[0], ":")
			i, err := strconv.Atoi(idx)
			if err != nil {
				CliPrintWarning("parse tmux output: invalid index (%s): %s", idx, out)
				return
			}
			index = i
			break
		}
	}

	return
}

// TmuxNewWindow split tmux window, and run command in the new pane
func TmuxNewWindow(name, cmd string) error {
	if os.Getenv("TMUX") == "" ||
		!util.IsCommandExist("tmux") {
		return errors.New("You need to run emp3r0r under `tmux`")
	}

	tmuxCmd := fmt.Sprintf("tmux new-window -n %s '%s || read'", name, cmd)
	job := exec.Command("/bin/sh", "-c", tmuxCmd)
	out, err := job.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %s", err, out)
	}

	return nil
}

// TmuxSplit split tmux window, and run command in the new pane
func TmuxSplit(hV, cmd string) error {
	if os.Getenv("TMUX") == "" ||
		!util.IsCommandExist("tmux") ||
		!util.IsCommandExist("less") {

		return errors.New("You need to run emp3r0r under `tmux`, and make sure `less` is installed")
	}

	job := fmt.Sprintf("tmux split-window -%s '%s || read'", hV, cmd)

	out, err := exec.Command("/bin/sh", "-c", job).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %s", err, out)
	}

	return nil
}
