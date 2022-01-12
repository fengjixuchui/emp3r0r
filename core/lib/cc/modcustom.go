package cc

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strconv"

	emp3r0r_data "github.com/jm33-m0/emp3r0r/core/lib/data"
)

// ModConfig config.json of a module
// {
//     "name": "LES",
//     "exec": "les.sh",
//     "platform": "Linux",
//     "author": "jm33-ng",
//     "date": "2022-01-12",
//     "comment": "https://github.com/mzet-/linux-exploit-suggester",
//     "options": {
//         "args": ["--checksec", "run les.sh with this commandline arg"]
//     }
// }
type ModConfig struct {
	Name     string `json:"name"`
	Exec     string `json:"exec"`
	Platform string `json:"platform"`
	Author   string `json:"author"`
	Date     string `json:"date"`
	Comment  string `json:"comment"`

	// option: [value, help]
	Options map[string][]string `json:"options"`
}

// moduleCustom run a custom module
func moduleCustom() {
}

// scan custom modules in ModuleDir,
// and update ModuleHelpers, ModuleDocs
func InitModules() {
	dirs, err := ioutil.ReadDir(ModuleDir)
	if err != nil {
		CliPrintError("Failed to scan custom modules: %v", err)
		return
	}

	for _, dir := range dirs {
		if !dir.IsDir() {
			continue
		}
		config_file := ModuleDir + dir.Name() + "/config.json"
		config, err := readModCondig(config_file)
		if err != nil {
			CliPrintWarning("Reading config from %s: %v", dir.Name(), err)
			continue
		}

		ModuleHelpers[config.Name] = moduleCustom
		emp3r0r_data.ModuleComments[config.Name] = config.Comment

		err = updateModuleHelp(config)
		if err != nil {
			CliPrintWarning("Loading config from %s: %v", config.Name, err)
			continue
		}
		CliPrintInfo("Loaded module %s", strconv.Quote(config.Name))
	}
	CliPrintInfo("Loaded %d modules", len(ModuleHelpers))
}

// readModCondig read config.json of a module
func readModCondig(file string) (pconfig *ModConfig, err error) {
	// read JSON
	jsonData, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("Read %s: %v", file, err)
	}

	// parse the json
	var config = ModConfig{}
	err = json.Unmarshal(jsonData, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON config: %v", err)
	}
	pconfig = &config
	return
}

func updateModuleHelp(config *ModConfig) error {
	for opt, val_help := range config.Options {
		if len(val_help) < 2 {
			return fmt.Errorf("%s config error: %s incomplete", config.Name, opt)
		}
		emp3r0r_data.ModuleHelp[config.Name] = map[string]string{opt: val_help[1]}
	}
	return nil
}
