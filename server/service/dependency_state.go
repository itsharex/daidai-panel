package service

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"daidai-panel/config"
	"daidai-panel/model"
)

func DependencyInstalled(depType, name string) bool {
	name = strings.TrimSpace(name)
	if depType == "" || name == "" {
		return false
	}

	depsDir := filepath.Join(config.C.Data.Dir, "deps")
	switch depType {
	case model.DepTypeNodeJS:
		modDir := filepath.Join(depsDir, "nodejs", "node_modules", filepath.FromSlash(name))
		if info, err := os.Stat(modDir); err == nil {
			return info.IsDir()
		}
	case model.DepTypePython:
		for _, pipBin := range []string{
			filepath.Join(depsDir, "python", "venv", "bin", "pip"),
			filepath.Join(depsDir, "python", "venv", "Scripts", "pip.exe"),
		} {
			if _, err := os.Stat(pipBin); err == nil {
				if out, err := exec.Command(pipBin, "show", name).CombinedOutput(); err == nil && strings.Contains(string(out), "Name:") {
					return true
				}
			}
		}
	case model.DepTypeLinux:
		if _, err := exec.LookPath(name); err == nil {
			return true
		}
		for _, probe := range []struct {
			binary string
			args   []string
		}{
			{binary: "apk", args: []string{"info", "-e", name}},
			{binary: "dpkg-query", args: []string{"-W", "-f=${Status}", name}},
			{binary: "rpm", args: []string{"-q", name}},
		} {
			if _, err := exec.LookPath(probe.binary); err != nil {
				continue
			}
			if out, err := exec.Command(probe.binary, probe.args...).CombinedOutput(); err == nil {
				if probe.binary != "dpkg-query" || strings.Contains(string(out), "install ok installed") {
					return true
				}
			}
		}
	}

	return false
}
