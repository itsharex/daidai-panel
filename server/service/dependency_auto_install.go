package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"daidai-panel/config"
	"daidai-panel/model"
)

var (
	autoInstallNodeModuleRe = regexp.MustCompile(`(?:Cannot find module|Error \[ERR_MODULE_NOT_FOUND\].*)\s*'([^']+)'`)
	autoInstallPyModuleRe   = regexp.MustCompile(`(?:ModuleNotFoundError|ImportError):\s*No module named\s+'([^']+)'`)
	autoInstallGoModuleRe   = regexp.MustCompile(`(?:no required module provides package|missing go\.sum entry for module providing package)\s+([^\s:;]+)`)
)

type AutoInstallCandidate struct {
	Manager       string
	RequestedName string
	PackageName   string
	DisplayName   string
	WorkDir       string
	RecordType    string
	RecordName    string
}

type AutoInstallResult struct {
	Success bool
	Log     string
	Error   string
}

func DetectAutoInstallCandidate(ext, output, workDir string) *AutoInstallCandidate {
	ext = strings.ToLower(strings.TrimSpace(ext))

	switch ext {
	case ".py":
		if matches := autoInstallPyModuleRe.FindStringSubmatch(output); len(matches) > 1 {
			requested := strings.Split(matches[1], ".")[0]
			packageName := ResolvePythonAutoInstallPackage(requested)
			return &AutoInstallCandidate{
				Manager:       "python",
				RequestedName: requested,
				PackageName:   packageName,
				DisplayName:   formatAutoInstallDisplayName(requested, packageName),
				WorkDir:       workDir,
				RecordType:    model.DepTypePython,
				RecordName:    packageName,
			}
		}
	case ".js", ".ts":
		if matches := autoInstallNodeModuleRe.FindStringSubmatch(output); len(matches) > 1 {
			requested := strings.TrimSpace(matches[1])
			if requested == "" || strings.HasPrefix(requested, ".") || strings.HasPrefix(requested, "/") {
				return nil
			}
			return &AutoInstallCandidate{
				Manager:       "nodejs",
				RequestedName: requested,
				PackageName:   requested,
				DisplayName:   requested,
				WorkDir:       workDir,
				RecordType:    model.DepTypeNodeJS,
				RecordName:    requested,
			}
		}
	case ".go":
		moduleRoot := findNearestAncestorWithFile(workDir, "go.mod")
		if moduleRoot == "" {
			return nil
		}
		if matches := autoInstallGoModuleRe.FindStringSubmatch(output); len(matches) > 1 {
			moduleName := strings.TrimSpace(matches[1])
			if moduleName == "" {
				return nil
			}
			return &AutoInstallCandidate{
				Manager:       "go",
				RequestedName: moduleName,
				PackageName:   moduleName,
				DisplayName:   moduleName,
				WorkDir:       moduleRoot,
			}
		}
	}

	return nil
}

func InstallAutoDependency(candidate *AutoInstallCandidate, envVars map[string]string) AutoInstallResult {
	if candidate == nil {
		return AutoInstallResult{Error: "未找到可自动安装的依赖"}
	}

	baseEnv := buildEnvSlice(envVars)
	depsDir := filepath.Join(config.C.Data.Dir, "deps")

	switch candidate.Manager {
	case "python":
		venvPip := filepath.Join(depsDir, "python", "venv", "bin", "pip3")
		if _, err := os.Stat(venvPip); err != nil {
			venvPip = "pip3"
		}
		cmd := exec.Command(venvPip, "install", candidate.PackageName)
		cmd.Env = PipInstallEnv(baseEnv, CurrentPipMirror())
		out, err := cmd.CombinedOutput()
		return completeAutoInstall(candidate, out, err)
	case "nodejs":
		nodeDir := filepath.Join(depsDir, "nodejs")
		_ = os.MkdirAll(nodeDir, 0755)
		cmd := exec.Command("npm", "install", candidate.PackageName, "--prefix", nodeDir)
		cmd.Env = NpmInstallEnv(baseEnv, CurrentNpmMirror())
		out, err := cmd.CombinedOutput()
		return completeAutoInstall(candidate, out, err)
	case "go":
		cmd := exec.Command("go", "get", candidate.PackageName)
		cmd.Dir = candidate.WorkDir
		cmd.Env = baseEnv
		out, err := cmd.CombinedOutput()
		return completeAutoInstall(candidate, out, err)
	default:
		return AutoInstallResult{Error: fmt.Sprintf("不支持的自动安装类型: %s", candidate.Manager)}
	}
}

func completeAutoInstall(candidate *AutoInstallCandidate, out []byte, err error) AutoInstallResult {
	logText := string(out)
	if err != nil {
		return AutoInstallResult{
			Success: false,
			Log:     logText,
			Error:   strings.TrimSpace(logText),
		}
	}

	if candidate.RecordType != "" && candidate.RecordName != "" {
		RecordAutoInstalledDep(candidate.RecordType, candidate.RecordName, logText)
	}

	return AutoInstallResult{
		Success: true,
		Log:     logText,
	}
}

func formatAutoInstallDisplayName(requested, packageName string) string {
	requested = strings.TrimSpace(requested)
	packageName = strings.TrimSpace(packageName)
	if requested == "" {
		return packageName
	}
	if packageName == "" || strings.EqualFold(requested, packageName) {
		return requested
	}
	return requested + " -> " + packageName
}

func findNearestAncestorWithFile(startDir, targetFile string) string {
	current := strings.TrimSpace(startDir)
	if current == "" {
		return ""
	}

	for {
		candidate := filepath.Join(current, targetFile)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return current
		}

		parent := filepath.Dir(current)
		if parent == current {
			return ""
		}
		current = parent
	}
}
