package handler

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"daidai-panel/config"
	"daidai-panel/database"
	"daidai-panel/model"
	"daidai-panel/service"
)

var scriptInterpreterMap = map[string][]string{
	".py": {"python", "-u"},
	".js": {"node"},
	".ts": {"npx", "ts-node"},
	".sh": {"bash"},
}

var scriptLanguageExtMap = map[string]string{
	"python":     ".py",
	"javascript": ".js",
	"typescript": ".ts",
	"shell":      ".sh",
}

var scriptEnvPassthroughKeys = []string{
	"PATH", "HOME", "USER", "LANG", "SYSTEMROOT", "PATHEXT", "TEMP", "TMP",
}

func newDebugRun() *debugRun {
	return &debugRun{
		Logs:   []string{},
		Status: "running",
	}
}

func (h *ScriptHandler) storeRun(runID string, run *debugRun) {
	h.mu.Lock()
	h.debugRuns[runID] = run
	h.mu.Unlock()
}

func (h *ScriptHandler) loadRun(runID string) (*debugRun, bool) {
	h.mu.Lock()
	run, exists := h.debugRuns[runID]
	h.mu.Unlock()
	return run, exists
}

func (h *ScriptHandler) deleteRun(runID string) (*debugRun, bool) {
	h.mu.Lock()
	run, exists := h.debugRuns[runID]
	if exists {
		delete(h.debugRuns, runID)
	}
	h.mu.Unlock()
	return run, exists
}

func (run *debugRun) setProcess(process *os.Process) {
	run.mu.Lock()
	run.Process = process
	run.mu.Unlock()
}

func (run *debugRun) appendLog(line string) {
	run.mu.Lock()
	run.Logs = append(run.Logs, line)
	run.mu.Unlock()
}

func (run *debugRun) logOutput() string {
	run.mu.Lock()
	defer run.mu.Unlock()
	return strings.Join(run.Logs, "\n")
}

func (run *debugRun) snapshot() ([]string, bool, *int, string) {
	run.mu.Lock()
	defer run.mu.Unlock()

	logs := make([]string, len(run.Logs))
	copy(logs, run.Logs)

	var exitCode *int
	if run.ExitCode != nil {
		value := *run.ExitCode
		exitCode = &value
	}

	return logs, run.Done, exitCode, run.Status
}

func (run *debugRun) stop() {
	run.mu.Lock()
	defer run.mu.Unlock()

	if run.Process == nil || run.Done {
		return
	}

	run.Process.Kill()
	run.Status = "stopped"
	exitCode := -1
	run.ExitCode = &exitCode
	run.Done = true
}

func (run *debugRun) killIfRunning() {
	run.mu.Lock()
	defer run.mu.Unlock()

	if run.Process != nil && !run.Done {
		run.Process.Kill()
	}
}

func (run *debugRun) finish(exitCode int, waitErr error, elapsed float64) {
	run.mu.Lock()
	defer run.mu.Unlock()

	if run.Status == "stopped" {
		return
	}

	run.ExitCode = &exitCode
	run.Done = true
	if exitCode == 0 {
		run.Status = "success"
		run.Logs = append(run.Logs, fmt.Sprintf("[进程结束, 退出码: %d, 耗时: %.2f秒]", exitCode, elapsed))
		return
	}

	run.Status = "failed"
	errMsg := ""
	if waitErr != nil {
		errMsg = waitErr.Error()
	}
	if errMsg != "" {
		run.Logs = append(run.Logs, fmt.Sprintf("[进程异常退出, 退出码: %d, 错误: %s, 耗时: %.2f秒]", exitCode, errMsg, elapsed))
		return
	}
	run.Logs = append(run.Logs, fmt.Sprintf("[进程异常退出, 退出码: %d, 耗时: %.2f秒]", exitCode, elapsed))
}

func scriptCommandParts(ext, target string) ([]string, error) {
	baseCmd, ok := scriptInterpreterMap[ext]
	if !ok {
		return nil, fmt.Errorf("不支持执行此文件类型")
	}

	cmdParts := append([]string{}, baseCmd...)
	cmdParts = append(cmdParts, target)
	return cmdParts, nil
}

func buildScriptExecEnv(workDir string) ([]string, map[string]string) {
	envMap := buildManagedScriptEnvMap(workDir)
	return buildProcessEnv(envMap), envMap
}

func buildManagedScriptEnvMap(workDir string) map[string]string {
	var envVars []model.EnvVar
	database.DB.Where("enabled = ?", true).Find(&envVars)

	envMap := make(map[string]string)
	for _, e := range envVars {
		if existing, ok := envMap[e.Name]; ok {
			envMap[e.Name] = existing + "&" + e.Value
		} else {
			envMap[e.Name] = e.Value
		}
	}

	depsDir := filepath.Join(config.C.Data.Dir, "deps")
	nodeBin := filepath.Join(depsDir, "nodejs", "node_modules", ".bin")
	nodeModules := filepath.Join(depsDir, "nodejs", "node_modules")
	venvBin := filepath.Join(depsDir, "python", "venv", "bin")

	envMap["NODE_PATH"] = nodeModules
	if currentPath := os.Getenv("PATH"); currentPath != "" {
		envMap["PATH"] = strings.Join([]string{nodeBin, venvBin, currentPath}, string(os.PathListSeparator))
	}

	venvLib := filepath.Join(depsDir, "python", "venv", "lib")
	if entries, dirErr := os.ReadDir(venvLib); dirErr == nil {
		for _, entry := range entries {
			if entry.IsDir() && strings.HasPrefix(entry.Name(), "python") {
				envMap["PYTHONPATH"] = filepath.Join(venvLib, entry.Name(), "site-packages")
				break
			}
		}
	}
	service.AppendScriptHelperPaths(envMap, config.C.Data.ScriptsDir)
	if helperEnv, err := service.BuildNotifyHelperEnv(config.C.Data.ScriptsDir, workDir, config.C.Server.Port, nil, 2*time.Hour); err == nil {
		for key, value := range helperEnv {
			envMap[key] = value
		}
	}

	return envMap
}

func buildProcessEnv(envMap map[string]string) []string {
	env := []string{}
	for _, key := range scriptEnvPassthroughKeys {
		if value := os.Getenv(key); value != "" {
			env = append(env, key+"="+value)
		}
	}

	for key, value := range envMap {
		env = append(env, key+"="+value)
	}

	return env
}

func newScriptCommand(cmdParts []string, workDir string, env []string) *exec.Cmd {
	cmd := exec.Command(cmdParts[0], cmdParts[1:]...)
	cmd.Dir = workDir
	cmd.Env = env
	return cmd
}

func startTrackedCommand(cmd *exec.Cmd, run *debugRun) (*io.PipeWriter, chan struct{}, error) {
	pipeReader, pipeWriter := io.Pipe()
	cmd.Stdout = pipeWriter
	cmd.Stderr = pipeWriter

	if err := cmd.Start(); err != nil {
		pipeWriter.Close()
		return nil, nil, err
	}

	run.setProcess(cmd.Process)
	scanDone := collectRunLogs(pipeReader, run)
	return pipeWriter, scanDone, nil
}

func collectRunLogs(reader io.Reader, run *debugRun) chan struct{} {
	done := make(chan struct{})

	go func() {
		scanner := bufio.NewScanner(reader)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		for scanner.Scan() {
			run.appendLog(scanner.Text())
		}
		close(done)
	}()

	return done
}

func waitTrackedCommand(cmd *exec.Cmd, pipeWriter *io.PipeWriter, scanDone chan struct{}) error {
	err := cmd.Wait()
	pipeWriter.Close()
	<-scanDone
	return err
}

func resolveExitCode(err error) int {
	if err == nil {
		return 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}
	return 1
}
