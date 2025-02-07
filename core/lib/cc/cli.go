//go:build linux
// +build linux

package cc

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	cowsay "github.com/Code-Hex/Neo-cowsay/v2"
	"github.com/alecthomas/chroma/quick"
	"github.com/fatih/color"
	emp3r0r_def "github.com/jm33-m0/emp3r0r/core/lib/emp3r0r_def"
	"github.com/jm33-m0/emp3r0r/core/lib/tun"
	"github.com/jm33-m0/emp3r0r/core/lib/util"
	"github.com/olekukonko/tablewriter"
	"github.com/reeflective/console"
)

const (
	AppName = "emp3r0r"
)

var (
	// Store agents' output
	AgentOuputLogFile = ""

	// Emp3r0rConsole: the main console interface
	Emp3r0rConsole = console.New(AppName)
)

// CliMain launches the commandline UI
func CliMain() {
	// start all services
	go TLSServer()
	go ShadowsocksServer()
	go InitModules()

	// unlock incomplete downloads
	err := UnlockDownloads()
	if err != nil {
		LogDebug("UnlockDownloads: %v", err)
	}
	mainMenu := Emp3r0rConsole.NewMenu("")
	Emp3r0rConsole.SetPrintLogo(CliBanner)

	// History
	histFile := fmt.Sprintf("%s/%s.history", AppName, EmpWorkSpace)
	mainMenu.AddHistorySourceFile(AppName, histFile)

	// Commands
	mainMenu.SetCommands(Emp3r0rCommands(Emp3r0rConsole))

	// Interrupts
	mainMenu.AddInterrupt(io.EOF, exitEmp3r0r)

	// prompt
	prompt := mainMenu.Prompt()
	prompt.Primary = SetDynamicPrompt
	prompt.Secondary = func() string { return ">" }
	prompt.Right = func() string { return color.CyanString(time.Now().Format("03:04:05")) }
	prompt.Transient = func() string { return ">>>" }
	Emp3r0rConsole.NewlineBefore = true
	Emp3r0rConsole.NewlineAfter = true
	Emp3r0rConsole.NewlineWhenEmpty = true

	// Syntax highlighting
	Emp3r0rConsole.Shell().SyntaxHighlighter = highLighter

	// Tmux setup
	err = TmuxInitWindows()
	if err != nil {
		Logger.Fatal("Fatal TMUX error: %v, please run `tmux kill-session -t emp3r0r` and re-run emp3r0r", err)
	}
	defer TmuxDeinitWindows()

	// Redirect logs to agent response pane
	agent_resp_pane_tty, err := os.OpenFile(AgentRespPane.TTY, os.O_RDWR, 0)
	if err != nil {
		Logger.Fatal("Failed to open agent response pane: %v", err)
	}
	Logger.AddWriter(agent_resp_pane_tty)
	go Logger.Start()

	// Run the console
	Emp3r0rConsole.Start()
}

func highLighter(line []rune) string {
	var highlightedStr strings.Builder
	err := quick.Highlight(&highlightedStr, string(line), "shell", "terminal256", "monokai")
	if err != nil {
		return string(line)
	}

	return highlightedStr.String()
}

// SetDynamicPrompt set prompt with module and target info
func SetDynamicPrompt() string {
	shortName := "local" // if no target is selected
	prompt_arrow := color.New(color.Bold, color.FgHiCyan).Sprintf("\n$ ")
	prompt_name := color.New(color.Bold, color.FgBlack, color.BgHiWhite).Sprint(AppName)
	transport := color.New(color.FgRed).Sprint("local")

	if CurrentTarget != nil && IsAgentExist(CurrentTarget) {
		shortName = strings.Split(CurrentTarget.Tag, "-agent")[0]
		if CurrentTarget.HasRoot {
			prompt_arrow = color.New(color.Bold, color.FgHiGreen).Sprint("\n# ")
			prompt_name = color.New(color.Bold, color.FgBlack, color.BgHiGreen).Sprint(AppName)
		}
		transport = getTransport(CurrentTarget.Transport)
	}
	if CurrentMod == "<blank>" {
		CurrentMod = "none" // if no module is selected
	}
	agent_name := color.New(color.FgCyan, color.Underline).Sprint(shortName)
	mod_name := color.New(color.FgHiBlue).Sprint(CurrentMod)

	dynamicPrompt := fmt.Sprintf("%s - %s @%s (%s) "+prompt_arrow,
		prompt_name,
		transport,
		agent_name,
		mod_name,
	)
	return dynamicPrompt
}

func getTransport(transportStr string) string {
	transportStr = strings.ToLower(transportStr)
	switch {
	case strings.Contains(transportStr, "http2"):
		return color.New(color.FgHiBlue).Sprint("http2")
	case strings.Contains(transportStr, "shadowsocks"):
		if strings.Contains(transportStr, "kcp") {
			return color.New(color.FgHiMagenta).Sprint("kcp")
		}
		return color.New(color.FgHiMagenta).Sprint("ss")
	case strings.Contains(transportStr, "tor"):
		return color.New(color.FgHiGreen).Sprint("tor")
	case strings.Contains(transportStr, "cdn"):
		return color.New(color.FgGreen).Sprint("cdn")
	case strings.Contains(transportStr, "reverse proxy"):
		return color.New(color.FgHiCyan).Sprint("rproxy")
	case strings.Contains(transportStr, "auto proxy"):
		return color.New(color.FgHiYellow).Sprint("aproxy")
	case strings.Contains(transportStr, "proxy"):
		return color.New(color.FgHiYellow).Sprint("proxy")
	default:
		return color.New(color.FgHiWhite).Sprint("unknown")
	}
}

// CliBanner prints banner
func CliBanner(console *console.Console) {
	data, encodingErr := base64.StdEncoding.DecodeString(cliBannerB64)
	if encodingErr != nil {
		Logger.Fatal("failed to print banner: %v", encodingErr.Error())
	}
	banner := strings.Builder{}
	banner.Write(data)

	// print banner line by line
	cow, encodingErr := cowsay.New(
		cowsay.BallonWidth(100),
		cowsay.Random(),
	)
	if encodingErr != nil {
		Logger.Fatal("CowSay: %v", encodingErr)
	}

	// C2 names
	encodingErr = LoadCACrt()
	if encodingErr != nil {
		Logger.Fatal("Failed to parse CA cert: %v", encodingErr)
	}
	c2_names := tun.NamesInCert(ServerCrtFile)
	if len(c2_names) <= 0 {
		Logger.Fatal("C2 has no names?")
	}
	name_list := strings.Join(c2_names, ", ")

	say, encodingErr := cow.Say(fmt.Sprintf("welcome! you are using version %s,\n"+
		"C2 listening on *:%s,\n"+
		"Shadowsocks server port *:%s,\n"+
		"KCP port *:%s,\n"+
		"C2 names: %s\n"+
		"CA fingerprint: %s",
		emp3r0r_def.Version,
		RuntimeConfig.CCPort,
		RuntimeConfig.ShadowsocksServerPort,
		RuntimeConfig.KCPServerPort,
		name_list,
		RuntimeConfig.CAFingerprint))
	if encodingErr != nil {
		Logger.Fatal("CowSay: %v", encodingErr)
	}
	banner.WriteString(color.CyanString("%s\n\n", say))
	fmt.Print(banner.String())
}

// CliPrettyPrint prints two-column help info
func CliPrettyPrint(header1, header2 string, map2write *map[string]string) {
	if IsAPIEnabled {
		// send to socket
		var resp APIResponse
		msg, marshalErr := json.Marshal(map2write)
		if marshalErr != nil {
			Emp3r0rConsole.Printf("CliPrettyPrint: %v\n", marshalErr)
		}
		resp.MsgData = msg
		resp.Alert = false
		resp.MsgType = JSON
		data, marshalErr := json.Marshal(resp)
		if marshalErr != nil {
			Emp3r0rConsole.Printf("CliPrettyPrint: %v\n", marshalErr)
		}
		_, marshalErr = APIConn.Write([]byte(data))
		if marshalErr != nil {
			Emp3r0rConsole.Printf("CliPrettyPrint: %v\n", marshalErr)
		}
	}

	// build table
	tdata := [][]string{}
	tableString := &strings.Builder{}
	table := tablewriter.NewWriter(tableString)
	table.SetHeader([]string{header1, header2})
	table.SetBorder(true)
	table.SetRowLine(true)
	table.SetAutoWrapText(true)
	table.SetColWidth(50)

	// color
	table.SetHeaderColor(tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor})

	table.SetColumnColor(tablewriter.Colors{tablewriter.FgBlueColor},
		tablewriter.Colors{tablewriter.FgBlueColor})

	// fill table
	for c1, c2 := range *map2write {
		tdata = append(tdata,
			[]string{util.SplitLongLine(c1, 50), util.SplitLongLine(c2, 50)})
	}
	table.AppendBulk(tdata)
	table.Render()
	out := tableString.String()
	AdaptiveTable(out)
	LogMsg("\n%s", out)
}

// encoded logo of emp3r0r
const cliBannerB64 string = `
CuKWkeKWkeKWkeKWkeKWkeKWkeKWkSDilpHilpHilpEgICAg4paR4paR4paRIOKWkeKWkeKWkeKW
keKWkeKWkSAg4paR4paR4paR4paR4paR4paRICDilpHilpHilpHilpHilpHilpEgICDilpHilpHi
lpHilpHilpHilpEgIOKWkeKWkeKWkeKWkeKWkeKWkQrilpLilpIgICAgICDilpLilpLilpLilpIg
IOKWkuKWkuKWkuKWkiDilpLilpIgICDilpLilpIgICAgICDilpLilpIg4paS4paSICAg4paS4paS
IOKWkuKWkiAg4paS4paS4paS4paSIOKWkuKWkiAgIOKWkuKWkgrilpLilpLilpLilpLilpIgICDi
lpLilpIg4paS4paS4paS4paSIOKWkuKWkiDilpLilpLilpLilpLilpLilpIgICDilpLilpLilpLi
lpLilpIgIOKWkuKWkuKWkuKWkuKWkuKWkiAg4paS4paSIOKWkuKWkiDilpLilpIg4paS4paS4paS
4paS4paS4paSCuKWk+KWkyAgICAgIOKWk+KWkyAg4paT4paTICDilpPilpMg4paT4paTICAgICAg
ICAgICDilpPilpMg4paT4paTICAg4paT4paTIOKWk+KWk+KWk+KWkyAg4paT4paTIOKWk+KWkyAg
IOKWk+KWkwrilojilojilojilojilojilojilogg4paI4paIICAgICAg4paI4paIIOKWiOKWiCAg
ICAgIOKWiOKWiOKWiOKWiOKWiOKWiCAg4paI4paIICAg4paI4paIICDilojilojilojilojiloji
loggIOKWiOKWiCAgIOKWiOKWiAoKCmEgbGludXggcG9zdC1leHBsb2l0YXRpb24gZnJhbWV3b3Jr
IG1hZGUgYnkgbGludXggdXNlcgoKaHR0cHM6Ly9naXRodWIuY29tL2ptMzMtbTAvZW1wM3IwcgoK
Cg==
`

// automatically resize CommandPane according to table width
func AdaptiveTable(tableString string) {
	TmuxUpdatePanes()
	row_len := len(strings.Split(tableString, "\n")[0])
	if AgentRespPane.Width < row_len {
		LogDebug("Command Pane %d vs %d table width, resizing", CommandPane.Width, row_len)
		AgentRespPane.ResizePane("x", row_len)
	}
}
